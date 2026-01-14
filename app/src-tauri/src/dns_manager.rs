// DNS Manager Module
// Trait-based abstraction supporting automatic DNS configuration on macOS and Linux.
//
// ARCHITECTURE:
// - DnsProvider trait defines common interface for all platforms
// - MacOsDnsProvider implements /etc/resolver-based DNS (port 53535)
// - LinuxDnsProvider implements systemd-resolved configuration (port 53535)
//
// DNS STRATEGY:
// This module configures the OS to resolve *.vibe.space domains to 127.0.0.1
// by directing queries to our custom DNS server running on localhost:53535.
//
// PLATFORM IMPLEMENTATIONS:
//
// 1. macOS (using /etc/resolver/):
//    - Creates /etc/resolver/vibe.space with nameserver 127.0.0.1:53535
//    - Uses osascript for graphical sudo prompt
//    - Requires no system-wide DNS changes
//    - Scoped to vibe.space domain only
//
// 2. Linux (using systemd-resolved):
//    - Configures resolved to forward *.vibe.space to 127.0.0.1:53535
//    - Uses pkexec for PolicyKit graphical prompt
//    - Creates /etc/systemd/resolved.conf.d/vibespace.conf
//    - Restarts systemd-resolved service
//
// SECURITY:
// - Unprivileged DNS server (port 53535, not 53 - avoids mDNS conflict)
// - OS-level configuration requires sudo (user consent via GUI)
// - Only affects vibe.space domain (no system-wide DNS hijacking)
// - Automatic cleanup on uninstall
//
// See: ADR 0007 - DNS Resolution Strategy for Local Vibespaces

use serde::{Deserialize, Serialize};
use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::thread;
use std::time::Duration;

// ============================================================================
// Common Types
// ============================================================================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DnsStatus {
    /// DNS server binary is installed in ~/.vibespace/bin/
    pub installed: bool,
    /// DNS server process is running
    pub running: bool,
    /// OS-level DNS configuration is active (resolver/systemd-resolved)
    pub configured: bool,
    /// DNS server process ID (if running)
    pub pid: Option<u32>,
    /// Error message (if any)
    pub error: Option<String>,
    /// Suggested action for the user
    pub suggested_action: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DnsSetupProgress {
    pub stage: String,
    pub progress: u8,
    pub message: String,
}

#[derive(Debug, Clone, PartialEq)]
pub enum Platform {
    MacOS,
    Linux,
    Unsupported,
}

// ============================================================================
// DnsProvider Trait - Common interface for all platforms
// ============================================================================

/// DnsProvider defines the interface that all platforms must implement.
/// This abstraction allows the same Tauri commands to work across macOS and Linux
/// with platform-specific DNS configuration strategies.
pub trait DnsProvider: Send {
    /// Check if DNS server binary is installed
    fn is_installed(&self) -> bool;

    /// Check if DNS server process is running
    fn is_running(&self) -> bool;

    /// Check if OS-level DNS configuration is active
    fn is_configured(&self) -> bool;

    /// Get comprehensive DNS status
    fn get_status(&self) -> DnsStatus;

    /// Setup DNS (install binary + configure OS + start server)
    /// This is the main entry point for automatic DNS setup
    fn setup(&self, progress_callback: Box<dyn Fn(DnsSetupProgress)>) -> Result<(), String>;

    /// Start DNS server process
    fn start(&self) -> Result<(), String>;

    /// Stop DNS server process
    fn stop(&self) -> Result<(), String>;

    /// Cleanup DNS configuration (stop server + remove OS config + remove binary)
    fn cleanup(&self) -> Result<(), String>;

    /// Get DNS server process ID
    #[allow(dead_code)]
    fn get_pid(&self) -> Option<u32>;
}

// ============================================================================
// DnsManager - Facade that delegates to active provider
// ============================================================================

pub struct DnsManager {
    provider: Box<dyn DnsProvider>,
    #[allow(dead_code)]
    platform: Platform,
}

impl DnsManager {
    /// Create a new DnsManager with platform auto-detection
    pub fn new() -> Result<Self, String> {
        let platform = detect_platform();

        if platform == Platform::Unsupported {
            return Err("Unsupported platform for DNS management. vibespace requires macOS or Linux.".to_string());
        }

        let provider: Box<dyn DnsProvider> = match platform {
            Platform::MacOS => Box::new(MacOsDnsProvider::new()?),
            Platform::Linux => Box::new(LinuxDnsProvider::new()?),
            Platform::Unsupported => unreachable!(),
        };

        Ok(DnsManager { provider, platform })
    }

    #[allow(dead_code)]
    pub fn is_installed(&self) -> bool {
        self.provider.is_installed()
    }

    #[allow(dead_code)]
    pub fn is_running(&self) -> bool {
        self.provider.is_running()
    }

    #[allow(dead_code)]
    pub fn is_configured(&self) -> bool {
        self.provider.is_configured()
    }

    pub fn get_status(&self) -> DnsStatus {
        self.provider.get_status()
    }

    pub fn setup<F>(&self, progress_callback: F) -> Result<(), String>
    where
        F: Fn(DnsSetupProgress) + 'static,
    {
        self.provider.setup(Box::new(progress_callback))
    }

    pub fn start(&self) -> Result<(), String> {
        self.provider.start()
    }

    pub fn stop(&self) -> Result<(), String> {
        self.provider.stop()
    }

    pub fn cleanup(&self) -> Result<(), String> {
        self.provider.cleanup()
    }

    #[allow(dead_code)]
    pub fn get_pid(&self) -> Option<u32> {
        self.provider.get_pid()
    }
}

// ============================================================================
// MacOsDnsProvider - /etc/resolver implementation
// ============================================================================

struct MacOsDnsProvider {
    install_dir: PathBuf,
    dns_binary_path: PathBuf,
    resolver_file: PathBuf,
    pid_file: PathBuf,
}

impl MacOsDnsProvider {
    fn new() -> Result<Self, String> {
        let home_dir = dirs::home_dir()
            .ok_or_else(|| "Failed to determine home directory".to_string())?;

        let install_dir = home_dir.join(".vibespace");
        let dns_binary_path = install_dir.join("bin").join("dnsd");
        let resolver_file = PathBuf::from("/etc/resolver/vibe.space");
        let pid_file = install_dir.join("dnsd.pid");

        Ok(MacOsDnsProvider {
            install_dir,
            dns_binary_path,
            resolver_file,
            pid_file,
        })
    }

    /// Extract DNS binary from Tauri resources to ~/.vibespace/bin/
    fn install_binary(&self) -> Result<(), String> {
        // Get resource directory
        let resource_dir = get_resource_dir()
            .ok_or_else(|| "Failed to get resource directory".to_string())?;

        // Detect architecture
        let arch = if cfg!(target_arch = "aarch64") {
            "arm64"
        } else {
            "amd64"
        };

        let dns_src = resource_dir.join(format!("binaries/dnsd-darwin-{}", arch));
        let dns_dest = &self.dns_binary_path;

        // Create bin directory if needed
        if let Some(parent) = dns_dest.parent() {
            fs::create_dir_all(parent)
                .map_err(|e| format!("Failed to create bin directory: {}", e))?;
        }

        // Copy binary
        fs::copy(&dns_src, dns_dest)
            .map_err(|e| format!("Failed to copy DNS binary from {:?}: {}", dns_src, e))?;

        // Make executable
        set_executable(dns_dest)?;

        Ok(())
    }

    /// Configure /etc/resolver/vibe.space to point to our DNS server
    /// Uses osascript for graphical sudo prompt
    fn configure_resolver(&self) -> Result<(), String> {
        let resolver_content = "nameserver 127.0.0.1\nport 53535\n";

        // Create temp file with resolver configuration
        let temp_file = format!("/tmp/vibespace_resolver_{}", uuid::Uuid::new_v4());
        fs::write(&temp_file, resolver_content)
            .map_err(|e| format!("Failed to write temp resolver file: {}", e))?;

        // Use osascript to get graphical sudo prompt
        // This is more user-friendly than terminal sudo
        let script = format!(
            r#"do shell script "mkdir -p /etc/resolver && cp {} /etc/resolver/vibe.space" with administrator privileges"#,
            temp_file
        );

        let output = Command::new("osascript")
            .arg("-e")
            .arg(&script)
            .output()
            .map_err(|e| format!("Failed to configure DNS resolver: {}", e))?;

        // Clean up temp file
        let _ = fs::remove_file(&temp_file);

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("Failed to configure DNS resolver: {}", stderr));
        }

        Ok(())
    }

    /// Remove /etc/resolver/vibe.space configuration
    fn unconfigure_resolver(&self) -> Result<(), String> {
        if !self.resolver_file.exists() {
            return Ok(());
        }

        let script = "do shell script \"rm -f /etc/resolver/vibe.space\" with administrator privileges";

        let output = Command::new("osascript")
            .arg("-e")
            .arg(script)
            .output()
            .map_err(|e| format!("Failed to remove DNS resolver: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("Failed to remove DNS resolver: {}", stderr));
        }

        Ok(())
    }

    /// Start DNS server process in background
    fn start_dns_server(&self) -> Result<(), String> {
        if !self.dns_binary_path.exists() {
            return Err("DNS binary not installed".to_string());
        }

        // Check if already running
        if let Some(pid) = self.read_pid_file() {
            if is_process_running(pid) {
                return Ok(()); // Already running
            }
        }

        // Start DNS server in background
        let log_file = fs::File::create(self.install_dir.join("dnsd.log"))
            .map_err(|e| format!("Failed to create log file: {}", e))?;

        let child = Command::new(&self.dns_binary_path)
            .stdout(Stdio::from(log_file.try_clone().unwrap()))
            .stderr(Stdio::from(log_file))
            .spawn()
            .map_err(|e| format!("Failed to start DNS server: {}", e))?;

        let pid = child.id();

        // Save PID to file
        fs::write(&self.pid_file, pid.to_string())
            .map_err(|e| format!("Failed to write PID file: {}", e))?;

        // Wait a moment for server to start
        thread::sleep(Duration::from_millis(500));

        // Verify it's running
        if !is_process_running(pid) {
            return Err("DNS server failed to start".to_string());
        }

        Ok(())
    }

    /// Stop DNS server process
    fn stop_dns_server(&self) -> Result<(), String> {
        if let Some(pid) = self.read_pid_file() {
            if is_process_running(pid) {
                // Kill process
                let _ = Command::new("kill")
                    .arg(pid.to_string())
                    .output();

                // Wait for process to exit
                for _ in 0..10 {
                    if !is_process_running(pid) {
                        break;
                    }
                    thread::sleep(Duration::from_millis(100));
                }

                // Force kill if still running
                if is_process_running(pid) {
                    let _ = Command::new("kill")
                        .arg("-9")
                        .arg(pid.to_string())
                        .output();
                }
            }

            // Remove PID file
            let _ = fs::remove_file(&self.pid_file);
        }

        Ok(())
    }

    fn read_pid_file(&self) -> Option<u32> {
        fs::read_to_string(&self.pid_file)
            .ok()?
            .trim()
            .parse::<u32>()
            .ok()
    }
}

impl DnsProvider for MacOsDnsProvider {
    fn is_installed(&self) -> bool {
        self.dns_binary_path.exists()
    }

    fn is_running(&self) -> bool {
        if let Some(pid) = self.read_pid_file() {
            is_process_running(pid)
        } else {
            false
        }
    }

    fn is_configured(&self) -> bool {
        self.resolver_file.exists()
    }

    fn get_status(&self) -> DnsStatus {
        let installed = self.is_installed();
        let running = self.is_running();
        let configured = self.is_configured();
        let pid = self.read_pid_file();

        let suggested_action = if !installed || !configured {
            Some("setup".to_string())
        } else if !running {
            Some("start".to_string())
        } else {
            None
        };

        DnsStatus {
            installed,
            running,
            configured,
            pid,
            error: None,
            suggested_action,
        }
    }

    fn setup(&self, progress_callback: Box<dyn Fn(DnsSetupProgress)>) -> Result<(), String> {
        progress_callback(DnsSetupProgress {
            stage: "installing".to_string(),
            progress: 10,
            message: "Installing DNS server binary...".to_string(),
        });

        self.install_binary()?;

        progress_callback(DnsSetupProgress {
            stage: "configuring".to_string(),
            progress: 40,
            message: "Configuring DNS resolver (requires sudo)...".to_string(),
        });

        self.configure_resolver()?;

        progress_callback(DnsSetupProgress {
            stage: "starting".to_string(),
            progress: 70,
            message: "Starting DNS server...".to_string(),
        });

        self.start_dns_server()?;

        progress_callback(DnsSetupProgress {
            stage: "verifying".to_string(),
            progress: 90,
            message: "Verifying DNS resolution...".to_string(),
        });

        // Verify DNS is working
        thread::sleep(Duration::from_secs(1));
        if !self.is_running() {
            return Err("DNS server failed to start".to_string());
        }

        progress_callback(DnsSetupProgress {
            stage: "complete".to_string(),
            progress: 100,
            message: "DNS setup complete".to_string(),
        });

        Ok(())
    }

    fn start(&self) -> Result<(), String> {
        self.start_dns_server()
    }

    fn stop(&self) -> Result<(), String> {
        self.stop_dns_server()
    }

    fn cleanup(&self) -> Result<(), String> {
        // Stop server
        self.stop_dns_server()?;

        // Remove OS configuration
        self.unconfigure_resolver()?;

        // Remove binary
        if self.dns_binary_path.exists() {
            fs::remove_file(&self.dns_binary_path)
                .map_err(|e| format!("Failed to remove DNS binary: {}", e))?;
        }

        // Remove PID file
        if self.pid_file.exists() {
            let _ = fs::remove_file(&self.pid_file);
        }

        Ok(())
    }

    fn get_pid(&self) -> Option<u32> {
        self.read_pid_file()
    }
}

// ============================================================================
// LinuxDnsProvider - systemd-resolved implementation
// ============================================================================

struct LinuxDnsProvider {
    install_dir: PathBuf,
    dns_binary_path: PathBuf,
    resolved_conf_dir: PathBuf,
    resolved_conf_file: PathBuf,
    pid_file: PathBuf,
}

impl LinuxDnsProvider {
    fn new() -> Result<Self, String> {
        let home_dir = dirs::home_dir()
            .ok_or_else(|| "Failed to determine home directory".to_string())?;

        let install_dir = home_dir.join(".vibespace");
        let dns_binary_path = install_dir.join("bin").join("dnsd");
        let resolved_conf_dir = PathBuf::from("/etc/systemd/resolved.conf.d");
        let resolved_conf_file = resolved_conf_dir.join("vibespace.conf");
        let pid_file = install_dir.join("dnsd.pid");

        Ok(LinuxDnsProvider {
            install_dir,
            dns_binary_path,
            resolved_conf_dir,
            resolved_conf_file,
            pid_file,
        })
    }

    /// Extract DNS binary from Tauri resources to ~/.vibespace/bin/
    fn install_binary(&self) -> Result<(), String> {
        let resource_dir = get_resource_dir()
            .ok_or_else(|| "Failed to get resource directory".to_string())?;

        // Detect architecture
        let arch = if cfg!(target_arch = "aarch64") {
            "arm64"
        } else {
            "amd64"
        };

        let dns_src = resource_dir.join(format!("binaries/dnsd-linux-{}", arch));
        let dns_dest = &self.dns_binary_path;

        // Create bin directory if needed
        if let Some(parent) = dns_dest.parent() {
            fs::create_dir_all(parent)
                .map_err(|e| format!("Failed to create bin directory: {}", e))?;
        }

        // Copy binary
        fs::copy(&dns_src, dns_dest)
            .map_err(|e| format!("Failed to copy DNS binary from {:?}: {}", dns_src, e))?;

        // Make executable
        set_executable(dns_dest)?;

        Ok(())
    }

    /// Configure systemd-resolved to forward *.vibe.space to our DNS server
    /// Uses pkexec for PolicyKit graphical prompt
    fn configure_resolved(&self) -> Result<(), String> {
        let resolved_content = "[Resolve]\nDNS=127.0.0.1:53535\nDomains=~vibe.space\n";

        // Create temp file
        let temp_file = format!("/tmp/vibespace_resolved_{}", uuid::Uuid::new_v4());
        fs::write(&temp_file, resolved_content)
            .map_err(|e| format!("Failed to write temp resolved config: {}", e))?;

        // Use pkexec for graphical sudo prompt
        let output = Command::new("pkexec")
            .arg("sh")
            .arg("-c")
            .arg(format!(
                "mkdir -p {} && cp {} {}",
                self.resolved_conf_dir.display(),
                temp_file,
                self.resolved_conf_file.display()
            ))
            .output()
            .map_err(|e| format!("Failed to configure systemd-resolved: {}", e))?;

        // Clean up temp file
        let _ = fs::remove_file(&temp_file);

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("Failed to configure systemd-resolved: {}", stderr));
        }

        // Restart systemd-resolved
        let restart_output = Command::new("pkexec")
            .arg("systemctl")
            .arg("restart")
            .arg("systemd-resolved")
            .output()
            .map_err(|e| format!("Failed to restart systemd-resolved: {}", e))?;

        if !restart_output.status.success() {
            let stderr = String::from_utf8_lossy(&restart_output.stderr);
            return Err(format!("Failed to restart systemd-resolved: {}", stderr));
        }

        Ok(())
    }

    /// Remove systemd-resolved configuration
    fn unconfigure_resolved(&self) -> Result<(), String> {
        if !self.resolved_conf_file.exists() {
            return Ok(());
        }

        let output = Command::new("pkexec")
            .arg("rm")
            .arg("-f")
            .arg(&self.resolved_conf_file)
            .output()
            .map_err(|e| format!("Failed to remove resolved config: {}", e))?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(format!("Failed to remove resolved config: {}", stderr));
        }

        // Restart systemd-resolved
        let restart_output = Command::new("pkexec")
            .arg("systemctl")
            .arg("restart")
            .arg("systemd-resolved")
            .output()
            .map_err(|e| format!("Failed to restart systemd-resolved: {}", e))?;

        if !restart_output.status.success() {
            let stderr = String::from_utf8_lossy(&restart_output.stderr);
            return Err(format!("Failed to restart systemd-resolved: {}", stderr));
        }

        Ok(())
    }

    /// Start DNS server process in background
    fn start_dns_server(&self) -> Result<(), String> {
        if !self.dns_binary_path.exists() {
            return Err("DNS binary not installed".to_string());
        }

        // Check if already running
        if let Some(pid) = self.read_pid_file() {
            if is_process_running(pid) {
                return Ok(()); // Already running
            }
        }

        // Start DNS server in background
        let log_file = fs::File::create(self.install_dir.join("dnsd.log"))
            .map_err(|e| format!("Failed to create log file: {}", e))?;

        let child = Command::new(&self.dns_binary_path)
            .stdout(Stdio::from(log_file.try_clone().unwrap()))
            .stderr(Stdio::from(log_file))
            .spawn()
            .map_err(|e| format!("Failed to start DNS server: {}", e))?;

        let pid = child.id();

        // Save PID to file
        fs::write(&self.pid_file, pid.to_string())
            .map_err(|e| format!("Failed to write PID file: {}", e))?;

        // Wait a moment for server to start
        thread::sleep(Duration::from_millis(500));

        // Verify it's running
        if !is_process_running(pid) {
            return Err("DNS server failed to start".to_string());
        }

        Ok(())
    }

    /// Stop DNS server process
    fn stop_dns_server(&self) -> Result<(), String> {
        if let Some(pid) = self.read_pid_file() {
            if is_process_running(pid) {
                // Kill process
                let _ = Command::new("kill")
                    .arg(pid.to_string())
                    .output();

                // Wait for process to exit
                for _ in 0..10 {
                    if !is_process_running(pid) {
                        break;
                    }
                    thread::sleep(Duration::from_millis(100));
                }

                // Force kill if still running
                if is_process_running(pid) {
                    let _ = Command::new("kill")
                        .arg("-9")
                        .arg(pid.to_string())
                        .output();
                }
            }

            // Remove PID file
            let _ = fs::remove_file(&self.pid_file);
        }

        Ok(())
    }

    fn read_pid_file(&self) -> Option<u32> {
        fs::read_to_string(&self.pid_file)
            .ok()?
            .trim()
            .parse::<u32>()
            .ok()
    }
}

impl DnsProvider for LinuxDnsProvider {
    fn is_installed(&self) -> bool {
        self.dns_binary_path.exists()
    }

    fn is_running(&self) -> bool {
        if let Some(pid) = self.read_pid_file() {
            is_process_running(pid)
        } else {
            false
        }
    }

    fn is_configured(&self) -> bool {
        self.resolved_conf_file.exists()
    }

    fn get_status(&self) -> DnsStatus {
        let installed = self.is_installed();
        let running = self.is_running();
        let configured = self.is_configured();
        let pid = self.read_pid_file();

        let suggested_action = if !installed || !configured {
            Some("setup".to_string())
        } else if !running {
            Some("start".to_string())
        } else {
            None
        };

        DnsStatus {
            installed,
            running,
            configured,
            pid,
            error: None,
            suggested_action,
        }
    }

    fn setup(&self, progress_callback: Box<dyn Fn(DnsSetupProgress)>) -> Result<(), String> {
        progress_callback(DnsSetupProgress {
            stage: "installing".to_string(),
            progress: 10,
            message: "Installing DNS server binary...".to_string(),
        });

        self.install_binary()?;

        progress_callback(DnsSetupProgress {
            stage: "configuring".to_string(),
            progress: 40,
            message: "Configuring systemd-resolved (requires sudo)...".to_string(),
        });

        self.configure_resolved()?;

        progress_callback(DnsSetupProgress {
            stage: "starting".to_string(),
            progress: 70,
            message: "Starting DNS server...".to_string(),
        });

        self.start_dns_server()?;

        progress_callback(DnsSetupProgress {
            stage: "verifying".to_string(),
            progress: 90,
            message: "Verifying DNS resolution...".to_string(),
        });

        // Verify DNS is working
        thread::sleep(Duration::from_secs(1));
        if !self.is_running() {
            return Err("DNS server failed to start".to_string());
        }

        progress_callback(DnsSetupProgress {
            stage: "complete".to_string(),
            progress: 100,
            message: "DNS setup complete".to_string(),
        });

        Ok(())
    }

    fn start(&self) -> Result<(), String> {
        self.start_dns_server()
    }

    fn stop(&self) -> Result<(), String> {
        self.stop_dns_server()
    }

    fn cleanup(&self) -> Result<(), String> {
        // Stop server
        self.stop_dns_server()?;

        // Remove OS configuration
        self.unconfigure_resolved()?;

        // Remove binary
        if self.dns_binary_path.exists() {
            fs::remove_file(&self.dns_binary_path)
                .map_err(|e| format!("Failed to remove DNS binary: {}", e))?;
        }

        // Remove PID file
        if self.pid_file.exists() {
            let _ = fs::remove_file(&self.pid_file);
        }

        Ok(())
    }

    fn get_pid(&self) -> Option<u32> {
        self.read_pid_file()
    }
}

// ============================================================================
// Helper Functions
// ============================================================================

fn detect_platform() -> Platform {
    match env::consts::OS {
        "macos" => Platform::MacOS,
        "linux" => Platform::Linux,
        _ => Platform::Unsupported,
    }
}

fn get_resource_dir() -> Option<PathBuf> {
    // Get resource directory from Tauri app
    // In production: /path/to/app.app/Contents/Resources/
    // In dev mode: /path/to/vibespace/app/src-tauri/

    let exe_path = env::current_exe().ok()?;

    // Try production path first (macOS app bundle)
    let resources_dir = exe_path.parent()?.parent()?.join("Resources");
    if resources_dir.exists() {
        return Some(resources_dir);
    }

    // Fallback to dev mode path
    let dev_dir = exe_path.parent()?.parent()?.parent()?;
    if dev_dir.join("binaries").exists() {
        return Some(dev_dir.to_path_buf());
    }

    None
}

fn set_executable(path: &Path) -> Result<(), String> {
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = fs::metadata(path)
            .map_err(|e| format!("Failed to get file permissions: {}", e))?
            .permissions();
        perms.set_mode(0o755);
        fs::set_permissions(path, perms)
            .map_err(|e| format!("Failed to set executable permissions: {}", e))?;
    }
    Ok(())
}

fn is_process_running(pid: u32) -> bool {
    Command::new("kill")
        .arg("-0")
        .arg(pid.to_string())
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

// ============================================================================
// Port Forwarding - Forward 443 -> 30443 for HTTPS without port number
// ============================================================================

/// launchd label for the port forwarding daemon (macOS)
#[cfg(target_os = "macos")]
const LAUNCHD_LABEL: &str = "space.vibe.portfwd";

/// Path to the launchd plist file (macOS)
#[cfg(target_os = "macos")]
const LAUNCHD_PLIST_PATH: &str = "/Library/LaunchDaemons/space.vibe.portfwd.plist";

/// Setup port forwarding from privileged ports to NodePorts
/// This allows users to access https://project.vibe.space without specifying a port
///
/// On macOS: Uses launchd for process supervision (auto-restart on crash)
/// On Linux: Uses systemd or direct process management
/// Runs as a background process with admin privileges
pub fn setup_port_forwarding() -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        setup_macos_port_forwarding()
    }

    #[cfg(target_os = "linux")]
    {
        setup_linux_port_forwarding()
    }

    #[cfg(not(any(target_os = "macos", target_os = "linux")))]
    {
        Err("Port forwarding not supported on this platform".to_string())
    }
}

/// Remove port forwarding configuration
pub fn cleanup_port_forwarding() -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        cleanup_macos_port_forwarding()
    }

    #[cfg(target_os = "linux")]
    {
        cleanup_linux_port_forwarding()
    }

    #[cfg(not(any(target_os = "macos", target_os = "linux")))]
    {
        Ok(())
    }
}

/// Check if port forwarding is running
pub fn is_port_forwarding_configured() -> bool {
    #[cfg(target_os = "macos")]
    {
        is_launchd_service_running()
    }

    #[cfg(target_os = "linux")]
    {
        // Linux still uses PID file approach
        let home_dir = match dirs::home_dir() {
            Some(d) => d,
            None => return false,
        };

        let pid_file = home_dir.join(".vibespace").join("port-forward.pid");
        if !pid_file.exists() {
            return false;
        }

        if let Ok(pid_str) = fs::read_to_string(&pid_file) {
            if let Ok(pid) = pid_str.trim().parse::<u32>() {
                return is_process_running(pid);
            }
        }

        false
    }

    #[cfg(not(any(target_os = "macos", target_os = "linux")))]
    {
        false
    }
}

/// Check if the launchd service is loaded and running (macOS)
#[cfg(target_os = "macos")]
fn is_launchd_service_running() -> bool {
    // Check if the plist exists first
    if !std::path::Path::new(LAUNCHD_PLIST_PATH).exists() {
        return false;
    }

    // Check if the service is loaded using launchctl list
    let output = Command::new("sudo")
        .arg("launchctl")
        .arg("list")
        .arg(LAUNCHD_LABEL)
        .output();

    match output {
        Ok(out) => out.status.success(),
        Err(_) => false,
    }
}

#[cfg(target_os = "macos")]
fn setup_macos_port_forwarding() -> Result<(), String> {
    let home_dir = dirs::home_dir()
        .ok_or_else(|| "Failed to determine home directory".to_string())?;

    let vibespace_dir = home_dir.join(".vibespace");
    fs::create_dir_all(&vibespace_dir)
        .map_err(|e| format!("Failed to create vibespace dir: {}", e))?;

    // Check if already running
    if is_port_forwarding_configured() {
        println!("Port forwarding already running via launchd");
        return Ok(());
    }

    // Get bundled portfwd binary path
    let portfwd_path = get_portfwd_binary_path()
        .ok_or_else(|| "Failed to locate bundled portfwd binary".to_string())?;

    if !portfwd_path.exists() {
        return Err(format!("portfwd binary not found at {:?}", portfwd_path));
    }

    // Copy portfwd to ~/.vibespace/bin/ for launchd to use
    let local_portfwd = vibespace_dir.join("bin").join("portfwd");
    fs::create_dir_all(local_portfwd.parent().unwrap())
        .map_err(|e| format!("Failed to create bin dir: {}", e))?;
    fs::copy(&portfwd_path, &local_portfwd)
        .map_err(|e| format!("Failed to copy portfwd: {}", e))?;

    // Make executable
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = fs::metadata(&local_portfwd)
            .map_err(|e| format!("Failed to get permissions: {}", e))?
            .permissions();
        perms.set_mode(0o755);
        fs::set_permissions(&local_portfwd, perms)
            .map_err(|e| format!("Failed to set permissions: {}", e))?;
    }

    // Generate launchd plist
    let plist_content = generate_launchd_plist(&local_portfwd)?;

    // Write plist to temp location
    let temp_plist = vibespace_dir.join("space.vibe.portfwd.plist");
    fs::write(&temp_plist, &plist_content)
        .map_err(|e| format!("Failed to write plist: {}", e))?;

    // Install plist to /Library/LaunchDaemons/ and load it (requires admin)
    // This single osascript call: copies plist, loads it, and starts the daemon
    let script = format!(
        r#"do shell script "cp '{}' '{}' && launchctl load '{}'" with administrator privileges"#,
        temp_plist.display(),
        LAUNCHD_PLIST_PATH,
        LAUNCHD_PLIST_PATH
    );

    let output = Command::new("osascript")
        .arg("-e")
        .arg(&script)
        .output()
        .map_err(|e| format!("Failed to setup port forwarding: {}", e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("Failed to install launchd service: {}", stderr));
    }

    // Clean up temp plist
    let _ = fs::remove_file(&temp_plist);

    println!("Port forwarding installed via launchd: 443 -> 30443");
    println!("Service will auto-restart if it crashes");
    Ok(())
}

/// Generate the launchd plist XML content for portfwd
#[cfg(target_os = "macos")]
fn generate_launchd_plist(portfwd_path: &std::path::Path) -> Result<String, String> {
    let log_path = dirs::home_dir()
        .ok_or_else(|| "Failed to determine home directory".to_string())?
        .join(".vibespace")
        .join("portfwd.log");

    Ok(format!(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{}</string>
        <string>443</string>
        <string>127.0.0.1:30443</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{}</string>
    <key>StandardErrorPath</key>
    <string>{}</string>
</dict>
</plist>"#,
        LAUNCHD_LABEL,
        portfwd_path.display(),
        log_path.display(),
        log_path.display()
    ))
}

/// Get the path to the bundled portfwd binary
fn get_portfwd_binary_path() -> Option<PathBuf> {
    let resource_dir = get_resource_dir()?;

    // Detect OS and architecture
    let os = if cfg!(target_os = "macos") {
        "darwin"
    } else if cfg!(target_os = "linux") {
        "linux"
    } else {
        return None;
    };

    let arch = if cfg!(target_arch = "aarch64") {
        "arm64"
    } else {
        "amd64"
    };

    Some(resource_dir.join(format!("binaries/portfwd-{}-{}", os, arch)))
}

#[cfg(target_os = "macos")]
fn cleanup_macos_port_forwarding() -> Result<(), String> {
    // Check if launchd service exists
    if !std::path::Path::new(LAUNCHD_PLIST_PATH).exists() {
        println!("Port forwarding launchd service not installed");
        return Ok(());
    }

    // Unload and remove the launchd service (requires admin)
    let script = format!(
        r#"do shell script "launchctl unload '{}' 2>/dev/null || true; rm -f '{}'" with administrator privileges"#,
        LAUNCHD_PLIST_PATH,
        LAUNCHD_PLIST_PATH
    );

    let output = Command::new("osascript")
        .arg("-e")
        .arg(&script)
        .output()
        .map_err(|e| format!("Failed to cleanup port forwarding: {}", e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        // Don't fail on cleanup errors, just log them
        eprintln!("Warning: cleanup may have partially failed: {}", stderr);
    }

    // Also clean up any old PID file if it exists (from previous implementation)
    let home_dir = dirs::home_dir();
    if let Some(home) = home_dir {
        let old_pid_file = home.join(".vibespace").join("port-forward.pid");
        let _ = fs::remove_file(&old_pid_file);
    }

    println!("Port forwarding launchd service unloaded and removed");
    Ok(())
}

#[cfg(target_os = "linux")]
fn setup_linux_port_forwarding() -> Result<(), String> {
    let home_dir = dirs::home_dir()
        .ok_or_else(|| "Failed to determine home directory".to_string())?;

    let vibespace_dir = home_dir.join(".vibespace");
    let pid_file = vibespace_dir.join("port-forward.pid");

    // Check if already running
    if is_port_forwarding_configured() {
        println!("Port forwarding already running");
        return Ok(());
    }

    // Get bundled portfwd binary path
    let portfwd_path = get_portfwd_binary_path()
        .ok_or_else(|| "Failed to locate bundled portfwd binary".to_string())?;

    if !portfwd_path.exists() {
        return Err(format!("portfwd binary not found at {:?}", portfwd_path));
    }

    // Start portfwd in background with pkexec
    let script = format!("{} 443 127.0.0.1:30443 & echo $!", portfwd_path.display());

    let output = Command::new("pkexec")
        .arg("sh")
        .arg("-c")
        .arg(&script)
        .output()
        .map_err(|e| format!("Failed to setup port forwarding: {}", e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("Failed to setup port forwarding: {}", stderr));
    }

    // Save the PID
    let pid = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if !pid.is_empty() {
        fs::write(&pid_file, &pid)
            .map_err(|e| format!("Failed to save port forward PID: {}", e))?;
    }

    println!("Port forwarding started: 443 -> 30443 (PID: {})", pid);
    Ok(())
}

#[cfg(target_os = "linux")]
fn cleanup_linux_port_forwarding() -> Result<(), String> {
    let home_dir = dirs::home_dir()
        .ok_or_else(|| "Failed to determine home directory".to_string())?;

    let pid_file = home_dir.join(".vibespace").join("port-forward.pid");

    if let Ok(pid_str) = fs::read_to_string(&pid_file) {
        if let Ok(pid) = pid_str.trim().parse::<u32>() {
            // Kill the socat process
            let script = format!("kill {} 2>/dev/null || true", pid);
            let _ = Command::new("pkexec")
                .arg("sh")
                .arg("-c")
                .arg(&script)
                .output();
        }
    }

    // Remove PID file
    let _ = fs::remove_file(&pid_file);

    println!("Port forwarding stopped");
    Ok(())
}

// ============================================================================
// TLS Certificate Management (mkcert)
// ============================================================================

/// Setup TLS certificates using mkcert for locally-trusted HTTPS
/// Creates wildcard cert for *.vibe.space
pub fn setup_tls_certificates() -> Result<(PathBuf, PathBuf), String> {
    let home_dir = dirs::home_dir()
        .ok_or_else(|| "Failed to determine home directory".to_string())?;

    let certs_dir = home_dir.join(".vibespace").join("certs");
    fs::create_dir_all(&certs_dir)
        .map_err(|e| format!("Failed to create certs directory: {}", e))?;

    let cert_file = certs_dir.join("vibe.space.pem");
    let key_file = certs_dir.join("vibe.space-key.pem");

    // Check if certs already exist
    if cert_file.exists() && key_file.exists() {
        println!("TLS certificates already exist");
        return Ok((cert_file, key_file));
    }

    // Get bundled mkcert binary path
    let mkcert_path = get_mkcert_binary_path()
        .ok_or_else(|| "Failed to locate bundled mkcert binary".to_string())?;

    if !mkcert_path.exists() {
        return Err(format!("mkcert binary not found at {:?}", mkcert_path));
    }

    // Install local CA if not already done (one-time operation)
    // mkcert -install creates CA files and adds to trust store
    // On macOS, mkcert calls sudo internally which hangs without TTY
    // So we: 1) Set TRUST_STORES=nss to skip system trust, 2) Manually add CA via osascript
    println!("Installing mkcert local CA...");

    // First, run mkcert -install with TRUST_STORES=nss to skip system keychain
    // This creates the CA files without needing sudo
    let ca_output = Command::new(&mkcert_path)
        .arg("-install")
        .env("TRUST_STORES", "nss")
        .output()
        .map_err(|e| format!("Failed to create mkcert CA: {}", e))?;

    // mkcert stores CA at ~/Library/Application Support/mkcert/rootCA.pem
    let ca_root = home_dir.join("Library/Application Support/mkcert/rootCA.pem");

    if ca_root.exists() {
        println!("Adding CA to system trust store (requires admin)...");

        #[cfg(target_os = "macos")]
        {
            // Use osascript to add CA to system keychain
            let script = format!(
                r#"do shell script "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain '{}'" with administrator privileges"#,
                ca_root.display()
            );

            let trust_output = Command::new("osascript")
                .arg("-e")
                .arg(&script)
                .output();

            match trust_output {
                Ok(output) if output.status.success() => {
                    println!("CA added to system trust store");
                }
                Ok(output) => {
                    let stderr = String::from_utf8_lossy(&output.stderr);
                    // Don't fail if already trusted or user cancelled
                    if !stderr.contains("SecTrustSettingsSetTrustSettings") && !stderr.contains("User canceled") {
                        println!("Warning: Could not add CA to trust store: {}", stderr);
                    }
                }
                Err(e) => {
                    println!("Warning: Could not add CA to trust store: {}", e);
                }
            }
        }

        #[cfg(target_os = "linux")]
        {
            // On Linux, mkcert handles trust stores differently (nss, etc.)
            // Re-run with default trust stores
            let _ = Command::new(&mkcert_path)
                .arg("-install")
                .output();
        }
    } else if !ca_output.status.success() {
        let stderr = String::from_utf8_lossy(&ca_output.stderr);
        println!("Warning: mkcert CA creation failed: {}", stderr);
    }

    // Generate wildcard certificate for *.vibe.space
    println!("Generating TLS certificate for *.vibe.space...");
    let cert_output = Command::new(&mkcert_path)
        .current_dir(&certs_dir)
        .args([
            "-cert-file", "vibe.space.pem",
            "-key-file", "vibe.space-key.pem",
            "*.vibe.space",
            "vibe.space",
        ])
        .output()
        .map_err(|e| format!("Failed to generate certificate: {}", e))?;

    if !cert_output.status.success() {
        let stderr = String::from_utf8_lossy(&cert_output.stderr);
        return Err(format!("Failed to generate certificate: {}", stderr));
    }

    println!("TLS certificate generated successfully");
    Ok((cert_file, key_file))
}

/// Get the path to the bundled mkcert binary
fn get_mkcert_binary_path() -> Option<PathBuf> {
    let resource_dir = get_resource_dir()?;

    // Detect OS and architecture
    let os = if cfg!(target_os = "macos") {
        "darwin"
    } else if cfg!(target_os = "linux") {
        "linux"
    } else {
        return None;
    };

    let arch = if cfg!(target_arch = "aarch64") {
        "arm64"
    } else {
        "amd64"
    };

    Some(resource_dir.join(format!("binaries/mkcert-{}-{}", os, arch)))
}

/// Check if TLS certificates exist
pub fn is_tls_configured() -> bool {
    let home_dir = match dirs::home_dir() {
        Some(d) => d,
        None => return false,
    };

    let certs_dir = home_dir.join(".vibespace").join("certs");
    let cert_file = certs_dir.join("vibe.space.pem");
    let key_file = certs_dir.join("vibe.space-key.pem");

    cert_file.exists() && key_file.exists()
}


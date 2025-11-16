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
        let resolved_content = "[Resolve]\nDNS=127.0.0.1:5353\nDomains=~vibe.space\n";

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

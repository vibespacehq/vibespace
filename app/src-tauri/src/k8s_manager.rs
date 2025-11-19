// Kubernetes Manager Module
// Trait-based abstraction supporting both Local Mode and Remote Mode deployments.
//
// ARCHITECTURE:
// - K8sProvider trait defines common interface for all deployment modes
// - LocalK8sProvider implements bundled Kubernetes (Colima/k3s on user's machine)
// - RemoteK8sProvider (future) implements remote API connection (VPS deployment)
//
// DEPLOYMENT MODES:
//
// 1. LOCAL MODE (implemented in this file):
//    - All components run on user's machine
//    - Tauri app + Go API server + bundled k8s
//    - LocalK8sProvider manages Colima (macOS) or k3s (Linux)
//    - Vibespaces run in local cluster
//
// 2. REMOTE MODE (planned for Post-MVP):
//    - Control plane (Tauri app) on user's machine
//    - Infrastructure (API server + k8s) on VPS
//    - RemoteK8sProvider manages connection to remote API
//    - No bundled k8s installed locally
//    - Vibespaces run on VPS
//
// The K8sManager delegates to the active provider based on configuration.
// Frontend and Tauri commands use the same interface regardless of mode.
//
// See: ADR 0006 - Bundled Kubernetes Runtime

use serde::{Deserialize, Serialize};
use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::thread;
use std::time::Duration;

// ============================================================================
// Common Types (used by both Local and Remote modes)
// ============================================================================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KubernetesStatus {
    pub installed: bool,
    pub running: bool,
    pub version: Option<String>,
    pub is_external: bool,
    pub error: Option<String>,
    pub suggested_action: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InstallProgress {
    pub stage: String,
    pub progress: u8,
    pub message: String,
}

#[derive(Debug, Clone, PartialEq)]
pub enum DeploymentMode {
    Local,   // Bundled k8s on user's machine
    #[allow(dead_code)]
    Remote,  // Connect to k8s on VPS (planned for Post-MVP - see ADR 0006)
}

// ============================================================================
// K8sProvider Trait - Common interface for all deployment modes
// ============================================================================

/// K8sProvider defines the interface that all deployment modes must implement.
/// This abstraction allows the same Tauri commands and frontend code to work
/// with both Local Mode (bundled k8s) and Remote Mode (VPS connection).
pub trait K8sProvider: Send {
    /// Check if Kubernetes is installed/configured for this mode
    fn is_installed(&self) -> bool;

    /// Check if Kubernetes is currently running/accessible
    fn is_running(&self) -> bool;

    /// Get comprehensive status information
    fn get_status(&self) -> KubernetesStatus;

    /// Install/configure Kubernetes for this mode
    /// - Local Mode: Install Colima/k3s binaries
    /// - Remote Mode: Configure connection to remote API endpoint
    fn install(&self, progress_callback: Box<dyn Fn(InstallProgress)>) -> Result<(), String>;

    /// Start Kubernetes cluster or establish connection
    /// - Local Mode: Start Colima VM or k3s process
    /// - Remote Mode: Establish VPN tunnel or verify API connectivity
    fn start(&self) -> Result<(), String>;

    /// Stop Kubernetes cluster or disconnect
    /// - Local Mode: Stop Colima VM or k3s process
    /// - Remote Mode: Close VPN tunnel (or no-op)
    fn stop(&self) -> Result<(), String>;

    /// Uninstall/unconfigure Kubernetes for this mode
    /// - Local Mode: Remove Colima/k3s binaries and data
    /// - Remote Mode: Remove saved connection configuration
    fn uninstall(&self) -> Result<(), String>;

    /// Get kubeconfig path for kubectl access
    /// Planned for Remote Mode (Post-MVP) - will be used for remote kubectl access
    #[allow(dead_code)]
    fn get_kubeconfig_path(&self) -> Option<PathBuf>;
}

// ============================================================================
// K8sManager - Facade that delegates to active provider
// ============================================================================

pub struct K8sManager {
    provider: Box<dyn K8sProvider>,
    /// Deployment mode - used to determine provider behavior
    /// Currently only Local mode is active, Remote planned for Post-MVP
    #[allow(dead_code)]
    mode: DeploymentMode,
}

impl K8sManager {
    /// Create a new K8sManager with the specified deployment mode.
    /// Currently only Local mode is implemented.
    pub fn new(mode: DeploymentMode) -> Result<Self, String> {
        let provider: Box<dyn K8sProvider> = match mode {
            DeploymentMode::Local => Box::new(LocalK8sProvider::new()?),
            DeploymentMode::Remote => {
                // TODO: Implement RemoteK8sProvider in Post-MVP phase
                // For now, return an error
                return Err("Remote mode not yet implemented. Use Local mode.".to_string());
            }
        };

        Ok(K8sManager { provider, mode })
    }

    /// Create K8sManager with default mode (Local)
    pub fn new_local() -> Result<Self, String> {
        Self::new(DeploymentMode::Local)
    }

    /// Check if Kubernetes is installed
    /// Planned for use in frontend status checks and diagnostics
    #[allow(dead_code)]
    pub fn is_installed(&self) -> bool {
        self.provider.is_installed()
    }

    /// Check if Kubernetes is running
    /// Planned for use in frontend status checks and auto-start logic
    #[allow(dead_code)]
    pub fn is_running(&self) -> bool {
        self.provider.is_running()
    }

    pub fn get_status(&self) -> KubernetesStatus {
        self.provider.get_status()
    }

    pub fn install<F>(&self, progress_callback: F) -> Result<(), String>
    where
        F: Fn(InstallProgress) + 'static,
    {
        self.provider.install(Box::new(progress_callback))
    }

    pub fn start(&self) -> Result<(), String> {
        self.provider.start()
    }

    pub fn stop(&self) -> Result<(), String> {
        self.provider.stop()
    }

    pub fn uninstall(&self) -> Result<(), String> {
        self.provider.uninstall()
    }

    /// Get kubeconfig path for kubectl access
    /// Planned for Remote Mode - will be used to configure kubectl for remote clusters
    #[allow(dead_code)]
    pub fn get_kubeconfig_path(&self) -> Option<PathBuf> {
        self.provider.get_kubeconfig_path()
    }

    /// Get the current deployment mode
    /// Planned for UI display and conditional behavior based on mode
    #[allow(dead_code)]
    pub fn get_mode(&self) -> &DeploymentMode {
        &self.mode
    }
}

// ============================================================================
// LocalK8sProvider - Bundled Kubernetes implementation (Local Mode)
// ============================================================================

#[derive(Debug, Clone, PartialEq)]
enum Platform {
    MacOS,
    Linux,
    Unsupported,
}

struct LocalK8sProvider {
    platform: Platform,
    install_dir: PathBuf,
    kubeconfig_path: PathBuf,
}

impl LocalK8sProvider {
    fn new() -> Result<Self, String> {
        let platform = detect_platform();

        if platform == Platform::Unsupported {
            return Err("Unsupported platform. vibespace requires macOS or Linux.".to_string());
        }

        let home_dir = dirs::home_dir()
            .ok_or_else(|| "Failed to determine home directory".to_string())?;

        let install_dir = home_dir.join(".vibespace");
        let kubeconfig_path = get_kubeconfig_path(&platform)?;

        Ok(LocalK8sProvider {
            platform,
            install_dir,
            kubeconfig_path,
        })
    }

    /// Check if bundled Colima is installed (macOS only)
    /// Checks both the installation directory (~/.vibespace/bin/) and the bundled resources
    fn is_colima_installed(&self) -> bool {
        if self.platform != Platform::MacOS {
            return false;
        }

        // Check if already copied to install directory
        let colima_bin = self.install_dir.join("bin").join("colima");
        if colima_bin.exists() {
            return true;
        }

        // Check if bundled with the app (not yet copied)
        // The binaries are bundled as Tauri resources
        // For now, we consider it "not installed" if not copied yet
        // The install() method will copy them from bundled resources to install_dir
        false
    }

    /// Check if bundled k3s is installed (Linux only)
    fn is_k3s_installed(&self) -> bool {
        if self.platform != Platform::Linux {
            return false;
        }

        let k3s_bin = self.install_dir.join("bin").join("k3s");
        k3s_bin.exists()
    }

    /// Check if Colima VM is running (macOS)
    fn is_colima_running(&self) -> bool {
        if !self.is_colima_installed() {
            return false;
        }

        // Set PATH so colima can find bundled lima
        let lima_bin_dir = self.install_dir.join("lima/bin");
        let bin_dir = self.install_dir.join("bin");
        let current_path = env::var("PATH").unwrap_or_default();
        let new_path = format!("{}:{}:{}", lima_bin_dir.display(), bin_dir.display(), current_path);

        let colima_bin = self.install_dir.join("bin").join("colima");
        let output = Command::new(&colima_bin)
            .env("PATH", new_path)
            .arg("status")
            .output();

        match output {
            Ok(output) => {
                // Colima outputs to stderr, not stdout
                let stderr = String::from_utf8_lossy(&output.stderr);
                stderr.contains("colima is running")
            }
            Err(_) => false,
        }
    }

    /// Check if k3s process is running (Linux)
    fn is_k3s_running(&self) -> bool {
        if !self.is_k3s_installed() {
            return false;
        }

        // Check if k3s process exists
        let output = Command::new("pgrep")
            .arg("-f")
            .arg("k3s server")
            .output();

        match output {
            Ok(output) => output.status.success(),
            Err(_) => false,
        }
    }

    /// Get Kubernetes version from kubectl
    fn get_version(&self) -> Option<String> {
        let kubectl_path = self.install_dir.join("bin").join("kubectl");
        if !kubectl_path.exists() {
            return None;
        }

        let output = Command::new(&kubectl_path)
            .env("KUBECONFIG", &self.kubeconfig_path)
            .arg("version")
            .arg("--short")
            .arg("--client=false")
            .output();

        match output {
            Ok(output) if output.status.success() => {
                let stdout = String::from_utf8_lossy(&output.stdout);
                Some(stdout.trim().to_string())
            }
            _ => None,
        }
    }

    /// Check if external kubectl exists (for backward compatibility)
    /// Planned for hybrid detection mode where users can use external kubectl
    #[allow(dead_code)]
    fn has_external_kubectl(&self) -> bool {
        Command::new("which")
            .arg("kubectl")
            .output()
            .map(|output| output.status.success())
            .unwrap_or(false)
    }
}

impl K8sProvider for LocalK8sProvider {
    fn is_installed(&self) -> bool {
        match self.platform {
            Platform::MacOS => self.is_colima_installed(),
            Platform::Linux => self.is_k3s_installed(),
            Platform::Unsupported => false,
        }
    }

    fn is_running(&self) -> bool {
        match self.platform {
            Platform::MacOS => self.is_colima_running(),
            Platform::Linux => self.is_k3s_running(),
            Platform::Unsupported => false,
        }
    }

    fn get_status(&self) -> KubernetesStatus {
        // Check bundled k8s
        let installed = self.is_installed();
        let running = self.is_running();
        let version = if running { self.get_version() } else { None };

        if installed || running {
            return KubernetesStatus {
                installed,
                running,
                version,
                is_external: false,
                error: None,
                suggested_action: if !running {
                    Some("start".to_string())
                } else {
                    None
                },
            };
        }

        // Not installed - return status indicating installation needed
        // Note: We don't auto-detect external kubectl installations to avoid confusion
        // Users with existing k8s can manually choose to migrate later
        KubernetesStatus {
            installed: false,
            running: false,
            version: None,
            is_external: false,
            error: None,
            suggested_action: Some("install".to_string()),
        }
    }

    fn install(&self, progress_callback: Box<dyn Fn(InstallProgress)>) -> Result<(), String> {
        // Create installation directory
        fs::create_dir_all(self.install_dir.join("bin"))
            .map_err(|e| format!("Failed to create installation directory: {}", e))?;

        match self.platform {
            Platform::MacOS => self.install_colima(&progress_callback),
            Platform::Linux => self.install_k3s(&progress_callback),
            Platform::Unsupported => Err("Unsupported platform".to_string()),
        }
    }

    fn start(&self) -> Result<(), String> {
        match self.platform {
            Platform::MacOS => self.start_colima(),
            Platform::Linux => self.start_k3s(),
            Platform::Unsupported => Err("Unsupported platform".to_string()),
        }
    }

    fn stop(&self) -> Result<(), String> {
        match self.platform {
            Platform::MacOS => self.stop_colima(),
            Platform::Linux => self.stop_k3s(),
            Platform::Unsupported => Err("Unsupported platform".to_string()),
        }
    }

    fn uninstall(&self) -> Result<(), String> {
        // Stop first
        let _ = self.stop();

        // Remove installation directory
        fs::remove_dir_all(&self.install_dir)
            .map_err(|e| format!("Failed to remove installation directory: {}", e))?;

        Ok(())
    }

    fn get_kubeconfig_path(&self) -> Option<PathBuf> {
        if self.kubeconfig_path.exists() {
            Some(self.kubeconfig_path.clone())
        } else {
            None
        }
    }
}

// Platform-specific implementation for macOS (Colima)
impl LocalK8sProvider {
    fn install_colima<F>(&self, progress_callback: F) -> Result<(), String>
    where
        F: Fn(InstallProgress),
    {
        progress_callback(InstallProgress {
            stage: "extracting".to_string(),
            progress: 10,
            message: "Extracting Colima binaries...".to_string(),
        });

        // Get resource path from Tauri
        let resource_dir = get_resource_dir()
            .ok_or_else(|| "Failed to get resource directory".to_string())?;

        println!("Resource directory: {:?}", resource_dir);
        println!("Install directory: {:?}", self.install_dir);

        // Copy binaries from resources to install directory
        // We bundle Colima, full Lima distribution, and kubectl
        let colima_src = resource_dir.join("binaries/macos/colima");
        let lima_dist_src = resource_dir.join("binaries/macos/lima-dist");

        // Detect architecture for kubectl binary
        let arch = if cfg!(target_arch = "aarch64") {
            "arm64"
        } else {
            "amd64"
        };
        let kubectl_src = resource_dir.join(format!("binaries/kubectl-darwin-{}", arch));

        let bin_dir = self.install_dir.join("bin");
        let lima_dist_dest = self.install_dir.join("lima");
        let colima_dest = bin_dir.join("colima");
        let kubectl_dest = bin_dir.join("kubectl");

        // Copy Colima and kubectl binaries
        fs::copy(&colima_src, &colima_dest)
            .map_err(|e| format!("Failed to copy colima binary from {:?}: {}", colima_src, e))?;
        fs::copy(&kubectl_src, &kubectl_dest)
            .map_err(|e| format!("Failed to copy kubectl binary from {:?}: {}", kubectl_src, e))?;

        // Copy full Lima distribution (bin/, share/, etc.)
        copy_dir_recursive(&lima_dist_src, &lima_dist_dest)
            .map_err(|e| format!("Failed to copy Lima distribution from {:?}: {}", lima_dist_src, e))?;

        // Make binaries executable
        set_executable(&colima_dest)?;
        set_executable(&kubectl_dest)?;
        set_executable(&lima_dist_dest.join("bin/limactl"))?;
        set_executable(&lima_dist_dest.join("bin/lima"))?;

        progress_callback(InstallProgress {
            stage: "installing".to_string(),
            progress: 50,
            message: "Starting Colima VM...".to_string(),
        });

        // Start Colima with Kubernetes
        // Must use bash wrapper with PATH so Colima can find Lima
        let lima_bin_dir = lima_dist_dest.join("bin");
        let current_path = env::var("PATH").unwrap_or_default();
        let new_path = format!("{}:{}:{}", lima_bin_dir.display(), bin_dir.display(), current_path);

        let command_str = format!(
            "PATH='{}' '{}' start --kubernetes --cpu 2 --memory 4 --disk 60",
            new_path,
            colima_dest.display()
        );

        println!("Installing Colima with Kubernetes: {}", command_str);

        use std::time::Duration;

        // Spawn Colima without waiting
        let mut child = Command::new("bash")
            .arg("-c")
            .arg(&command_str)
            .spawn()
            .map_err(|e| format!("Failed to start Colima: {}", e))?;

        // Emit progress updates periodically while Colima installs
        // Note: Colima has 2 phases (Docker provision + VM restart + k8s provision)
        // We map this to a single progress bar (0-60%)
        let progress_messages = [
            "Starting virtual machine...",
            "Creating VM instance...",
            "Preparing disk...",
            "Booting VM...",
            "Waiting for SSH...",
            "Configuring Docker...",
            "Restarting for Kubernetes...",
            "Downloading Kubernetes...",
            "Installing Kubernetes...",
            "Starting Kubernetes...",
        ];

        let mut iteration = 0;
        let max_iterations = progress_messages.len();

        loop {
            // Check if process finished
            match child.try_wait() {
                Ok(Some(status)) => {
                    if !status.success() {
                        return Err("Colima failed to start".to_string());
                    }
                    break;
                }
                Ok(None) => {
                    // Still running, emit progress update
                    if iteration < max_iterations {
                        let percentage = ((iteration + 1) * 60) / max_iterations;
                        progress_callback(InstallProgress {
                            stage: "installing".to_string(),
                            progress: percentage as u8,
                            message: progress_messages[iteration].to_string(),
                        });
                        iteration += 1;
                    }
                    std::thread::sleep(Duration::from_secs(5));
                }
                Err(e) => {
                    return Err(format!("Error checking Colima status: {}", e));
                }
            }
        }

        // Configure insecure registry for local development
        println!("Colima started successfully, now configuring insecure registry");
        std::thread::sleep(Duration::from_secs(2)); // Wait for colima.yaml to be written

        self.configure_colima_registry()?;

        // Restart Colima to apply registry configuration
        println!("Restarting Colima to apply insecure registry configuration");
        self.restart_colima(&new_path)?;

        progress_callback(InstallProgress {
            stage: "verifying".to_string(),
            progress: 90,
            message: "Verifying cluster...".to_string(),
        });

        // Wait for cluster to be ready
        self.wait_for_cluster_ready()?;

        progress_callback(InstallProgress {
            stage: "complete".to_string(),
            progress: 100,
            message: "Kubernetes installed successfully".to_string(),
        });

        Ok(())
    }

    fn install_k3s<F>(&self, progress_callback: F) -> Result<(), String>
    where
        F: Fn(InstallProgress),
    {
        progress_callback(InstallProgress {
            stage: "extracting".to_string(),
            progress: 10,
            message: "Extracting k3s binaries...".to_string(),
        });

        // Get resource path from Tauri
        let resource_dir = get_resource_dir()
            .ok_or_else(|| "Failed to get resource directory".to_string())?;

        // Copy binaries from resources to install directory
        let k3s_src = resource_dir.join("binaries/linux/k3s");
        let kubectl_src = resource_dir.join("binaries/kubectl-linux-amd64");

        let bin_dir = self.install_dir.join("bin");
        let k3s_dest = bin_dir.join("k3s");
        let kubectl_dest = bin_dir.join("kubectl");

        fs::copy(&k3s_src, &k3s_dest)
            .map_err(|e| format!("Failed to copy k3s binary: {}", e))?;
        fs::copy(&kubectl_src, &kubectl_dest)
            .map_err(|e| format!("Failed to copy kubectl binary: {}", e))?;

        // Make binaries executable
        set_executable(&k3s_dest)?;
        set_executable(&kubectl_dest)?;

        progress_callback(InstallProgress {
            stage: "installing".to_string(),
            progress: 50,
            message: "Starting k3s server...".to_string(),
        });

        // Start k3s in background
        let k3s_data_dir = self.install_dir.join("k3s-data");
        fs::create_dir_all(&k3s_data_dir)
            .map_err(|e| format!("Failed to create k3s data directory: {}", e))?;

        let log_file = fs::File::create(self.install_dir.join("k3s.log"))
            .map_err(|e| format!("Failed to create log file: {}", e))?;

        Command::new(&k3s_dest)
            .arg("server")
            .arg("--data-dir")
            .arg(&k3s_data_dir)
            .arg("--write-kubeconfig")
            .arg(&self.kubeconfig_path)
            .arg("--write-kubeconfig-mode")
            .arg("644")
            .arg("--disable")
            .arg("traefik")
            .stdout(Stdio::from(log_file.try_clone().unwrap()))
            .stderr(Stdio::from(log_file))
            .spawn()
            .map_err(|e| format!("Failed to start k3s: {}", e))?;

        progress_callback(InstallProgress {
            stage: "verifying".to_string(),
            progress: 90,
            message: "Verifying cluster...".to_string(),
        });

        // Wait for cluster to be ready
        self.wait_for_cluster_ready()?;

        progress_callback(InstallProgress {
            stage: "complete".to_string(),
            progress: 100,
            message: "Kubernetes installed successfully".to_string(),
        });

        Ok(())
    }

    fn start_colima(&self) -> Result<(), String> {
        let colima_bin = self.install_dir.join("bin").join("colima");
        let bin_dir = self.install_dir.join("bin");
        let lima_bin_dir = self.install_dir.join("lima/bin");

        // Add our bin directories to PATH so colima can find limactl and guest agents
        let current_path = env::var("PATH").unwrap_or_default();
        let new_path = format!("{}:{}:{}", lima_bin_dir.display(), bin_dir.display(), current_path);

        println!("Starting Colima from: {:?}", colima_bin);
        println!("PATH: {}", new_path);

        // Use bash to run colima with PATH set
        // This ensures the environment is properly inherited by Colima's subprocesses (like limactl)
        let command_str = format!("PATH='{}' '{}' start", new_path, colima_bin.display());

        println!("Executing: bash -c {}", command_str);

        let status = Command::new("bash")
            .arg("-c")
            .arg(&command_str)
            .status()
            .map_err(|e| format!("Failed to start Colima: {}", e))?;

        if !status.success() {
            return Err("Failed to start Colima".to_string());
        }

        // Now that Colima has started and created colima.yaml, configure insecure registry
        println!("Colima started successfully, now configuring insecure registry");

        // Wait a moment for colima.yaml to be fully written
        std::thread::sleep(std::time::Duration::from_secs(2));

        // Update colima.yaml with insecure-registries config
        self.configure_colima_registry()?;

        // Restart Colima to apply the configuration
        println!("Restarting Colima to apply insecure registry configuration");
        self.restart_colima(&new_path)?;

        // Wait for cluster to be ready after restart
        self.wait_for_cluster_ready()?;

        Ok(())
    }

    fn restart_colima(&self, path_env: &str) -> Result<(), String> {
        let colima_bin = self.install_dir.join("bin").join("colima");

        println!("Stopping Colima...");
        let stop_command = format!("PATH='{}' '{}' stop", path_env, colima_bin.display());
        let status = Command::new("bash")
            .arg("-c")
            .arg(&stop_command)
            .status()
            .map_err(|e| format!("Failed to stop Colima: {}", e))?;

        if !status.success() {
            return Err("Failed to stop Colima".to_string());
        }

        // Wait a moment for clean shutdown
        std::thread::sleep(std::time::Duration::from_secs(3));

        println!("Starting Colima again...");
        let start_command = format!("PATH='{}' '{}' start", path_env, colima_bin.display());
        let status = Command::new("bash")
            .arg("-c")
            .arg(&start_command)
            .status()
            .map_err(|e| format!("Failed to restart Colima: {}", e))?;

        if !status.success() {
            return Err("Failed to restart Colima".to_string());
        }

        // Wait for Colima to fully start
        std::thread::sleep(std::time::Duration::from_secs(2));

        // Restart Docker daemon inside VM to apply insecure-registries config
        // Based on GitHub issue #834: config changes require Docker daemon restart
        println!("Restarting Docker daemon inside Colima VM to apply registry configuration...");

        let daemon_reload = format!("PATH='{}' '{}' ssh sudo systemctl daemon-reload", path_env, colima_bin.display());
        let status = Command::new("bash")
            .arg("-c")
            .arg(&daemon_reload)
            .status()
            .map_err(|e| format!("Failed to reload systemd daemon: {}", e))?;

        if !status.success() {
            println!("Warning: Failed to reload systemd daemon, continuing anyway...");
        }

        let docker_restart = format!("PATH='{}' '{}' ssh sudo systemctl restart docker", path_env, colima_bin.display());
        let status = Command::new("bash")
            .arg("-c")
            .arg(&docker_restart)
            .status()
            .map_err(|e| format!("Failed to restart Docker daemon: {}", e))?;

        if !status.success() {
            return Err("Failed to restart Docker daemon inside Colima VM".to_string());
        }

        println!("Docker daemon restarted successfully");

        Ok(())
    }

    fn configure_colima_registry(&self) -> Result<(), String> {
        // Colima config file path
        let home_dir = env::var("HOME").map_err(|_| "HOME environment variable not set".to_string())?;
        let config_path = PathBuf::from(&home_dir)
            .join(".colima")
            .join("default")
            .join("colima.yaml");

        println!("Checking Colima config at: {:?}", config_path);

        // Colima should have created this file by now
        if !config_path.exists() {
            return Err(format!(
                "Colima config file not found at {:?}. Colima may not have started properly.",
                config_path
            ));
        }

        // Read existing config
        let config_content = fs::read_to_string(&config_path)
            .map_err(|e| format!("Failed to read Colima config: {}", e))?;

        println!("Current config content:\n{}", config_content);

        // Check if already configured
        if config_content.contains("host.docker.internal:30500") {
            println!("Colima already configured for local registry");
            return Ok(());
        }

        // Replace "docker: {}" with our insecure registry config
        // Use official Colima YAML format: 2 spaces for indentation, dash aligned with parent key
        let updated_content = config_content.replace(
            "docker: {}",
            "docker:\n  insecure-registries:\n  - host.docker.internal:30500"
        );

        // Verify the replacement worked
        if updated_content == config_content {
            return Err(format!(
                "Failed to update Colima config: 'docker: {{}}' pattern not found. Config may have unexpected format."
            ));
        }

        println!("Updated config content:\n{}", updated_content);

        // Write updated config
        fs::write(&config_path, updated_content)
            .map_err(|e| format!("Failed to update Colima config: {}", e))?;

        println!("Successfully updated Colima config with insecure registry");
        Ok(())
    }

    fn start_k3s(&self) -> Result<(), String> {
        let k3s_bin = self.install_dir.join("bin").join("k3s");
        let k3s_data_dir = self.install_dir.join("k3s-data");

        let log_file = fs::File::create(self.install_dir.join("k3s.log"))
            .map_err(|e| format!("Failed to create log file: {}", e))?;

        Command::new(&k3s_bin)
            .arg("server")
            .arg("--data-dir")
            .arg(&k3s_data_dir)
            .arg("--write-kubeconfig")
            .arg(&self.kubeconfig_path)
            .arg("--write-kubeconfig-mode")
            .arg("644")
            .arg("--disable")
            .arg("traefik")
            .stdout(Stdio::from(log_file.try_clone().unwrap()))
            .stderr(Stdio::from(log_file))
            .spawn()
            .map_err(|e| format!("Failed to start k3s: {}", e))?;

        self.wait_for_cluster_ready()?;
        Ok(())
    }

    fn stop_colima(&self) -> Result<(), String> {
        let colima_bin = self.install_dir.join("bin").join("colima");

        let status = Command::new(&colima_bin)
            .arg("stop")
            .status()
            .map_err(|e| format!("Failed to stop Colima: {}", e))?;

        if status.success() {
            Ok(())
        } else {
            Err("Failed to stop Colima".to_string())
        }
    }

    fn stop_k3s(&self) -> Result<(), String> {
        // Kill k3s process
        let output = Command::new("pkill")
            .arg("-f")
            .arg("k3s server")
            .output()
            .map_err(|e| format!("Failed to stop k3s: {}", e))?;

        if output.status.success() {
            Ok(())
        } else {
            Err("Failed to stop k3s".to_string())
        }
    }

    fn wait_for_cluster_ready(&self) -> Result<(), String> {
        let kubectl_path = self.install_dir.join("bin").join("kubectl");
        let max_attempts = 60;
        let delay = Duration::from_secs(2);

        for _ in 0..max_attempts {
            let output = Command::new(&kubectl_path)
                .env("KUBECONFIG", &self.kubeconfig_path)
                .arg("cluster-info")
                .output();

            if let Ok(output) = output {
                if output.status.success() {
                    return Ok(());
                }
            }

            thread::sleep(delay);
        }

        Err("Timeout waiting for cluster to be ready".to_string())
    }
}

// ============================================================================
// RemoteK8sProvider - Remote API connection (Remote Mode)
// PLACEHOLDER - To be implemented in Post-MVP phase
// ============================================================================

// TODO: Implement RemoteK8sProvider for Remote Mode deployment
//
// struct RemoteK8sProvider {
//     endpoint: String,        // https://my-vps.example.com:8090
//     auth_token: String,      // JWT or API key
//     tunnel: Option<WireGuard>, // Optional VPN tunnel
// }
//
// impl K8sProvider for RemoteK8sProvider {
//     fn is_installed(&self) -> bool {
//         // Check if connection config exists
//     }
//
//     fn is_running(&self) -> bool {
//         // HTTP GET to /api/v1/health
//     }
//
//     fn get_status(&self) -> KubernetesStatus {
//         // HTTP GET to /api/v1/cluster/status
//     }
//
//     fn install<F>(&self, _progress_callback: F) -> Result<(), String> {
//         // Save endpoint and auth token to config file
//         // No actual k8s installation needed
//     }
//
//     fn start(&self) -> Result<(), String> {
//         // Establish WireGuard tunnel if configured
//         // Or just verify API connectivity
//     }
//
//     fn stop(&self) -> Result<(), String> {
//         // Close WireGuard tunnel
//     }
//
//     fn uninstall(&self) -> Result<(), String> {
//         // Remove saved connection config
//     }
//
//     fn get_kubeconfig_path(&self) -> Option<PathBuf> {
//         // Return None (remote mode doesn't use local kubeconfig)
//         // Or return a proxy kubeconfig that tunnels through API
//     }
// }

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

fn get_kubeconfig_path(platform: &Platform) -> Result<PathBuf, String> {
    let home_dir = dirs::home_dir()
        .ok_or_else(|| "Failed to determine home directory".to_string())?;

    match platform {
        // Colima updates ~/.kube/config with the "colima" context
        Platform::MacOS => Ok(home_dir.join(".kube/config")),
        Platform::Linux => Ok(home_dir.join(".kube/config")),
        Platform::Unsupported => Err("Unsupported platform".to_string()),
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
    // exe is at target/debug/vibespace, we need to go up to src-tauri/
    let dev_dir = exe_path.parent()?.parent()?.parent()?;
    if dev_dir.join("binaries").exists() {
        return Some(dev_dir.to_path_buf());
    }

    None
}

/// Recursively copy a directory and all its contents
fn copy_dir_recursive(src: &Path, dest: &Path) -> std::io::Result<()> {
    if !dest.exists() {
        fs::create_dir_all(dest)?;
    }

    for entry in fs::read_dir(src)? {
        let entry = entry?;
        let file_type = entry.file_type()?;
        let src_path = entry.path();
        let dest_path = dest.join(entry.file_name());

        if file_type.is_symlink() {
            // Handle symlinks by reading the target and creating a new symlink
            let link_target = fs::read_link(&src_path)?;

            // Remove existing symlink if present
            if dest_path.exists() || dest_path.is_symlink() {
                fs::remove_file(&dest_path)?;
            }

            #[cfg(unix)]
            {
                use std::os::unix::fs::symlink;
                symlink(&link_target, &dest_path)?;
            }

            #[cfg(windows)]
            {
                use std::os::windows::fs::symlink_file;
                symlink_file(&link_target, &dest_path)?;
            }
        } else if file_type.is_dir() {
            copy_dir_recursive(&src_path, &dest_path)?;
        } else {
            fs::copy(&src_path, &dest_path)?;
        }
    }

    Ok(())
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


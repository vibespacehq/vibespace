// Prevents additional console window on Windows in release builds
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod k8s_manager;
mod dns_manager;

use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use base64::{engine::general_purpose, Engine as _};
use tauri::Emitter;
use k8s_manager::{K8sManager, KubernetesStatus, InstallProgress};
use dns_manager::{DnsManager, DnsStatus, DnsSetupProgress};
use rand::RngCore;
use serde::{Deserialize, Serialize};
use ssh_key::{Algorithm, LineEnding, PrivateKey};
use std::fs;
use std::path::PathBuf;
use std::process::Command;
use std::sync::Mutex;
use tauri::Manager;

// ============================================================================
// Global State Guards
// ============================================================================

// Guard to prevent multiple simultaneous Kubernetes installations
// Without this, multiple clicks or state changes could spawn multiple Colima/k3s processes
static INSTALL_GUARD: Mutex<bool> = Mutex::new(false);

// ============================================================================
// Types
// ============================================================================

#[derive(Debug, Serialize, Deserialize)]
struct CredentialData {
    name: String,
    cred_type: String,
    data: serde_json::Value,
}

#[derive(Debug, Serialize)]
struct CredentialSummary {
    id: String,
    name: String,
    cred_type: String,
    created_at: String,
}

#[derive(Debug, Serialize)]
struct SshKeyPair {
    id: String,
    name: String,
    public_key: String,
    key_type: String,
    created_at: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct HostEntry {
    ip: String,
    hostname: String,
}

// ============================================================================
// Utility Functions
// ============================================================================

fn is_valid_ip(ip: &str) -> bool {
    // Simple IPv4 validation
    let parts: Vec<&str> = ip.split('.').collect();
    if parts.len() != 4 {
        return false;
    }
    parts.iter().all(|part| {
        part.parse::<u8>().is_ok()
    })
}

fn is_valid_hostname(hostname: &str) -> bool {
    // Hostname validation: alphanumeric, hyphens, dots
    // No path separators or special characters
    if hostname.is_empty() || hostname.len() > 253 {
        return false;
    }

    // Check for path traversal attempts
    if hostname.contains("..") || hostname.contains('/') || hostname.contains('\\') {
        return false;
    }

    // Valid hostname characters: a-z, A-Z, 0-9, hyphen, dot
    hostname.chars().all(|c| c.is_alphanumeric() || c == '-' || c == '.')
}

fn get_vibespace_dir() -> Result<PathBuf, String> {
    let home = dirs::home_dir().ok_or("Failed to get home directory")?;
    let vibespace_dir = home.join(".vibespace");

    if !vibespace_dir.exists() {
        fs::create_dir_all(&vibespace_dir)
            .map_err(|e| format!("Failed to create vibespace directory: {}", e))?;
    }

    Ok(vibespace_dir)
}

fn get_credential_dir() -> Result<PathBuf, String> {
    let vibespace_dir = get_vibespace_dir()?;
    let cred_dir = vibespace_dir.join("credential");

    if !cred_dir.exists() {
        fs::create_dir_all(&cred_dir)
            .map_err(|e| format!("Failed to create credential directory: {}", e))?;
    }

    Ok(cred_dir)
}

fn get_encryption_key() -> Result<Vec<u8>, String> {
    let vibespace_dir = get_vibespace_dir()?;
    let key_file = vibespace_dir.join(".key");

    if key_file.exists() {
        fs::read(&key_file)
            .map_err(|e| format!("Failed to read encryption key: {}", e))
    } else {
        // Generate new key
        let mut key = vec![0u8; 32];
        OsRng.fill_bytes(&mut key);

        fs::write(&key_file, &key)
            .map_err(|e| format!("Failed to save encryption key: {}", e))?;

        Ok(key)
    }
}

fn encrypt_data(data: &str) -> Result<String, String> {
    let key_bytes = get_encryption_key()?;

    // Validate key length before creating cipher
    if key_bytes.len() != 32 {
        return Err(format!("Invalid encryption key length: expected 32 bytes, got {}", key_bytes.len()));
    }

    let key = aes_gcm::Key::<Aes256Gcm>::from_slice(&key_bytes);
    let cipher = Aes256Gcm::new(key);

    let mut nonce_bytes = [0u8; 12];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);

    let ciphertext = cipher
        .encrypt(nonce, data.as_bytes())
        .map_err(|e| format!("Encryption failed: {}", e))?;

    // Combine nonce + ciphertext
    let mut result = nonce_bytes.to_vec();
    result.extend_from_slice(&ciphertext);

    Ok(general_purpose::STANDARD.encode(&result))
}

#[allow(dead_code)]
fn decrypt_data(encrypted: &str) -> Result<String, String> {
    let key_bytes = get_encryption_key()?;

    // Validate key length before creating cipher
    if key_bytes.len() != 32 {
        return Err(format!("Invalid encryption key length: expected 32 bytes, got {}", key_bytes.len()));
    }

    let key = aes_gcm::Key::<Aes256Gcm>::from_slice(&key_bytes);
    let cipher = Aes256Gcm::new(key);

    let combined = general_purpose::STANDARD.decode(encrypted)
        .map_err(|e| format!("Base64 decode failed: {}", e))?;

    if combined.len() < 12 {
        return Err("Invalid encrypted data".to_string());
    }

    let (nonce_bytes, ciphertext) = combined.split_at(12);
    let nonce = Nonce::from_slice(nonce_bytes);

    let plaintext = cipher
        .decrypt(nonce, ciphertext)
        .map_err(|e| format!("Decryption failed: {}", e))?;

    String::from_utf8(plaintext)
        .map_err(|e| format!("UTF-8 decode failed: {}", e))
}

// ============================================================================
// Tauri Commands - Bundled Kubernetes
// ============================================================================

#[tauri::command]
async fn get_kubernetes_status() -> Result<KubernetesStatus, String> {
    println!("Getting Kubernetes status...");

    // Use Local Mode (bundled k8s). Remote Mode not yet implemented.
    let manager = K8sManager::new_local()?;
    let status = manager.get_status();

    println!("Kubernetes status: installed={}, running={}, is_external={}",
        status.installed, status.running, status.is_external);

    Ok(status)
}

#[tauri::command]
async fn install_kubernetes(app_handle: tauri::AppHandle) -> Result<(), String> {
    println!("Installing bundled Kubernetes...");

    // Guard: Prevent multiple simultaneous installations
    {
        let mut guard = INSTALL_GUARD.lock()
            .map_err(|e| format!("Failed to acquire installation lock: {}", e))?;

        if *guard {
            println!("Installation already in progress, ignoring duplicate request");
            return Err("Installation already in progress".to_string());
        }

        *guard = true;
    }

    // Use Local Mode (bundled k8s). Remote Mode not yet implemented.
    let manager = K8sManager::new_local()?;

    // Spawn installation in background thread to avoid blocking
    std::thread::spawn(move || {
        // Clone app_handle for error handling
        let app_handle_clone = app_handle.clone();

        let result = manager.install(move |progress: InstallProgress| {
            let _ = app_handle.emit("install-progress", &progress);
            println!("Install progress: {} - {}", progress.stage, progress.message);
        });

        if let Err(e) = result {
            eprintln!("Kubernetes installation failed: {}", e);
            let _ = app_handle_clone.emit("install-progress", &InstallProgress {
                stage: "error".to_string(),
                progress: 0,
                message: format!("Installation failed: {}", e),
            });
        } else {
            println!("Kubernetes installed successfully");
            // NOTE: Harbor CA trust removed - using simple Docker Registry with HTTP (no TLS)
        }

        // Release the guard when installation completes (success or error)
        if let Ok(mut guard) = INSTALL_GUARD.lock() {
            *guard = false;
            println!("Installation guard released");
        }
    });

    // Return immediately - frontend will track progress via events
    Ok(())
}

#[tauri::command]
async fn start_kubernetes() -> Result<(), String> {
    println!("Starting Kubernetes...");

    let manager = K8sManager::new_local()?;
    manager.start()?;

    println!("Kubernetes started successfully");
    Ok(())
}

#[tauri::command]
async fn stop_kubernetes() -> Result<(), String> {
    println!("Stopping Kubernetes...");

    let manager = K8sManager::new_local()?;
    manager.stop()?;

    println!("Kubernetes stopped successfully");
    Ok(())
}

#[tauri::command]
async fn uninstall_kubernetes() -> Result<(), String> {
    println!("Uninstalling Kubernetes...");

    let manager = K8sManager::new_local()?;
    manager.uninstall()?;

    println!("Kubernetes uninstalled successfully");
    Ok(())
}

#[tauri::command]
async fn get_os_type() -> Result<String, String> {
    Ok(std::env::consts::OS.to_string())
}

// ============================================================================
// Tauri Commands - DNS
// ============================================================================

#[tauri::command]
async fn get_dns_status() -> Result<DnsStatus, String> {
    println!("Getting DNS status...");

    let manager = DnsManager::new()?;
    let status = manager.get_status();

    println!("DNS status: installed={}, running={}, configured={}",
        status.installed, status.running, status.configured);

    Ok(status)
}

#[tauri::command]
async fn setup_dns(app_handle: tauri::AppHandle) -> Result<(), String> {
    println!("Setting up DNS...");

    let manager = DnsManager::new()?;

    // Spawn setup in background thread to avoid blocking
    std::thread::spawn(move || {
        let app_handle_clone = app_handle.clone();

        let result = manager.setup(move |progress: DnsSetupProgress| {
            let _ = app_handle.emit("dns-setup-progress", &progress);
            println!("DNS setup progress: {} - {}", progress.stage, progress.message);
        });

        if let Err(e) = result {
            eprintln!("DNS setup failed: {}", e);
            let _ = app_handle_clone.emit("dns-setup-progress", &DnsSetupProgress {
                stage: "error".to_string(),
                progress: 0,
                message: format!("DNS setup failed: {}", e),
            });
        } else {
            println!("DNS setup completed successfully");
        }
    });

    // Return immediately - frontend will track progress via events
    Ok(())
}

#[tauri::command]
async fn start_dns() -> Result<(), String> {
    println!("Starting DNS...");

    let manager = DnsManager::new()?;
    manager.start()?;

    println!("DNS started successfully");
    Ok(())
}

#[tauri::command]
async fn stop_dns() -> Result<(), String> {
    println!("Stopping DNS...");

    let manager = DnsManager::new()?;
    manager.stop()?;

    println!("DNS stopped successfully");
    Ok(())
}

#[tauri::command]
async fn cleanup_dns() -> Result<(), String> {
    println!("Cleaning up DNS...");

    let manager = DnsManager::new()?;
    manager.cleanup()?;

    println!("DNS cleanup completed successfully");
    Ok(())
}

// ============================================================================
// Tauri Commands - Credentials
// ============================================================================

#[tauri::command]
async fn save_credential(cred_type: String, data: CredentialData) -> Result<String, String> {
    println!("Saving credential: {} ({})", data.name, cred_type);

    let cred_dir = get_credential_dir()?;
    let id = uuid::Uuid::new_v4().to_string();
    let cred_file = cred_dir.join(format!("{}.json", id));

    // Encrypt sensitive data
    let data_json = serde_json::to_string(&data)
        .map_err(|e| format!("Failed to serialize credential: {}", e))?;
    let encrypted = encrypt_data(&data_json)?;

    // Save metadata + encrypted data
    let metadata = serde_json::json!({
        "id": id,
        "name": data.name,
        "type": cred_type,
        "created_at": chrono::Utc::now().to_rfc3339(),
        "encrypted_data": encrypted
    });

    let metadata_json = serde_json::to_string_pretty(&metadata)
        .map_err(|e| format!("Failed to serialize metadata: {}", e))?;

    fs::write(&cred_file, metadata_json)
        .map_err(|e| format!("Failed to save credential: {}", e))?;

    Ok(id)
}

#[tauri::command]
async fn get_credentials() -> Result<Vec<CredentialSummary>, String> {
    println!("Listing credentials...");

    let cred_dir = get_credential_dir()?;
    let entries = fs::read_dir(&cred_dir)
        .map_err(|e| format!("Failed to read credential directory: {}", e))?;

    let mut credentials = Vec::new();

    for entry in entries {
        let entry = entry.map_err(|e| format!("Failed to read entry: {}", e))?;
        let path = entry.path();

        if path.extension().and_then(|s| s.to_str()) == Some("json") {
            let content = fs::read_to_string(&path)
                .map_err(|e| format!("Failed to read credential file: {}", e))?;

            let metadata: serde_json::Value = serde_json::from_str(&content)
                .map_err(|e| format!("Failed to parse credential metadata: {}", e))?;

            credentials.push(CredentialSummary {
                id: metadata["id"].as_str().unwrap_or("").to_string(),
                name: metadata["name"].as_str().unwrap_or("").to_string(),
                cred_type: metadata["type"].as_str().unwrap_or("").to_string(),
                created_at: metadata["created_at"].as_str().unwrap_or("").to_string(),
            });
        }
    }

    Ok(credentials)
}

#[tauri::command]
async fn delete_credential(id: String) -> Result<(), String> {
    println!("Deleting credential: {}", id);

    // Validate UUID format to prevent path traversal
    uuid::Uuid::parse_str(&id)
        .map_err(|_| "Invalid credential ID format".to_string())?;

    let cred_dir = get_credential_dir()?;
    let cred_file = cred_dir.join(format!("{}.json", id));

    if !cred_file.exists() {
        return Err(format!("Credential not found: {}", id));
    }

    fs::remove_file(&cred_file)
        .map_err(|e| format!("Failed to delete credential: {}", e))?;

    Ok(())
}

#[tauri::command]
async fn generate_ssh_key(name: String, key_type: String) -> Result<SshKeyPair, String> {
    println!("Generating SSH key: {} ({})", name, key_type);

    let algorithm = match key_type.as_str() {
        "ed25519" => Algorithm::Ed25519,
        "rsa" => Algorithm::Rsa { hash: None },
        _ => return Err(format!("Unsupported key type: {}", key_type)),
    };

    // Generate key pair
    let private_key = PrivateKey::random(&mut OsRng, algorithm)
        .map_err(|e| format!("Failed to generate private key: {}", e))?;

    let public_key = private_key.public_key();
    let public_key_str = public_key.to_openssh()
        .map_err(|e| format!("Failed to encode public key: {}", e))?;

    // Save private key
    let cred_dir = get_credential_dir()?;
    let id = uuid::Uuid::new_v4().to_string();
    let key_file = cred_dir.join(format!("{}_id_{}", id, key_type));

    let private_key_str = private_key
        .to_openssh(LineEnding::LF)
        .map_err(|e| format!("Failed to encode private key: {}", e))?;

    let encrypted = encrypt_data(private_key_str.as_ref())?;

    // Save metadata
    let metadata = serde_json::json!({
        "id": id,
        "name": name,
        "type": "ssh_key",
        "key_type": key_type,
        "public_key": public_key_str,
        "created_at": chrono::Utc::now().to_rfc3339(),
        "encrypted_private_key": encrypted
    });

    let metadata_json = serde_json::to_string_pretty(&metadata)
        .map_err(|e| format!("Failed to serialize key metadata: {}", e))?;

    fs::write(&key_file, metadata_json)
        .map_err(|e| format!("Failed to save SSH key: {}", e))?;

    Ok(SshKeyPair {
        id: id.clone(),
        name,
        public_key: public_key_str,
        key_type,
        created_at: metadata["created_at"].as_str().unwrap().to_string(),
    })
}

#[tauri::command]
async fn update_hosts_file(entries: Vec<HostEntry>) -> Result<(), String> {
    println!("Updating /etc/hosts with {} entries", entries.len());

    // Validate all entries first
    for entry in &entries {
        if !is_valid_ip(&entry.ip) {
            return Err(format!("Invalid IP address: {}", entry.ip));
        }
        if !is_valid_hostname(&entry.hostname) {
            return Err(format!("Invalid hostname: {}", entry.hostname));
        }
    }

    // Read current hosts file
    let hosts_content = fs::read_to_string("/etc/hosts")
        .map_err(|e| format!("Failed to read /etc/hosts: {}", e))?;

    let mut lines: Vec<String> = hosts_content.lines().map(|s| s.to_string()).collect();

    // Remove old vibespace entries
    lines.retain(|line| !line.contains("# vibespace-managed"));

    // Add new vibespace entries
    lines.push("".to_string());
    lines.push("# vibespace-managed entries".to_string());
    for entry in entries {
        lines.push(format!("{}\t{}\t# vibespace-managed", entry.ip, entry.hostname));
    }

    let new_content = lines.join("\n");

    // Write with sudo (requires user to enter password)
    // Use secure random temp file to prevent race conditions
    let temp_file = format!("/tmp/vibespace_hosts_{}", uuid::Uuid::new_v4());
    fs::write(&temp_file, &new_content)
        .map_err(|e| format!("Failed to write temp hosts file: {}", e))?;

    let output = Command::new("sudo")
        .args(["cp", &temp_file, "/etc/hosts"])
        .output()
        .map_err(|e| format!("Failed to update /etc/hosts: {}", e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        // Clean up temp file before returning error
        let _ = fs::remove_file(&temp_file);
        return Err(format!("Failed to update /etc/hosts: {}", stderr));
    }

    // Clean up temp file
    let _ = fs::remove_file(&temp_file);

    Ok(())
}

// ============================================================================
// API Server Management
// ============================================================================

fn start_api_server(app_handle: tauri::AppHandle) {
    use std::thread;

    thread::spawn(move || {
        println!("Starting API server...");

        // Kill any existing process on port 8090 (handles dev restarts)
        #[cfg(target_os = "macos")]
        {
            let _ = Command::new("bash")
                .args(["-c", "lsof -ti:8090 | xargs kill -9 2>/dev/null || true"])
                .output();
        }
        #[cfg(target_os = "linux")]
        {
            let _ = Command::new("bash")
                .args(["-c", "fuser -k 8090/tcp 2>/dev/null || true"])
                .output();
        }

        // Small delay to ensure port is freed
        thread::sleep(std::time::Duration::from_millis(100));

        // Determine API server binary path
        let api_server_path = if cfg!(debug_assertions) {
            // Development mode: run from source using 'go run'
            let current_dir = std::env::current_dir()
                .expect("Failed to get current directory");
            let project_root = current_dir
                .parent()
                .expect("Failed to get parent directory")
                .parent()
                .expect("Failed to get project root")
                .to_path_buf();

            let api_dir = project_root.join("api");

            println!("Running API server in dev mode from: {:?}", api_dir);

            let mut cmd = Command::new("go");
            cmd.arg("run")
                .arg("cmd/server/main.go")
                .current_dir(api_dir)
                .stdout(std::process::Stdio::piped())
                .stderr(std::process::Stdio::piped());

            match cmd.spawn() {
                Ok(mut child) => {
                    println!("API server started (PID: {:?})", child.id());

                    // Store the child process so it gets killed when app exits
                    let _app_handle = app_handle.clone();

                    // Log output
                    if let Some(stdout) = child.stdout.take() {
                        use std::io::{BufRead, BufReader};
                        thread::spawn(move || {
                            let reader = BufReader::new(stdout);
                            for line in reader.lines().map_while(Result::ok) {
                                println!("[API] {}", line);
                            }
                        });
                    }

                    if let Some(stderr) = child.stderr.take() {
                        use std::io::{BufRead, BufReader};
                        thread::spawn(move || {
                            let reader = BufReader::new(stderr);
                            for line in reader.lines().map_while(Result::ok) {
                                eprintln!("[API] {}", line);
                            }
                        });
                    }

                    // Wait for the child process to exit
                    let _ = child.wait();
                    println!("API server exited");
                }
                Err(e) => {
                    eprintln!("Failed to start API server: {}", e);
                }
            }

            return;
        } else {
            // Production mode: run compiled binary
            let resource_path = app_handle
                .path()
                .resource_dir()
                .expect("Failed to get resource dir");

            #[cfg(target_os = "windows")]
            let binary_name = "api-server.exe";
            #[cfg(not(target_os = "windows"))]
            let binary_name = "api-server";

            resource_path.join(binary_name)
        };

        // Production mode execution
        if !cfg!(debug_assertions) {
            match Command::new(&api_server_path).spawn() {
                Ok(mut child) => {
                    println!("API server started (PID: {:?})", child.id());
                    let _ = child.wait();
                    println!("API server exited");
                }
                Err(e) => {
                    eprintln!("Failed to start API server: {}", e);
                    eprintln!("API server path: {:?}", api_server_path);
                }
            }
        }
    });
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests;

// ============================================================================
// Main
// ============================================================================

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![
            // Bundled Kubernetes
            get_kubernetes_status,
            install_kubernetes,
            start_kubernetes,
            stop_kubernetes,
            uninstall_kubernetes,
            get_os_type,
            // DNS
            get_dns_status,
            setup_dns,
            start_dns,
            stop_dns,
            cleanup_dns,
            // NOTE: Certificate Trust removed - using simple Docker Registry with HTTP (no TLS)
            // Credentials
            save_credential,
            get_credentials,
            delete_credential,
            generate_ssh_key,
            update_hosts_file
        ])
        .setup(|app| {
            // Initialize vibespace directory on first run
            if let Err(e) = get_vibespace_dir() {
                eprintln!("Failed to initialize vibespace directory: {}", e);
            }

            // Start API server
            start_api_server(app.handle().clone());

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

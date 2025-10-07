// Prevents additional console window on Windows in release builds
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use base64::{engine::general_purpose, Engine as _};
use rand::RngCore;
use serde::{Deserialize, Serialize};
use ssh_key::{Algorithm, LineEnding, PrivateKey};
use std::fs;
use std::path::PathBuf;
use std::process::Command;
use tauri::Manager;

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

fn get_workspace_dir() -> Result<PathBuf, String> {
    let home = dirs::home_dir().ok_or("Failed to get home directory")?;
    let workspace_dir = home.join(".workspace");

    if !workspace_dir.exists() {
        fs::create_dir_all(&workspace_dir)
            .map_err(|e| format!("Failed to create workspace directory: {}", e))?;
    }

    Ok(workspace_dir)
}

fn get_credential_dir() -> Result<PathBuf, String> {
    let workspace_dir = get_workspace_dir()?;
    let cred_dir = workspace_dir.join("credential");

    if !cred_dir.exists() {
        fs::create_dir_all(&cred_dir)
            .map_err(|e| format!("Failed to create credential directory: {}", e))?;
    }

    Ok(cred_dir)
}

fn get_encryption_key() -> Result<Vec<u8>, String> {
    let workspace_dir = get_workspace_dir()?;
    let key_file = workspace_dir.join(".key");

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
// Tauri Commands
// ============================================================================

#[tauri::command]
async fn install_k3s(app_handle: tauri::AppHandle) -> Result<String, String> {
    println!("Installing k3s...");

    // Get bundled script from Tauri resources
    let resource_path = app_handle
        .path()
        .resolve("install_k3s.sh", tauri::path::BaseDirectory::Resource)
        .map_err(|e| format!("Failed to resolve resource path: {}", e))?;

    if !resource_path.exists() {
        return Err("k3s installation script not found in app resources".to_string());
    }

    // Execute installation script
    let output = Command::new("bash")
        .arg(&resource_path)
        .output()
        .map_err(|e| format!("Failed to execute k3s installation: {}", e))?;

    if output.status.success() {
        let stdout = String::from_utf8_lossy(&output.stdout);
        Ok(stdout.to_string())
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        Err(format!("k3s installation failed: {}", stderr))
    }
}

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

    // Remove old workspace entries
    lines.retain(|line| !line.contains("# workspace-managed"));

    // Add new workspace entries
    lines.push("".to_string());
    lines.push("# workspace-managed entries".to_string());
    for entry in entries {
        lines.push(format!("{}\t{}\t# workspace-managed", entry.ip, entry.hostname));
    }

    let new_content = lines.join("\n");

    // Write with sudo (requires user to enter password)
    // Use secure random temp file to prevent race conditions
    let temp_file = format!("/tmp/workspace_hosts_{}", uuid::Uuid::new_v4());
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
// Main
// ============================================================================

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![
            install_k3s,
            save_credential,
            get_credentials,
            delete_credential,
            generate_ssh_key,
            update_hosts_file
        ])
        .setup(|_app| {
            // Initialize workspace directory on first run
            if let Err(e) = get_workspace_dir() {
                eprintln!("Failed to initialize workspace directory: {}", e);
            }

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

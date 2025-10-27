use super::*;
use base64::engine::general_purpose;
use base64::Engine;
use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use rand::RngCore;

// ============================================================================
// Test Helper Functions
// ============================================================================

/// Test-specific encryption that doesn't depend on filesystem
fn test_encrypt_data(data: &str) -> Result<String, String> {
    // Use a fixed test key (32 bytes)
    let key_bytes = [42u8; 32];

    let cipher = Aes256Gcm::new_from_slice(&key_bytes)
        .map_err(|e| format!("Failed to create cipher: {}", e))?;

    // Generate random nonce (12 bytes for AES-GCM)
    let mut nonce_bytes = [0u8; 12];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);

    // Encrypt the data
    let ciphertext = cipher
        .encrypt(nonce, data.as_bytes())
        .map_err(|e| format!("Encryption failed: {}", e))?;

    // Combine nonce + ciphertext and encode as base64
    let mut result = nonce_bytes.to_vec();
    result.extend_from_slice(&ciphertext);

    Ok(general_purpose::STANDARD.encode(&result))
}

/// Test-specific decryption that doesn't depend on filesystem
fn test_decrypt_data(encrypted: &str) -> Result<String, String> {
    // Use the same fixed test key
    let key_bytes = [42u8; 32];

    let cipher = Aes256Gcm::new_from_slice(&key_bytes)
        .map_err(|e| format!("Failed to create cipher: {}", e))?;

    // Decode from base64
    let decoded = general_purpose::STANDARD
        .decode(encrypted)
        .map_err(|e| format!("Base64 decode failed: {}", e))?;

    // Extract nonce (first 12 bytes) and ciphertext (rest)
    if decoded.len() < 12 {
        return Err("Invalid encrypted data: too short".to_string());
    }

    let (nonce_bytes, ciphertext) = decoded.split_at(12);
    let nonce = Nonce::from_slice(nonce_bytes);

    // Decrypt the data
    let plaintext = cipher
        .decrypt(nonce, ciphertext)
        .map_err(|e| format!("Decryption failed: {}", e))?;

    String::from_utf8(plaintext).map_err(|e| format!("UTF-8 decode failed: {}", e))
}

// ============================================================================
// IP Validation Tests
// ============================================================================

#[test]
fn test_valid_ipv4() {
    assert!(is_valid_ip("192.168.1.1"));
    assert!(is_valid_ip("10.0.0.1"));
    assert!(is_valid_ip("172.16.0.1"));
    assert!(is_valid_ip("127.0.0.1"));
    assert!(is_valid_ip("0.0.0.0"));
    assert!(is_valid_ip("255.255.255.255"));
}

#[test]
fn test_invalid_ipv4() {
    assert!(!is_valid_ip("256.1.1.1")); // Out of range
    assert!(!is_valid_ip("192.168.1")); // Too few octets
    assert!(!is_valid_ip("192.168.1.1.1")); // Too many octets
    assert!(!is_valid_ip("192.168.1.a")); // Non-numeric
    assert!(!is_valid_ip("192.168.-1.1")); // Negative
    assert!(!is_valid_ip("")); // Empty
    assert!(!is_valid_ip("localhost")); // Hostname
}

// ============================================================================
// Hostname Validation Tests
// ============================================================================

#[test]
fn test_valid_hostname() {
    assert!(is_valid_hostname("example.com"));
    assert!(is_valid_hostname("sub.example.com"));
    assert!(is_valid_hostname("my-app.local"));
    assert!(is_valid_hostname("workspace-123"));
    assert!(is_valid_hostname("a"));
    assert!(is_valid_hostname("test123"));
}

#[test]
fn test_invalid_hostname() {
    // Path traversal attempts
    assert!(!is_valid_hostname("../etc/passwd"));
    assert!(!is_valid_hostname(".."));
    assert!(!is_valid_hostname("test/../etc"));
    assert!(!is_valid_hostname("test/file"));
    assert!(!is_valid_hostname("test\\file"));

    // Invalid characters
    assert!(!is_valid_hostname("test@example.com"));
    assert!(!is_valid_hostname("test example"));
    assert!(!is_valid_hostname("test_example"));
    assert!(!is_valid_hostname("test#example"));

    // Length limits
    assert!(!is_valid_hostname("")); // Empty
    assert!(!is_valid_hostname(&"a".repeat(254))); // Too long
}

// ============================================================================
// Encryption/Decryption Tests
// ============================================================================

#[test]
fn test_encrypt_decrypt_roundtrip() {
    let original = "sensitive-api-key-12345";
    let encrypted = test_encrypt_data(original).expect("Encryption should succeed");
    let decrypted = test_decrypt_data(&encrypted).expect("Decryption should succeed");

    assert_eq!(original, decrypted);
    assert_ne!(original, encrypted); // Encrypted should be different
}

#[test]
fn test_encrypt_different_nonces() {
    let data = "test-data";
    let encrypted1 = test_encrypt_data(data).expect("Encryption 1 should succeed");
    let encrypted2 = test_encrypt_data(data).expect("Encryption 2 should succeed");

    // Same data should produce different ciphertexts (due to random nonce)
    assert_ne!(encrypted1, encrypted2);

    // But both should decrypt to the same plaintext
    let decrypted1 = test_decrypt_data(&encrypted1).expect("Decryption 1 should succeed");
    let decrypted2 = test_decrypt_data(&encrypted2).expect("Decryption 2 should succeed");
    assert_eq!(decrypted1, decrypted2);
    assert_eq!(decrypted1, data);
}

#[test]
fn test_decrypt_invalid_base64() {
    let result = test_decrypt_data("not-valid-base64!!!");
    assert!(result.is_err());
    assert!(result.unwrap_err().contains("Base64 decode failed"));
}

#[test]
fn test_decrypt_too_short() {
    // Valid base64 but too short to contain nonce + ciphertext
    let result = test_decrypt_data("YWJjZA=="); // "abcd" in base64 (only 4 bytes)
    assert!(result.is_err());
    assert!(result.unwrap_err().contains("Invalid encrypted data"));
}

#[test]
fn test_decrypt_corrupted_data() {
    // Encrypt valid data with more content to ensure adequate ciphertext length
    let encrypted = test_encrypt_data("test data with sufficient length for corruption test")
        .expect("Encryption should succeed");
    let mut bytes = general_purpose::STANDARD
        .decode(&encrypted)
        .expect("Should decode");

    // Ensure we have enough data to corrupt (nonce + ciphertext + tag)
    // AES-GCM produces: 12-byte nonce + ciphertext + 16-byte tag
    assert!(bytes.len() > 28, "Encrypted data should be long enough");

    // Corrupt the ciphertext portion (after nonce, before tag)
    bytes[15] ^= 0xFF; // Flip bits in middle of ciphertext

    let corrupted = general_purpose::STANDARD.encode(&bytes);
    let result = test_decrypt_data(&corrupted);

    assert!(result.is_err());
    assert!(result.unwrap_err().contains("Decryption failed"));
}

#[test]
fn test_encrypt_empty_string() {
    let encrypted = test_encrypt_data("").expect("Encrypting empty string should succeed");
    let decrypted = test_decrypt_data(&encrypted).expect("Decrypting should succeed");
    assert_eq!(decrypted, "");
}

#[test]
fn test_encrypt_unicode() {
    let original = "Hello 世界 🌍 Rust!";
    let encrypted = test_encrypt_data(original).expect("Encryption should succeed");
    let decrypted = test_decrypt_data(&encrypted).expect("Decryption should succeed");
    assert_eq!(original, decrypted);
}

#[test]
fn test_encrypt_large_data() {
    let original = "a".repeat(10000); // 10KB of data
    let encrypted = test_encrypt_data(&original).expect("Encryption should succeed");
    let decrypted = test_decrypt_data(&encrypted).expect("Decryption should succeed");
    assert_eq!(original, decrypted);
}

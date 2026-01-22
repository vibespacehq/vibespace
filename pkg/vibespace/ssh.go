package vibespace

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// SSHKeyPaths contains paths for vibespace SSH keys
type SSHKeyPaths struct {
	Dir        string // ~/.vibespace/ssh/
	PrivateKey string // ~/.vibespace/ssh/vibespace_ed25519
	PublicKey  string // ~/.vibespace/ssh/vibespace_ed25519.pub
}

// GetSSHKeyPaths returns the paths for vibespace SSH keys
func GetSSHKeyPaths() (SSHKeyPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return SSHKeyPaths{}, fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".vibespace", "ssh")
	return SSHKeyPaths{
		Dir:        dir,
		PrivateKey: filepath.Join(dir, "vibespace_ed25519"),
		PublicKey:  filepath.Join(dir, "vibespace_ed25519.pub"),
	}, nil
}

// EnsureSSHKey ensures a dedicated SSH keypair exists for vibespace.
// Generates a new ed25519 keypair if one doesn't exist.
// Returns the public key content.
func EnsureSSHKey() (string, error) {
	paths, err := GetSSHKeyPaths()
	if err != nil {
		return "", err
	}

	// Check if keypair already exists
	if _, err := os.Stat(paths.PrivateKey); err == nil {
		// Key exists, read and return public key
		pubKeyData, err := os.ReadFile(paths.PublicKey)
		if err != nil {
			return "", fmt.Errorf("private key exists but failed to read public key: %w", err)
		}
		return string(pubKeyData), nil
	}

	// Generate new keypair
	return generateSSHKeyPair(paths)
}

// generateSSHKeyPair generates a new ed25519 SSH keypair
func generateSSHKeyPair(paths SSHKeyPaths) (string, error) {
	// Ensure directory exists
	if err := os.MkdirAll(paths.Dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create SSH directory: %w", err)
	}

	// Generate ed25519 keypair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	// Convert private key to OpenSSH format
	privKeyPEM, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Write private key
	if err := os.WriteFile(paths.PrivateKey, pem.EncodeToMemory(privKeyPEM), 0600); err != nil {
		return "", fmt.Errorf("failed to write private key: %w", err)
	}

	// Convert public key to OpenSSH authorized_keys format
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("failed to create SSH public key: %w", err)
	}
	pubKeyStr := string(ssh.MarshalAuthorizedKey(sshPubKey))

	// Write public key
	if err := os.WriteFile(paths.PublicKey, []byte(pubKeyStr), 0644); err != nil {
		return "", fmt.Errorf("failed to write public key: %w", err)
	}

	return pubKeyStr, nil
}

// GetSSHPrivateKeyPath returns the path to the vibespace private key.
// Returns empty string if key doesn't exist.
func GetSSHPrivateKeyPath() string {
	paths, err := GetSSHKeyPaths()
	if err != nil {
		return ""
	}

	if _, err := os.Stat(paths.PrivateKey); err != nil {
		return ""
	}

	return paths.PrivateKey
}

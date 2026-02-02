package remote

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
)

// ConnectOptions contains options for connecting to a remote server.
type ConnectOptions struct {
	Token string // Invite token (contains server pubkey, endpoint, assigned IP)
}

// Connect sets up a connection to a remote vibespace server using an invite token.
// Returns the client's public key that needs to be added to the server.
// The server will assign an IP when the client is added.
func Connect(opts ConnectOptions) (clientPubKey string, err error) {
	// Check if already connected
	state, err := LoadRemoteState()
	if err != nil {
		return "", fmt.Errorf("failed to load remote state: %w", err)
	}
	if state.Connected {
		return "", fmt.Errorf("already connected to %s: %w", state.ServerHost, vserrors.ErrRemoteAlreadyConnected)
	}

	// Decode invite token
	invite, err := DecodeInviteToken(opts.Token)
	if err != nil {
		return "", fmt.Errorf("invalid invite token: %w", err)
	}

	// Install WireGuard if needed
	if !IsWireGuardInstalled() {
		slog.Info("WireGuard not installed, installing...")
		ctx := context.Background()
		if err := InstallWireGuard(ctx); err != nil {
			return "", fmt.Errorf("failed to install WireGuard: %w", err)
		}
		slog.Info("WireGuard installed successfully")
	}

	// Generate local key pair
	slog.Info("generating WireGuard key pair")
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return "", fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Save keypair and server info (IP will be set during activate)
	state.ServerHost = invite.Endpoint
	state.ServerEndpoint = invite.Endpoint
	state.ServerIP = invite.ServerIP
	state.PublicKey = keyPair.PublicKey
	state.ServerPublicKey = invite.ServerPublicKey

	// Store private key for later use during activate
	vsHome, err := getVibespaceHome()
	if err != nil {
		return "", err
	}
	privateKeyPath := filepath.Join(vsHome, "wg-client.key")
	if err := os.WriteFile(privateKeyPath, []byte(keyPair.PrivateKey), 0600); err != nil {
		return "", fmt.Errorf("failed to save private key: %w", err)
	}

	if err := state.Save(); err != nil {
		return "", fmt.Errorf("failed to save remote state: %w", err)
	}

	// Return the client public key - user needs to add this to the server
	return keyPair.PublicKey, nil
}

// Activate brings up the WireGuard tunnel and fetches kubeconfig.
// Call this after the server has added the client's public key.
// The assignedIP is the IP address assigned by the server (e.g., "10.100.0.2/32").
func Activate(assignedIP string) error {
	state, err := LoadRemoteState()
	if err != nil {
		return fmt.Errorf("failed to load remote state: %w", err)
	}

	if state.PublicKey == "" {
		return fmt.Errorf("no pending connection: run 'vibespace remote connect <token>' first")
	}

	if state.Connected {
		return fmt.Errorf("already connected")
	}

	// Set the assigned IP (strip /32 suffix if present for display, but keep full for config)
	state.LocalIP = assignedIP

	// Write WireGuard client config with the assigned IP
	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}
	privateKeyPath := filepath.Join(vsHome, "wg-client.key")
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	config := &ClientConfig{
		PrivateKey:      strings.TrimSpace(string(privateKey)),
		Address:         assignedIP,
		ServerPublicKey: state.ServerPublicKey,
		ServerEndpoint:  state.ServerEndpoint,
		ServerIP:        state.ServerIP,
	}

	tempPath, err := WriteClientConfig(config)
	if err != nil {
		return fmt.Errorf("failed to write WireGuard config: %w", err)
	}

	// Install config to /etc/wireguard (requires sudo)
	if err := InstallConfig(tempPath); err != nil {
		return fmt.Errorf("failed to install WireGuard config: %w", err)
	}

	// Save state with LocalIP before bringing up WireGuard (QuickUp reads it from disk)
	if err := state.Save(); err != nil {
		return fmt.Errorf("failed to save remote state: %w", err)
	}

	// Bring up WireGuard
	slog.Info("starting WireGuard tunnel")
	if err := QuickUp(); err != nil {
		return fmt.Errorf("failed to start WireGuard: %w", err)
	}

	// Wait for tunnel connectivity
	slog.Info("waiting for tunnel connectivity")
	if err := waitForConnectivity(state.ServerIP, 30*time.Second); err != nil {
		QuickDown()
		return fmt.Errorf("tunnel did not establish: %w", err)
	}

	// Fetch kubeconfig from management API
	slog.Info("fetching kubeconfig from server")
	kubeconfig, err := FetchKubeconfigFromServer(state.ServerIP)
	if err != nil {
		QuickDown()
		return fmt.Errorf("failed to fetch kubeconfig (is the client added on server?): %w", err)
	}

	// Save kubeconfig
	kubeconfigPath, err := GetRemoteKubeconfigPath()
	if err != nil {
		QuickDown()
		return err
	}
	if err := os.WriteFile(kubeconfigPath, kubeconfig, 0600); err != nil {
		QuickDown()
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	// Mark as connected
	state.Connected = true
	state.ConnectedAt = time.Now()

	if err := state.Save(); err != nil {
		QuickDown()
		return fmt.Errorf("failed to save remote state: %w", err)
	}

	slog.Info("connected to remote server", "localIP", state.LocalIP)
	return nil
}

// Disconnect disconnects from the remote server.
func Disconnect() error {
	state, err := LoadRemoteState()
	if err != nil {
		return fmt.Errorf("failed to load remote state: %w", err)
	}

	if !state.Connected && state.PublicKey == "" {
		return fmt.Errorf("not connected to any remote server: %w", vserrors.ErrRemoteNotConnected)
	}

	// Bring down WireGuard
	slog.Info("stopping WireGuard tunnel")
	if err := QuickDown(); err != nil {
		slog.Warn("failed to stop WireGuard", "error", err)
	}

	// Remove remote kubeconfig
	kubeconfigPath, _ := GetRemoteKubeconfigPath()
	if kubeconfigPath != "" {
		os.Remove(kubeconfigPath)
	}

	// Remove client private key
	vsHome, _ := getVibespaceHome()
	if vsHome != "" {
		os.Remove(filepath.Join(vsHome, "wg-client.key"))
	}

	// Clear remote state
	state.Connected = false
	state.ServerHost = ""
	state.ServerEndpoint = ""
	state.LocalIP = ""
	state.ServerIP = ""
	state.PublicKey = ""
	state.ServerPublicKey = ""
	state.ConnectedAt = time.Time{}

	if err := state.Save(); err != nil {
		return fmt.Errorf("failed to save remote state: %w", err)
	}

	slog.Info("disconnected from remote server")
	return nil
}

// GetStatus returns the current remote connection status.
func GetStatus() (*RemoteState, error) {
	return LoadRemoteState()
}

// waitForConnectivity pings the server until it responds or timeout.
func waitForConnectivity(serverIP string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	attempt := 0
	for time.Now().Before(deadline) {
		attempt++
		// Try to reach the management API
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://%s:%d/health", serverIP, DefaultManagementPort))
		if err == nil {
			resp.Body.Close()
			slog.Info("tunnel connectivity established", "attempts", attempt)
			return nil
		}
		slog.Debug("waiting for connectivity", "attempt", attempt, "error", err)
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for connectivity to %s", serverIP)
}

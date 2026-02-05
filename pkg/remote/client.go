package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
)

// ConnectOptions contains options for connecting to a remote server.
type ConnectOptions struct {
	Token    string // Invite token (contains server pubkey, endpoint, cert fingerprint)
	Hostname string // Client hostname (sent to server for identification)
}

// Connect performs one-shot registration and tunnel activation.
// It registers with the server via HTTPS, receives an IP assignment,
// configures WireGuard, brings up the tunnel, and fetches kubeconfig.
func Connect(opts ConnectOptions) error {
	// Check if already connected
	state, err := LoadRemoteState()
	if err != nil {
		return fmt.Errorf("failed to load remote state: %w", err)
	}
	if state.Connected {
		return fmt.Errorf("already connected to %s: %w", state.ServerHost, vserrors.ErrRemoteAlreadyConnected)
	}

	// Decode invite token
	invite, err := DecodeInviteToken(opts.Token)
	if err != nil {
		return fmt.Errorf("invalid invite token: %w", err)
	}

	// Install WireGuard if needed
	if !IsWireGuardInstalled() {
		slog.Info("WireGuard not installed, installing...")
		ctx := context.Background()
		if err := InstallWireGuard(ctx); err != nil {
			return fmt.Errorf("failed to install WireGuard: %w", err)
		}
		slog.Info("WireGuard installed successfully")
	}

	// Generate local key pair
	slog.Info("generating WireGuard key pair")
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Store private key
	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}
	privateKeyPath := filepath.Join(vsHome, "wg-client.key")
	if err := os.WriteFile(privateKeyPath, []byte(keyPair.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	// Determine hostname
	hostname := opts.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	// Register with server via HTTPS
	slog.Info("registering with server")
	regResp, err := registerWithServer(invite, keyPair.PublicKey, hostname)
	if err != nil {
		return fmt.Errorf("failed to register with server: %w", err)
	}

	// Save state
	state.ServerHost = invite.Endpoint
	state.ServerEndpoint = regResp.ServerEndpoint
	state.ServerIP = regResp.ServerIP
	state.PublicKey = keyPair.PublicKey
	state.ServerPublicKey = regResp.ServerPublicKey
	state.LocalIP = regResp.AssignedIP

	// Write WireGuard client config
	config := &ClientConfig{
		PrivateKey:      keyPair.PrivateKey,
		Address:         regResp.AssignedIP,
		ServerPublicKey: regResp.ServerPublicKey,
		ServerEndpoint:  regResp.ServerEndpoint,
		ServerIP:        regResp.ServerIP,
	}

	tempPath, err := WriteClientConfig(config)
	if err != nil {
		return fmt.Errorf("failed to write WireGuard config: %w", err)
	}

	// Install config to /etc/wireguard (requires sudo)
	if err := InstallConfig(tempPath); err != nil {
		return fmt.Errorf("failed to install WireGuard config: %w", err)
	}

	// Save state before QuickUp (macOS reads LocalIP from state on disk)
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
	if err := waitForConnectivity(regResp.ServerIP, 30*time.Second); err != nil {
		QuickDown()
		return fmt.Errorf("tunnel did not establish: %w", err)
	}

	// Fetch kubeconfig from management API
	slog.Info("fetching kubeconfig from server")
	kubeconfig, err := FetchKubeconfigFromServer(regResp.ServerIP)
	if err != nil {
		QuickDown()
		return fmt.Errorf("failed to fetch kubeconfig: %w", err)
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

// registerWithServer calls the server's HTTPS registration endpoint.
func registerWithServer(invite *InviteToken, publicKey, hostname string) (*RegisterResponse, error) {
	// Determine registration URL
	host := invite.Host
	if host == "" {
		// Fall back to extracting from endpoint
		if h, _, err := splitHostPort(invite.Endpoint); err == nil {
			host = h
		} else {
			host = invite.Endpoint
		}
	}

	regURL := fmt.Sprintf("https://%s:%d/register", host, DefaultRegistrationPort)

	// Build request
	reqBody := RegisterRequest{
		Token:     mustEncodeInviteToken(invite),
		PublicKey: publicKey,
		Hostname:  hostname,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTPS client with certificate pinning
	httpClient := &http.Client{Timeout: 15 * time.Second}
	if invite.CertFingerprint != "" {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: PinningTLSConfig(invite.CertFingerprint),
		}
	}

	resp, err := httpClient.Post(regURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("token rejected by server (expired or invalid)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registration failed with status %d", resp.StatusCode)
	}

	var regResp RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, fmt.Errorf("failed to decode registration response: %w", err)
	}

	return &regResp, nil
}

// Activate is deprecated. Use Connect() which handles registration and activation in one step.
func Activate(assignedIP string) error {
	return fmt.Errorf("activate is deprecated: use 'vibespace remote connect <token>' which handles registration and activation automatically")
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

// splitHostPort is a safe wrapper around net.SplitHostPort.
func splitHostPort(hostport string) (host, port string, err error) {
	return net.SplitHostPort(hostport)
}

// mustEncodeInviteToken re-encodes an invite token. If encoding fails, returns empty string.
func mustEncodeInviteToken(token *InviteToken) string {
	encoded, _ := EncodeInviteToken(token)
	return encoded
}

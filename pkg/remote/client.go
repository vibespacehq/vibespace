package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
)

// ConnectOptions contains options for connecting to a remote server.
type ConnectOptions struct {
	Host  string // SSH host (user@hostname or hostname)
	Token string // Registration token
	Name  string // Optional client name
}

// Connect connects to a remote vibespace server.
func Connect(opts ConnectOptions) error {
	// Check if already connected
	state, err := LoadRemoteState()
	if err != nil {
		return fmt.Errorf("failed to load remote state: %w", err)
	}
	if state.Connected {
		return fmt.Errorf("already connected to %s: %w", state.ServerHost, vserrors.ErrRemoteAlreadyConnected)
	}

	// Install WireGuard if needed
	if !IsWireGuardInstalled() {
		slog.Info("WireGuard not installed, downloading...")
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

	// Register with server via SSH
	slog.Info("registering with server via SSH", "host", opts.Host)
	regResp, err := RegisterViaSSH(opts.Host, keyPair.PublicKey, opts.Token, opts.Name)
	if err != nil {
		return fmt.Errorf("failed to register with server: %w", err)
	}

	// Extract hostname from SSH host for endpoint
	hostname := opts.Host
	if idx := strings.Index(hostname, "@"); idx >= 0 {
		hostname = hostname[idx+1:]
	}
	endpoint := fmt.Sprintf("%s:%d", hostname, DefaultWireGuardPort)

	// Write WireGuard client config
	slog.Info("writing WireGuard configuration")
	config := &ClientConfig{
		PrivateKey:      keyPair.PrivateKey,
		Address:         regResp.AssignedIP,
		ServerPublicKey: regResp.ServerPublicKey,
		ServerEndpoint:  endpoint,
		ServerIP:        regResp.ServerIP,
	}

	tempPath, err := WriteClientConfig(config)
	if err != nil {
		return fmt.Errorf("failed to write WireGuard config: %w", err)
	}

	// Install config (requires sudo)
	slog.Info("installing WireGuard configuration (requires sudo)")
	if err := InstallConfig(tempPath); err != nil {
		return fmt.Errorf("failed to install WireGuard config: %w", err)
	}

	// Bring up WireGuard
	slog.Info("starting WireGuard tunnel")
	if err := QuickUp(); err != nil {
		return fmt.Errorf("failed to start WireGuard: %w", err)
	}

	// Wait for tunnel to establish
	slog.Info("waiting for tunnel to establish")
	time.Sleep(2 * time.Second)

	// Fetch kubeconfig from management API
	slog.Info("fetching kubeconfig from server")
	kubeconfig, err := FetchKubeconfigFromServer(regResp.ServerIP)
	if err != nil {
		// Try to clean up
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

	// Update remote state
	state.Connected = true
	state.ServerHost = opts.Host
	state.ServerEndpoint = endpoint
	state.LocalIP = strings.TrimSuffix(regResp.AssignedIP, "/32")
	state.ServerIP = regResp.ServerIP
	state.ConnectedAt = time.Now()
	state.PublicKey = keyPair.PublicKey
	state.ServerPublicKey = regResp.ServerPublicKey

	if err := state.Save(); err != nil {
		QuickDown()
		return fmt.Errorf("failed to save remote state: %w", err)
	}

	slog.Info("connected to remote server", "host", opts.Host, "localIP", state.LocalIP)
	return nil
}

// Disconnect disconnects from the remote server.
func Disconnect() error {
	state, err := LoadRemoteState()
	if err != nil {
		return fmt.Errorf("failed to load remote state: %w", err)
	}

	if !state.Connected {
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

	// Clear remote state
	state.Connected = false
	state.ServerHost = ""
	state.ServerEndpoint = ""
	state.LocalIP = ""
	state.ServerIP = ""
	state.ConnectedAt = time.Time{}

	if err := state.Save(); err != nil {
		return fmt.Errorf("failed to save remote state: %w", err)
	}

	slog.Info("disconnected from remote server")
	return nil
}

// RegisterViaSSH registers with the server by calling _remote-register via SSH.
func RegisterViaSSH(host, publicKey, token, name string) (*RegistrationResponse, error) {
	// Build the command to run on the server
	args := []string{
		host,
		"vibespace", "_remote-register",
		"--pubkey", publicKey,
		"--token", token,
	}
	if name != "" {
		args = append(args, "--name", name)
	}

	cmd := exec.Command("ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Debug("executing SSH command", "host", host)
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("SSH registration failed: %s", strings.TrimSpace(errMsg))
	}

	// Parse JSON response
	var resp RegistrationResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse registration response: %w (output: %s)", err, stdout.String())
	}

	return &resp, nil
}

// GetStatus returns the current remote connection status.
func GetStatus() (*RemoteState, error) {
	return LoadRemoteState()
}

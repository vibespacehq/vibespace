package remote

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"strconv"

	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
)

// chownToRealUser changes file ownership to the real user when running under sudo.
func chownToRealUser(paths ...string) {
	uidStr := os.Getenv("SUDO_UID")
	gidStr := os.Getenv("SUDO_GID")
	if uidStr == "" || gidStr == "" {
		return
	}
	uid, err1 := strconv.Atoi(uidStr)
	gid, err2 := strconv.Atoi(gidStr)
	if err1 != nil || err2 != nil {
		return
	}
	for _, p := range paths {
		os.Chown(p, uid, gid)
	}
}

// mgmtHTTPClient returns an HTTP client for the management API with cert pinning.
// Loads the saved cert fingerprint from RemoteState and uses PinningTLSConfig.
// Falls back to InsecureSkipVerify if no fingerprint is available (e.g. during
// initial connectivity check before state is fully saved).
func mgmtHTTPClient(timeout time.Duration) *http.Client {
	var tlsCfg *tls.Config
	if state, err := LoadRemoteState(); err == nil && state.CertFingerprint != "" {
		tlsCfg = PinningTLSConfig(state.CertFingerprint)
	} else {
		tlsCfg = &tls.Config{InsecureSkipVerify: true}
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}
}

// cleanHostname returns a clean hostname suitable for client identification.
// On macOS, os.Hostname() often returns garbled names (e.g., "Yagizdagabaks-MacBook-Pro.local").
// This tries scutil first (macOS-specific), then falls back to os.Hostname() with domain stripping.
func cleanHostname() string {
	if runtime.GOOS == "darwin" {
		// Try scutil --get ComputerName (user-friendly name)
		if out, err := exec.Command("scutil", "--get", "ComputerName").Output(); err == nil {
			name := strings.TrimSpace(string(out))
			if name != "" {
				return name
			}
		}
		// Try scutil --get LocalHostName (Bonjour name without .local)
		if out, err := exec.Command("scutil", "--get", "LocalHostName").Output(); err == nil {
			name := strings.TrimSpace(string(out))
			if name != "" {
				return name
			}
		}
	}

	// Fallback: os.Hostname() with domain stripping
	if h, err := os.Hostname(); err == nil && h != "" {
		// Strip domain suffix (e.g., "foo.local" -> "foo")
		if idx := strings.IndexByte(h, '.'); idx > 0 {
			return h[:idx]
		}
		return h
	}

	// Last resort: use USER env var
	if user := os.Getenv("USER"); user != "" {
		return user + "-client"
	}
	return "client"
}

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
		hostname = cleanHostname()
	}

	// Register with server via HTTPS
	slog.Info("registering with server")
	regResp, err := registerWithServer(invite, keyPair.PublicKey, hostname)
	if err != nil {
		return fmt.Errorf("failed to register with server: %w", err)
	}

	// Populate state (but don't save yet - wait until tunnel is confirmed up)
	state.ServerHost = invite.Endpoint
	state.ServerEndpoint = regResp.ServerEndpoint
	state.ServerIP = regResp.ServerIP
	state.PublicKey = keyPair.PublicKey
	state.ServerPublicKey = regResp.ServerPublicKey
	state.LocalIP = regResp.AssignedIP
	state.CertFingerprint = invite.CertFingerprint

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

	// Bring up WireGuard - pass AssignedIP explicitly to avoid needing state on disk
	slog.Info("starting WireGuard tunnel")
	if err := QuickUp(regResp.AssignedIP); err != nil {
		return fmt.Errorf("failed to start WireGuard: %w", err)
	}

	// Wait for tunnel connectivity
	// Save cert fingerprint early so mgmtHTTPClient can use it for pinning
	state.Save() // best-effort, connectivity check can fall back to InsecureSkipVerify
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

	// Mark as connected and save final state
	state.Connected = true
	state.ConnectedAt = time.Now()

	if err := state.Save(); err != nil {
		QuickDown()
		return fmt.Errorf("failed to save remote state: %w", err)
	}

	// Fix file ownership when running under sudo so non-root commands can read them
	statePath, _ := getVibespaceHome()
	if statePath != "" {
		chownToRealUser(
			kubeconfigPath,
			filepath.Join(statePath, RemoteStateFile),
		)
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

// Disconnect disconnects from the remote server.
func Disconnect() error {
	state, err := LoadRemoteState()
	if err != nil {
		return fmt.Errorf("failed to load remote state: %w", err)
	}

	if !state.Connected && state.PublicKey == "" {
		return fmt.Errorf("not connected to any remote server: %w", vserrors.ErrRemoteNotConnected)
	}

	// Fire-and-forget: notify server we're disconnecting
	if state.Connected && state.ServerIP != "" && state.PublicKey != "" {
		go notifyServerDisconnect(state.ServerIP, state.PublicKey)
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
	state.CertFingerprint = ""
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
		// Try to reach the management API (TLS)
		client := mgmtHTTPClient(2 * time.Second)
		resp, err := client.Get(fmt.Sprintf("https://%s:%d/health", serverIP, DefaultManagementPort))
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

// notifyServerDisconnect sends a best-effort disconnect notification to the server.
func notifyServerDisconnect(serverIP, publicKey string) {
	client := mgmtHTTPClient(2 * time.Second)
	body, _ := json.Marshal(map[string]string{"public_key": publicKey})
	url := fmt.Sprintf("https://%s:%d/disconnect", serverIP, DefaultManagementPort)
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return // fire-and-forget
	}
	resp.Body.Close()
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

// ConnectionWatcher monitors the WireGuard tunnel and automatically reconnects if it drops.
type ConnectionWatcher struct {
	serverIP      string
	checkInterval time.Duration
	maxRetries    int
	onDisconnect  func()
	onReconnect   func()
	stopCh        chan struct{}
	stopped       chan struct{}
}

// NewConnectionWatcher creates a new connection watcher.
func NewConnectionWatcher(serverIP string) *ConnectionWatcher {
	return &ConnectionWatcher{
		serverIP:      serverIP,
		checkInterval: 15 * time.Second,
		maxRetries:    3,
		stopCh:        make(chan struct{}),
		stopped:       make(chan struct{}),
	}
}

// OnDisconnect sets a callback invoked when the tunnel goes down.
func (w *ConnectionWatcher) OnDisconnect(fn func()) {
	w.onDisconnect = fn
}

// OnReconnect sets a callback invoked when the tunnel is restored.
func (w *ConnectionWatcher) OnReconnect(fn func()) {
	w.onReconnect = fn
}

// Start begins monitoring the connection in a goroutine.
func (w *ConnectionWatcher) Start() {
	go w.run()
}

// Stop stops the connection watcher.
func (w *ConnectionWatcher) Stop() {
	close(w.stopCh)
	<-w.stopped
}

func (w *ConnectionWatcher) run() {
	defer close(w.stopped)

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	consecutiveFailures := 0

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if w.ping() {
				if consecutiveFailures > 0 {
					slog.Info("connection restored")
					consecutiveFailures = 0
					if w.onReconnect != nil {
						w.onReconnect()
					}
				}
				continue
			}

			consecutiveFailures++
			slog.Warn("health check failed", "failures", consecutiveFailures, "max", w.maxRetries)

			if consecutiveFailures >= w.maxRetries {
				slog.Info("attempting reconnect")
				if w.onDisconnect != nil {
					w.onDisconnect()
				}

				if err := w.reconnect(); err != nil {
					slog.Error("reconnect failed", "error", err)
				} else {
					slog.Info("reconnect successful")
					consecutiveFailures = 0
					if w.onReconnect != nil {
						w.onReconnect()
					}
				}
			}
		}
	}
}

func (w *ConnectionWatcher) ping() bool {
	client := mgmtHTTPClient(2 * time.Second)
	resp, err := client.Get(fmt.Sprintf("https://%s:%d/health", w.serverIP, DefaultManagementPort))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (w *ConnectionWatcher) reconnect() error {
	// Bring down the interface
	if err := QuickDown(); err != nil {
		slog.Warn("QuickDown failed during reconnect", "error", err)
	}

	// Wait briefly before bringing back up
	time.Sleep(2 * time.Second)

	// Bring it back up
	if err := QuickUp(); err != nil {
		return fmt.Errorf("QuickUp failed: %w", err)
	}

	// Wait for connectivity
	return waitForConnectivity(w.serverIP, 30*time.Second)
}

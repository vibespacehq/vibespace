package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Server represents the remote mode server.
type Server struct {
	state              *ServerState
	mgmtServer         *http.Server // Private management API (WireGuard IP only)
	registrationServer *http.Server // Public registration API (0.0.0.0)
}

// NewServer creates a new remote server instance.
func NewServer() (*Server, error) {
	state, err := LoadServerState()
	if err != nil {
		return nil, fmt.Errorf("failed to load server state: %w", err)
	}

	return &Server{
		state: state,
	}, nil
}

// GenerateInviteToken generates a signed invite token containing server connection info.
func (s *Server) GenerateInviteToken(publicEndpoint string, ttl time.Duration) (string, error) {
	// Ensure WireGuard is initialized so we have a public key
	if s.state.PublicKey == "" {
		if err := s.InitializeWireGuard(); err != nil {
			return "", fmt.Errorf("failed to initialize WireGuard: %w", err)
		}
	}

	signingPub, signingPriv, err := s.ensureSigningKey()
	if err != nil {
		return "", err
	}

	if ttl <= 0 {
		ttl = DefaultInviteTokenTTL
	}

	token := &InviteToken{
		ServerPublicKey:  s.state.PublicKey,
		Endpoint:         publicEndpoint,
		ServerIP:         serverWGIP(s.state.ServerIP),
		ExpiresAt:        time.Now().Add(ttl).Unix(),
		Nonce:            newTokenNonce(),
		SigningPublicKey: signingPub,
	}

	if err := SignInviteToken(token, signingPriv); err != nil {
		return "", err
	}

	return EncodeInviteToken(token)
}

// AddClient adds a new client to the WireGuard configuration.
// Returns the assigned IP for the client.
func (s *Server) AddClient(name, publicKey string) (string, error) {
	// Check if client already exists
	existing := s.state.FindClientByPublicKey(publicKey)
	if existing != nil {
		return existing.AssignedIP, nil
	}

	// Allocate IP
	assignedIP := s.state.AllocateClientIP()

	// Add to state
	s.state.AddClient(name, publicKey, assignedIP)
	if err := s.state.Save(); err != nil {
		return "", fmt.Errorf("failed to save state: %w", err)
	}

	// Update WireGuard config
	if err := s.WriteWireGuardConfig(); err != nil {
		return "", fmt.Errorf("failed to update WireGuard config: %w", err)
	}

	// Reload WireGuard
	if err := s.reloadWireGuard(); err != nil {
		slog.Warn("failed to reload WireGuard", "error", err)
	}

	return assignedIP, nil
}

// InitializeWireGuard initializes WireGuard server configuration.
func (s *Server) InitializeWireGuard() error {
	// Check if already initialized
	if s.state.PublicKey != "" {
		slog.Debug("WireGuard already initialized")
		return nil
	}

	// Generate key pair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	s.state.PublicKey = keyPair.PublicKey

	// Store private key securely
	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}

	privateKeyPath := filepath.Join(vsHome, "wg-server.key")
	if err := os.WriteFile(privateKeyPath, []byte(keyPair.PrivateKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}
	s.state.PrivateKeyPath = privateKeyPath

	if err := s.state.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// WriteWireGuardConfig writes the WireGuard server configuration.
func (s *Server) WriteWireGuardConfig() error {
	// Read private key
	privateKey, err := os.ReadFile(s.state.PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	// Build client configs
	var clients []ServerClientConfig
	for _, client := range s.state.Clients {
		clients = append(clients, ServerClientConfig{
			PublicKey:  client.PublicKey,
			AllowedIPs: client.AssignedIP,
		})
	}

	config := &ServerConfig{
		PrivateKey: strings.TrimSpace(string(privateKey)),
		Address:    s.state.ServerIP,
		ListenPort: s.state.ListenPort,
		Clients:    clients,
	}

	tempPath, err := WriteServerConfig(config)
	if err != nil {
		return err
	}

	// Install the config
	if err := InstallConfig(tempPath); err != nil {
		return err
	}

	return nil
}

// Start starts the WireGuard interface and management API.
func (s *Server) Start(ctx context.Context, foreground bool) error {
	// Initialize WireGuard if needed
	if err := s.InitializeWireGuard(); err != nil {
		return fmt.Errorf("failed to initialize WireGuard: %w", err)
	}

	// Write WireGuard config
	if err := s.WriteWireGuardConfig(); err != nil {
		return fmt.Errorf("failed to write WireGuard config: %w", err)
	}

	// Bring up WireGuard interface
	if !IsInterfaceUp() {
		if err := QuickUp(); err != nil {
			return fmt.Errorf("failed to start WireGuard: %w", err)
		}
	}

	s.state.Running = true
	if err := s.state.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Start private management API (WireGuard IP only)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/kubeconfig", s.handleKubeconfig)

	mgmtAddr := fmt.Sprintf("%s:%d", serverWGIP(s.state.ServerIP), DefaultManagementPort)
	s.mgmtServer = &http.Server{
		Addr:    mgmtAddr,
		Handler: mux,
	}

	slog.Info("starting management API", "addr", mgmtAddr)

	if foreground {
		// Run in foreground
		return s.mgmtServer.ListenAndServe()
	}

	// Run in background
	go func() {
		if err := s.mgmtServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("management API error", "error", err)
		}
	}()

	return nil
}

// Stop stops the server and WireGuard interface.
func (s *Server) Stop(ctx context.Context) error {
	// Stop HTTP server
	if s.mgmtServer != nil {
		if err := s.mgmtServer.Shutdown(ctx); err != nil {
			slog.Warn("failed to shutdown HTTP server", "error", err)
		}
	}

	// Bring down WireGuard
	if err := QuickDown(); err != nil {
		slog.Warn("failed to stop WireGuard", "error", err)
	}

	s.state.Running = false
	return s.state.Save()
}

// handleHealth handles GET /health requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleKubeconfig handles GET /kubeconfig requests.
func (s *Server) handleKubeconfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read local kubeconfig and modify server address
	vsHome, err := getVibespaceHome()
	if err != nil {
		http.Error(w, "failed to get vibespace home", http.StatusInternalServerError)
		return
	}

	kubeconfigPath := filepath.Join(vsHome, "kubeconfig")
	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		http.Error(w, "kubeconfig not found", http.StatusNotFound)
		return
	}

	// Replace the server address with the WireGuard IP
	// The kubeconfig typically has server: https://127.0.0.1:6443
	// We need to replace it with server: https://10.100.0.1:6443
	content := string(data)
	wgIP := serverWGIP(s.state.ServerIP)
	content = strings.ReplaceAll(content, "127.0.0.1", wgIP)
	content = strings.ReplaceAll(content, "localhost", wgIP)

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Write([]byte(content))
}

// reloadWireGuard reloads the WireGuard configuration.
func (s *Server) reloadWireGuard() error {
	if err := SyncWireGuardConfig(); err == nil {
		return nil
	}
	// Fallback to down+up if sync fails
	if err := QuickDown(); err != nil {
		return err
	}
	return QuickUp()
}

func (s *Server) ensureSigningKey() (string, []byte, error) {
	if s.state.SigningPublicKey != "" && s.state.SigningPrivateKeyPath != "" {
		key, err := os.ReadFile(s.state.SigningPrivateKeyPath)
		if err == nil && len(key) == ed25519.PrivateKeySize {
			return s.state.SigningPublicKey, key, nil
		}
	}

	vsHome, err := getVibespaceHome()
	if err != nil {
		return "", nil, err
	}

	pub, priv, err := GenerateSigningKey()
	if err != nil {
		return "", nil, err
	}

	privateKeyPath := filepath.Join(vsHome, "remote-signing.key")
	if err := os.WriteFile(privateKeyPath, priv, 0600); err != nil {
		return "", nil, fmt.Errorf("failed to write signing key: %w", err)
	}

	s.state.SigningPublicKey = pub
	s.state.SigningPrivateKeyPath = privateKeyPath
	if err := s.state.Save(); err != nil {
		return "", nil, fmt.Errorf("failed to save signing key: %w", err)
	}

	return pub, priv, nil
}

func newTokenNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func serverWGIP(serverIP string) string {
	if serverIP == "" {
		return DefaultServerIP
	}
	return strings.Split(serverIP, "/")[0]
}

// SpawnServe spawns the serve process in the background (daemonizes).
func SpawnServe() error {
	// Get paths
	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}

	// Check if already running
	if IsServeRunning() {
		return fmt.Errorf("serve is already running")
	}

	// Clean up stale files
	pidFile := filepath.Join(vsHome, "serve.pid")
	os.Remove(pidFile)

	// Get the path to the current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build command: vibespace serve --foreground
	cmd := exec.Command(executable, "serve", "--foreground")

	// Detach from parent (creates new session)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Redirect stdout/stderr to log file
	logFile := filepath.Join(vsHome, "serve.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil

	// Start the serve process
	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("failed to start serve: %w", err)
	}

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		slog.Warn("failed to write serve pid file", "error", err)
	}

	// Wait for serve to be ready (WireGuard interface up)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if IsInterfaceUp() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for serve to start")
}

// IsServeRunning checks if the serve process is running.
func IsServeRunning() bool {
	// Check if WireGuard interface is up
	return IsInterfaceUp()
}

// FetchKubeconfigFromServer fetches the kubeconfig from the server's management API.
func FetchKubeconfigFromServer(serverIP string) ([]byte, error) {
	url := fmt.Sprintf("http://%s:%d/kubeconfig", serverIP, DefaultManagementPort)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch kubeconfig: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch kubeconfig: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

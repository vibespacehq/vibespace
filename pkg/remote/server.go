package remote

import (
	"context"
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
	state            *ServerState
	mgmtServer       *http.Server // Private management API (WireGuard IP only)
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

// GenerateInviteToken generates an invite token containing server connection info.
// Each token has a pre-allocated IP for the client.
func (s *Server) GenerateInviteToken(publicEndpoint string) (string, error) {
	// Ensure WireGuard is initialized so we have a public key
	if s.state.PublicKey == "" {
		if err := s.InitializeWireGuard(); err != nil {
			return "", fmt.Errorf("failed to initialize WireGuard: %w", err)
		}
	}

	// Pre-allocate an IP for this client
	assignedIP := s.state.AllocateClientIP()
	if err := s.state.Save(); err != nil {
		return "", fmt.Errorf("failed to save state: %w", err)
	}

	token := &InviteToken{
		ServerPublicKey: s.state.PublicKey,
		Endpoint:        publicEndpoint,
		AssignedIP:      assignedIP,
		ServerIP:        strings.TrimSuffix(s.state.ServerIP, "/24"),
	}

	return EncodeInviteToken(token)
}

// AddClient adds a new client to the WireGuard configuration.
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

	tempPath, _ := WriteServerConfig(config)

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

	mgmtAddr := fmt.Sprintf("%s:%d", DefaultServerIP, DefaultManagementPort)
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
	content = strings.ReplaceAll(content, "127.0.0.1", DefaultServerIP)
	content = strings.ReplaceAll(content, "localhost", DefaultServerIP)

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Write([]byte(content))
}

// reloadWireGuard reloads the WireGuard configuration.
func (s *Server) reloadWireGuard() error {
	// The simplest way is to use wg syncconf, but that requires the interface to be up
	// For simplicity, we'll do down+up
	if err := QuickDown(); err != nil {
		return err
	}
	return QuickUp()
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

// StopServe stops the serve process.
func StopServe() error {
	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}

	// Bring down WireGuard
	if err := QuickDown(); err != nil {
		slog.Warn("failed to stop WireGuard", "error", err)
	}

	// Kill the serve process
	pidFile := filepath.Join(vsHome, "serve.pid")
	data, err := os.ReadFile(pidFile)
	if err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err == nil {
			if process, err := os.FindProcess(pid); err == nil {
				process.Signal(syscall.SIGTERM)
				time.Sleep(500 * time.Millisecond)
				process.Signal(syscall.SIGKILL)
			}
		}
	}

	os.Remove(pidFile)

	// Update state
	state, err := LoadServerState()
	if err == nil {
		state.Running = false
		state.Save()
	}

	return nil
}

// GetServerPublicKey returns the server's WireGuard public key.
func GetServerPublicKey() (string, error) {
	state, err := LoadServerState()
	if err != nil {
		return "", err
	}
	return state.PublicKey, nil
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

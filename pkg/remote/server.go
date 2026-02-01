package remote

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RegistrationRequest is the request body for client registration.
type RegistrationRequest struct {
	PublicKey string `json:"public_key"`
	Token     string `json:"token"`
	Name      string `json:"name,omitempty"` // Optional client name
}

// RegistrationResponse is the response for successful registration.
type RegistrationResponse struct {
	AssignedIP      string `json:"assigned_ip"`
	ServerPublicKey string `json:"server_public_key"`
	ServerEndpoint  string `json:"server_endpoint"`
	ServerIP        string `json:"server_ip"`
}

// Server represents the remote mode server.
type Server struct {
	state       *ServerState
	httpServer  *http.Server
	listenAddr  string
}

// NewServer creates a new remote server instance.
func NewServer() (*Server, error) {
	state, err := LoadServerState()
	if err != nil {
		return nil, fmt.Errorf("failed to load server state: %w", err)
	}

	return &Server{
		state:      state,
		listenAddr: fmt.Sprintf("%s:%d", DefaultServerIP, DefaultManagementPort),
	}, nil
}

// GenerateToken generates a new registration token with the given TTL.
func (s *Server) GenerateToken(ttl time.Duration) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := "vs-" + hex.EncodeToString(tokenBytes)

	// Add to state
	s.state.RegistrationTokens = append(s.state.RegistrationTokens, RegistrationToken{
		Token:     token,
		ExpiresAt: time.Now().Add(ttl),
		Used:      false,
	})

	if err := s.state.Save(); err != nil {
		return "", fmt.Errorf("failed to save state: %w", err)
	}

	return token, nil
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

	// Start management API
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/kubeconfig", s.handleKubeconfig)
	mux.HandleFunc("/register", s.handleRegister)

	s.httpServer = &http.Server{
		Addr:    s.listenAddr,
		Handler: mux,
	}

	slog.Info("starting management API", "addr", s.listenAddr)

	if foreground {
		// Run in foreground
		return s.httpServer.ListenAndServe()
	}

	// Run in background
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("management API error", "error", err)
		}
	}()

	return nil
}

// Stop stops the server and WireGuard interface.
func (s *Server) Stop(ctx context.Context) error {
	// Stop HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
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

// handleRegister handles POST /register requests.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate token
	tokenIdx := s.state.ValidateToken(req.Token)
	if tokenIdx < 0 {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Check if client already registered
	existing := s.state.FindClientByPublicKey(req.PublicKey)
	if existing != nil {
		// Return existing registration
		resp := RegistrationResponse{
			AssignedIP:      existing.AssignedIP,
			ServerPublicKey: s.state.PublicKey,
			ServerEndpoint:  s.getPublicEndpoint(r),
			ServerIP:        strings.TrimSuffix(s.state.ServerIP, "/24"),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Mark token as used
	s.state.MarkTokenUsed(tokenIdx)

	// Allocate IP for new client
	assignedIP := s.state.AllocateClientIP()

	// Determine client name
	clientName := req.Name
	if clientName == "" {
		clientName = fmt.Sprintf("client-%d", len(s.state.Clients)+1)
	}

	// Add client
	s.state.AddClient(clientName, req.PublicKey, assignedIP)

	// Save state and update WireGuard config
	if err := s.state.Save(); err != nil {
		http.Error(w, "failed to save state", http.StatusInternalServerError)
		return
	}

	if err := s.WriteWireGuardConfig(); err != nil {
		http.Error(w, "failed to update WireGuard config", http.StatusInternalServerError)
		return
	}

	// Reload WireGuard to pick up new peer
	if err := s.reloadWireGuard(); err != nil {
		slog.Warn("failed to reload WireGuard", "error", err)
	}

	resp := RegistrationResponse{
		AssignedIP:      assignedIP,
		ServerPublicKey: s.state.PublicKey,
		ServerEndpoint:  s.getPublicEndpoint(r),
		ServerIP:        strings.TrimSuffix(s.state.ServerIP, "/24"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getPublicEndpoint returns the public endpoint for clients to connect to.
func (s *Server) getPublicEndpoint(r *http.Request) string {
	// Try to get from X-Forwarded-Host or Host header
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	// Remove port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	return fmt.Sprintf("%s:%d", host, s.state.ListenPort)
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

// RegisterClient is the internal command handler for _remote-register.
// Called via SSH from the client.
func RegisterClient(pubKey, token, name string) (*RegistrationResponse, error) {
	server, err := NewServer()
	if err != nil {
		return nil, err
	}

	// Validate token
	tokenIdx := server.state.ValidateToken(token)
	if tokenIdx < 0 {
		return nil, fmt.Errorf("invalid or expired token")
	}

	// Check if client already registered
	existing := server.state.FindClientByPublicKey(pubKey)
	if existing != nil {
		return &RegistrationResponse{
			AssignedIP:      existing.AssignedIP,
			ServerPublicKey: server.state.PublicKey,
			ServerIP:        strings.TrimSuffix(server.state.ServerIP, "/24"),
		}, nil
	}

	// Mark token as used
	server.state.MarkTokenUsed(tokenIdx)

	// Allocate IP for new client
	assignedIP := server.state.AllocateClientIP()

	// Determine client name
	clientName := name
	if clientName == "" {
		clientName = fmt.Sprintf("client-%d", len(server.state.Clients)+1)
	}

	// Add client
	server.state.AddClient(clientName, pubKey, assignedIP)

	// Save state
	if err := server.state.Save(); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	// Update WireGuard config
	if err := server.WriteWireGuardConfig(); err != nil {
		return nil, fmt.Errorf("failed to update WireGuard config: %w", err)
	}

	// Reload WireGuard
	if err := server.reloadWireGuard(); err != nil {
		slog.Warn("failed to reload WireGuard", "error", err)
	}

	return &RegistrationResponse{
		AssignedIP:      assignedIP,
		ServerPublicKey: server.state.PublicKey,
		ServerIP:        strings.TrimSuffix(server.state.ServerIP, "/24"),
	}, nil
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

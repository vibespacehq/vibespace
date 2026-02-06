package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

// Server represents the remote mode server.
type Server struct {
	state              *ServerState
	mgmtServer         *http.Server // Private management API (WireGuard IP only)
	registrationServer *http.Server // Public registration API (0.0.0.0)
	kubeProxy          net.Listener
	certFingerprint    string // SHA256 fingerprint of registration TLS cert
	rateLimiters       sync.Map // per-IP *rate.Limiter for registration endpoint
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

	// Extract host (without port) for registration URL
	host := publicEndpoint
	if h, _, err := net.SplitHostPort(publicEndpoint); err == nil {
		host = h
	}

	token := &InviteToken{
		ServerPublicKey:  s.state.PublicKey,
		Endpoint:         publicEndpoint,
		ServerIP:         serverWGIP(s.state.ServerIP),
		ExpiresAt:        time.Now().Add(ttl).Unix(),
		Nonce:            newTokenNonce(),
		SigningPublicKey: signingPub,
		CertFingerprint:  s.certFingerprint,
		Host:             host,
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

// ListClients returns all registered clients.
func (s *Server) ListClients() []ClientRegistration {
	return s.state.Clients
}

// RemoveClient removes a client by name or public key.
func (s *Server) RemoveClient(nameOrKey string) error {
	idx := -1
	for i, c := range s.state.Clients {
		if c.Name == nameOrKey || c.PublicKey == nameOrKey || c.Hostname == nameOrKey {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("client %q not found", nameOrKey)
	}

	removed := s.state.Clients[idx]
	s.state.Clients = append(s.state.Clients[:idx], s.state.Clients[idx+1:]...)

	if err := s.state.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Rewrite WireGuard config
	if err := s.WriteWireGuardConfig(); err != nil {
		return fmt.Errorf("failed to update WireGuard config: %w", err)
	}

	// Reload WireGuard
	if err := s.reloadWireGuard(); err != nil {
		slog.Warn("failed to reload WireGuard after client removal", "error", err)
	}

	slog.Info("client removed", "name", removed.Name, "ip", removed.AssignedIP)
	return nil
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
	// Check firewall before starting
	for _, result := range CheckFirewall() {
		if !result.Status {
			slog.Warn("firewall check failed", "check", result.Check, "message", result.Message)
		}
	}

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
		if err := QuickUp(s.state.ServerIP); err != nil {
			return fmt.Errorf("failed to start WireGuard: %w", err)
		}
	}

	s.state.Running = true
	if err := s.state.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Ensure TLS cert exists (shared between registration and management APIs)
	mgmtCert, err := s.ensureRegistrationCert()
	if err != nil {
		return fmt.Errorf("failed to ensure TLS cert: %w", err)
	}

	// Start private management API (WireGuard IP only) with TLS
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/kubeconfig", s.handleKubeconfig)
	mux.HandleFunc("/disconnect", s.handleDisconnect)

	mgmtAddr := fmt.Sprintf("%s:%d", serverWGIP(s.state.ServerIP), DefaultManagementPort)
	s.mgmtServer = &http.Server{
		Addr:    mgmtAddr,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{mgmtCert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	slog.Info("starting management API", "addr", mgmtAddr, "tls", true)

	// Start public registration API (reuses the already-loaded cert)
	if err := s.startRegistrationAPI(); err != nil {
		slog.Warn("failed to start registration API", "error", err)
	}

	s.startKubeAPIProxy()

	if foreground {
		// Run in foreground with TLS
		return s.mgmtServer.ListenAndServeTLS("", "")
	}

	// Run in background with TLS
	go func() {
		if err := s.mgmtServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			slog.Error("management API error", "error", err)
		}
	}()

	return nil
}

// Stop stops the server and WireGuard interface.
func (s *Server) Stop(ctx context.Context) error {
	// Stop HTTP servers
	if s.mgmtServer != nil {
		if err := s.mgmtServer.Shutdown(ctx); err != nil {
			slog.Warn("failed to shutdown management server", "error", err)
		}
	}
	if s.registrationServer != nil {
		if err := s.registrationServer.Shutdown(ctx); err != nil {
			slog.Warn("failed to shutdown registration server", "error", err)
		}
	}

	s.stopKubeAPIProxy()

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

// handleDisconnect handles POST /disconnect - optional notification from client.
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PublicKey string `json:"public_key"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.PublicKey != "" {
		slog.Info("client disconnected", "public_key", req.PublicKey[:8]+"...")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
	return QuickUp(s.state.ServerIP)
}

// RegisterRequest is the JSON body for POST /register.
type RegisterRequest struct {
	Token     string `json:"token"`
	PublicKey string `json:"public_key"`
	Hostname  string `json:"hostname"`
}

// RegisterResponse is returned on successful registration.
type RegisterResponse struct {
	AssignedIP      string `json:"assigned_ip"`
	ServerPublicKey string `json:"server_public_key"`
	ServerEndpoint  string `json:"server_endpoint"`
	ServerIP        string `json:"server_ip"`
}

// ensureRegistrationCert generates and caches the self-signed TLS cert for the registration API.
func (s *Server) ensureRegistrationCert() (tls.Certificate, error) {
	vsHome, err := getVibespaceHome()
	if err != nil {
		return tls.Certificate{}, err
	}

	certPath := filepath.Join(vsHome, "reg-cert.pem")
	keyPath := filepath.Join(vsHome, "reg-key.pem")

	// Try to load existing cert
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		// Recompute fingerprint from the loaded cert
		if len(cert.Certificate) > 0 {
			s.certFingerprint = certSHA256(cert.Certificate[0])
		}
		return cert, nil
	}

	// Detect public IP for the cert SAN
	host, err := DetectPublicIP()
	if err != nil {
		host = "0.0.0.0"
	}

	certPEM, keyPEM, fingerprint, err := GenerateSelfSignedCert(host)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate registration cert: %w", err)
	}

	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to write key: %w", err)
	}

	s.certFingerprint = fingerprint

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to parse cert: %w", err)
	}
	return cert, nil
}

// startRegistrationAPI starts the public HTTPS registration endpoint on 0.0.0.0:7781.
func (s *Server) startRegistrationAPI() error {
	cert, err := s.ensureRegistrationCert()
	if err != nil {
		return fmt.Errorf("failed to ensure registration cert: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/register", s.handleRegister)

	addr := fmt.Sprintf("0.0.0.0:%d", DefaultRegistrationPort)
	s.registrationServer = &http.Server{
		Addr:    addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	slog.Info("starting registration API", "addr", addr, "fingerprint", s.certFingerprint)

	go func() {
		if err := s.registrationServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			slog.Error("registration API error", "error", err)
		}
	}()

	return nil
}

// getRateLimiter returns a per-IP rate limiter (5 req/min, burst 3).
func (s *Server) getRateLimiter(ip string) *rate.Limiter {
	if v, ok := s.rateLimiters.Load(ip); ok {
		return v.(*rate.Limiter)
	}
	limiter := rate.NewLimiter(rate.Every(12*time.Second), 3) // 5/min, burst 3
	s.rateLimiters.Store(ip, limiter)
	return limiter
}

// handleRegister handles POST /register - one-shot client registration.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Rate limit by IP
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	if !s.getRateLimiter(ip).Allow() {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	// Bound request body to 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate token
	invite, err := DecodeInviteToken(req.Token)
	if err != nil {
		slog.Warn("registration rejected: invalid token", "error", err)
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}
	_ = invite // Token is valid

	// Validate public key format (base64, should decode to 32 bytes)
	keyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(keyBytes) != 32 {
		http.Error(w, "invalid public key format", http.StatusBadRequest)
		return
	}

	// Determine client name
	name := req.Hostname
	if name == "" {
		name = "client"
	}

	// Check if already registered (idempotent)
	existing := s.state.FindClientByPublicKey(req.PublicKey)
	if existing != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RegisterResponse{
			AssignedIP:      existing.AssignedIP,
			ServerPublicKey: s.state.PublicKey,
			ServerEndpoint:  invite.Endpoint,
			ServerIP:        serverWGIP(s.state.ServerIP),
		})
		return
	}

	// Add client
	assignedIP, err := s.AddClient(name, req.PublicKey)
	if err != nil {
		slog.Error("registration failed", "error", err)
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	// Update hostname if provided
	if req.Hostname != "" {
		for i := range s.state.Clients {
			if s.state.Clients[i].PublicKey == req.PublicKey {
				s.state.Clients[i].Hostname = req.Hostname
				s.state.Save()
				break
			}
		}
	}

	slog.Info("client registered", "name", name, "ip", assignedIP)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RegisterResponse{
		AssignedIP:      assignedIP,
		ServerPublicKey: s.state.PublicKey,
		ServerEndpoint:  invite.Endpoint,
		ServerIP:        serverWGIP(s.state.ServerIP),
	})
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
	url := fmt.Sprintf("https://%s:%d/kubeconfig", serverIP, DefaultManagementPort)

	// Use cert pinning if fingerprint is available, skip-verify otherwise
	var tlsCfg *tls.Config
	if state, err := LoadRemoteState(); err == nil && state.CertFingerprint != "" {
		tlsCfg = PinningTLSConfig(state.CertFingerprint)
	} else {
		tlsCfg = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch kubeconfig: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch kubeconfig: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (s *Server) startKubeAPIProxy() {
	addr := fmt.Sprintf("%s:6443", serverWGIP(s.state.ServerIP))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Warn("failed to start kube API proxy", "addr", addr, "error", err)
		return
	}
	s.kubeProxy = ln

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				slog.Warn("kube API proxy accept error", "error", err)
				continue
			}
			go proxyToKubeAPI(conn)
		}
	}()
	slog.Info("kube API proxy listening", "addr", addr)
}

func (s *Server) stopKubeAPIProxy() {
	if s.kubeProxy != nil {
		s.kubeProxy.Close()
		s.kubeProxy = nil
	}
}

func certSHA256(der []byte) string {
	hash := sha256.Sum256(der)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func proxyToKubeAPI(client net.Conn) {
	defer client.Close()

	upstream, err := net.Dial("tcp", "127.0.0.1:6443")
	if err != nil {
		return
	}
	defer upstream.Close()

	go io.Copy(upstream, client)
	io.Copy(client, upstream)
}

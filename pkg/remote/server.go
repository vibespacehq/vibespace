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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

// Server represents the remote mode server.
type Server struct {
	mu                 sync.Mutex // Protects client registration (AddClient/RemoveClient)
	state              *ServerState
	mgmtServer         *http.Server // Private management API (WireGuard IP only)
	registrationServer *http.Server // Public registration API (0.0.0.0)
	kubeProxy          net.Listener
	kubeProxySem       *semaphore.Weighted // Limits concurrent kube API proxy connections
	certFingerprint    string              // SHA256 fingerprint of registration TLS cert
	rateLimiters       sync.Map            // per-IP *rate.Limiter for registration endpoint
}

// NewServer creates a new remote server instance.
func NewServer() (*Server, error) {
	state, err := LoadServerState()
	if err != nil {
		return nil, fmt.Errorf("failed to load server state: %w", err)
	}

	return &Server{
		state:        state,
		kubeProxySem: semaphore.NewWeighted(50),
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

	// Ensure registration cert exists so we can include its fingerprint in the token
	if s.certFingerprint == "" {
		if _, err := s.ensureRegistrationCert(); err != nil {
			return "", fmt.Errorf("failed to ensure registration cert: %w", err)
		}
	}

	if ttl <= 0 {
		ttl = DefaultInviteTokenTTL()
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

// CertFingerprint returns the SHA256 fingerprint of the registration TLS cert.
func (s *Server) CertFingerprint() string {
	return s.certFingerprint
}

// AddClient adds a new client to the WireGuard configuration.
// Returns the assigned IP for the client. Thread-safe via mutex.
func (s *Server) AddClient(name, publicKey, hostname string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if client already exists
	existing := s.state.FindClientByPublicKey(publicKey)
	if existing != nil {
		return existing.AssignedIP, nil
	}

	// Allocate IP
	assignedIP := s.state.AllocateClientIP()

	// Add to state (with hostname)
	s.state.AddClient(name, publicKey, assignedIP, hostname)
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

// RemoveClient removes a client by name, hostname, or public key (full or prefix >= 8 chars).
// Returns an error listing all candidates if the identifier is ambiguous.
func (s *Server) RemoveClient(nameOrKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var matches []int
	for i, c := range s.state.Clients {
		if c.Name == nameOrKey || c.PublicKey == nameOrKey || c.Hostname == nameOrKey {
			matches = append(matches, i)
			continue
		}
		// Match by public key prefix (>= 8 chars)
		if len(nameOrKey) >= 8 && strings.HasPrefix(c.PublicKey, nameOrKey) {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return fmt.Errorf("client %q not found", nameOrKey)
	}
	if len(matches) > 1 {
		var candidates []string
		for _, i := range matches {
			c := s.state.Clients[i]
			candidates = append(candidates, fmt.Sprintf("  - name=%q hostname=%q ip=%s key=%s...",
				c.Name, c.Hostname, c.AssignedIP, c.PublicKey[:8]))
		}
		return fmt.Errorf("ambiguous identifier %q matches %d clients:\n%s\nUse the full public key to disambiguate",
			nameOrKey, len(matches), strings.Join(candidates, "\n"))
	}

	idx := matches[0]
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
	// Check if already initialized AND key file still exists
	if s.state.PublicKey != "" && s.state.PrivateKeyPath != "" {
		if _, err := os.Stat(s.state.PrivateKeyPath); err == nil {
			slog.Debug("WireGuard already initialized")
			return nil
		}
		slog.Warn("WireGuard key file missing, reinitializing", "path", s.state.PrivateKeyPath)
		s.state.PublicKey = ""
		s.state.PrivateKeyPath = ""
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
// SetupWireGuard performs all sudo-requiring WireGuard setup: key init, config write,
// and interface bring-up. Call this from a process that has a terminal (before daemonizing).
func (s *Server) SetupWireGuard() error {
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
	if IsInterfaceUp() {
		// Verify running interface matches our keys — tear down if stale
		runningKey := InterfacePublicKey()
		if runningKey != "" && runningKey != s.state.PublicKey {
			slog.Warn("stale WireGuard interface detected, tearing down",
				"running_key", runningKey, "expected_key", s.state.PublicKey)
			if err := QuickDown(); err != nil {
				slog.Warn("failed to tear down stale interface", "error", err)
			}
		}
	}
	if !IsInterfaceUp() {
		if err := QuickUp(s.state.ServerIP); err != nil {
			return fmt.Errorf("failed to start WireGuard: %w", err)
		}
	}

	s.state.Running = true
	if err := s.state.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

func (s *Server) Start(ctx context.Context, foreground bool) error {
	// SetupWireGuard handles all sudo-requiring work (config, interface).
	// When daemonizing, SpawnServe calls it in the parent (which has a TTY).
	// In standalone foreground mode (user ran --foreground directly), do it here.
	// Skip if the interface is already up (parent already did the setup).
	if !IsInterfaceUp() {
		if err := s.SetupWireGuard(); err != nil {
			return err
		}
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

	mgmtAddr := fmt.Sprintf("%s:%d", serverWGIP(s.state.ServerIP), DefaultManagementPort())
	s.mgmtServer = &http.Server{
		Addr:              mgmtAddr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{mgmtCert},
			MinVersion:   tls.VersionTLS13,
		},
	}

	slog.Info("starting management API", "addr", mgmtAddr, "tls", true)

	// Start public registration API (reuses the already-loaded cert)
	if err := s.startRegistrationAPI(); err != nil {
		slog.Warn("failed to start registration API", "error", err)
	}

	s.startKubeAPIProxy()

	// Write PID file for liveness detection
	if vsHome, err := getVibespaceHome(); err == nil {
		pidFile := filepath.Join(vsHome, "serve.pid")
		os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)
	}

	if foreground {
		// Run mgmt API in background goroutine so we return to caller for banner/signal handling
		go func() {
			if err := s.mgmtServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				slog.Error("management API error", "error", err)
			}
		}()
		return nil
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

	// Clean up PID file
	if vsHome, err := getVibespaceHome(); err == nil {
		os.Remove(filepath.Join(vsHome, "serve.pid"))
	}

	s.state.Running = false
	return s.state.Save()
}

// peerFromRequest identifies the registered client making the request
// by matching the source IP against WireGuard-assigned IPs.
func (s *Server) peerFromRequest(r *http.Request) *ClientRegistration {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Clients {
		clientIP := strings.TrimSuffix(s.state.Clients[i].AssignedIP, "/32")
		if clientIP == ip {
			return &s.state.Clients[i]
		}
	}
	return nil
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

	if peer := s.peerFromRequest(r); peer == nil {
		http.Error(w, "unauthorized: unknown peer", http.StatusUnauthorized)
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

// handleDisconnect handles POST /disconnect - removes the calling peer.
// The caller's identity is derived from their WireGuard source IP,
// not from the request body, to prevent peers from disconnecting others.
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to prevent memory exhaustion (body is ignored but may be sent)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10) // 1 KB

	peer := s.peerFromRequest(r)
	if peer == nil {
		http.Error(w, "unauthorized: unknown peer", http.StatusUnauthorized)
		return
	}

	slog.Info("client disconnecting", "name", peer.Name, "ip", peer.AssignedIP)
	if err := s.RemoveClient(peer.PublicKey); err != nil {
		slog.Warn("failed to remove disconnecting client", "error", err)
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

// securityHeaders wraps an http.Handler with standard security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// startRegistrationAPI starts the public HTTPS registration endpoint on 0.0.0.0:7781.
func (s *Server) startRegistrationAPI() error {
	cert, err := s.ensureRegistrationCert()
	if err != nil {
		return fmt.Errorf("failed to ensure registration cert: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/register", s.handleRegister)

	addr := fmt.Sprintf("0.0.0.0:%d", DefaultRegistrationPort())
	s.registrationServer = &http.Server{
		Addr:              addr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
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

	// Reject replayed tokens — each nonce can only be used once
	s.mu.Lock()
	s.state.PruneExpiredNonces()
	alreadyUsed := s.state.CheckAndRecordNonce(invite.Nonce, invite.ExpiresAt)
	if alreadyUsed {
		s.mu.Unlock()
		slog.Warn("registration rejected: token already used", "nonce", invite.Nonce[:8]+"...")
		http.Error(w, "token already used", http.StatusConflict)
		return
	}
	if err := s.state.Save(); err != nil {
		s.mu.Unlock()
		slog.Error("failed to save nonce state", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.mu.Unlock()

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

	// AddClient is idempotent and mutex-protected (handles duplicate check atomically)
	assignedIP, err := s.AddClient(name, req.PublicKey, req.Hostname)
	if err != nil {
		slog.Error("registration failed", "error", err)
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
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
			// Verify stored public key matches the private key on disk
			derivedPub := ed25519.PrivateKey(key).Public().(ed25519.PublicKey)
			derivedPubStr := base64.RawURLEncoding.EncodeToString(derivedPub)
			if derivedPubStr == s.state.SigningPublicKey {
				return s.state.SigningPublicKey, key, nil
			}
			slog.Warn("signing key mismatch, regenerating", "stored", s.state.SigningPublicKey[:8]+"...", "derived", derivedPubStr[:8]+"...")
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
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func serverWGIP(serverIP string) string {
	if serverIP == "" {
		return DefaultServerIP()
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

	// Clean up stale interface/PID if process died
	CleanupStaleServe()

	// Do all sudo-requiring WireGuard setup here in the parent process,
	// which still has a terminal for password prompts. The daemon child
	// (Setsid: true) has no TTY and cannot prompt for sudo.
	server, err := NewServer()
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	if err := server.SetupWireGuard(); err != nil {
		return fmt.Errorf("failed to setup WireGuard: %w", err)
	}

	// Pre-generate the TLS cert before spawning the daemon to avoid a race
	// where both parent (token generation) and daemon (registration API)
	// call ensureRegistrationCert() concurrently and produce different certs.
	if _, err := server.ensureRegistrationCert(); err != nil {
		return fmt.Errorf("failed to ensure registration cert: %w", err)
	}

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
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0600); err != nil {
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

// IsServeRunning checks if the serve process is running by verifying PID file + process liveness.
// Falls back to WireGuard interface check if no PID file exists.
func IsServeRunning() bool {
	vsHome, err := getVibespaceHome()
	if err != nil {
		return false
	}

	pidFile := filepath.Join(vsHome, "serve.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		// No PID file — check interface as fallback for manually started servers
		return IsInterfaceUp()
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}

	// Check if process is alive (kill -0)
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process is dead — interface may be orphaned
		return false
	}

	return true
}

// KillServeProcess sends SIGTERM to the running serve daemon and waits for it to exit.
func KillServeProcess() error {
	vsHome, err := getVibespaceHome()
	if err != nil {
		return err
	}

	pidFile := filepath.Join(vsHome, "serve.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("no serve.pid file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid pid: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	// Wait briefly for process to exit
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			// Process exited
			os.Remove(pidFile)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Force kill if still alive
	proc.Signal(syscall.SIGKILL)
	os.Remove(pidFile)
	return nil
}

// CleanupStaleServe tears down an orphaned WireGuard interface when the serve
// process has died but the interface is still up.
func CleanupStaleServe() {
	vsHome, err := getVibespaceHome()
	if err != nil {
		return
	}

	pidFile := filepath.Join(vsHome, "serve.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return // no PID file
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		os.Remove(pidFile)
		return
	}

	// Check if process is alive
	proc, err := os.FindProcess(pid)
	if err == nil {
		if err := proc.Signal(syscall.Signal(0)); err == nil {
			return // process is alive, nothing to clean up
		}
	}

	// Process is dead but interface might be up — clean up
	if IsInterfaceUp() {
		slog.Info("cleaning up orphaned WireGuard interface from stale serve process")
		QuickDown()
	}
	os.Remove(pidFile)
}

// FetchKubeconfigFromServer fetches the kubeconfig from the server's management API.
// Requires a cert fingerprint for TLS pinning — never falls back to InsecureSkipVerify.
func FetchKubeconfigFromServer(serverIP, certFingerprint string) ([]byte, error) {
	url := fmt.Sprintf("https://%s:%d/kubeconfig", serverIP, DefaultManagementPort())

	if certFingerprint == "" {
		return nil, fmt.Errorf("cert fingerprint required to fetch kubeconfig")
	}
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: PinningTLSConfig(certFingerprint)},
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
			if !s.kubeProxySem.TryAcquire(1) {
				slog.Warn("kube API proxy connection limit reached, rejecting")
				conn.Close()
				continue
			}
			go func() {
				defer s.kubeProxySem.Release(1)
				proxyToKubeAPI(conn)
			}()
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

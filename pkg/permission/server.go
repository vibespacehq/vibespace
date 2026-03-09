package permission

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vibespacehq/vibespace/pkg/config"
)

// DefaultPermissionPort returns the default port for the permission server.
func DefaultPermissionPort() int { return config.Global().Ports.Permission }

// RequestTimeout is the maximum time to wait for a permission decision.
const RequestTimeout = 5 * time.Minute

// pendingRequest tracks a permission request waiting for a decision.
type pendingRequest struct {
	Request    *Request
	ResponseCh chan Decision
}

// Server is an HTTP server that receives permission requests from Claude hooks.
type Server struct {
	port        int
	authToken   string
	pending     map[string]*pendingRequest
	requestChan chan *Request
	mu          sync.Mutex
	server      *http.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewServer creates a new permission server with a random auth token.
func NewServer(port int) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	token := generateAuthToken()
	return &Server{
		port:        port,
		authToken:   token,
		pending:     make(map[string]*pendingRequest),
		requestChan: make(chan *Request, 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// generateAuthToken returns a random 32-byte hex token.
func generateAuthToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to uuid if crypto/rand fails (shouldn't happen)
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}

// Start starts the permission server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/permission", s.handlePermission)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:           s.requireAuth(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("permission server starting", "port", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait briefly to check for immediate startup errors
	select {
	case err := <-errCh:
		return fmt.Errorf("permission server failed to start: %w", err)
	case <-time.After(100 * time.Millisecond):
		slog.Info("permission server started", "port", s.port)
		return nil
	}
}

// Stop stops the permission server.
func (s *Server) Stop() {
	s.cancel()

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}

	// Deny all pending requests
	s.mu.Lock()
	for id, pending := range s.pending {
		slog.Debug("denying pending request on shutdown", "id", id)
		select {
		case pending.ResponseCh <- DecisionDeny:
		default:
		}
		delete(s.pending, id)
	}
	s.mu.Unlock()

	slog.Info("permission server stopped")
}

// RequestChan returns a channel that receives new permission requests.
func (s *Server) RequestChan() <-chan *Request {
	return s.requestChan
}

// Respond sends a decision for a pending permission request.
func (s *Server) Respond(id string, decision Decision) error {
	s.mu.Lock()
	pending, ok := s.pending[id]
	if ok {
		delete(s.pending, id)
	}
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("no pending request with ID %s", id)
	}

	slog.Debug("sending permission decision", "id", id, "decision", decision)
	select {
	case pending.ResponseCh <- decision:
		return nil
	default:
		return fmt.Errorf("failed to send decision for request %s", id)
	}
}

// handlePermission handles POST /permission requests from the hook script.
func (s *Server) handlePermission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	slog.Debug("received permission request", "body_len", len(body))

	// Parse the incoming request
	// PreToolUse sends tool_name and tool_input in snake_case
	var incoming struct {
		AgentKey  string          `json:"agent_key"`
		ToolName  string          `json:"tool_name"`
		ToolInput json.RawMessage `json:"tool_input"`
	}
	if err := json.Unmarshal(body, &incoming); err != nil {
		slog.Error("failed to parse request", "error", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Create the permission request
	req := &Request{
		ID:        uuid.New().String(),
		AgentKey:  incoming.AgentKey,
		ToolName:  incoming.ToolName,
		ToolInput: incoming.ToolInput,
		Timestamp: time.Now(),
	}

	// Create response channel and register pending request
	respCh := make(chan Decision, 1)
	s.mu.Lock()
	s.pending[req.ID] = &pendingRequest{
		Request:    req,
		ResponseCh: respCh,
	}
	s.mu.Unlock()

	slog.Debug("permission request queued", "id", req.ID, "agent", req.AgentKey, "tool", req.ToolName)

	// Notify TUI of new request
	select {
	case s.requestChan <- req:
	default:
		slog.Warn("request channel full, dropping permission request", "id", req.ID)
		s.mu.Lock()
		delete(s.pending, req.ID)
		s.mu.Unlock()
		s.writeResponse(w, DecisionDeny)
		return
	}

	// Wait for decision with timeout
	select {
	case decision := <-respCh:
		slog.Debug("permission decision received", "id", req.ID, "decision", decision)
		s.writeResponse(w, decision)

	case <-time.After(RequestTimeout):
		slog.Warn("permission request timed out", "id", req.ID)
		s.mu.Lock()
		delete(s.pending, req.ID)
		s.mu.Unlock()
		s.writeResponse(w, DecisionDeny)

	case <-s.ctx.Done():
		slog.Debug("permission server shutting down", "id", req.ID)
		s.writeResponse(w, DecisionDeny)
	}
}

// writeResponse writes the permission decision response.
func (s *Server) writeResponse(w http.ResponseWriter, decision Decision) {
	reason := "Controlled by vibespace"
	if decision == DecisionAllow {
		reason = "Approved by user in vibespace TUI"
	} else if decision == DecisionDeny {
		reason = "Denied by user in vibespace TUI"
	}

	resp := Response{
		Decision: decision,
		Reason:   reason,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}

// handleHealth handles GET /health requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// requireAuth wraps an http.Handler to validate the Bearer token.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth || subtle.ConstantTimeCompare([]byte(token), []byte(s.authToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthToken returns the server's auth token for passing to hooks.
func (s *Server) AuthToken() string {
	return s.authToken
}

// Port returns the server's port.
func (s *Server) Port() int {
	return s.port
}

// PendingCount returns the number of pending permission requests.
func (s *Server) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

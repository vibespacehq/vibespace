package permission

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	s := NewServer(0)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.Port() != 0 {
		t.Errorf("Port() = %d, want 0", s.Port())
	}
	if s.PendingCount() != 0 {
		t.Errorf("PendingCount() = %d, want 0", s.PendingCount())
	}
}

func TestHandleHealth(t *testing.T) {
	s := NewServer(0)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", w.Body.String(), "ok")
	}
}

func TestHandleHealthWrongMethod(t *testing.T) {
	s := NewServer(0)
	// Health endpoint doesn't check method, so POST should also work
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandlePermission(t *testing.T) {
	s := NewServer(0)

	// Start a goroutine to consume the request and respond
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case req := <-s.RequestChan():
			if req.ToolName != "Bash" {
				t.Errorf("ToolName = %q, want %q", req.ToolName, "Bash")
			}
			if req.AgentKey != "claude-1@project" {
				t.Errorf("AgentKey = %q, want %q", req.AgentKey, "claude-1@project")
			}
			s.Respond(req.ID, DecisionAllow)
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for request")
		}
	}()

	body, _ := json.Marshal(map[string]interface{}{
		"agent_key":  "claude-1@project",
		"tool_name":  "Bash",
		"tool_input": json.RawMessage(`{"command":"ls"}`),
	})

	req := httptest.NewRequest(http.MethodPost, "/permission", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handlePermission(w, req)

	<-done

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Decision != DecisionAllow {
		t.Errorf("Decision = %q, want %q", resp.Decision, DecisionAllow)
	}
}

func TestHandlePermissionInvalidJSON(t *testing.T) {
	s := NewServer(0)

	req := httptest.NewRequest(http.MethodPost, "/permission", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	s.handlePermission(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlePermissionWrongMethod(t *testing.T) {
	s := NewServer(0)

	req := httptest.NewRequest(http.MethodGet, "/permission", nil)
	w := httptest.NewRecorder()
	s.handlePermission(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestRespondUnknownID(t *testing.T) {
	s := NewServer(0)

	err := s.Respond("nonexistent", DecisionAllow)
	if err == nil {
		t.Error("expected error for unknown ID")
	}
}

func TestPendingCount(t *testing.T) {
	s := NewServer(0)

	if s.PendingCount() != 0 {
		t.Errorf("initial PendingCount() = %d, want 0", s.PendingCount())
	}
}

func TestStopDeniesAllPending(t *testing.T) {
	s := NewServer(0)

	// Manually add a pending request
	respCh := make(chan Decision, 1)
	s.mu.Lock()
	s.pending["test-id"] = &pendingRequest{
		Request:    &Request{ID: "test-id"},
		ResponseCh: respCh,
	}
	s.mu.Unlock()

	if s.PendingCount() != 1 {
		t.Errorf("PendingCount() = %d, want 1", s.PendingCount())
	}

	s.Stop()

	select {
	case d := <-respCh:
		if d != DecisionDeny {
			t.Errorf("decision = %q, want %q", d, DecisionDeny)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for deny on pending request")
	}

	if s.PendingCount() != 0 {
		t.Errorf("after Stop, PendingCount() = %d, want 0", s.PendingCount())
	}
}

func TestStartStop(t *testing.T) {
	// Use port 0 to let OS pick an ephemeral port
	s := NewServer(0)

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	s.Stop()
}

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestSessionManager(t *testing.T) *AgentSessionManager {
	t.Helper()
	tmpDir := t.TempDir()
	return &AgentSessionManager{
		sessions: make(map[string]map[string]*AgentSessions),
		filePath: filepath.Join(tmpDir, "test_sessions.json"),
	}
}

func TestMatchesShortID(t *testing.T) {
	fullID := "12345678-abcd-efgh-ijkl-mnopqrstuvwx"

	// Full match
	if !matchesShortID(fullID, fullID) {
		t.Error("full match should return true")
	}

	// 8-char prefix
	if !matchesShortID(fullID, "12345678") {
		t.Error("8-char prefix should match")
	}

	// Too short (< 8)
	if matchesShortID(fullID, "1234") {
		t.Error("4-char prefix should not match (too short)")
	}

	// Mismatch
	if matchesShortID(fullID, "99999999") {
		t.Error("different prefix should not match")
	}
}

func TestFormatSessionList(t *testing.T) {
	// Empty
	result := FormatSessionList(nil, "")
	if result != "No sessions found" {
		t.Fatalf("expected 'No sessions found', got %q", result)
	}

	// With sessions and current marker
	sessions := []string{
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"11111111-2222-3333-4444-555555555555",
	}
	result = FormatSessionList(sessions, sessions[0])
	if !strings.Contains(result, "→") {
		t.Error("expected current marker '→' for current session")
	}
	if !strings.Contains(result, "aaaaaaaa-bbbb") {
		t.Error("expected truncated session ID")
	}
}

func TestGetOrCreateSession(t *testing.T) {
	mgr := newTestSessionManager(t)

	// First call creates
	id1 := mgr.GetOrCreateSession("multi-1", "agent@vs")
	if id1 == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Second call returns existing
	id2 := mgr.GetOrCreateSession("multi-1", "agent@vs")
	if id2 != id1 {
		t.Fatalf("expected same ID %q, got %q", id1, id2)
	}

	// Different multi-session creates new
	id3 := mgr.GetOrCreateSession("multi-2", "agent@vs")
	if id3 == id1 {
		t.Fatal("expected different ID for different multi-session")
	}
}

func TestGetSession(t *testing.T) {
	mgr := newTestSessionManager(t)

	// Missing returns empty
	if got := mgr.GetSession("multi-1", "agent@vs"); got != "" {
		t.Fatalf("expected empty for missing, got %q", got)
	}

	// After create, returns ID
	created := mgr.GetOrCreateSession("multi-1", "agent@vs")
	if got := mgr.GetSession("multi-1", "agent@vs"); got != created {
		t.Fatalf("expected %q, got %q", created, got)
	}
}

func TestNewSession(t *testing.T) {
	mgr := newTestSessionManager(t)

	id1 := mgr.NewSession("multi-1", "agent@vs")
	if id1 == "" {
		t.Fatal("expected non-empty")
	}

	id2 := mgr.NewSession("multi-1", "agent@vs")
	if id2 == "" {
		t.Fatal("expected non-empty")
	}
	if id2 == id1 {
		t.Fatal("new session should have different ID")
	}

	// History should have both, newest first
	history := mgr.ListSessions("multi-1", "agent@vs")
	if len(history) != 2 {
		t.Fatalf("expected 2 in history, got %d", len(history))
	}
	if history[0] != id2 {
		t.Fatalf("newest session should be first, got %q", history[0])
	}
}

func TestResumeSession(t *testing.T) {
	mgr := newTestSessionManager(t)

	id1 := mgr.NewSession("multi-1", "agent@vs")
	id2 := mgr.NewSession("multi-1", "agent@vs")

	// Current should be id2
	if got := mgr.GetCurrentSessionID("multi-1", "agent@vs"); got != id2 {
		t.Fatalf("expected current=%q, got %q", id2, got)
	}

	// Resume by full ID
	if err := mgr.ResumeSession("multi-1", "agent@vs", id1); err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	if got := mgr.GetCurrentSessionID("multi-1", "agent@vs"); got != id1 {
		t.Fatalf("expected current=%q after resume, got %q", id1, got)
	}

	// Resume by short prefix
	if err := mgr.ResumeSession("multi-1", "agent@vs", id2[:8]); err != nil {
		t.Fatalf("ResumeSession by short ID: %v", err)
	}

	// Not found
	if err := mgr.ResumeSession("multi-1", "agent@vs", "nonexistent-id-00"); err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestResumeSessionNoMultiSession(t *testing.T) {
	mgr := newTestSessionManager(t)
	err := mgr.ResumeSession("nonexistent", "agent@vs", "some-id")
	if err == nil {
		t.Fatal("expected error for non-existent multi-session")
	}
}

func TestResumeSessionNoAgent(t *testing.T) {
	mgr := newTestSessionManager(t)
	mgr.NewSession("multi-1", "agent@vs")
	err := mgr.ResumeSession("multi-1", "other@vs", "some-id")
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
}

func TestListSessions(t *testing.T) {
	mgr := newTestSessionManager(t)

	// Empty returns nil
	if got := mgr.ListSessions("multi-1", "agent@vs"); got != nil {
		t.Fatalf("expected nil for empty, got %v", got)
	}

	mgr.NewSession("multi-1", "agent@vs")
	mgr.NewSession("multi-1", "agent@vs")

	list := mgr.ListSessions("multi-1", "agent@vs")
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}

	// Should be a copy
	list[0] = "mutated"
	original := mgr.ListSessions("multi-1", "agent@vs")
	if original[0] == "mutated" {
		t.Fatal("ListSessions should return a copy")
	}
}

func TestUpdateSessionID(t *testing.T) {
	mgr := newTestSessionManager(t)

	oldID := mgr.GetOrCreateSession("multi-1", "agent@vs")
	mgr.UpdateSessionID("multi-1", "agent@vs", "new-id-123")

	if got := mgr.GetCurrentSessionID("multi-1", "agent@vs"); got != "new-id-123" {
		t.Fatalf("expected 'new-id-123', got %q", got)
	}

	// Old ID should be replaced in history
	history := mgr.ListSessions("multi-1", "agent@vs")
	for _, id := range history {
		if id == oldID {
			t.Fatal("old ID should be replaced in history")
		}
	}
}

func TestUpdateSessionIDNewAgent(t *testing.T) {
	mgr := newTestSessionManager(t)
	mgr.UpdateSessionID("multi-1", "new-agent@vs", "new-id-456")
	if got := mgr.GetCurrentSessionID("multi-1", "new-agent@vs"); got != "new-id-456" {
		t.Fatalf("expected 'new-id-456', got %q", got)
	}
}

func TestSessionManagerPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sessions.json")

	// Create manager and add data
	mgr1 := &AgentSessionManager{
		sessions: make(map[string]map[string]*AgentSessions),
		filePath: filePath,
	}
	id := mgr1.GetOrCreateSession("multi-1", "agent@vs")

	// Verify file was written
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected persistence file to exist")
	}

	// Create new manager from same file
	mgr2 := &AgentSessionManager{
		sessions: make(map[string]map[string]*AgentSessions),
		filePath: filePath,
	}
	mgr2.load()

	got := mgr2.GetSession("multi-1", "agent@vs")
	if got != id {
		t.Fatalf("expected persisted ID %q, got %q", id, got)
	}
}

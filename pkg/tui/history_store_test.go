package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *HistoryStore {
	t.Helper()
	return &HistoryStore{dir: t.TempDir()}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello_world"},
		{"path/to/file", "path_to_file"},
		{"test:file", "test_file"},
		{"a*b?c", "a_b_c"},
		{`a"b<c>d|e`, "a_b_c_d_e"},
		{"simple", "simple"},
		{"", "default"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHistoryStoreAppendAndLoad(t *testing.T) {
	store := newTestStore(t)

	msg1 := NewUserMessage("all", "hello")
	msg2 := NewAssistantMessage("agent@vs", "world")
	msg3 := NewSystemMessage("info")

	for _, msg := range []*Message{msg1, msg2, msg3} {
		if err := store.Append("test-session", msg); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	msgs, err := store.Load("test-session")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Fatalf("expected first msg content 'hello', got %q", msgs[0].Content)
	}
	if msgs[1].Type != MessageTypeAssistant {
		t.Fatalf("expected second msg type Assistant, got %v", msgs[1].Type)
	}
	if msgs[2].Sender != "system" {
		t.Fatalf("expected third msg sender 'system', got %q", msgs[2].Sender)
	}
}

func TestHistoryStoreLoadNonExistent(t *testing.T) {
	store := newTestStore(t)
	msgs, err := store.Load("nonexistent")
	if err != nil {
		t.Fatalf("Load non-existent should not error: %v", err)
	}
	if msgs != nil {
		t.Fatalf("expected nil for non-existent, got %v", msgs)
	}
}

func TestHistoryStoreClear(t *testing.T) {
	store := newTestStore(t)
	store.Append("test", NewSystemMessage("hello"))

	if err := store.Clear("test"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	msgs, _ := store.Load("test")
	if len(msgs) != 0 {
		t.Fatalf("expected 0 after clear, got %d", len(msgs))
	}
}

func TestHistoryStoreClearNonExistent(t *testing.T) {
	store := newTestStore(t)
	// Should not error
	if err := store.Clear("nonexistent"); err != nil {
		t.Fatalf("Clear non-existent should not error: %v", err)
	}
}

func TestHistoryStoreList(t *testing.T) {
	store := newTestStore(t)

	store.Append("session-a", NewSystemMessage("a"))
	store.Append("session-b", NewSystemMessage("b"))

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	found := map[string]bool{}
	for _, s := range sessions {
		found[s] = true
	}
	if !found["session-a"] || !found["session-b"] {
		t.Fatalf("expected session-a and session-b, got %v", sessions)
	}
}

func TestHistoryStoreListEmpty(t *testing.T) {
	store := newTestStore(t)
	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestHistoryStoreLoadTail(t *testing.T) {
	store := newTestStore(t)
	for i := 0; i < 10; i++ {
		store.Append("test", NewSystemMessage("msg"))
	}

	msgs, err := store.LoadTail("test", 3)
	if err != nil {
		t.Fatalf("LoadTail: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3, got %d", len(msgs))
	}

	// Tail should return fewer if fewer exist
	msgs, err = store.LoadTail("test", 20)
	if err != nil {
		t.Fatalf("LoadTail: %v", err)
	}
	if len(msgs) != 10 {
		t.Fatalf("expected 10 (all), got %d", len(msgs))
	}
}

func TestHistoryStoreExists(t *testing.T) {
	store := newTestStore(t)

	if store.Exists("test") {
		t.Fatal("should not exist before append")
	}

	store.Append("test", NewSystemMessage("hello"))
	if !store.Exists("test") {
		t.Fatal("should exist after append")
	}

	store.Clear("test")
	if store.Exists("test") {
		t.Fatal("should not exist after clear")
	}
}

func TestHistoryStoreTruncate(t *testing.T) {
	store := newTestStore(t)
	for i := 0; i < 10; i++ {
		store.Append("test", NewSystemMessage("msg"))
	}

	if err := store.Truncate("test", 3); err != nil {
		t.Fatalf("Truncate: %v", err)
	}

	msgs, _ := store.Load("test")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 after truncate, got %d", len(msgs))
	}
}

func TestHistoryStoreTruncateNoOp(t *testing.T) {
	store := newTestStore(t)
	for i := 0; i < 3; i++ {
		store.Append("test", NewSystemMessage("msg"))
	}

	// Truncate to more than we have — no-op
	if err := store.Truncate("test", 10); err != nil {
		t.Fatalf("Truncate: %v", err)
	}
	msgs, _ := store.Load("test")
	if len(msgs) != 3 {
		t.Fatalf("expected 3, got %d", len(msgs))
	}
}

func TestHistoryStoreSkipsMalformedLines(t *testing.T) {
	store := newTestStore(t)

	// Append a valid message
	store.Append("test", NewSystemMessage("valid"))

	// Inject a malformed line directly
	path := filepath.Join(store.dir, "test.jsonl")
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	f.Write([]byte("this is not json\n"))
	f.Close()

	// Append another valid message
	store.Append("test", NewSystemMessage("also valid"))

	msgs, err := store.Load("test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 valid messages (skipping malformed), got %d", len(msgs))
	}
}

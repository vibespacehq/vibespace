package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newEdgeTestStore creates a Store backed by a temp directory for edge tests.
func newEdgeTestStore(t *testing.T) *Store {
	t.Helper()
	return &Store{dir: t.TempDir()}
}

func TestStoreListEmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: filepath.Join(dir, "sessions")}
	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List on empty dir: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestStoreSaveAndGet(t *testing.T) {
	store := newEdgeTestStore(t)
	sess := &Session{
		Name:      "test-session",
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Vibespaces: []VibespaceEntry{
			{Name: "project1", Agents: []string{"claude-1"}},
		},
	}
	if err := store.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Get("test-session")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "test-session" {
		t.Errorf("Name = %q, want %q", got.Name, "test-session")
	}
	if len(got.Vibespaces) != 1 {
		t.Errorf("Vibespaces count = %d, want 1", len(got.Vibespaces))
	}
}

func TestStoreGetNonexistent(t *testing.T) {
	store := newEdgeTestStore(t)
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestStoreDeleteNonexistent(t *testing.T) {
	store := newEdgeTestStore(t)
	err := store.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for deleting nonexistent session")
	}
}

func TestStoreListMultiple(t *testing.T) {
	store := newEdgeTestStore(t)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		err := store.Save(&Session{
			Name:       name,
			CreatedAt:  time.Now(),
			LastUsed:   time.Now(),
			Vibespaces: []VibespaceEntry{},
		})
		if err != nil {
			t.Fatalf("Save(%s): %v", name, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestStoreCorruptFile(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}
	// Write corrupt JSON
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("not json{"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := store.Get("corrupt")
	if err == nil {
		t.Error("expected error for corrupt session file")
	}
}

func TestStoreExists(t *testing.T) {
	store := newEdgeTestStore(t)
	if store.Exists("nonexistent") {
		t.Error("Exists should return false for nonexistent")
	}
	err := store.Save(&Session{
		Name:       "exists",
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		Vibespaces: []VibespaceEntry{},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !store.Exists("exists") {
		t.Error("Exists should return true after Save")
	}
}

func TestStoreCreateNewSession(t *testing.T) {
	store := newEdgeTestStore(t)
	sess, err := store.Create("new-session")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.Name != "new-session" {
		t.Errorf("Name = %q, want %q", sess.Name, "new-session")
	}
	if sess.Layout.Mode != LayoutModeSplit {
		t.Errorf("Layout.Mode = %q, want %q", sess.Layout.Mode, LayoutModeSplit)
	}
	if !store.Exists("new-session") {
		t.Error("session should exist after Create")
	}
}

func TestStoreCreateDuplicate(t *testing.T) {
	store := newEdgeTestStore(t)
	_, err := store.Create("dup")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err = store.Create("dup")
	if err == nil {
		t.Error("expected error for duplicate Create")
	}
}

func TestValidateSessionNameValid(t *testing.T) {
	valid := []string{"alpha", "my-session", "test_123", "a", "A1-b2_c3"}
	for _, name := range valid {
		if err := ValidateSessionName(name); err != nil {
			t.Errorf("ValidateSessionName(%q) = %v, expected nil", name, err)
		}
	}
}

func TestValidateSessionNameInvalid(t *testing.T) {
	invalid := []string{"", "-starts-with-dash", "_starts-with-underscore", "has spaces", "has.dots", "has/slashes"}
	for _, name := range invalid {
		if err := ValidateSessionName(name); err == nil {
			t.Errorf("ValidateSessionName(%q) = nil, expected error", name)
		}
	}
}

func TestStoreSaveInvalidName(t *testing.T) {
	store := newEdgeTestStore(t)
	err := store.Save(&Session{
		Name:       "-invalid",
		CreatedAt:  time.Now(),
		Vibespaces: []VibespaceEntry{},
	})
	if err == nil {
		t.Error("Save with invalid name should return error")
	}
}

func TestStoreListIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}

	// Write a valid session
	store.Save(&Session{
		Name:       "valid",
		CreatedAt:  time.Now(),
		Vibespaces: []VibespaceEntry{},
	})

	// Write a non-JSON file that should be ignored
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a session"), 0600)

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session (non-JSON ignored), got %d", len(sessions))
	}
}

func TestStoreListSkipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}

	// Write a valid session
	store.Save(&Session{
		Name:       "good",
		CreatedAt:  time.Now(),
		Vibespaces: []VibespaceEntry{},
	})

	// Write a corrupt JSON file
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid"), 0600)

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session (corrupt skipped), got %d", len(sessions))
	}
	if sessions[0].Name != "good" {
		t.Errorf("session name = %q, want %q", sessions[0].Name, "good")
	}
}

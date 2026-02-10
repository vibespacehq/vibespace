package session

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return &Store{dir: t.TempDir()}
}

func TestSessionSaveLoad(t *testing.T) {
	store := newTestStore(t)

	session := &Session{
		Name:      "test-session",
		CreatedAt: time.Now(),
		Vibespaces: []VibespaceEntry{
			{Name: "project1", Agents: []string{"claude-1"}},
		},
		Layout: Layout{Mode: LayoutModeSplit},
	}

	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := store.Get("test-session")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	if loaded.Name != session.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, session.Name)
	}
	if len(loaded.Vibespaces) != 1 {
		t.Fatalf("Vibespaces length = %d, want 1", len(loaded.Vibespaces))
	}
	if loaded.Vibespaces[0].Name != "project1" {
		t.Errorf("Vibespace Name = %q, want %q", loaded.Vibespaces[0].Name, "project1")
	}
	if loaded.Layout.Mode != LayoutModeSplit {
		t.Errorf("Layout.Mode = %q, want %q", loaded.Layout.Mode, LayoutModeSplit)
	}
}

func TestSessionList(t *testing.T) {
	store := newTestStore(t)

	sessions := []Session{
		{Name: "alpha", CreatedAt: time.Now(), Vibespaces: []VibespaceEntry{}},
		{Name: "beta", CreatedAt: time.Now(), Vibespaces: []VibespaceEntry{}},
		{Name: "gamma", CreatedAt: time.Now(), Vibespaces: []VibespaceEntry{}},
	}

	for i := range sessions {
		if err := store.Save(&sessions[i]); err != nil {
			t.Fatalf("Save(%s) error: %v", sessions[i].Name, err)
		}
		// Small delay so LastUsed differs for sort order
		time.Sleep(10 * time.Millisecond)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("List() returned %d sessions, want 3", len(list))
	}

	// Should be sorted by LastUsed descending (most recent first)
	if list[0].Name != "gamma" {
		t.Errorf("first session = %q, want %q (most recent)", list[0].Name, "gamma")
	}
}

func TestSessionSaveNew(t *testing.T) {
	store := newTestStore(t)

	session := &Session{
		Name:       "unique",
		CreatedAt:  time.Now(),
		Vibespaces: []VibespaceEntry{},
	}

	if err := store.SaveNew(session); err != nil {
		t.Fatalf("SaveNew() first time error: %v", err)
	}

	// Duplicate should fail
	dup := &Session{
		Name:       "unique",
		CreatedAt:  time.Now(),
		Vibespaces: []VibespaceEntry{},
	}
	if err := store.SaveNew(dup); err == nil {
		t.Error("SaveNew() with duplicate name should return error")
	}
}

func TestSessionDelete(t *testing.T) {
	store := newTestStore(t)

	session := &Session{
		Name:       "to-delete",
		CreatedAt:  time.Now(),
		Vibespaces: []VibespaceEntry{},
	}

	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if err := store.Delete("to-delete"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Should not exist anymore
	if store.Exists("to-delete") {
		t.Error("session should not exist after deletion")
	}

	// Second delete should fail
	if err := store.Delete("to-delete"); err == nil {
		t.Error("Delete() on non-existent session should return error")
	}
}

func TestSessionGetNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Get() on non-existent session should return error")
	}
}

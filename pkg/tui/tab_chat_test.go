package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestChatTabTitle(t *testing.T) {
	ct := NewChatTab(nil, false)
	if ct.Title() != "Chat" {
		t.Fatalf("expected 'Chat', got %q", ct.Title())
	}
}

func TestChatTabShortHelp(t *testing.T) {
	ct := NewChatTab(nil, false)
	bindings := ct.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings")
	}
}

func TestChatTabNoSessionView(t *testing.T) {
	ct := NewChatTab(nil, false)
	ct.SetSize(120, 40)

	view := ct.View()
	plain := stripAnsi(view)
	if !strings.Contains(plain, "No active session") {
		t.Errorf("expected 'No active session', got: %s", plain)
	}
}

func TestChatTabNoSessionUpdate(t *testing.T) {
	ct := NewChatTab(nil, false)
	ct.SetSize(120, 40)

	// Update with no inner model should not panic
	_, cmd := ct.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Fatal("expected nil cmd for no-session chat tab")
	}
}

func TestChatTabInit(t *testing.T) {
	ct := NewChatTab(nil, false)

	// Init with no inner should return nil
	cmd := ct.Init()
	if cmd != nil {
		t.Fatal("expected nil cmd for init with no session")
	}

	// Second init is also nil (already inited)
	cmd = ct.Init()
	if cmd != nil {
		t.Fatal("expected nil cmd for re-init")
	}
}

func TestChatTabSetSize(t *testing.T) {
	ct := NewChatTab(nil, false)
	ct.SetSize(100, 30)

	if ct.width != 100 || ct.height != 30 {
		t.Fatalf("expected 100x30, got %dx%d", ct.width, ct.height)
	}
}

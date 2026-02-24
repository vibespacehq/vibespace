package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

func TestChatTabNoSessionViewViaApp(t *testing.T) {
	app := newTestApp(t)
	// ChatTab is at TabChat (index 1), so one Tab press from initial (index 0)
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if app.activeTab != TabChat {
		t.Fatalf("expected activeTab=%d (Chat), got %d (%s)", TabChat, app.activeTab, TabNames[app.activeTab])
	}
	view := app.View()
	stripped := ui.StripAnsi(view)
	if !strings.Contains(stripped, "No active session") {
		t.Errorf("expected 'No active session' in view, got: %s", stripped[:min(len(stripped), 200)])
	}
}

func TestChatTabTitleViaApp(t *testing.T) {
	app := newTestApp(t)
	chatTab := app.tabs[TabChat].(*ChatTab)
	title := chatTab.Title()
	if title != "Chat" {
		t.Errorf("Title() = %q, want %q", title, "Chat")
	}
}

func TestChatTabShortHelpViaApp(t *testing.T) {
	app := newTestApp(t)
	chatTab := app.tabs[TabChat].(*ChatTab)
	bindings := chatTab.ShortHelp()
	if len(bindings) == 0 {
		t.Error("ShortHelp() returned no bindings")
	}
	// Verify each binding has a non-empty key and help text
	for i, b := range bindings {
		h := b.Help()
		if h.Key == "" {
			t.Errorf("binding[%d] has empty key", i)
		}
		if h.Desc == "" {
			t.Errorf("binding[%d] has empty description", i)
		}
	}
}

func TestChatTabNoSessionViewDimmed(t *testing.T) {
	ct := NewChatTab(nil, false)
	ct.SetSize(120, 40)

	view := ct.View()
	stripped := ui.StripAnsi(view)
	// Should mention switching to Sessions tab
	if !strings.Contains(stripped, "Sessions") {
		t.Errorf("expected 'Sessions' hint in no-session view, got: %s", stripped)
	}
}

func TestChatTabViewWidthRespected(t *testing.T) {
	ct := NewChatTab(nil, false)
	ct.SetSize(80, 25)

	if ct.width != 80 || ct.height != 25 {
		t.Errorf("expected 80x25, got %dx%d", ct.width, ct.height)
	}

	view := ct.View()
	// Should produce some output even at smaller size
	if len(view) == 0 {
		t.Error("expected non-empty view at 80x25")
	}
}

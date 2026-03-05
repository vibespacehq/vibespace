package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

func TestAppTabSwitchView(t *testing.T) {
	app := newTestApp(t)

	// Initial tab is Vibespaces (index 0) — shows welcome cover (no tab bar)
	view := app.View()
	stripped := ui.StripAnsi(view)
	if !strings.Contains(stripped, "vibespace") {
		t.Errorf("initial view should contain 'vibespace' (welcome cover), got snippet: %s", stripped[:min(len(stripped), 200)])
	}

	// Switch to Chat tab (index 1, one Tab press)
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if app.activeTab != TabChat {
		t.Fatalf("after one Tab, expected activeTab=%d (Chat), got %d (%s)",
			TabChat, app.activeTab, TabNames[app.activeTab])
	}

	// Switch to Monitor tab (index 2, another Tab press)
	app.Update(tea.KeyMsg{Type: tea.KeyTab})
	view = app.View()
	stripped = ui.StripAnsi(view)
	if !strings.Contains(stripped, "Monitor") {
		t.Errorf("after tab switch, view should contain 'Monitor', got snippet: %s", stripped[:min(len(stripped), 200)])
	}
}

func TestAppNumberKeyTabSelectionExtended(t *testing.T) {
	app := newTestApp(t)

	// Press '2' to switch to Chat tab (1-indexed: 2 maps to index 1)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if app.activeTab != TabChat {
		t.Errorf("after pressing '2', activeTab = %d (%s), want %d (Chat)",
			app.activeTab, TabNames[app.activeTab], TabChat)
	}

	// Press '1' to switch back to Vibespaces
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if app.activeTab != TabVibespaces {
		t.Errorf("after pressing '1', activeTab = %d (%s), want %d (Vibespaces)",
			app.activeTab, TabNames[app.activeTab], TabVibespaces)
	}

	// Press '5' for last tab (Remote)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	if app.activeTab != TabRemote {
		t.Errorf("after pressing '5', activeTab = %d (%s), want %d (Remote)",
			app.activeTab, TabNames[app.activeTab], TabRemote)
	}

	// Press '3' for Monitor
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if app.activeTab != TabMonitor {
		t.Errorf("after pressing '3', activeTab = %d (%s), want %d (Monitor)",
			app.activeTab, TabNames[app.activeTab], TabMonitor)
	}
}

func TestAppPaletteOpenClose(t *testing.T) {
	app := newTestApp(t)

	// Open palette with ':'
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	if !app.palette.Visible() {
		t.Error("palette should be visible after ':'")
	}

	// Close with Escape
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if app.palette.Visible() {
		t.Error("palette should be hidden after Escape")
	}
}

func TestAppViewContainsAllTabs(t *testing.T) {
	app := newTestApp(t)

	// Switch away from welcome cover so the tab bar is visible
	app.switchTab(TabChat)
	view := app.View()
	stripped := ui.StripAnsi(view)

	tabs := []string{"Vibespaces", "Chat", "Monitor", "Sessions", "Remote"}
	for _, tab := range tabs {
		if !strings.Contains(stripped, tab) {
			t.Errorf("view should contain tab %q", tab)
		}
	}
}

func TestAppHelpOverlayToggle(t *testing.T) {
	app := newTestApp(t)

	// Open help with '?'
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !app.help.Visible() {
		t.Error("help should be visible after '?'")
	}

	// Close with Escape
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if app.help.Visible() {
		t.Error("help should be hidden after Escape")
	}
}

func TestAppShiftTabCyclesReverse(t *testing.T) {
	app := newTestApp(t)

	// Start at Vibespaces (0), shift+tab should go to Remote (4)
	app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if app.activeTab != TabRemote {
		t.Errorf("after shift+tab from 0, expected %d (Remote), got %d (%s)",
			TabRemote, app.activeTab, TabNames[app.activeTab])
	}

	// Another shift+tab should go to Sessions (3)
	app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if app.activeTab != TabSessions {
		t.Errorf("after shift+tab from Remote, expected %d (Sessions), got %d (%s)",
			TabSessions, app.activeTab, TabNames[app.activeTab])
	}
}

func TestAppSwitchTabLazyInit(t *testing.T) {
	app := newTestApp(t)

	// newTestApp does not call Init(), so no tabs are marked as inited yet.
	// Tabs that haven't been switched to should not be inited.
	// Switch to Monitor - should lazy-init it.
	app.switchTab(TabMonitor)
	if !app.tabInited[TabMonitor] {
		t.Error("Monitor tab should be inited after switching to it")
	}

	// Sessions tab should still not be inited
	if app.tabInited[TabSessions] {
		t.Error("Sessions tab should not be inited before switching to it")
	}

	// Switch to Sessions - should lazy-init it
	app.switchTab(TabSessions)
	if !app.tabInited[TabSessions] {
		t.Error("Sessions tab should be inited after switching to it")
	}
}

func TestAppViewNonEmptyForAllTabs(t *testing.T) {
	app := newTestApp(t)

	for i := 0; i < tabCount; i++ {
		app.switchTab(i)
		view := app.View()
		if len(view) == 0 {
			t.Errorf("tab %d (%s) rendered empty view", i, TabNames[i])
		}
	}
}

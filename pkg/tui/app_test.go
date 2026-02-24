package tui

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	zone "github.com/lrstanley/bubblezone"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// --- helpers ---

// stripAnsi removes ANSI escape sequences for text matching in rendered output.
func stripAnsi(s string) string {
	return ui.StripAnsi(s)
}

// containsText returns a WaitFor matcher that strips ANSI codes before checking.
func containsText(target string) func([]byte) bool {
	return func(bts []byte) bool {
		return strings.Contains(ui.StripAnsi(string(bts)), target)
	}
}

// newTestApp creates an App and sends a WindowSizeMsg so it's ready for direct tests.
func newTestApp(t *testing.T) *App {
	t.Helper()
	a := NewApp()
	// Initialize the zone manager (normally done in Init()) so View() works.
	zone.NewGlobal()
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return a
}

// --- teatest integration tests (no k8s) ---

func TestAppHelpOverlay(t *testing.T) {
	tm := teatest.NewTestModel(t, NewApp(), teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Send(tea.QuitMsg{}) })

	teatest.WaitFor(t, tm.Output(), containsText("Vibespaces"),
		teatest.WithDuration(3*time.Second))

	// Open help with ?
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	teatest.WaitFor(t, tm.Output(), containsText("Keybindings"),
		teatest.WithDuration(3*time.Second))

	// Close help with Esc
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	teatest.WaitFor(t, tm.Output(), containsText("Vibespaces"),
		teatest.WithDuration(3*time.Second))

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppPaletteOverlay(t *testing.T) {
	tm := teatest.NewTestModel(t, NewApp(), teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Send(tea.QuitMsg{}) })

	teatest.WaitFor(t, tm.Output(), containsText("Vibespaces"),
		teatest.WithDuration(3*time.Second))

	// Open palette with :
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	teatest.WaitFor(t, tm.Output(), containsText("Go to"),
		teatest.WithDuration(3*time.Second))

	// Close palette with Esc
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})
	teatest.WaitFor(t, tm.Output(), containsText("Vibespaces"),
		teatest.WithDuration(3*time.Second))

	tm.Send(tea.QuitMsg{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestAppCtrlCQuits(t *testing.T) {
	tm := teatest.NewTestModel(t, NewApp(), teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { tm.Send(tea.QuitMsg{}) })

	teatest.WaitFor(t, tm.Output(), containsText("Vibespaces"),
		teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// --- Direct Update/View tests (no k8s) ---

func TestAppInitialRender(t *testing.T) {
	a := newTestApp(t)
	out := stripAnsi(a.View())

	// The tab bar should show all 5 tab names
	for _, name := range TabNames {
		if !strings.Contains(out, name) {
			t.Errorf("initial render missing tab name %q", name)
		}
	}
}

func TestAppLoadingBeforeWindowSize(t *testing.T) {
	a := NewApp()
	// Before any WindowSizeMsg, View should return loading text
	out := a.View()
	if !bytes.Contains([]byte(out), []byte("Loading...")) {
		t.Fatalf("expected 'Loading...' before WindowSizeMsg, got: %s", out)
	}
}

func TestAppTabCycleForward(t *testing.T) {
	a := newTestApp(t)

	// Start on Vibespaces (0)
	if a.activeTab != TabVibespaces {
		t.Fatalf("expected initial tab %d, got %d", TabVibespaces, a.activeTab)
	}

	// Tab key cycles: 0→1→2→3→4→0
	expected := []int{TabChat, TabMonitor, TabSessions, TabRemote, TabVibespaces}
	for _, want := range expected {
		a.Update(tea.KeyMsg{Type: tea.KeyTab})
		if a.activeTab != want {
			t.Fatalf("after Tab, expected activeTab=%d (%s), got %d (%s)",
				want, TabNames[want], a.activeTab, TabNames[a.activeTab])
		}
	}
}

func TestAppTabCycleReverse(t *testing.T) {
	a := newTestApp(t)

	// Shift+Tab cycles backwards: 0→4→3→2→1→0
	expected := []int{TabRemote, TabSessions, TabMonitor, TabChat, TabVibespaces}
	for _, want := range expected {
		a.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		if a.activeTab != want {
			t.Fatalf("after Shift+Tab, expected activeTab=%d (%s), got %d (%s)",
				want, TabNames[want], a.activeTab, TabNames[a.activeTab])
		}
	}
}

func TestAppNumberKeyTabSwitch(t *testing.T) {
	a := newTestApp(t)

	keys := []struct {
		rune rune
		want int
	}{
		{'3', TabMonitor},
		{'5', TabRemote},
		{'2', TabChat},
		{'4', TabSessions},
		{'1', TabVibespaces},
	}
	for _, k := range keys {
		a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k.rune}})
		if a.activeTab != k.want {
			t.Fatalf("after key %q, expected activeTab=%d (%s), got %d (%s)",
				k.rune, k.want, TabNames[k.want], a.activeTab, TabNames[a.activeTab])
		}
	}
}

func TestAppTabSwitchDirect(t *testing.T) {
	a := newTestApp(t)

	// Start on Vibespaces (index 0)
	if a.activeTab != TabVibespaces {
		t.Fatalf("expected initial tab %d, got %d", TabVibespaces, a.activeTab)
	}

	// Switch to Monitor (index 2)
	a.switchTab(TabMonitor)
	if a.activeTab != TabMonitor {
		t.Fatalf("expected tab %d after switch, got %d", TabMonitor, a.activeTab)
	}
	if !a.tabInited[TabMonitor] {
		t.Fatal("expected Monitor tab to be lazy-inited after switch")
	}

	// Switch to same tab should be no-op
	cmd := a.switchTab(TabMonitor)
	if cmd != nil {
		t.Fatal("expected nil cmd when switching to same tab")
	}

	// Switch to all tabs to verify lazy init
	for i := 0; i < tabCount; i++ {
		a.switchTab(i)
		if !a.tabInited[i] {
			t.Fatalf("expected tab %d (%s) to be inited", i, TabNames[i])
		}
	}
}

func TestAppOverlayInterceptsKeys(t *testing.T) {
	a := newTestApp(t)

	// Open help overlay
	a.help.Show()
	a.help.SetSize(120, 40)
	startTab := a.activeTab

	// Send Tab key — should NOT switch tabs because help intercepts
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.activeTab != startTab {
		t.Fatalf("help overlay should intercept Tab key; tab changed from %d to %d", startTab, a.activeTab)
	}

	// Close help
	a.help.Hide()

	// Open palette overlay
	a.palette.Show()
	a.palette.SetSize(120, 40)

	// Send Tab key — should NOT switch tabs because palette intercepts
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.activeTab != startTab {
		t.Fatalf("palette overlay should intercept Tab key; tab changed from %d to %d", startTab, a.activeTab)
	}
}

func TestAppSpringAnimationSettles(t *testing.T) {
	a := newTestApp(t)

	// Switch to trigger animation
	a.switchTab(TabMonitor)
	if !a.animating {
		t.Fatal("expected animation to start after tab switch")
	}

	// Feed springTickMsg until animation settles (max 120 ticks = 2s at 60fps)
	for i := 0; i < 120; i++ {
		a.Update(springTickMsg{})
		if !a.animating {
			break
		}
	}

	if a.animating {
		t.Fatal("spring animation did not settle within 120 ticks")
	}

	// Final position should match target
	if math.Abs(a.highlightX-a.targetX) > 0.5 {
		t.Fatalf("highlightX=%f should be close to targetX=%f", a.highlightX, a.targetX)
	}
}

func TestComputeTabLayout(t *testing.T) {
	a := newTestApp(t)

	if len(a.tabOffsets) != tabCount {
		t.Fatalf("expected %d tab offsets, got %d", tabCount, len(a.tabOffsets))
	}
	if len(a.tabWidths) != tabCount {
		t.Fatalf("expected %d tab widths, got %d", tabCount, len(a.tabWidths))
	}

	// Offsets should be strictly increasing
	for i := 1; i < tabCount; i++ {
		if a.tabOffsets[i] <= a.tabOffsets[i-1] {
			t.Fatalf("tab offsets not strictly increasing: offset[%d]=%d <= offset[%d]=%d",
				i, a.tabOffsets[i], i-1, a.tabOffsets[i-1])
		}
	}

	// Each width should match the rendered label "  N Name  "
	for i := 0; i < tabCount; i++ {
		expectedLabel := []rune("  " + string(rune('1'+i)) + " " + TabNames[i] + "  ")
		if a.tabWidths[i] != len(expectedLabel) {
			t.Fatalf("tab %d width=%d, expected %d for label %q",
				i, a.tabWidths[i], len(expectedLabel), string(expectedLabel))
		}
	}

	// Consecutive offset difference should equal previous tab's width
	for i := 1; i < tabCount; i++ {
		diff := a.tabOffsets[i] - a.tabOffsets[i-1]
		if diff != a.tabWidths[i-1] {
			t.Fatalf("offset[%d]-offset[%d]=%d, expected tabWidths[%d]=%d",
				i, i-1, diff, i-1, a.tabWidths[i-1])
		}
	}
}

func TestBuildGradient(t *testing.T) {
	stops := brandGradient

	// n=0 → nil
	if got := buildGradient(0, stops); got != nil {
		t.Fatalf("buildGradient(0) should return nil, got %v", got)
	}

	// n=1 → 1 color
	if got := buildGradient(1, stops); len(got) != 1 {
		t.Fatalf("buildGradient(1) should return 1 color, got %d", len(got))
	}

	// n=10 → 10 colors
	got := buildGradient(10, stops)
	if len(got) != 10 {
		t.Fatalf("buildGradient(10) should return 10 colors, got %d", len(got))
	}

	// First and last colors should approximate the gradient stops
	if got[0] == "" {
		t.Fatal("first gradient color should not be empty")
	}
	if got[9] == "" {
		t.Fatal("last gradient color should not be empty")
	}

	// All colors should be valid hex
	for i, c := range got {
		s := string(c)
		if len(s) < 4 || s[0] != '#' {
			t.Fatalf("gradient color[%d]=%q is not a valid hex color", i, s)
		}
	}
}

// --- k8s-dependent tests (direct model) ---

func TestAppFullTabCycleWithK8s(t *testing.T) {
	if testing.Short() {
		t.Skip("requires k8s cluster")
	}

	a := newTestApp(t)

	// Cycle through all tabs, verify each renders non-empty content
	for i := 0; i < tabCount; i++ {
		a.switchTab(i)
		out := stripAnsi(a.View())
		if len(out) == 0 {
			t.Fatalf("tab %d (%s) rendered empty view", i, TabNames[i])
		}
		// Tab bar should always contain the tab name
		if !strings.Contains(out, TabNames[i]) {
			t.Errorf("tab %d view missing tab name %q in tab bar", i, TabNames[i])
		}
	}
}

func TestAppVibespacesTabWithK8s(t *testing.T) {
	if testing.Short() {
		t.Skip("requires k8s cluster")
	}

	a := newTestApp(t)
	out := stripAnsi(a.View())

	// Vibespaces tab should render content (table, empty state, or error)
	if !strings.Contains(out, "Vibespaces") {
		t.Fatal("vibespaces tab missing from view")
	}
}

func TestAppMonitorTabWithK8s(t *testing.T) {
	if testing.Short() {
		t.Skip("requires k8s cluster")
	}

	a := newTestApp(t)
	a.switchTab(TabMonitor)
	out := stripAnsi(a.View())

	// Monitor tab should render metrics content or unavailable message
	if !strings.Contains(out, "Monitor") {
		t.Fatal("monitor tab missing from view")
	}
}

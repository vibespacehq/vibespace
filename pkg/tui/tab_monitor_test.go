package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/metrics"
	"github.com/vibespacehq/vibespace/pkg/model"
)

func newTestMonitorTab() *MonitorTab {
	shared := &SharedState{}
	tab := NewMonitorTab(shared)
	tab.SetSize(120, 40)
	return tab
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		millis int64
		want   string
	}{
		{0, "0m"},
		{100, "100m"},
		{1000, "1000m"},
	}
	for _, tt := range tests {
		got := formatCPU(tt.millis)
		if got != tt.want {
			t.Errorf("formatCPU(%d) = %q, want %q", tt.millis, got, tt.want)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0Mi"},
		{1024 * 1024, "1Mi"},
		{512 * 1024 * 1024, "512Mi"},
	}
	for _, tt := range tests {
		got := formatMemory(tt.bytes)
		if got != tt.want {
			t.Errorf("formatMemory(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestMonitorBar(t *testing.T) {
	// Zero total returns detail only
	got := monitorBar(100, 0, 10, "100m/0m")
	if got != "100m/0m" {
		t.Fatalf("expected detail only for zero total, got %q", got)
	}

	// Normal bar
	got = monitorBar(50, 100, 10, "50m/100m")
	plain := stripAnsi(got)
	if !strings.Contains(plain, "50%") {
		t.Fatalf("expected '50%%' in bar, got %q", plain)
	}
	if !strings.Contains(plain, "50m/100m") {
		t.Fatalf("expected detail in bar, got %q", plain)
	}

	// Very small usage shows <1%
	got = monitorBar(1, 10000, 10, "1m/10000m")
	plain = stripAnsi(got)
	if !strings.Contains(plain, "<1%") && !strings.Contains(plain, "0%") {
		t.Fatalf("expected '<1%%' or '0%%' for tiny usage, got %q", plain)
	}
}

func TestMonitorBarOverflow(t *testing.T) {
	// Used > total should cap at 100%
	got := monitorBar(200, 100, 10, "200m/100m")
	plain := stripAnsi(got)
	if !strings.Contains(plain, "100%") {
		t.Fatalf("expected '100%%' for overflow, got %q", plain)
	}
}

func TestMonitorTabUnavailableView(t *testing.T) {
	tab := newTestMonitorTab()
	tab.available = false
	tab.err = "metrics API not responding"

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "not available") {
		t.Error("unavailable view should contain 'not available'")
	}
	if !strings.Contains(view, "metrics API not responding") {
		t.Error("unavailable view should show error message")
	}
}

func TestMonitorTabPickerToggle(t *testing.T) {
	tab := newTestMonitorTab()
	tab.available = true
	tab.pickerItems = []string{"all", "test", "project"}

	// Key "v" opens picker
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if !tab.pickerOpen {
		t.Fatal("expected pickerOpen=true after 'v'")
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "all vibespaces") {
		t.Error("picker view should contain 'all vibespaces'")
	}
	if !strings.Contains(view, "test") {
		t.Error("picker view should contain picker items")
	}

	// Esc closes picker
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.pickerOpen {
		t.Fatal("expected pickerOpen=false after Esc")
	}
}

func TestMonitorTabPauseToggle(t *testing.T) {
	tab := newTestMonitorTab()
	tab.available = true

	if tab.paused {
		t.Fatal("should start unpaused")
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !tab.paused {
		t.Fatal("expected paused=true after 'p'")
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if tab.paused {
		t.Fatal("expected paused=false after second 'p'")
	}
}

func TestMonitorTabPickerNavigation(t *testing.T) {
	tab := newTestMonitorTab()
	tab.pickerOpen = true
	tab.pickerItems = []string{"all", "test", "project"}
	tab.pickerCursor = 0

	// j moves down
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.pickerCursor != 1 {
		t.Fatalf("expected cursor=1, got %d", tab.pickerCursor)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.pickerCursor != 2 {
		t.Fatalf("expected cursor=2, got %d", tab.pickerCursor)
	}

	// j at end stays
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.pickerCursor != 2 {
		t.Fatalf("expected cursor=2 (clamped), got %d", tab.pickerCursor)
	}

	// k moves up
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.pickerCursor != 1 {
		t.Fatalf("expected cursor=1, got %d", tab.pickerCursor)
	}
}

func TestMonitorTabPickerEnterSelectsFilter(t *testing.T) {
	tab := newTestMonitorTab()
	tab.pickerOpen = true
	tab.pickerItems = []string{"all", "test", "project"}
	tab.pickerCursor = 1 // "test"

	tab.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if tab.pickerOpen {
		t.Fatal("expected picker closed after enter")
	}
	if tab.filterVS != "test" {
		t.Fatalf("expected filterVS='test', got %q", tab.filterVS)
	}

	// Select "all" resets filter
	tab.pickerOpen = true
	tab.pickerCursor = 0
	tab.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if tab.filterVS != "" {
		t.Fatalf("expected filterVS='' for 'all', got %q", tab.filterVS)
	}
}

func TestMonitorTabDashboardView(t *testing.T) {
	tab := newTestMonitorTab()
	tab.available = true
	tab.pods = []metrics.PodMetrics{
		{
			Name:             "claude-1-pod",
			AgentName:        "claude-1",
			VibspaceName:     "test",
			CPUMillis:        250,
			CPULimitMillis:   1000,
			MemoryBytes:      256 * 1024 * 1024,
			MemoryLimitBytes: 1024 * 1024 * 1024,
		},
	}
	tab.nodes = []metrics.NodeMetrics{
		{
			Name:                   "node-1",
			CPUMillis:              500,
			CPUAllocatableMillis:   4000,
			MemoryBytes:            2 * 1024 * 1024 * 1024,
			MemoryAllocatableBytes: 8 * 1024 * 1024 * 1024,
		},
	}

	view := stripAnsi(tab.View())

	// Should contain table elements
	if !strings.Contains(view, "Vibespace") {
		t.Error("dashboard should contain 'Vibespace' header")
	}
	if !strings.Contains(view, "node-1") {
		t.Error("dashboard should contain node name")
	}
	if !strings.Contains(view, "claude-1") {
		t.Error("dashboard should contain agent name")
	}
}

func TestMonitorTabRebuildPickerItems(t *testing.T) {
	tab := newTestMonitorTab()
	tab.vibespaces = []*model.Vibespace{
		{Name: "project-a"},
		{Name: "project-b"},
		{Name: "project-a"}, // duplicate
	}

	tab.rebuildPickerItems()

	if len(tab.pickerItems) != 3 { // "all" + 2 unique
		t.Fatalf("expected 3 picker items, got %d: %v", len(tab.pickerItems), tab.pickerItems)
	}
	if tab.pickerItems[0] != "all" {
		t.Fatalf("expected first item 'all', got %q", tab.pickerItems[0])
	}
}

func TestMonitorTabFilteredPods(t *testing.T) {
	tab := newTestMonitorTab()
	tab.pods = []metrics.PodMetrics{
		{VibspaceName: "a", AgentName: "agent-1"},
		{VibspaceName: "b", AgentName: "agent-2"},
		{VibspaceName: "a", AgentName: "agent-3"},
	}

	// No filter returns all
	tab.filterVS = ""
	got := tab.filteredPods()
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}

	// Filter to "a"
	tab.filterVS = "a"
	got = tab.filteredPods()
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestMonitorTabBarWidth(t *testing.T) {
	tab := newTestMonitorTab()

	tab.width = 130
	if tab.barWidth() != 15 {
		t.Fatalf("expected 15 for width=130, got %d", tab.barWidth())
	}

	tab.width = 110
	if tab.barWidth() != 12 {
		t.Fatalf("expected 12 for width=110, got %d", tab.barWidth())
	}

	tab.width = 80
	if tab.barWidth() != 10 {
		t.Fatalf("expected 10 for width=80, got %d", tab.barWidth())
	}
}

func TestMonitorTabTitle(t *testing.T) {
	tab := newTestMonitorTab()
	if tab.Title() != "Monitor" {
		t.Fatalf("expected 'Monitor', got %q", tab.Title())
	}
}

func TestMonitorTabShortHelp(t *testing.T) {
	tab := newTestMonitorTab()
	bindings := tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings")
	}
}

func TestMonitorTabComputePercentages(t *testing.T) {
	tab := newTestMonitorTab()
	tab.pods = []metrics.PodMetrics{
		{
			VibspaceName:     "test",
			CPUMillis:        250,
			CPULimitMillis:   1000,
			MemoryBytes:      256 * 1024 * 1024,
			MemoryLimitBytes: 1024 * 1024 * 1024,
		},
	}
	tab.nodes = []metrics.NodeMetrics{
		{
			CPUMillis:              500,
			CPUAllocatableMillis:   4000,
			MemoryBytes:            2 * 1024 * 1024 * 1024,
			MemoryAllocatableBytes: 8 * 1024 * 1024 * 1024,
		},
	}

	cpuPct, memPct := tab.computePercentages()
	if cpuPct < 0 || cpuPct > 100 {
		t.Fatalf("cpuPct=%f out of range", cpuPct)
	}
	if memPct < 0 || memPct > 100 {
		t.Fatalf("memPct=%f out of range", memPct)
	}
}

func TestMonitorTabRenderTotals(t *testing.T) {
	tab := newTestMonitorTab()
	tab.available = true
	tab.pods = []metrics.PodMetrics{
		{
			VibspaceName:     "test",
			AgentName:        "a1",
			CPUMillis:        100,
			CPULimitMillis:   500,
			MemoryBytes:      100 * 1024 * 1024,
			MemoryLimitBytes: 500 * 1024 * 1024,
		},
	}
	tab.nodes = []metrics.NodeMetrics{
		{
			CPUMillis:              200,
			CPUAllocatableMillis:   2000,
			MemoryBytes:            500 * 1024 * 1024,
			MemoryAllocatableBytes: 4 * 1024 * 1024 * 1024,
		},
	}

	result := tab.renderTotals(tab.pods)
	plain := stripAnsi(result)
	if plain == "" {
		t.Fatal("renderTotals should return non-empty output")
	}
}

func TestMonitorTabDashboardViewWithFilter(t *testing.T) {
	tab := newTestMonitorTab()
	tab.available = true
	tab.pods = []metrics.PodMetrics{
		{VibspaceName: "alpha", AgentName: "a1", CPUMillis: 100, CPULimitMillis: 500, MemoryBytes: 100 * 1024 * 1024, MemoryLimitBytes: 500 * 1024 * 1024},
		{VibspaceName: "beta", AgentName: "a2", CPUMillis: 200, CPULimitMillis: 500, MemoryBytes: 200 * 1024 * 1024, MemoryLimitBytes: 500 * 1024 * 1024},
	}
	tab.nodes = []metrics.NodeMetrics{
		{Name: "node-1", CPUMillis: 300, CPUAllocatableMillis: 4000, MemoryBytes: 1024 * 1024 * 1024, MemoryAllocatableBytes: 8 * 1024 * 1024 * 1024},
	}

	// Filter to "alpha" only
	tab.filterVS = "alpha"
	view := stripAnsi(tab.View())
	if !strings.Contains(view, "a1") {
		t.Error("filtered view should contain a1")
	}
}

func TestIndentBlock(t *testing.T) {
	input := "line1\nline2\n"
	got := indentBlock(input, "  ")
	if !strings.Contains(got, "  line1") {
		t.Errorf("expected indented lines, got %q", got)
	}
}

func TestIndentTableWithHeaderGap(t *testing.T) {
	input := "Header\nRow1\nRow2"
	got := indentTableWithHeaderGap(input)
	lines := strings.Split(got, "\n")
	// Should have gap after first line
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines (header + gap + 2 rows), got %d", len(lines))
	}
	if strings.TrimSpace(lines[1]) != "" {
		t.Fatalf("expected blank line after header, got %q", lines[1])
	}
}

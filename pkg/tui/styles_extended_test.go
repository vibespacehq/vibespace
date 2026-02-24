package tui

import (
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/ui"
)

func TestNewStylesAllFieldsPopulated(t *testing.T) {
	s := NewStyles()

	// Verify all style fields are usable (render non-empty output containing the text)
	testStr := "test"
	styles := []struct {
		name  string
		style func() string
	}{
		{"Title", func() string { return s.Title.Render(testStr) }},
		{"Header", func() string { return s.Header.Render(testStr) }},
		{"InputArea", func() string { return s.InputArea.Render(testStr) }},
		{"Prompt", func() string { return s.Prompt.Render(testStr) }},
		{"StatusBar", func() string { return s.StatusBar.Render(testStr) }},
		{"Error", func() string { return s.Error.Render(testStr) }},
		{"Dim", func() string { return s.Dim.Render(testStr) }},
		{"Timestamp", func() string { return s.Timestamp.Render(testStr) }},
	}
	for _, tt := range styles {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.style()
			stripped := ui.StripAnsi(got)
			// Some styles add borders/padding, so check Contains instead of exact match
			if !strings.Contains(stripped, testStr) {
				t.Errorf("%s.Render(%q) stripped = %q, should contain %q", tt.name, testStr, stripped, testStr)
			}
		})
	}
}

func TestGetAgentColorCycles(t *testing.T) {
	// Ensure colors cycle and don't panic for large indices
	colors := make(map[string]bool)
	for i := 0; i < 20; i++ {
		c := GetAgentColor(i)
		colors[string(c)] = true
	}
	// Should have multiple distinct colors
	if len(colors) < 3 {
		t.Errorf("expected at least 3 distinct colors, got %d", len(colors))
	}
}

func TestUserLabelWithTargetAllExtended(t *testing.T) {
	label := UserLabelWithTarget("all")
	stripped := ui.StripAnsi(label)
	if !strings.Contains(stripped, "You") {
		t.Errorf("label should contain 'You', got %q", stripped)
	}
}

func TestUserLabelWithTargetSpecificExtended(t *testing.T) {
	label := UserLabelWithTarget("claude-1")
	stripped := ui.StripAnsi(label)
	if !strings.Contains(stripped, "You") {
		t.Errorf("label should contain 'You', got %q", stripped)
	}
	if !strings.Contains(stripped, "claude-1") {
		t.Errorf("label should contain target 'claude-1', got %q", stripped)
	}
}

func TestAgentLabelStyleExtended(t *testing.T) {
	color := GetAgentColor(0)
	style := AgentLabelStyle(color)
	rendered := style.Render("[claude-1]")
	stripped := ui.StripAnsi(rendered)
	if stripped != "[claude-1]" {
		t.Errorf("AgentLabelStyle rendered = %q, want %q", stripped, "[claude-1]")
	}
}

func TestNewStylesAdditionalFields(t *testing.T) {
	s := NewStyles()

	testStr := "x"
	additionalStyles := []struct {
		name  string
		style func() string
	}{
		{"Subtitle", func() string { return s.Subtitle.Render(testStr) }},
		{"Bold", func() string { return s.Bold.Render(testStr) }},
		{"Success", func() string { return s.Success.Render(testStr) }},
		{"Warning", func() string { return s.Warning.Render(testStr) }},
		{"UserLabel", func() string { return s.UserLabel.Render(testStr) }},
		{"ToolLabel", func() string { return s.ToolLabel.Render(testStr) }},
		{"Thinking", func() string { return s.Thinking.Render(testStr) }},
		{"Input", func() string { return s.Input.Render(testStr) }},
		{"InputCursor", func() string { return s.InputCursor.Render(testStr) }},
		{"HelpKey", func() string { return s.HelpKey.Render(testStr) }},
		{"HelpDesc", func() string { return s.HelpDesc.Render(testStr) }},
	}
	for _, tt := range additionalStyles {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.style()
			stripped := ui.StripAnsi(got)
			if stripped != testStr {
				t.Errorf("%s.Render(%q) stripped = %q, want %q", tt.name, testStr, stripped, testStr)
			}
		})
	}
}

func TestUserLabelArrowFormat(t *testing.T) {
	// Verify the label includes the arrow separator
	label := UserLabelWithTarget("agent-1@vs")
	stripped := ui.StripAnsi(label)
	if !strings.Contains(stripped, "\u2192") { // → character
		t.Errorf("label should contain arrow, got %q", stripped)
	}
}

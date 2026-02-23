package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

func TestErrorHints(t *testing.T) {
	tests := []struct {
		err      error
		wantHint string
	}{
		{vserrors.ErrVibespaceNotFound, "vibespace list"},
		{vserrors.ErrAgentNotFound, "agent list"},
		{vserrors.ErrClusterNotInitialized, "vibespace init"},
		{vserrors.ErrClusterNotRunning, "vibespace init"},
		{vserrors.ErrDaemonNotRunning, "auto-start"},
		{vserrors.ErrForwardNotFound, "forward list"},
		{vserrors.ErrNoAgents, "agent create"},
		{vserrors.ErrRemoteNotConnected, "remote connect"},
		{vserrors.ErrWireGuardNotAvailable, "wireguard-tools"},
		{vserrors.ErrRemoteAlreadyConnected, "remote disconnect"},
		{vserrors.ErrInvalidToken, "generate-token"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			hint := getErrorHint(tt.err)
			if hint == "" {
				t.Errorf("getErrorHint(%v) returned empty string", tt.err)
			}
			if len(hint) < 5 {
				t.Errorf("getErrorHint(%v) returned suspiciously short hint: %q", tt.err, hint)
			}
		})
	}
}

func TestErrorHintUnknown(t *testing.T) {
	hint := getErrorHint(errors.New("unknown error"))
	if hint != "" {
		t.Errorf("getErrorHint(unknown) = %q, want empty string", hint)
	}
}

func TestOutputColorHelpersNoColor(t *testing.T) {
	out := NewOutput(OutputConfig{NoColor: true})

	tests := []struct {
		name string
		fn   func(string) string
	}{
		{"Green", out.Green},
		{"Yellow", out.Yellow},
		{"Red", out.Red},
		{"Bold", out.Bold},
		{"Dim", out.Dim},
		{"Teal", out.Teal},
		{"Orange", out.Orange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn("hello")
			if got != "hello" {
				t.Errorf("%s(\"hello\") with noColor = %q, want \"hello\"", tt.name, got)
			}
		})
	}
}

func TestOutputColorHelpersWithColor(t *testing.T) {
	// Force lipgloss to emit ANSI codes even without a TTY
	lipgloss.SetColorProfile(2) // ANSI256
	defer lipgloss.SetColorProfile(0)

	// Also force COLORTERM so lipgloss detects color support
	os.Setenv("COLORTERM", "truecolor")
	defer os.Unsetenv("COLORTERM")

	out := NewOutput(OutputConfig{})
	out.noColor = false
	out.styles = ui.NewStyles()

	tests := []struct {
		name string
		fn   func(string) string
	}{
		{"Green", out.Green},
		{"Yellow", out.Yellow},
		{"Red", out.Red},
		{"Bold", out.Bold},
		{"Dim", out.Dim},
		{"Teal", out.Teal},
		{"Orange", out.Orange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn("hello")
			// Should contain ANSI escape sequence
			if !strings.Contains(got, "\x1b[") {
				t.Errorf("%s(\"hello\") with color should contain ANSI codes, got %q", tt.name, got)
			}
			// Should still contain the original text
			stripped := stripAnsi(got)
			if stripped != "hello" {
				t.Errorf("%s(\"hello\") stripped = %q, want \"hello\"", tt.name, stripped)
			}
		})
	}
}

func TestOutputTableDefaultMode(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{{"test", "running"}}

	got := ui.NewTable(headers, rows, false)
	stripped := stripAnsi(got)

	if !strings.Contains(stripped, "NAME") {
		t.Error("table should contain header NAME")
	}
	if !strings.Contains(stripped, "test") {
		t.Error("table should contain data 'test'")
	}

	// Should NOT contain box-drawing characters
	boxChars := []string{"─", "│", "╭", "╮", "╯", "╰"}
	for _, ch := range boxChars {
		if strings.Contains(got, ch) {
			t.Errorf("default table should not contain box character %q", ch)
		}
	}
}

package tui

import (
	"strings"
	"testing"
)

func TestNewStyles(t *testing.T) {
	s := NewStyles()

	// Verify styles render non-empty text (proves they are initialized)
	checks := map[string]string{
		"App":       s.App.Render("x"),
		"Title":     s.Title.Render("x"),
		"Error":     s.Error.Render("x"),
		"Dim":       s.Dim.Render("x"),
		"UserLabel": s.UserLabel.Render("x"),
		"CodeBlock": s.CodeBlock.Render("x"),
	}
	for name, rendered := range checks {
		if rendered == "" {
			t.Errorf("%s style rendered empty", name)
		}
	}
}

func TestGetAgentColor(t *testing.T) {
	// Test that colors are returned for various indices
	for i := 0; i < 10; i++ {
		color := GetAgentColor(i)
		if color == "" {
			t.Fatalf("GetAgentColor(%d) returned empty color", i)
		}
	}
}

func TestAgentLabelStyle(t *testing.T) {
	color := GetAgentColor(0)
	style := AgentLabelStyle(color)
	rendered := style.Render("[agent]")
	if rendered == "" {
		t.Fatal("AgentLabelStyle should render non-empty")
	}
	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "[agent]") {
		t.Fatalf("expected '[agent]' in rendered output, got %q", plain)
	}
}

func TestUserLabelWithTarget(t *testing.T) {
	result := UserLabelWithTarget("all")
	plain := stripAnsi(result)
	if !strings.Contains(plain, "You") {
		t.Fatalf("expected 'You' in label, got %q", plain)
	}
	if !strings.Contains(plain, "all") {
		t.Fatalf("expected 'all' in label, got %q", plain)
	}
}

func TestUserLabelWithTargetSpecific(t *testing.T) {
	result := UserLabelWithTarget("claude-1@test")
	plain := stripAnsi(result)
	if !strings.Contains(plain, "claude-1@test") {
		t.Fatalf("expected target in label, got %q", plain)
	}
}

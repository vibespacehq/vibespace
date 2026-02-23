package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/permission"
)

func newTestRequest(agentKey, toolName string, input any) *permission.Request {
	data, _ := json.Marshal(input)
	return &permission.Request{
		ID:        "test-id",
		AgentKey:  agentKey,
		ToolName:  toolName,
		ToolInput: json.RawMessage(data),
	}
}

func TestNewPermissionPrompt(t *testing.T) {
	req := newTestRequest("claude-1@test", "Bash", map[string]string{"command": "ls"})
	p := NewPermissionPrompt(req)

	if p == nil {
		t.Fatal("expected non-nil prompt")
	}
	if p.Selected() != 1 {
		t.Fatalf("expected default selected=1 (Deny), got %d", p.Selected())
	}
	if p.Request() != req {
		t.Fatal("Request() should return the original request")
	}
}

func TestPermissionPromptMoveLeftRight(t *testing.T) {
	req := newTestRequest("claude-1@test", "Bash", map[string]string{"command": "ls"})
	p := NewPermissionPrompt(req)

	// Default is Deny (1)
	p.MoveLeft()
	if p.Selected() != 0 {
		t.Fatalf("expected 0 (Allow) after MoveLeft, got %d", p.Selected())
	}

	p.MoveRight()
	if p.Selected() != 1 {
		t.Fatalf("expected 1 (Deny) after MoveRight, got %d", p.Selected())
	}
}

func TestPermissionPromptGetDecision(t *testing.T) {
	req := newTestRequest("claude-1@test", "Bash", map[string]string{"command": "ls"})
	p := NewPermissionPrompt(req)

	// Default Deny
	if p.GetDecision() != permission.DecisionDeny {
		t.Fatalf("expected Deny, got %s", p.GetDecision())
	}

	p.MoveLeft()
	if p.GetDecision() != permission.DecisionAllow {
		t.Fatalf("expected Allow, got %s", p.GetDecision())
	}
}

func TestPermissionPromptSetSize(t *testing.T) {
	req := newTestRequest("claude-1@test", "Bash", map[string]string{"command": "ls"})
	p := NewPermissionPrompt(req)

	p.SetSize(120, 40)
	if p.width != 120 || p.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", p.width, p.height)
	}
}

func TestPermissionPromptView(t *testing.T) {
	req := newTestRequest("claude-1@test", "Bash", map[string]string{"command": "echo hello"})
	p := NewPermissionPrompt(req)
	p.SetSize(120, 40)

	view := stripAnsi(p.View())

	checks := []string{"Permission Request", "claude-1@test", "Bash", "Allow", "Deny"}
	for _, want := range checks {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestPermissionPromptViewDenySelected(t *testing.T) {
	req := newTestRequest("agent@vs", "Read", map[string]string{"file_path": "/etc/hosts"})
	p := NewPermissionPrompt(req)
	p.SetSize(80, 30)

	view := stripAnsi(p.View())
	if !strings.Contains(view, "Deny") {
		t.Error("view missing Deny button")
	}
}

func TestPermissionPromptViewAllowSelected(t *testing.T) {
	req := newTestRequest("agent@vs", "Read", map[string]string{"file_path": "/etc/hosts"})
	p := NewPermissionPrompt(req)
	p.SetSize(80, 30)
	p.MoveLeft()

	view := stripAnsi(p.View())
	if !strings.Contains(view, "Allow") {
		t.Error("view missing Allow button")
	}
}

func TestFormatToolInputBash(t *testing.T) {
	req := newTestRequest("a@b", "Bash", map[string]string{"command": "echo hello"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "echo hello" {
		t.Fatalf("expected 'echo hello', got %q", got)
	}
}

func TestFormatToolInputRead(t *testing.T) {
	req := newTestRequest("a@b", "Read", map[string]string{"file_path": "/tmp/test.txt"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "/tmp/test.txt" {
		t.Fatalf("expected '/tmp/test.txt', got %q", got)
	}
}

func TestFormatToolInputGrep(t *testing.T) {
	req := newTestRequest("a@b", "Grep", map[string]string{"pattern": "TODO", "path": "/src"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "TODO in /src" {
		t.Fatalf("expected 'TODO in /src', got %q", got)
	}
}

func TestFormatToolInputGrepNoPath(t *testing.T) {
	req := newTestRequest("a@b", "Grep", map[string]string{"pattern": "TODO"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "TODO" {
		t.Fatalf("expected 'TODO', got %q", got)
	}
}

func TestFormatToolInputEmpty(t *testing.T) {
	req := &permission.Request{
		ID:        "test",
		AgentKey:  "a@b",
		ToolName:  "Bash",
		ToolInput: json.RawMessage(""),
	}
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFormatToolInputWebFetch(t *testing.T) {
	req := newTestRequest("a@b", "WebFetch", map[string]string{"url": "https://example.com"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "https://example.com" {
		t.Fatalf("expected URL, got %q", got)
	}
}

func TestFormatToolInputWebSearch(t *testing.T) {
	req := newTestRequest("a@b", "WebSearch", map[string]string{"query": "golang testing"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "golang testing" {
		t.Fatalf("expected 'golang testing', got %q", got)
	}
}

func TestFormatToolInputGlob(t *testing.T) {
	req := newTestRequest("a@b", "Glob", map[string]string{"pattern": "**/*.go"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got != "**/*.go" {
		t.Fatalf("expected '**/*.go', got %q", got)
	}
}

func TestFormatToolInputFallbackRawJSON(t *testing.T) {
	req := newTestRequest("a@b", "UnknownTool", map[string]string{"foo": "bar"})
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if got == "" {
		t.Fatal("expected non-empty fallback")
	}
}

func TestFormatToolInputTruncatesLongRaw(t *testing.T) {
	long := strings.Repeat("x", 100)
	req := &permission.Request{
		ID:        "test",
		AgentKey:  "a@b",
		ToolName:  "UnknownTool",
		ToolInput: json.RawMessage(`"` + long + `"`),
	}
	p := NewPermissionPrompt(req)
	got := p.formatToolInput()
	if len(got) > 50 {
		t.Fatalf("expected truncated output, got len=%d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected '...' suffix, got %q", got)
	}
}

func TestPermissionPromptViewNoSize(t *testing.T) {
	req := newTestRequest("agent@vs", "Bash", map[string]string{"command": "ls"})
	p := NewPermissionPrompt(req)
	// Don't set size — should still render (no centering)
	view := p.View()
	if view == "" {
		t.Fatal("expected non-empty view without size")
	}
}

func TestPermissionPromptViewNarrowWidth(t *testing.T) {
	req := newTestRequest("agent@vs", "Bash", map[string]string{"command": "ls"})
	p := NewPermissionPrompt(req)
	p.SetSize(50, 20)
	view := stripAnsi(p.View())
	if !strings.Contains(view, "Permission Request") {
		t.Error("narrow view missing title")
	}
}

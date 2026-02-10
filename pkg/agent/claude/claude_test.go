package claude

import (
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/agent"
)

func TestParseStreamLineAssistant(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello world"}]}}`
	a := &Agent{}
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false for valid JSON")
	}
	if msg == nil {
		t.Fatal("ParseStreamLine returned nil message")
	}
	if msg.Type != "text" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "text")
	}
	if msg.Text != "Hello world" {
		t.Errorf("msg.Text = %q, want %q", msg.Text, "Hello world")
	}
}

func TestParseStreamLineToolUse(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"Read","input":{"file_path":"/tmp/test.go"}}]}}`
	a := &Agent{}
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "tool_use" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "tool_use")
	}
	if msg.ToolName != "Read" {
		t.Errorf("msg.ToolName = %q, want %q", msg.ToolName, "Read")
	}
	if msg.ToolInput != "/tmp/test.go" {
		t.Errorf("msg.ToolInput = %q, want %q", msg.ToolInput, "/tmp/test.go")
	}
}

func TestParseStreamLineContentBlockStart(t *testing.T) {
	line := `{"type":"content_block_start","content_block":{"type":"tool_use","id":"tool_2","name":"Bash","input":{"command":"ls -la"}}}`
	a := &Agent{}
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "tool_use" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "tool_use")
	}
	if msg.ToolName != "Bash" {
		t.Errorf("msg.ToolName = %q, want %q", msg.ToolName, "Bash")
	}
}

func TestParseStreamLineResult(t *testing.T) {
	line := `{"type":"result","result":"Task completed","is_error":false}`
	a := &Agent{}
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "done" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "done")
	}
	if msg.Result != "Task completed" {
		t.Errorf("msg.Result = %q, want %q", msg.Result, "Task completed")
	}
	if msg.IsError {
		t.Error("msg.IsError = true, want false")
	}
}

func TestParseStreamLineError(t *testing.T) {
	line := `{"type":"error","error":"something went wrong","is_error":true}`
	a := &Agent{}
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "error" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "error")
	}
	if msg.Text != "something went wrong" {
		t.Errorf("msg.Text = %q, want %q", msg.Text, "something went wrong")
	}
	if !msg.IsError {
		t.Error("msg.IsError = false, want true")
	}
}

func TestParseStreamLineSystem(t *testing.T) {
	line := `{"type":"system","subtype":"status","result":"Processing..."}`
	a := &Agent{}
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg == nil {
		t.Fatal("ParseStreamLine returned nil for system message with result")
	}
	if msg.Type != "system" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "system")
	}
}

func TestParseStreamLineSystemInit(t *testing.T) {
	// init subtype should return nil message
	line := `{"type":"system","subtype":"init","result":"initialized"}`
	a := &Agent{}
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg != nil {
		t.Error("system init should return nil message")
	}
}

func TestParseStreamLineInvalid(t *testing.T) {
	a := &Agent{}
	msg, ok := a.ParseStreamLine("not json at all")
	if ok {
		t.Error("ParseStreamLine returned ok=true for invalid JSON")
	}
	if msg == nil {
		t.Fatal("ParseStreamLine returned nil for invalid JSON")
	}
	if msg.Type != "text" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "text")
	}
	if msg.Text != "not json at all" {
		t.Errorf("msg.Text = %q, want original line", msg.Text)
	}
}

func TestParseStreamLineEmpty(t *testing.T) {
	a := &Agent{}
	msg, ok := a.ParseStreamLine("")
	if ok {
		t.Error("ParseStreamLine returned ok=true for empty line")
	}
	if msg != nil {
		t.Error("ParseStreamLine should return nil for empty line")
	}
}

func TestBuildPrintModeCommand(t *testing.T) {
	a := &Agent{}

	t.Run("new session", func(t *testing.T) {
		config := &agent.Config{
			Model:    "test-model",
			MaxTurns: 10,
		}
		cmd := a.BuildPrintModeCommand("sess-123", false, config)
		if !strings.Contains(cmd, "--session-id") {
			t.Error("new session command should contain --session-id")
		}
		if !strings.Contains(cmd, "sess-123") {
			t.Error("command should contain session ID")
		}
		if !strings.Contains(cmd, "--model") {
			t.Error("command should contain --model")
		}
		if !strings.Contains(cmd, "test-model") {
			t.Error("command should contain model name")
		}
		if !strings.Contains(cmd, "--max-turns") {
			t.Error("command should contain --max-turns")
		}
	})

	t.Run("resume session", func(t *testing.T) {
		cmd := a.BuildPrintModeCommand("sess-123", true, &agent.Config{})
		if !strings.Contains(cmd, "--resume") {
			t.Error("resume command should contain --resume")
		}
		if strings.Contains(cmd, "--session-id") {
			t.Error("resume command should not contain --session-id")
		}
	})

	t.Run("skip permissions", func(t *testing.T) {
		config := &agent.Config{SkipPermissions: true}
		cmd := a.BuildPrintModeCommand("sess-123", false, config)
		if !strings.Contains(cmd, "--dangerously-skip-permissions") {
			t.Error("command should contain --dangerously-skip-permissions")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		cmd := a.BuildPrintModeCommand("sess-123", false, nil)
		if !strings.Contains(cmd, "--allowedTools") {
			t.Error("nil config should produce command with default --allowedTools")
		}
	})
}

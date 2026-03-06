package codex

import (
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/agent"
)

func TestParseStreamLineThreadStarted(t *testing.T) {
	a := &Agent{}
	line := `{"type":"thread.started","thread_id":"thread_abc123"}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg == nil {
		t.Fatal("ParseStreamLine returned nil message")
	}
	if msg.Type != "session_started" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "session_started")
	}
	if msg.SessionID != "thread_abc123" {
		t.Errorf("msg.SessionID = %q, want %q", msg.SessionID, "thread_abc123")
	}
}

func TestParseStreamLineThreadStartedNoID(t *testing.T) {
	a := &Agent{}
	line := `{"type":"thread.started"}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg != nil {
		t.Error("thread.started without thread_id should return nil message")
	}
}

func TestParseStreamLineTurnStarted(t *testing.T) {
	a := &Agent{}
	line := `{"type":"turn.started"}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg != nil {
		t.Error("turn.started should return nil message")
	}
}

func TestParseStreamLineAgentMessage(t *testing.T) {
	a := &Agent{}
	line := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Hello from Codex"}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg == nil {
		t.Fatal("ParseStreamLine returned nil message")
	}
	if msg.Type != "text" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "text")
	}
	if msg.Text != "Hello from Codex" {
		t.Errorf("msg.Text = %q, want %q", msg.Text, "Hello from Codex")
	}
}

func TestParseStreamLineReasoning(t *testing.T) {
	a := &Agent{}
	line := `{"type":"item.completed","item":{"id":"item_1","type":"reasoning","text":"thinking..."}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg != nil {
		t.Error("reasoning items should return nil message (hidden)")
	}
}

func TestParseStreamLineToolCall(t *testing.T) {
	a := &Agent{}
	line := `{"type":"item.completed","item":{"id":"tool_1","type":"tool_call","name":"shell","arguments":"{\"command\":\"ls -la\"}"}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg == nil {
		t.Fatal("ParseStreamLine returned nil message")
	}
	if msg.Type != "tool_use" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "tool_use")
	}
	if msg.ToolName != "shell" {
		t.Errorf("msg.ToolName = %q, want %q", msg.ToolName, "shell")
	}
	if msg.ToolID != "tool_1" {
		t.Errorf("msg.ToolID = %q, want %q", msg.ToolID, "tool_1")
	}
}

func TestParseStreamLineFunctionCall(t *testing.T) {
	a := &Agent{}
	line := `{"type":"item.completed","item":{"id":"fn_1","type":"function_call","name":"file_read","arguments":"{\"path\":\"/tmp/test.go\"}"}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "tool_use" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "tool_use")
	}
	if msg.ToolName != "file_read" {
		t.Errorf("msg.ToolName = %q, want %q", msg.ToolName, "file_read")
	}
}

func TestParseStreamLineToolResult(t *testing.T) {
	a := &Agent{}
	line := `{"type":"item.completed","item":{"type":"tool_result","output":"file contents here"}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "tool_result" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "tool_result")
	}
	if msg.Text != "file contents here" {
		t.Errorf("msg.Text = %q, want %q", msg.Text, "file contents here")
	}
}

func TestParseStreamLineFunctionResult(t *testing.T) {
	a := &Agent{}
	line := `{"type":"item.completed","item":{"type":"function_result","output":"result data"}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "tool_result" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "tool_result")
	}
}

func TestParseStreamLineTurnCompleted(t *testing.T) {
	a := &Agent{}
	line := `{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg == nil {
		t.Fatal("ParseStreamLine returned nil message")
	}
	if msg.Type != "done" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "done")
	}
}

func TestParseStreamLineError(t *testing.T) {
	a := &Agent{}
	line := `{"type":"error","message":"rate limit exceeded"}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Type != "error" {
		t.Errorf("msg.Type = %q, want %q", msg.Type, "error")
	}
	if msg.Text != "rate limit exceeded" {
		t.Errorf("msg.Text = %q, want %q", msg.Text, "rate limit exceeded")
	}
	if !msg.IsError {
		t.Error("msg.IsError = false, want true")
	}
}

func TestParseStreamLineErrorWithItemText(t *testing.T) {
	a := &Agent{}
	line := `{"type":"error","message":"generic","item":{"text":"detailed error info"}}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false")
	}
	if msg.Text != "detailed error info" {
		t.Errorf("msg.Text = %q, want %q (item.text should override message)", msg.Text, "detailed error info")
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

func TestParseStreamLineInvalidJSON(t *testing.T) {
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

func TestParseStreamLineUnknownType(t *testing.T) {
	a := &Agent{}
	line := `{"type":"unknown.event"}`
	msg, ok := a.ParseStreamLine(line)
	if !ok {
		t.Fatal("ParseStreamLine returned ok=false for unknown type")
	}
	if msg != nil {
		t.Error("unknown event type should return nil message")
	}
}

func TestBuildPrintModeCommand(t *testing.T) {
	a := &Agent{}

	t.Run("new session", func(t *testing.T) {
		config := &agent.Config{Model: "gpt-4o"}
		cmd := a.BuildPrintModeCommand("sess-123", false, config)
		if !strings.Contains(cmd, "codex exec") {
			t.Error("command should contain 'codex exec'")
		}
		if !strings.Contains(cmd, "--json") {
			t.Error("command should contain --json")
		}
		if !strings.Contains(cmd, "--yolo") {
			t.Error("command should contain --yolo")
		}
		if !strings.Contains(cmd, "--model") {
			t.Error("command should contain --model")
		}
		if !strings.Contains(cmd, "gpt-4o") {
			t.Error("command should contain model name")
		}
		if strings.Contains(cmd, "resume") {
			t.Error("new session should not contain 'resume'")
		}
		// Should end with - for stdin
		if !strings.HasSuffix(strings.TrimSuffix(cmd, "'"), "-") {
			t.Error("command should end with - for stdin reading")
		}
	})

	t.Run("resume session", func(t *testing.T) {
		cmd := a.BuildPrintModeCommand("sess-456", true, &agent.Config{})
		if !strings.Contains(cmd, "resume") {
			t.Error("resume command should contain 'resume'")
		}
		if !strings.Contains(cmd, "sess-456") {
			t.Error("resume command should contain session ID")
		}
	})

	t.Run("resume without session ID", func(t *testing.T) {
		cmd := a.BuildPrintModeCommand("", true, &agent.Config{})
		if strings.Contains(cmd, "resume") {
			t.Error("resume with empty session ID should not contain 'resume'")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		cmd := a.BuildPrintModeCommand("sess-123", false, nil)
		if !strings.Contains(cmd, "--yolo") {
			t.Error("nil config should still include --yolo")
		}
		if strings.Contains(cmd, "--model") {
			t.Error("nil config should not include --model")
		}
	})

	t.Run("reasoning effort", func(t *testing.T) {
		config := &agent.Config{ReasoningEffort: "high"}
		cmd := a.BuildPrintModeCommand("sess-123", false, config)
		if !strings.Contains(cmd, "-c") {
			t.Error("command should contain -c flag for reasoning effort")
		}
		if !strings.Contains(cmd, "model_reasoning_effort=high") {
			t.Error("command should contain reasoning effort config")
		}
	})

	t.Run("wraps in bash", func(t *testing.T) {
		cmd := a.BuildPrintModeCommand("sess-123", false, &agent.Config{})
		if !strings.HasPrefix(cmd, "bash -l -c") {
			t.Errorf("command should start with 'bash -l -c', got: %s", cmd)
		}
		if !strings.Contains(cmd, `cd "$VIBESPACE_WORKDIR"`) {
			t.Error("command should cd to $VIBESPACE_WORKDIR")
		}
	})
}

func TestBuildInteractiveCommand(t *testing.T) {
	a := &Agent{}

	t.Run("new session", func(t *testing.T) {
		args := a.BuildInteractiveCommand("", &agent.Config{})
		cmd := strings.Join(args, " ")
		if cmd != "codex --yolo" {
			t.Errorf("cmd = %q, want %q", cmd, "codex --yolo")
		}
	})

	t.Run("resume session", func(t *testing.T) {
		args := a.BuildInteractiveCommand("sess-789", &agent.Config{})
		cmd := strings.Join(args, " ")
		if !strings.Contains(cmd, "codex resume sess-789") {
			t.Errorf("cmd = %q, should contain 'codex resume sess-789'", cmd)
		}
	})

	t.Run("with model", func(t *testing.T) {
		config := &agent.Config{Model: "o3"}
		args := a.BuildInteractiveCommand("", config)
		cmd := strings.Join(args, " ")
		if !strings.Contains(cmd, "--model") {
			t.Error("command should contain --model")
		}
		if !strings.Contains(cmd, "o3") {
			t.Error("command should contain model name")
		}
	})

	t.Run("returns slice not string", func(t *testing.T) {
		args := a.BuildInteractiveCommand("", &agent.Config{})
		if len(args) < 2 {
			t.Fatalf("expected at least 2 args, got %d", len(args))
		}
		if args[0] != "codex" {
			t.Errorf("first arg = %q, want %q", args[0], "codex")
		}
	})
}

func TestExtractCodexToolInput(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		args     string
		expected string
	}{
		{"shell command", "shell", `{"command":"ls -la /tmp"}`, "ls -la /tmp"},
		{"file_read path", "file_read", `{"path":"/tmp/test.go"}`, "/tmp/test.go"},
		{"file_write path", "file_write", `{"path":"/tmp/output.txt"}`, "/tmp/output.txt"},
		{"file_edit path", "file_edit", `{"path":"/tmp/edit.go"}`, "/tmp/edit.go"},
		{"web_search query", "web_search", `{"query":"golang testing"}`, "golang testing"},
		{"browser url", "browser", `{"url":"https://example.com"}`, "https://example.com"},
		{"unknown tool", "unknown_tool", `{"data":"value"}`, ""},
		{"empty arguments", "shell", ``, ""},
		{"invalid json", "shell", `not json`, ""},
		{"shell truncation", "shell", `{"command":"` + strings.Repeat("a", 100) + `"}`, strings.Repeat("a", 47) + "..."},
		{"browser url truncation", "browser", `{"url":"` + strings.Repeat("x", 100) + `"}`, strings.Repeat("x", 37) + "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCodexToolInput(tt.tool, []byte(tt.args))
			if result != tt.expected {
				t.Errorf("extractCodexToolInput(%q, ...) = %q, want %q", tt.tool, result, tt.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"toolong", 5, "to..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"ab", 1, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestAgentMetadata(t *testing.T) {
	a := &Agent{}

	if a.Type() != agent.TypeCodex {
		t.Errorf("Type() = %q, want %q", a.Type(), agent.TypeCodex)
	}
	if a.DisplayName() != "Codex CLI" {
		t.Errorf("DisplayName() = %q, want %q", a.DisplayName(), "Codex CLI")
	}
	if a.DefaultAgentPrefix() != "codex" {
		t.Errorf("DefaultAgentPrefix() = %q, want %q", a.DefaultAgentPrefix(), "codex")
	}
	if a.ConfigDirectory() != ".codex" {
		t.Errorf("ConfigDirectory() = %q, want %q", a.ConfigDirectory(), ".codex")
	}
	if a.SessionIDFlag() != "" {
		t.Errorf("SessionIDFlag() = %q, want empty", a.SessionIDFlag())
	}
	if a.ResumeFlag() != "resume" {
		t.Errorf("ResumeFlag() = %q, want %q", a.ResumeFlag(), "resume")
	}

	tools := a.SupportedTools()
	if len(tools) != 6 {
		t.Errorf("SupportedTools() returned %d tools, want 6", len(tools))
	}

	if err := a.ValidateConfig(nil); err != nil {
		t.Errorf("ValidateConfig(nil) = %v, want nil", err)
	}
	if err := a.ValidateConfig(&agent.Config{Model: "test"}); err != nil {
		t.Errorf("ValidateConfig(config) = %v, want nil", err)
	}
}

package cli

import (
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

func TestPrintAgentConfigClaude(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{
		SkipPermissions:  false,
		ShareCredentials: false,
		Model:            "opus",
		MaxTurns:         10,
	}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})
	stripped := ui.StripAnsi(got)

	// Header
	if !strings.Contains(stripped, "claude-1") {
		t.Error("should contain agent name")
	}
	if !strings.Contains(stripped, "claude-code") {
		t.Error("should contain agent type")
	}

	// Fields
	for _, want := range []string{
		"skip_permissions",
		"false",
		"share_credentials",
		"allowed_tools",
		"disallowed_tools",
		"model",
		"opus",
		"max_turns",
		"10",
	} {
		if !strings.Contains(stripped, want) {
			t.Errorf("output missing %q:\n%s", want, stripped)
		}
	}
}

func TestPrintAgentConfigCodex(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{
		Model:           "gpt-5.2-codex",
		ReasoningEffort: "high",
	}

	got := captureStdout(t, func() {
		printAgentConfig("codex-1", agent.TypeCodex, config)
	})
	stripped := ui.StripAnsi(got)

	if !strings.Contains(stripped, "codex-1") {
		t.Error("should contain agent name")
	}
	if !strings.Contains(stripped, "codex") {
		t.Error("should contain agent type")
	}
	if !strings.Contains(stripped, "always") {
		t.Error("codex should show skip_permissions=always")
	}
	if !strings.Contains(stripped, "gpt-5.2-codex") {
		t.Error("should contain model name")
	}
	if !strings.Contains(stripped, "reasoning_effort") {
		t.Error("should contain reasoning_effort for codex")
	}
	if !strings.Contains(stripped, "high") {
		t.Error("should contain reasoning effort value")
	}
}

func TestPrintAgentConfigSkipPermissions(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{
		SkipPermissions: true,
	}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})
	stripped := ui.StripAnsi(got)

	// When skip_permissions is true, allowed_tools should show "all"
	lines := strings.Split(stripped, "\n")
	for _, line := range lines {
		if strings.Contains(line, "skip_permissions") && !strings.Contains(line, "true") {
			t.Errorf("skip_permissions should be 'true', got line: %s", line)
		}
		if strings.Contains(line, "allowed_tools") && !strings.Contains(line, "all") {
			t.Errorf("allowed_tools should be 'all' when skip_permissions=true, got line: %s", line)
		}
	}
}

func TestPrintAgentConfigCustomTools(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{
		AllowedTools:    []string{"Bash", "Read", "Write"},
		DisallowedTools: []string{"Edit"},
	}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})
	stripped := ui.StripAnsi(got)

	if !strings.Contains(stripped, "Bash, Read, Write") {
		t.Errorf("should contain allowed tools, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "Edit") {
		t.Errorf("should contain disallowed tools, got:\n%s", stripped)
	}
}

func TestPrintAgentConfigDefaultTools(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})
	stripped := ui.StripAnsi(got)

	if !strings.Contains(stripped, "(default)") {
		t.Errorf("should show (default) for default allowed tools, got:\n%s", stripped)
	}
}

func TestPrintAgentConfigDefaults(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})
	stripped := ui.StripAnsi(got)

	// Default model
	if !strings.Contains(stripped, "default") {
		t.Error("empty model should show 'default'")
	}
	// Default max_turns
	if !strings.Contains(stripped, "unlimited") {
		t.Error("zero max_turns should show 'unlimited'")
	}
}

func TestPrintAgentConfigSystemPrompt(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{
		SystemPrompt: "You are a helpful assistant that writes Go code",
	}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})
	stripped := ui.StripAnsi(got)

	if !strings.Contains(stripped, "system_prompt") {
		t.Error("should contain system_prompt label")
	}
	// Long prompt should be truncated
	if !strings.Contains(stripped, "...") {
		t.Error("long system prompt should be truncated with ...")
	}
}

func TestPrintAgentConfigNoSystemPrompt(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})

	if strings.Contains(got, "system_prompt") {
		t.Error("should not show system_prompt when empty")
	}
}

func TestPrintAgentConfigShareCredentials(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})

	config := &agent.Config{
		ShareCredentials: true,
	}

	got := captureStdout(t, func() {
		printAgentConfig("claude-1", agent.TypeClaudeCode, config)
	})
	stripped := ui.StripAnsi(got)

	lines := strings.Split(stripped, "\n")
	for _, line := range lines {
		if strings.Contains(line, "share_credentials") && !strings.Contains(line, "true") {
			t.Errorf("share_credentials should be 'true', got line: %s", line)
		}
	}
}

package agent

import (
	"strings"
	"testing"
)

func TestParseType(t *testing.T) {
	tests := []struct {
		input string
		want  Type
	}{
		{"claude-code", TypeClaudeCode},
		{"claude", TypeClaudeCode},
		{"codex", TypeCodex},
		{"", TypeClaudeCode},
		{"CLAUDE-CODE", TypeClaudeCode},
		{"  codex  ", TypeCodex},
		{"unknown", TypeClaudeCode},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseType(tt.input); got != tt.want {
				t.Errorf("ParseType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	if !TypeClaudeCode.IsValid() {
		t.Error("TypeClaudeCode.IsValid() = false, want true")
	}
	if !TypeCodex.IsValid() {
		t.Error("TypeCodex.IsValid() = false, want true")
	}
	if Type("unknown").IsValid() {
		t.Error(`Type("unknown").IsValid() = true, want false`)
	}
}

func TestConfigClone(t *testing.T) {
	original := &Config{
		SkipPermissions: true,
		AllowedTools:    []string{"Read", "Write"},
		Model:           "test-model",
		Extra:           map[string]interface{}{"key": "value"},
	}

	clone := original.Clone()

	// Verify values are equal
	if clone.SkipPermissions != original.SkipPermissions {
		t.Error("Clone did not preserve SkipPermissions")
	}
	if clone.Model != original.Model {
		t.Error("Clone did not preserve Model")
	}

	// Verify slices are independent
	clone.AllowedTools[0] = "modified"
	if original.AllowedTools[0] == "modified" {
		t.Error("Clone shares AllowedTools slice with original")
	}

	// Verify maps are independent
	clone.Extra["key"] = "changed"
	if original.Extra["key"] == "changed" {
		t.Error("Clone shares Extra map with original")
	}
}

func TestConfigCloneNil(t *testing.T) {
	var c *Config
	if c.Clone() != nil {
		t.Error("nil.Clone() should return nil")
	}
}

func TestConfigMerge(t *testing.T) {
	base := &Config{
		Model:        "base-model",
		AllowedTools: []string{"Read"},
		MaxTurns:     5,
	}

	override := &Config{
		Model:           "override-model",
		SkipPermissions: true,
	}

	result := base.Merge(override)

	if result.Model != "override-model" {
		t.Errorf("Merge Model = %q, want %q", result.Model, "override-model")
	}
	if !result.SkipPermissions {
		t.Error("Merge should set SkipPermissions from override")
	}
	if result.MaxTurns != 5 {
		t.Errorf("Merge MaxTurns = %d, want 5 (preserved from base)", result.MaxTurns)
	}
}

func TestConfigMergeNilBase(t *testing.T) {
	var base *Config
	other := &Config{Model: "test"}
	result := base.Merge(other)
	if result.Model != "test" {
		t.Error("nil.Merge(other) should return clone of other")
	}
}

func TestConfigMergeNilOther(t *testing.T) {
	base := &Config{Model: "test"}
	result := base.Merge(nil)
	if result.Model != "test" {
		t.Error("base.Merge(nil) should return base")
	}
}

func TestConfigIsEmpty(t *testing.T) {
	var nilConfig *Config
	if !nilConfig.IsEmpty() {
		t.Error("nil config should be empty")
	}

	if !(&Config{}).IsEmpty() {
		t.Error("zero config should be empty")
	}

	if (&Config{Model: "test"}).IsEmpty() {
		t.Error("config with Model should not be empty")
	}

	if (&Config{SkipPermissions: true}).IsEmpty() {
		t.Error("config with SkipPermissions should not be empty")
	}

	if (&Config{AllowedTools: []string{"Read"}}).IsEmpty() {
		t.Error("config with AllowedTools should not be empty")
	}
}

func TestDefaultAllowedTools(t *testing.T) {
	tools := DefaultAllowedTools()
	if len(tools) != 6 {
		t.Errorf("DefaultAllowedTools() returned %d items, want 6", len(tools))
	}

	expected := []string{"Bash(read_only:true)", "Read", "Write", "Edit", "Glob", "Grep"}
	for i, want := range expected {
		if tools[i] != want {
			t.Errorf("DefaultAllowedTools()[%d] = %q, want %q", i, tools[i], want)
		}
	}
}

func TestAllowedToolsString(t *testing.T) {
	c := &Config{AllowedTools: []string{"Read", "Write", "Bash"}}
	got := c.AllowedToolsString()
	if got != "Read,Write,Bash" {
		t.Errorf("AllowedToolsString() = %q, want %q", got, "Read,Write,Bash")
	}
}

func TestAllowedToolsStringDefault(t *testing.T) {
	c := &Config{}
	got := c.AllowedToolsString()
	if !strings.Contains(got, "Read") {
		t.Errorf("empty config AllowedToolsString() should contain default tools, got %q", got)
	}
}

func TestConfigValidate(t *testing.T) {
	c := &Config{
		Model:           "test",
		MaxTurns:        10,
		SystemPrompt:    "test prompt",
		ReasoningEffort: "high",
	}
	if c.IsEmpty() {
		t.Error("config with multiple fields set should not be empty")
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"it's", `'it'\''s'`},
		{"$(rm -rf /)", "'$(rm -rf /)'"},
		{"`whoami`", "'`whoami`'"},
		{"a;b", "'a;b'"},
		{"a|b", "'a|b'"},
		{"", "''"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ShellQuote(tt.input)
			if got != tt.want {
				t.Errorf("ShellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShellQuoteArgs(t *testing.T) {
	args := []string{"claude", "--resume", "abc-123"}
	got := ShellQuoteArgs(args)
	want := "'claude' '--resume' 'abc-123'"
	if got != want {
		t.Errorf("ShellQuoteArgs = %q, want %q", got, want)
	}
}

func TestShellQuoteArgsInjection(t *testing.T) {
	// Simulate a malicious session ID
	args := []string{"claude", "--resume", "'; rm -rf / #"}
	got := ShellQuoteArgs(args)
	// The single quote in the malicious input should be escaped
	if !strings.Contains(got, `'\''`) {
		t.Errorf("ShellQuoteArgs did not escape single quote in: %s", got)
	}
	// Should not contain unescaped single quote that would break out
	// Count opening/closing quotes — every arg produces 'arg' with internal ' escaped
	if strings.Count(got, "rm -rf") != 1 {
		t.Error("malicious command should be contained within quotes")
	}
}

func TestWrapForSSHRemote(t *testing.T) {
	args := []string{"claude", "--model", "test-model"}
	got := WrapForSSHRemote(args)
	if !strings.HasPrefix(got, "bash -l -c '") {
		t.Errorf("WrapForSSHRemote should start with bash -l -c, got: %s", got)
	}
	if !strings.Contains(got, `cd "$VIBESPACE_WORKDIR"`) {
		t.Error("should contain cd to VIBESPACE_WORKDIR")
	}
	if !strings.Contains(got, "claude") {
		t.Error("should contain claude command")
	}
}

func TestWrapForSSHRemoteParentheses(t *testing.T) {
	// Ensure Bash(read_only:true) doesn't cause syntax errors
	args := []string{"claude", "--allowedTools", "Bash(read_only:true),Read,Write"}
	got := WrapForSSHRemote(args)
	// The value should be double-quoted inside the single-quoted bash -c context
	if !strings.Contains(got, `"Bash(read_only:true),Read,Write"`) {
		t.Errorf("parenthesized arg should be double-quoted, got: %s", got)
	}
}

func TestDoubleQuoteForBash(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"simple", "hello", `"hello"`},
		{"with dollar", "$HOME", `"\$HOME"`},
		{"with backtick", "`cmd`", "\"\\`cmd\\`\""},
		{"with double quote", `say "hi"`, `"say \"hi\""`},
		{"with backslash", `a\b`, `"a\\b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DoubleQuoteForBash(tt.input)
			if got != tt.want {
				t.Errorf("DoubleQuoteForBash(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestJoinArgsForBash(t *testing.T) {
	args := []string{"claude", "-p", "--model", "$(evil)", "--resume", "sess-123"}
	got := JoinArgsForBash(args)
	// Flags and command names should be unquoted
	if !strings.Contains(got, "claude ") {
		t.Error("claude should be unquoted")
	}
	if !strings.Contains(got, " -p ") {
		t.Error("-p should be unquoted")
	}
	// User values should be double-quoted with $ escaped (parens are safe in double quotes)
	if !strings.Contains(got, `"\$(evil)"`) {
		t.Errorf("$(evil) should be double-quoted with $ escaped, got: %s", got)
	}
}

package agent

import (
	"testing"
)

func TestConfigIsEmptyDefault(t *testing.T) {
	c := &Config{}
	if !c.IsEmpty() {
		t.Error("default Config should be empty")
	}
}

func TestConfigIsEmptyWithModel(t *testing.T) {
	c := &Config{Model: "opus"}
	if c.IsEmpty() {
		t.Error("Config with Model set should not be empty")
	}
}

func TestConfigIsEmptyWithSkipPermissions(t *testing.T) {
	c := &Config{SkipPermissions: true}
	if c.IsEmpty() {
		t.Error("Config with SkipPermissions should not be empty")
	}
}

func TestConfigIsEmptyWithMaxTurns(t *testing.T) {
	c := &Config{MaxTurns: 10}
	if c.IsEmpty() {
		t.Error("Config with MaxTurns should not be empty")
	}
}

func TestConfigIsEmptyWithSystemPrompt(t *testing.T) {
	c := &Config{SystemPrompt: "you are helpful"}
	if c.IsEmpty() {
		t.Error("Config with SystemPrompt should not be empty")
	}
}

func TestConfigIsEmptyWithDisallowedTools(t *testing.T) {
	c := &Config{DisallowedTools: []string{"Write"}}
	if c.IsEmpty() {
		t.Error("Config with DisallowedTools should not be empty")
	}
}

func TestConfigCloneDeepCopy(t *testing.T) {
	c := &Config{
		Model:            "opus",
		SkipPermissions:  true,
		AllowedTools:     []string{"Bash", "Read"},
		DisallowedTools:  []string{"Write"},
		MaxTurns:         10,
		SystemPrompt:     "test prompt",
		ReasoningEffort:  "high",
		ShareCredentials: true,
		Extra:            map[string]interface{}{"key": "value"},
	}

	cloned := c.Clone()

	// Verify all values match
	if cloned.Model != c.Model {
		t.Errorf("cloned Model = %q, want %q", cloned.Model, c.Model)
	}
	if cloned.SkipPermissions != c.SkipPermissions {
		t.Errorf("cloned SkipPermissions = %v, want %v", cloned.SkipPermissions, c.SkipPermissions)
	}
	if cloned.ReasoningEffort != c.ReasoningEffort {
		t.Errorf("cloned ReasoningEffort = %q, want %q", cloned.ReasoningEffort, c.ReasoningEffort)
	}
	if cloned.ShareCredentials != c.ShareCredentials {
		t.Errorf("cloned ShareCredentials = %v, want %v", cloned.ShareCredentials, c.ShareCredentials)
	}
	if cloned.MaxTurns != c.MaxTurns {
		t.Errorf("cloned MaxTurns = %d, want %d", cloned.MaxTurns, c.MaxTurns)
	}
	if cloned.SystemPrompt != c.SystemPrompt {
		t.Errorf("cloned SystemPrompt = %q, want %q", cloned.SystemPrompt, c.SystemPrompt)
	}
	if len(cloned.AllowedTools) != len(c.AllowedTools) {
		t.Errorf("cloned AllowedTools length = %d, want %d", len(cloned.AllowedTools), len(c.AllowedTools))
	}
	if len(cloned.DisallowedTools) != len(c.DisallowedTools) {
		t.Errorf("cloned DisallowedTools length = %d, want %d", len(cloned.DisallowedTools), len(c.DisallowedTools))
	}

	// Verify it's a deep copy - modifying clone doesn't affect original
	cloned.AllowedTools[0] = "modified"
	if c.AllowedTools[0] == "modified" {
		t.Error("modifying clone AllowedTools should not affect original")
	}

	cloned.DisallowedTools[0] = "modified"
	if c.DisallowedTools[0] == "modified" {
		t.Error("modifying clone DisallowedTools should not affect original")
	}

	cloned.Extra["key"] = "changed"
	if c.Extra["key"] == "changed" {
		t.Error("modifying clone Extra should not affect original")
	}
}

func TestConfigCloneEmptySlices(t *testing.T) {
	c := &Config{
		Model: "opus",
	}
	cloned := c.Clone()
	if cloned.AllowedTools != nil {
		t.Error("nil AllowedTools should remain nil after clone")
	}
	if cloned.DisallowedTools != nil {
		t.Error("nil DisallowedTools should remain nil after clone")
	}
	if cloned.Extra != nil {
		t.Error("nil Extra should remain nil after clone")
	}
}

func TestGetAllowedToolsNilConfig(t *testing.T) {
	var c *Config
	tools := c.GetAllowedTools()
	if len(tools) != len(DefaultAllowedTools()) {
		t.Errorf("nil config GetAllowedTools() = %d tools, want %d (defaults)", len(tools), len(DefaultAllowedTools()))
	}
}

func TestGetAllowedToolsEmptySlice(t *testing.T) {
	c := &Config{AllowedTools: []string{}}
	tools := c.GetAllowedTools()
	if len(tools) != len(DefaultAllowedTools()) {
		t.Errorf("empty AllowedTools GetAllowedTools() = %d tools, want %d (defaults)", len(tools), len(DefaultAllowedTools()))
	}
}

func TestGetAllowedToolsCustom(t *testing.T) {
	c := &Config{AllowedTools: []string{"Read", "Write"}}
	tools := c.GetAllowedTools()
	if len(tools) != 2 {
		t.Errorf("custom AllowedTools GetAllowedTools() = %d tools, want 2", len(tools))
	}
	if tools[0] != "Read" || tools[1] != "Write" {
		t.Errorf("GetAllowedTools() = %v, want [Read Write]", tools)
	}
}

func TestDisallowedToolsStringNilConfig(t *testing.T) {
	var c *Config
	got := c.DisallowedToolsString()
	if got != "" {
		t.Errorf("nil config DisallowedToolsString() = %q, want empty", got)
	}
}

func TestDisallowedToolsStringEmpty(t *testing.T) {
	c := &Config{}
	got := c.DisallowedToolsString()
	if got != "" {
		t.Errorf("empty config DisallowedToolsString() = %q, want empty", got)
	}
}

func TestDisallowedToolsStringMultiple(t *testing.T) {
	c := &Config{DisallowedTools: []string{"Bash", "Write", "Edit"}}
	got := c.DisallowedToolsString()
	if got != "Bash,Write,Edit" {
		t.Errorf("DisallowedToolsString() = %q, want %q", got, "Bash,Write,Edit")
	}
}

func TestConfigMergePreservesReasoningEffort(t *testing.T) {
	base := &Config{ReasoningEffort: "low"}
	override := &Config{ReasoningEffort: "high"}
	result := base.Merge(override)
	if result.ReasoningEffort != "high" {
		t.Errorf("Merge ReasoningEffort = %q, want %q", result.ReasoningEffort, "high")
	}
}

func TestConfigMergePreservesShareCredentials(t *testing.T) {
	base := &Config{}
	override := &Config{ShareCredentials: true}
	result := base.Merge(override)
	if !result.ShareCredentials {
		t.Error("Merge should set ShareCredentials from override")
	}
}

func TestConfigMergeExtraFields(t *testing.T) {
	base := &Config{Extra: map[string]interface{}{"a": "1"}}
	override := &Config{Extra: map[string]interface{}{"b": "2"}}
	result := base.Merge(override)

	if result.Extra["a"] != "1" {
		t.Error("Merge should preserve base Extra fields")
	}
	if result.Extra["b"] != "2" {
		t.Error("Merge should add override Extra fields")
	}
}

func TestParseTypeCaseSensitivity(t *testing.T) {
	tests := []struct {
		input string
		want  Type
	}{
		{"CLAUDE-CODE", TypeClaudeCode},
		{"Claude-Code", TypeClaudeCode},
		{"CODEX", TypeCodex},
		{"Codex", TypeCodex},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseType(tt.input); got != tt.want {
				t.Errorf("ParseType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTypeString(t *testing.T) {
	if TypeClaudeCode.String() != "claude-code" {
		t.Errorf("TypeClaudeCode.String() = %q, want %q", TypeClaudeCode.String(), "claude-code")
	}
	if TypeCodex.String() != "codex" {
		t.Errorf("TypeCodex.String() = %q, want %q", TypeCodex.String(), "codex")
	}
}

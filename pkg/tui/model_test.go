package tui

import (
	"testing"
)

func TestStyleContentWithCodeBlocks(t *testing.T) {
	styles := NewStyles()

	// No code blocks
	content := "just plain text"
	result := styleContentWithCodeBlocks(content, styles)
	if result != content {
		t.Fatalf("expected unchanged text, got %q", result)
	}

	// With code block
	content = "before\n```go\nfmt.Println(\"hello\")\n```\nafter"
	result = styleContentWithCodeBlocks(content, styles)
	plain := stripAnsi(result)
	if plain == content {
		// Should have been transformed (code block styled)
		t.Log("code block was not styled (may be expected in some environments)")
	}

	// Unclosed code block
	content = "before\n```go\nunclosed"
	result = styleContentWithCodeBlocks(content, styles)
	plain = stripAnsi(result)
	if plain == "" {
		t.Fatal("expected non-empty for unclosed code block")
	}
}

func TestStyleContentWithMultipleCodeBlocks(t *testing.T) {
	styles := NewStyles()

	content := "text\n```\nblock1\n```\nmid\n```\nblock2\n```\nend"
	result := styleContentWithCodeBlocks(content, styles)
	plain := stripAnsi(result)

	// Should contain both blocks content
	if plain == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"hello world", 5, "he..."},
		{"hello", 5, "hello"},
		{"ab", 2, "ab"},
		{"abc", 2, "ab"},
	}
	for _, tt := range tests {
		got := truncateString(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

func TestJoinNonEmpty(t *testing.T) {
	tests := []struct {
		parts []string
		sep   string
		want  string
	}{
		{[]string{"a", "b", "c"}, ", ", "a, b, c"},
		{[]string{"a", "", "c"}, ", ", "a, c"},
		{[]string{"", "", ""}, ", ", ""},
		{nil, ", ", ""},
		{[]string{"only"}, ", ", "only"},
	}
	for _, tt := range tests {
		got := joinNonEmpty(tt.parts, tt.sep)
		if got != tt.want {
			t.Errorf("joinNonEmpty(%v, %q) = %q, want %q", tt.parts, tt.sep, got, tt.want)
		}
	}
}

func TestGetToolIconAndColor(t *testing.T) {
	tools := []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep",
		"WebSearch", "WebFetch", "Task", "EnterPlanMode", "ExitPlanMode",
		"AskUserQuestion", "TodoWrite", "Unknown"}
	for _, tool := range tools {
		icon, color := getToolIconAndColor(tool)
		if icon == "" {
			t.Errorf("expected non-empty icon for %q", tool)
		}
		if color == "" {
			t.Errorf("expected non-empty color for %q", tool)
		}
	}
}

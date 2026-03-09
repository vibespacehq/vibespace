package tui

import (
	"strings"
	"testing"
)

func TestRenderMarkdownPlainText(t *testing.T) {
	content := "just plain text"
	result := renderMarkdown(content)
	plain := stripAnsi(result)
	if !strings.Contains(plain, "just plain text") {
		t.Fatalf("expected plain text preserved, got %q", plain)
	}
}

func TestRenderMarkdownCodeBlock(t *testing.T) {
	content := "before\n```go\nfmt.Println(\"hello\")\n```\nafter"
	result := renderMarkdown(content)
	plain := stripAnsi(result)
	if !strings.Contains(plain, "before") {
		t.Error("expected 'before' in output")
	}
	if !strings.Contains(plain, "Println") {
		t.Error("expected code content in output")
	}
	if !strings.Contains(plain, "after") {
		t.Error("expected 'after' in output")
	}
}

func TestRenderMarkdownMultipleCodeBlocks(t *testing.T) {
	content := "text\n```\nblock1\n```\nmid\n```\nblock2\n```\nend"
	result := renderMarkdown(content)
	plain := stripAnsi(result)
	if !strings.Contains(plain, "block1") {
		t.Error("expected 'block1' in output")
	}
	if !strings.Contains(plain, "block2") {
		t.Error("expected 'block2' in output")
	}
}

func TestRenderMarkdownFormatting(t *testing.T) {
	content := "**bold** and *italic* and `code`"
	result := renderMarkdown(content)
	plain := stripAnsi(result)
	if !strings.Contains(plain, "bold") {
		t.Error("expected 'bold' in output")
	}
	if !strings.Contains(plain, "italic") {
		t.Error("expected 'italic' in output")
	}
	if !strings.Contains(plain, "code") {
		t.Error("expected 'code' in output")
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

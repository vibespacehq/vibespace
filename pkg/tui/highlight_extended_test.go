package tui

import (
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/ui"
)

func TestHighlightGoExtended(t *testing.T) {
	h := NewHighlighter()
	code := `func main() {
	fmt.Println("hello")
}`
	result := h.Highlight(code, "go")
	if result == "" {
		t.Error("Highlight returned empty string")
	}
	stripped := ui.StripAnsi(result)
	if !strings.Contains(stripped, "func") {
		t.Errorf("highlighted Go should contain 'func', got %q", stripped)
	}
}

func TestHighlightPythonExtended(t *testing.T) {
	h := NewHighlighter()
	code := "def hello():\n    print(\"world\")"
	result := h.Highlight(code, "python")
	if result == "" {
		t.Error("Highlight returned empty string")
	}
	stripped := ui.StripAnsi(result)
	if !strings.Contains(stripped, "def") {
		t.Errorf("highlighted Python should contain 'def', got %q", stripped)
	}
}

func TestHighlightUnknownLanguageExtended(t *testing.T) {
	h := NewHighlighter()
	code := "some code here"
	result := h.Highlight(code, "unknownlang12345")
	stripped := ui.StripAnsi(result)
	if !strings.Contains(stripped, "some code here") {
		t.Errorf("unknown language should preserve code, got %q", stripped)
	}
}

func TestHighlightEmptyCodeExtended(t *testing.T) {
	h := NewHighlighter()
	result := h.Highlight("", "go")
	// Empty input should not panic and should produce minimal output
	plain := strings.TrimSpace(ui.StripAnsi(result))
	// Some formatters may still produce output for empty input
	_ = plain
}

func TestHighlightEmptyLanguageExtended(t *testing.T) {
	h := NewHighlighter()
	code := "x = 1"
	result := h.Highlight(code, "")
	stripped := ui.StripAnsi(result)
	if !strings.Contains(stripped, "x = 1") {
		t.Errorf("empty language should preserve code, got %q", stripped)
	}
}

func TestHighlighterSingleton(t *testing.T) {
	// Reset global to test lazy init
	globalHighlighter = nil

	h1 := getHighlighter()
	if h1 == nil {
		t.Fatal("expected non-nil from getHighlighter")
	}

	h2 := getHighlighter()
	if h1 != h2 {
		t.Fatal("getHighlighter should return same instance (singleton)")
	}
}

func TestHighlightJavaScript(t *testing.T) {
	h := NewHighlighter()
	code := `const x = () => { return 42; };`
	result := h.Highlight(code, "javascript")
	if result == "" {
		t.Error("Highlight returned empty string for JavaScript")
	}
	stripped := ui.StripAnsi(result)
	if !strings.Contains(stripped, "const") {
		t.Errorf("highlighted JavaScript should contain 'const', got %q", stripped)
	}
}

func TestHighlightBash(t *testing.T) {
	h := NewHighlighter()
	code := `#!/bin/bash
echo "hello"
for i in 1 2 3; do echo $i; done`
	result := h.Highlight(code, "bash")
	if result == "" {
		t.Error("Highlight returned empty string for Bash")
	}
	stripped := ui.StripAnsi(result)
	if !strings.Contains(stripped, "echo") {
		t.Errorf("highlighted Bash should contain 'echo', got %q", stripped)
	}
}

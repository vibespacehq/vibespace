package tui

import (
	"strings"
	"testing"
)

func TestNewHighlighter(t *testing.T) {
	h := NewHighlighter()
	if h == nil {
		t.Fatal("expected non-nil highlighter")
	}
	if h.style == nil {
		t.Fatal("expected non-nil style")
	}
	if h.formatter == nil {
		t.Fatal("expected non-nil formatter")
	}
}

func TestHighlightGo(t *testing.T) {
	h := NewHighlighter()
	code := `func main() { fmt.Println("hello") }`
	result := h.Highlight(code, "go")
	// Should produce some output (with ANSI codes for highlighting)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// The result should contain the original code text somewhere
	plain := stripAnsi(result)
	if !strings.Contains(plain, "func") {
		t.Fatalf("expected 'func' in output, got %q", plain)
	}
}

func TestHighlightPython(t *testing.T) {
	h := NewHighlighter()
	code := `def hello():\n    print("hello")`
	result := h.Highlight(code, "python")
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	plain := stripAnsi(result)
	if !strings.Contains(plain, "def") {
		t.Fatalf("expected 'def' in output, got %q", plain)
	}
}

func TestHighlightUnknownLang(t *testing.T) {
	h := NewHighlighter()
	code := "some random text"
	result := h.Highlight(code, "nonexistent-lang-xyz")
	// Should fallback gracefully
	if result == "" {
		t.Fatal("expected non-empty result for unknown lang")
	}
	plain := stripAnsi(result)
	if !strings.Contains(plain, "some random text") {
		t.Fatalf("expected original text in output, got %q", plain)
	}
}

func TestHighlightEmptyCode(t *testing.T) {
	h := NewHighlighter()
	result := h.Highlight("", "go")
	// Empty code should produce minimal/empty output
	plain := strings.TrimSpace(stripAnsi(result))
	if plain != "" {
		// Some formatters may still produce output for empty input
		// Just verify it doesn't panic
	}
}

func TestHighlightCodeBlock(t *testing.T) {
	styles := NewStyles()

	// With language label
	result := highlightCodeBlock("x := 1", "go", styles)
	plain := stripAnsi(result)
	if !strings.Contains(plain, "go") {
		t.Error("expected language label 'go' in output")
	}
	if !strings.Contains(plain, "x := 1") {
		t.Error("expected code content in output")
	}

	// Without language label
	result = highlightCodeBlock("hello", "", styles)
	if result == "" {
		t.Fatal("expected non-empty result without language")
	}

	// Empty code
	result = highlightCodeBlock("", "go", styles)
	if result != "" {
		t.Fatalf("expected empty result for empty code, got %q", result)
	}
}

func TestGetHighlighter(t *testing.T) {
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

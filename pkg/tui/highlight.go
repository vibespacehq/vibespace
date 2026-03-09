package tui

import (
	"bytes"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/vibespacehq/vibespace/pkg/config"
)

// Highlighter provides syntax highlighting for code blocks
type Highlighter struct {
	style     *chroma.Style
	formatter chroma.Formatter
}

// NewHighlighter creates a new syntax highlighter
func NewHighlighter() *Highlighter {
	// Use configured syntax theme (default: monokai)
	themeName := config.Global().TUI.SyntaxTheme
	style := styles.Get(themeName)
	if style == nil {
		style = styles.Fallback
	}

	// Use terminal256 formatter for wide terminal compatibility
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	return &Highlighter{
		style:     style,
		formatter: formatter,
	}
}

// Highlight applies syntax highlighting to code based on language
func (h *Highlighter) Highlight(code, lang string) string {
	// Get lexer for the language
	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexers.Get(lang)
	}
	if lexer == nil {
		// Try to guess from content
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		// Fall back to plain text
		lexer = lexers.Fallback
	}

	// Coalesce runs of identical token types for cleaner output
	lexer = chroma.Coalesce(lexer)

	// Tokenize the code
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code // Return unhighlighted on error
	}

	// Format to string
	var buf bytes.Buffer
	if err := h.formatter.Format(&buf, h.style, iterator); err != nil {
		return code // Return unhighlighted on error
	}

	return buf.String()
}

// Global highlighter instance (lazy initialized)
var globalHighlighter *Highlighter

// getHighlighter returns the global highlighter, creating it if needed
func getHighlighter() *Highlighter {
	if globalHighlighter == nil {
		globalHighlighter = NewHighlighter()
	}
	return globalHighlighter
}


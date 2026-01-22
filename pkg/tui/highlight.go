package tui

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// Highlighter provides syntax highlighting for code blocks
type Highlighter struct {
	style     *chroma.Style
	formatter chroma.Formatter
}

// NewHighlighter creates a new syntax highlighter
func NewHighlighter() *Highlighter {
	// Use a dark theme that works well in terminal
	style := styles.Get("monokai")
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

// highlightCodeBlock applies syntax highlighting to a code block
func highlightCodeBlock(code, lang string, styles Styles) string {
	// Trim trailing whitespace but preserve leading for indentation
	code = strings.TrimRight(code, " \t\n")

	if code == "" {
		return ""
	}

	var result strings.Builder

	// Add language label if present
	if lang != "" {
		langLabel := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Render(lang)
		result.WriteString("\n" + langLabel + "\n")
	} else {
		result.WriteString("\n")
	}

	// Apply syntax highlighting
	highlighter := getHighlighter()
	highlighted := highlighter.Highlight(code, lang)

	// Style each line with background for code block appearance
	bgStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1E1E2E")).
		Padding(0, 1)

	lines := strings.Split(highlighted, "\n")
	for i, line := range lines {
		// Apply background to each line (preserves ANSI colors from highlighter)
		if line == "" {
			result.WriteString(bgStyle.Render(" "))
		} else {
			// We need to apply background without overriding foreground colors
			// Just add the line as-is since chroma already styled it
			result.WriteString(" " + line + " ")
		}
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	result.WriteString("\n")

	return result.String()
}

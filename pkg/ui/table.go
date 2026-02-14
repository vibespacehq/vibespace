package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ansiRegex matches ANSI SGR escape sequences for width measurement.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripAnsi removes ANSI escape sequences from a string.
func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// TableOptions configures table rendering.
type TableOptions struct {
	// NoColor disables all styling (for NO_COLOR env or non-TTY).
	NoColor bool
	// HeaderColor overrides the default header color (Teal).
	HeaderColor lipgloss.Color
	// HeaderStyle, if non-nil, overrides the default header style entirely.
	HeaderStyle *lipgloss.Style
}

// NewTable creates a styled table with the given headers and rows.
// If noColor is true, returns a plain text table instead.
func NewTable(headers []string, rows [][]string, noColor bool) string {
	return NewTableWithOptions(headers, rows, TableOptions{NoColor: noColor})
}

// NewTableWithOptions creates a styled table with custom options.
func NewTableWithOptions(headers []string, rows [][]string, opts TableOptions) string {
	if opts.NoColor {
		return renderPlainTable(headers, rows)
	}
	return renderColumnAligned(headers, rows, opts)
}

// renderColumnAligned renders a column-aligned table with colored headers.
func renderColumnAligned(headers []string, rows [][]string, opts TableOptions) string {
	var headerStyle lipgloss.Style
	if opts.HeaderStyle != nil {
		headerStyle = *opts.HeaderStyle
	} else {
		headerColor := opts.HeaderColor
		if headerColor == "" {
			headerColor = Teal
		}
		headerStyle = lipgloss.NewStyle().Bold(true).Foreground(headerColor)
	}
	gap := "  " // 2-space gap between columns

	// Compute max column widths (using visible width, stripping ANSI)
	numCols := len(headers)
	widths := make([]int, numCols)
	for i, h := range headers {
		w := lipgloss.Width(h)
		if w > widths[i] {
			widths[i] = w
		}
	}
	for _, row := range rows {
		for i := 0; i < numCols && i < len(row); i++ {
			w := lipgloss.Width(row[i])
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	var sb strings.Builder

	// Render headers
	for i, h := range headers {
		styled := headerStyle.Render(h)
		if i < numCols-1 {
			// Pad based on visible width
			visible := lipgloss.Width(h)
			padding := widths[i] - visible
			sb.WriteString(styled)
			sb.WriteString(strings.Repeat(" ", padding))
			sb.WriteString(gap)
		} else {
			sb.WriteString(styled)
		}
	}
	sb.WriteString("\n")

	// Render data rows
	for _, row := range rows {
		for i := 0; i < numCols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			if i < numCols-1 {
				visible := lipgloss.Width(cell)
				padding := widths[i] - visible
				if padding < 0 {
					padding = 0
				}
				sb.WriteString(cell)
				sb.WriteString(strings.Repeat(" ", padding))
				sb.WriteString(gap)
			} else {
				sb.WriteString(cell)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderPlainTable renders a simple tab-separated table for scripting.
func renderPlainTable(headers []string, rows [][]string) string {
	var sb strings.Builder

	// Headers
	sb.WriteString(strings.Join(headers, "\t"))
	sb.WriteString("\n")

	// Rows
	for _, row := range rows {
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteString("\n")
	}

	return sb.String()
}

// PlainTableRows renders just the data rows as tab-separated values (no headers).
// Useful for --plain mode without --header.
func PlainTableRows(rows [][]string) string {
	var sb strings.Builder
	for _, row := range rows {
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteString("\n")
	}
	return sb.String()
}

// PlainTableWithHeader renders tab-separated rows with optional header.
func PlainTableWithHeader(headers []string, rows [][]string, includeHeader bool) string {
	if includeHeader {
		return renderPlainTable(headers, rows)
	}
	return PlainTableRows(rows)
}

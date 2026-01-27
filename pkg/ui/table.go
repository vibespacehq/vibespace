package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// TableOptions configures table rendering.
type TableOptions struct {
	// NoColor disables all styling (for NO_COLOR env or non-TTY).
	NoColor bool
	// HeaderColor overrides the default header color (Teal).
	HeaderColor lipgloss.Color
	// BorderColor overrides the default border color (ColorMuted).
	BorderColor lipgloss.Color
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

	headerColor := opts.HeaderColor
	if headerColor == "" {
		headerColor = Teal
	}

	borderColor := opts.BorderColor
	if borderColor == "" {
		borderColor = ColorMuted
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(borderColor)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Bold(true).
					Foreground(headerColor).
					Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	return t.String()
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

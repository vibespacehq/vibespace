package ui

import "github.com/charmbracelet/lipgloss"

// Styles contains reusable lipgloss styles for CLI output.
type Styles struct {
	// Text styles
	Bold    lipgloss.Style
	Dim     lipgloss.Style
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	// Status text
	StatusRunning lipgloss.Style
	StatusStopped lipgloss.Style
	StatusError   lipgloss.Style
}

// NewStyles creates a new Styles instance with brand colors.
func NewStyles() Styles {
	return Styles{
		Bold:    lipgloss.NewStyle().Bold(true),
		Dim:     lipgloss.NewStyle().Foreground(ColorDim),
		Success: lipgloss.NewStyle().Foreground(ColorSuccess),
		Error:   lipgloss.NewStyle().Foreground(ColorError),
		Warning: lipgloss.NewStyle().Foreground(ColorWarning),
		Info:    lipgloss.NewStyle().Foreground(Teal),

		StatusRunning: lipgloss.NewStyle().Foreground(ColorSuccess),
		StatusStopped: lipgloss.NewStyle().Foreground(ColorWarning),
		StatusError:   lipgloss.NewStyle().Foreground(ColorError),
	}
}

// PlainStyles returns styles that produce plain (unstyled) output.
// Used when NO_COLOR is set or output is not a TTY.
func PlainStyles() Styles {
	noStyle := lipgloss.NewStyle()
	return Styles{
		Bold:          noStyle,
		Dim:           noStyle,
		Success:       noStyle,
		Error:         noStyle,
		Warning:       noStyle,
		Info:          noStyle,
		StatusRunning: noStyle,
		StatusStopped: noStyle,
		StatusError:   noStyle,
	}
}

// AgentLabelStyle returns a bold style with the given agent color.
func AgentLabelStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

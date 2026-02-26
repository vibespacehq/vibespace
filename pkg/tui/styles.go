package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// No intermediate color vars — reference ui.* directly in NewStyles() so that
// ui.ApplyTheme() changes are picked up at runtime.

// Styles contains all TUI styles
type Styles struct {
	// Layout
	App          lipgloss.Style
	Header       lipgloss.Style
	OutputArea   lipgloss.Style
	InputArea    lipgloss.Style
	StatusBar    lipgloss.Style
	AgentSection lipgloss.Style

	// Text
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Prompt     lipgloss.Style
	Dim        lipgloss.Style
	Bold       lipgloss.Style
	Success    lipgloss.Style
	Warning    lipgloss.Style
	Error      lipgloss.Style
	AgentLabel lipgloss.Style

	// Chat view specific
	UserLabel lipgloss.Style // For [You → target] labels
	ToolLabel lipgloss.Style // For [agent: Tool] labels
	Timestamp lipgloss.Style // For subtle timestamps
	CodeBlock lipgloss.Style // For code block styling
	Thinking  lipgloss.Style // For thinking indicator

	// Input
	Input       lipgloss.Style
	InputCursor lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
}

// NewStyles creates the default TUI styles
func NewStyles() Styles {
	return Styles{
		// Layout
		App: lipgloss.NewStyle().
			Padding(0, 1),

		Header: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ui.ColorDim).
			Padding(0, 1).
			MarginBottom(1),

		OutputArea: lipgloss.NewStyle().
			Padding(0, 1),

		InputArea: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ui.ColorDim).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(ui.ColorDim).
			Padding(0, 1),

		AgentSection: lipgloss.NewStyle().
			MarginBottom(1),

		// Text
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorText),

		Subtitle: lipgloss.NewStyle().
			Foreground(ui.ColorDim),

		Prompt: lipgloss.NewStyle().
			Foreground(ui.Pink).
			Bold(true),

		Dim: lipgloss.NewStyle().
			Foreground(ui.ColorDim),

		Bold: lipgloss.NewStyle().
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(ui.Teal),

		Warning: lipgloss.NewStyle().
			Foreground(ui.Orange),

		Error: lipgloss.NewStyle().
			Foreground(ui.ColorError),

		AgentLabel: lipgloss.NewStyle().
			Bold(true),

		// Chat view specific
		UserLabel: lipgloss.NewStyle().
			Foreground(ui.ColorUser).
			Bold(true),

		ToolLabel: lipgloss.NewStyle().
			Foreground(ui.ColorTool).
			Italic(true),

		Timestamp: lipgloss.NewStyle().
			Foreground(ui.ColorTimestamp),

		CodeBlock: lipgloss.NewStyle().
			Foreground(ui.ColorCodeFg).
			Background(ui.ColorCodeBg).
			Padding(0, 1),

		Thinking: lipgloss.NewStyle().
			Foreground(ui.ColorThinking).
			Italic(true),

		// Input
		Input: lipgloss.NewStyle().
			Foreground(ui.ColorWhite),

		InputCursor: lipgloss.NewStyle().
			Foreground(ui.Pink),

		// Help
		HelpKey: lipgloss.NewStyle().
			Foreground(ui.Pink).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(ui.ColorDim),
	}
}

// GetAgentColor returns a color for an agent based on index
func GetAgentColor(index int) lipgloss.Color {
	return ui.GetAgentColor(index)
}

// AgentLabelStyle returns a style for an agent label with their assigned color
func AgentLabelStyle(color lipgloss.Color) lipgloss.Style {
	return ui.AgentLabelStyle(color)
}

// UserLabelWithTarget returns styled user label with target
func UserLabelWithTarget(target string) string {
	style := lipgloss.NewStyle().
		Foreground(ui.Teal).
		Bold(true)
	return style.Render("[You → " + target + "]")
}

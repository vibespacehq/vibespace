package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	primaryColor   = lipgloss.Color("#7B61FF") // Purple
	secondaryColor = lipgloss.Color("#00D9FF") // Cyan
	successColor   = lipgloss.Color("#00FF9F") // Green
	warningColor   = lipgloss.Color("#FFB800") // Yellow
	errorColor     = lipgloss.Color("#FF4D4D") // Red
	dimColor       = lipgloss.Color("#666666") // Gray
	whiteColor     = lipgloss.Color("#FFFFFF")
	blackColor     = lipgloss.Color("#000000")
)

// Agent colors palette
var agentColors = []lipgloss.Color{
	lipgloss.Color("#00D9FF"), // Cyan
	lipgloss.Color("#FF6B9D"), // Pink
	lipgloss.Color("#7B61FF"), // Purple
	lipgloss.Color("#00FF9F"), // Green
	lipgloss.Color("#FFB800"), // Yellow
	lipgloss.Color("#FF8C42"), // Orange
	lipgloss.Color("#4ECDC4"), // Teal
	lipgloss.Color("#F7DC6F"), // Light yellow
}

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
			BorderForeground(dimColor).
			Padding(0, 1).
			MarginBottom(1),

		OutputArea: lipgloss.NewStyle().
			Padding(0, 1),

		InputArea: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(dimColor).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1),

		AgentSection: lipgloss.NewStyle().
			MarginBottom(1),

		// Text
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(whiteColor),

		Subtitle: lipgloss.NewStyle().
			Foreground(dimColor),

		Prompt: lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true),

		Dim: lipgloss.NewStyle().
			Foreground(dimColor),

		Bold: lipgloss.NewStyle().
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(successColor),

		Warning: lipgloss.NewStyle().
			Foreground(warningColor),

		Error: lipgloss.NewStyle().
			Foreground(errorColor),

		AgentLabel: lipgloss.NewStyle().
			Bold(true),

		// Input
		Input: lipgloss.NewStyle().
			Foreground(whiteColor),

		InputCursor: lipgloss.NewStyle().
			Foreground(secondaryColor),

		// Help
		HelpKey: lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(dimColor),
	}
}

// GetAgentColor returns a color for an agent based on index
func GetAgentColor(index int) lipgloss.Color {
	return agentColors[index%len(agentColors)]
}

// AgentLabelStyle returns a style for an agent label with their assigned color
func AgentLabelStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

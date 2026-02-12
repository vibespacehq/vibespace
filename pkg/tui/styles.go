package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// Colors - imported from shared ui package with TUI-specific additions
var (
	// Brand colors from ui package
	secondaryColor = ui.Pink   // #F102F3
	successColor   = ui.Teal   // Use brand teal for success
	warningColor   = ui.Orange // #FF7D4B
	errorColor     = ui.ColorError
	dimColor       = ui.ColorDim
	whiteColor     = ui.ColorWhite
	blackColor     = ui.ColorBlack

	// TUI-specific colors
	userColor      = ui.ColorUser      // Green for user messages
	toolColor      = ui.ColorTool      // Tool use highlights
	timestampColor = ui.ColorTimestamp // Subtle timestamps
	codeBlockBg    = ui.ColorCodeBg    // Code block background
	codeBlockFg    = ui.ColorCodeFg    // Code block text
	thinkingColor  = ui.ColorThinking  // Pink for thinking indicator
)

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

		// Chat view specific
		UserLabel: lipgloss.NewStyle().
			Foreground(userColor).
			Bold(true),

		ToolLabel: lipgloss.NewStyle().
			Foreground(toolColor).
			Italic(true),

		Timestamp: lipgloss.NewStyle().
			Foreground(timestampColor),

		CodeBlock: lipgloss.NewStyle().
			Foreground(codeBlockFg).
			Background(codeBlockBg).
			Padding(0, 1),

		Thinking: lipgloss.NewStyle().
			Foreground(thinkingColor).
			Italic(true),

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

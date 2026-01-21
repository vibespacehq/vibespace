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

	// New colors for chat view
	userColor      = lipgloss.Color("#00FF9F") // Green for user messages
	toolColor      = lipgloss.Color("#FFB800") // Yellow for tool use
	timestampColor = lipgloss.Color("#555555") // Darker gray for timestamps
	codeBlockBg    = lipgloss.Color("#1a1a2e") // Dark blue background for code
	codeBlockFg    = lipgloss.Color("#87CEEB") // Light blue text for code
	thinkingColor  = lipgloss.Color("#FF6B9D") // Pink for thinking indicator
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

	// Chat view specific
	UserLabel  lipgloss.Style // For [You → target] labels
	ToolLabel  lipgloss.Style // For [agent: Tool] labels
	Timestamp  lipgloss.Style // For subtle timestamps
	CodeBlock  lipgloss.Style // For code block styling
	Thinking   lipgloss.Style // For thinking indicator

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
	return agentColors[index%len(agentColors)]
}

// AgentLabelStyle returns a style for an agent label with their assigned color
func AgentLabelStyle(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

// UserLabelWithTarget returns styled user label with target
func UserLabelWithTarget(target string) string {
	style := lipgloss.NewStyle().
		Foreground(userColor).
		Bold(true)
	return style.Render("[You → " + target + "]")
}

// ThinkingIndicator returns a styled thinking indicator
func ThinkingIndicator(dots string) string {
	style := lipgloss.NewStyle().
		Foreground(thinkingColor).
		Italic(true)
	return style.Render("(thinking" + dots + ")")
}

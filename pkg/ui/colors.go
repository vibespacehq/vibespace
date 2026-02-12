// Package ui provides shared UI components and styling for CLI and TUI.
package ui

import "github.com/charmbracelet/lipgloss"

// Brand colors - vibespace brand identity
var (
	Teal   = lipgloss.Color("#00ABAB")
	Pink   = lipgloss.Color("#F102F3")
	Orange = lipgloss.Color("#FF7D4B")
	Yellow = lipgloss.Color("#F5F50A")
)

// Semantic colors - derived from brand
var (
	ColorSuccess = Teal
	ColorError   = lipgloss.Color("#FF4D4D")
	ColorWarning = Orange
	ColorDim     = lipgloss.Color("#666666")
	ColorMuted   = lipgloss.Color("#444444")
	ColorWhite   = lipgloss.Color("#FFFFFF")
	ColorBlack   = lipgloss.Color("#000000")
)

// Extended semantic colors for TUI
var (
	ColorUser      = lipgloss.Color("#00FF9F") // Green for user messages
	ColorTool      = Orange                    // Tool use highlights
	ColorTimestamp = lipgloss.Color("#555555") // Subtle timestamps
	ColorCodeBg    = lipgloss.Color("#1a1a2e") // Code block background
	ColorCodeFg    = lipgloss.Color("#87CEEB") // Code block text
	ColorThinking  = Pink                      // Thinking indicator
)

// AgentColors provides a palette of 7 colors for agent identification.
// Teal is reserved for the user — agents never use it.
var AgentColors = []lipgloss.Color{
	Pink,                      // #F102F3 - Brand pink
	Orange,                    // #FF7D4B - Brand orange
	lipgloss.Color("#00D9FF"), // Cyan
	lipgloss.Color("#7B61FF"), // Purple
	Yellow,                    // #F5F50A - Brand yellow
	lipgloss.Color("#00FF9F"), // Green
	lipgloss.Color("#FF6B6B"), // Coral
}

// GetAgentColor returns a color for an agent based on index (cycles through palette).
func GetAgentColor(index int) lipgloss.Color {
	return AgentColors[index%len(AgentColors)]
}

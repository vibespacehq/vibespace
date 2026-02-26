// Package ui provides shared UI components and styling for CLI and TUI.
package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/config"
)

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
	ColorText    = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#FFFFFF"} // adapts to terminal bg
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

// ApplyTheme sets all package-level color vars from a ThemeConfig.
// Must be called before NewStyles() to take effect.
func ApplyTheme(theme config.ThemeConfig) {
	// Brand colors
	if theme.Brand.Teal != "" {
		Teal = lipgloss.Color(theme.Brand.Teal)
	}
	if theme.Brand.Pink != "" {
		Pink = lipgloss.Color(theme.Brand.Pink)
	}
	if theme.Brand.Orange != "" {
		Orange = lipgloss.Color(theme.Brand.Orange)
	}
	if theme.Brand.Yellow != "" {
		Yellow = lipgloss.Color(theme.Brand.Yellow)
	}

	// Semantic colors
	if theme.Semantic.Success != "" {
		ColorSuccess = lipgloss.Color(theme.Semantic.Success)
	}
	if theme.Semantic.Error != "" {
		ColorError = lipgloss.Color(theme.Semantic.Error)
	}
	if theme.Semantic.Warning != "" {
		ColorWarning = lipgloss.Color(theme.Semantic.Warning)
	}
	if theme.Semantic.Dim != "" {
		ColorDim = lipgloss.Color(theme.Semantic.Dim)
	}
	if theme.Semantic.Muted != "" {
		ColorMuted = lipgloss.Color(theme.Semantic.Muted)
	}
	if theme.Semantic.TextLight != "" && theme.Semantic.TextDark != "" {
		ColorText = lipgloss.AdaptiveColor{Light: theme.Semantic.TextLight, Dark: theme.Semantic.TextDark}
	}

	// TUI colors
	if theme.TUIColors.User != "" {
		ColorUser = lipgloss.Color(theme.TUIColors.User)
	}
	if theme.TUIColors.Tool != "" {
		ColorTool = lipgloss.Color(theme.TUIColors.Tool)
	}
	if theme.TUIColors.Timestamp != "" {
		ColorTimestamp = lipgloss.Color(theme.TUIColors.Timestamp)
	}
	if theme.TUIColors.CodeBg != "" {
		ColorCodeBg = lipgloss.Color(theme.TUIColors.CodeBg)
	}
	if theme.TUIColors.CodeFg != "" {
		ColorCodeFg = lipgloss.Color(theme.TUIColors.CodeFg)
	}
	if theme.TUIColors.Thinking != "" {
		ColorThinking = lipgloss.Color(theme.TUIColors.Thinking)
	}

	// Agent palette
	if len(theme.AgentPalette) > 0 {
		palette := make([]lipgloss.Color, len(theme.AgentPalette))
		for i, c := range theme.AgentPalette {
			palette[i] = lipgloss.Color(c)
		}
		AgentColors = palette
	}
}

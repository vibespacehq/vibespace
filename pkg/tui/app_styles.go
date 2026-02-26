package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// Layout constants used by App to calculate content area.
const (
	tabBarHeight    = 2 // tab text + border line
	statusBarHeight = 2 // border line + text
	borderH         = 2 // top + bottom border lines
	borderW         = 2 // left + right border
)

// --- App border ---

var appBorderStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ui.ColorMuted).
	Padding(0, 0)

// --- Tab bar ---

// getBrandGradient returns the colors for the animated tab underline,
// reading live ui.Teal/ui.Pink values so theme changes are picked up.
func getBrandGradient() []lipgloss.Color {
	return []lipgloss.Color{ui.Teal, ui.Pink}
}

// --- Overlay ---

var (
	overlayBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ui.Orange).
				Padding(1, 2)

	overlayTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.Orange).
				MarginBottom(1)

	overlayDimStyle = lipgloss.NewStyle().
			Foreground(ui.ColorDim)
)

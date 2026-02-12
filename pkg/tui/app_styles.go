package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// Tab bar heights used by App to calculate content area.
const (
	titleBarPad     = 1 // top padding to clear transparent terminal title bars
	tabBarHeight    = titleBarPad + 2 // pad + tab text + animated border
	statusBarHeight = 1
)

// --- Tab bar ---

var (
	activeTabLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.Teal).
				Padding(0, 2)

	inactiveTabLabelStyle = lipgloss.NewStyle().
				Foreground(ui.ColorDim).
				Padding(0, 2)
)

// brandGradient defines the colors for the animated tab underline.
var brandGradient = []lipgloss.Color{ui.Teal, ui.Pink}

// --- Status bar ---

var (
	statusBarStyle = lipgloss.NewStyle().
			Foreground(ui.ColorDim).
			Padding(0, 1)

	statusKeyStyle = lipgloss.NewStyle().
			Foreground(ui.Teal).
			Bold(true)

	statusDescStyle = lipgloss.NewStyle().
			Foreground(ui.ColorDim)
)

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

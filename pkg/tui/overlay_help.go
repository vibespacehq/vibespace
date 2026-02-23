package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// HelpOverlay renders a centered keybinding reference box.
type HelpOverlay struct {
	width, height int
	visible       bool
}

func NewHelpOverlay() *HelpOverlay {
	return &HelpOverlay{}
}

func (h *HelpOverlay) Toggle()       { h.visible = !h.visible }
func (h *HelpOverlay) Show()         { h.visible = true }
func (h *HelpOverlay) Hide()         { h.visible = false }
func (h *HelpOverlay) Visible() bool { return h.visible }

func (h *HelpOverlay) SetSize(w, height int) {
	h.width = w
	h.height = height
}

func (h *HelpOverlay) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", "?", "q":
			h.Hide()
		}
	}
	return nil
}

func (h *HelpOverlay) View() string {
	if !h.visible {
		return ""
	}

	keyStyle := lipgloss.NewStyle().Foreground(ui.Teal).Bold(true).Width(16)
	descStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)

	row := func(k, d string) string {
		return keyStyle.Render(k) + descStyle.Render(d)
	}

	var b strings.Builder
	b.WriteString(overlayTitleStyle.Render("Keybindings"))
	b.WriteString("\n\n")

	// Global
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render("Global"))
	b.WriteString("\n")
	b.WriteString(row("1-5", "Switch tab"))
	b.WriteString("\n")
	b.WriteString(row("Ctrl+1-5", "Switch tab (always)"))
	b.WriteString("\n")
	b.WriteString(row("Tab/Shift+Tab", "Next/prev tab"))
	b.WriteString("\n")
	b.WriteString(row("?", "Toggle this help"))
	b.WriteString("\n")
	b.WriteString(row(":", "Command palette"))
	b.WriteString("\n")
	b.WriteString(row("Ctrl+C", "Quit"))
	b.WriteString("\n\n")

	// Vibespaces
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render("Vibespaces"))
	b.WriteString("\n")
	b.WriteString(row("j/k", "Navigate rows"))
	b.WriteString("\n")
	b.WriteString(row("Enter", "Toggle expansion"))
	b.WriteString("\n")
	b.WriteString(row("n", "New vibespace"))
	b.WriteString("\n")
	b.WriteString(row("x", "Connect shell"))
	b.WriteString("\n\n")

	// Chat
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render("Chat"))
	b.WriteString("\n")
	b.WriteString(row("Enter", "Send message"))
	b.WriteString("\n")
	b.WriteString(row("@agent", "Mention agent"))
	b.WriteString("\n")
	b.WriteString(row("Ctrl+]", "Exit to tabs"))
	b.WriteString("\n\n")

	// Sessions
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render("Sessions"))
	b.WriteString("\n")
	b.WriteString(row("Enter", "Resume session"))
	b.WriteString("\n")
	b.WriteString(row("n/d", "New / delete"))
	b.WriteString("\n\n")

	b.WriteString(overlayDimStyle.Render("Press ? or Esc to close"))

	content := b.String()
	box := overlayBorderStyle.Render(content)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, box)
}

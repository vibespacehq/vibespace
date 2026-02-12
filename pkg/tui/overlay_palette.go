package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PaletteOverlay is a stub for the command palette.
type PaletteOverlay struct {
	width, height int
	visible       bool
}

func NewPaletteOverlay() *PaletteOverlay {
	return &PaletteOverlay{}
}

func (p *PaletteOverlay) Toggle() { p.visible = !p.visible }
func (p *PaletteOverlay) Show()   { p.visible = true }
func (p *PaletteOverlay) Hide()   { p.visible = false }
func (p *PaletteOverlay) Visible() bool { return p.visible }

func (p *PaletteOverlay) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *PaletteOverlay) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc", ":":
			p.Hide()
		}
	}
	return nil
}

func (p *PaletteOverlay) View() string {
	if !p.visible {
		return ""
	}

	content := overlayTitleStyle.Render("Command Palette") + "\n\n" +
		overlayDimStyle.Render("Coming soon.\n\nPress Esc to close.")

	box := overlayBorderStyle.Render(content)
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}

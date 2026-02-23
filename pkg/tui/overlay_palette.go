package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// paletteAction is a single entry in the command palette.
type paletteAction struct {
	label string // display text
	hint  string // right-aligned shortcut hint
	cmd   func() tea.Cmd
}

// PaletteOverlay implements a fuzzy-filtered command palette.
type PaletteOverlay struct {
	width, height int
	visible       bool
	input         textinput.Model
	actions       []paletteAction
	filtered      []int // indices into actions
	cursor        int
}

// NewPaletteOverlay creates a command palette with the default action set.
func NewPaletteOverlay() *PaletteOverlay {
	ti := textinput.New()
	ti.Prompt = ": "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.Orange).Bold(true)
	ti.CharLimit = 64

	p := &PaletteOverlay{input: ti}
	p.actions = p.defaultActions()
	p.resetFilter()
	return p
}

func (p *PaletteOverlay) defaultActions() []paletteAction {
	return []paletteAction{
		{label: "Go to Vibespaces", hint: "1", cmd: func() tea.Cmd {
			return func() tea.Msg { return SwitchTabMsg{Tab: TabVibespaces} }
		}},
		{label: "Go to Chat", hint: "2", cmd: func() tea.Cmd {
			return func() tea.Msg { return SwitchTabMsg{Tab: TabChat} }
		}},
		{label: "Go to Monitor", hint: "3", cmd: func() tea.Cmd {
			return func() tea.Msg { return SwitchTabMsg{Tab: TabMonitor} }
		}},
		{label: "Go to Sessions", hint: "4", cmd: func() tea.Cmd {
			return func() tea.Msg { return SwitchTabMsg{Tab: TabSessions} }
		}},
		{label: "Go to Remote", hint: "5", cmd: func() tea.Cmd {
			return func() tea.Msg { return SwitchTabMsg{Tab: TabRemote} }
		}},
		{label: "New vibespace", hint: "n", cmd: func() tea.Cmd {
			return tea.Batch(
				func() tea.Msg { return SwitchTabMsg{Tab: TabVibespaces} },
				func() tea.Msg { return PaletteNewVibespaceMsg{} },
			)
		}},
		{label: "New session", hint: "n", cmd: func() tea.Cmd {
			return tea.Batch(
				func() tea.Msg { return SwitchTabMsg{Tab: TabSessions} },
				func() tea.Msg { return PaletteNewSessionMsg{} },
			)
		}},
		{label: "Toggle help", hint: "?", cmd: func() tea.Cmd {
			return func() tea.Msg { return PaletteToggleHelpMsg{} }
		}},
		{label: "Quit", hint: "ctrl+c", cmd: func() tea.Cmd {
			return tea.Quit
		}},
	}
}

func (p *PaletteOverlay) Toggle() {
	if p.visible {
		p.Hide()
	} else {
		p.Show()
	}
}

func (p *PaletteOverlay) Show() {
	p.visible = true
	p.input.SetValue("")
	p.input.Focus()
	p.resetFilter()
}

func (p *PaletteOverlay) Hide() {
	p.visible = false
	p.input.Blur()
}

func (p *PaletteOverlay) Visible() bool { return p.visible }

func (p *PaletteOverlay) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *PaletteOverlay) resetFilter() {
	p.filtered = make([]int, len(p.actions))
	for i := range p.actions {
		p.filtered[i] = i
	}
	p.cursor = 0
}

// fuzzyMatch does case-insensitive substring matching on each word in the query.
func fuzzyMatch(label, query string) bool {
	label = strings.ToLower(label)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, word := range strings.Fields(query) {
		if !strings.Contains(label, word) {
			return false
		}
	}
	return true
}

func (p *PaletteOverlay) applyFilter() {
	query := p.input.Value()
	p.filtered = p.filtered[:0]
	for i, a := range p.actions {
		if fuzzyMatch(a.label, query) {
			p.filtered = append(p.filtered, i)
		}
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = max(0, len(p.filtered)-1)
	}
}

func (p *PaletteOverlay) execute() tea.Cmd {
	if len(p.filtered) == 0 {
		return nil
	}
	action := p.actions[p.filtered[p.cursor]]
	p.Hide()
	if action.cmd != nil {
		return action.cmd()
	}
	return nil
}

func (p *PaletteOverlay) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			p.Hide()
			return nil
		case "enter":
			return p.execute()
		case "up", "ctrl+p":
			if p.cursor > 0 {
				p.cursor--
			}
			return nil
		case "down", "ctrl+n":
			if p.cursor < len(p.filtered)-1 {
				p.cursor++
			}
			return nil
		}
	}

	// Forward to text input
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.applyFilter()
	return cmd
}

func (p *PaletteOverlay) View() string {
	if !p.visible {
		return ""
	}

	// Content width inside the overlay box (border=2, padding=4)
	const boxOverhead = 6
	contentW := p.width*50/100 - boxOverhead
	if contentW > 48 {
		contentW = 48
	}
	if contentW < 24 {
		contentW = 24
	}

	p.input.Width = contentW

	selectedStyle := lipgloss.NewStyle().
		Foreground(ui.ColorWhite).
		Background(lipgloss.Color("#333333")).
		Bold(true).
		Width(contentW)
	hintStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	emptyStyle := lipgloss.NewStyle().Foreground(ui.ColorDim).Italic(true)

	var b strings.Builder
	b.WriteString(p.input.View())
	b.WriteString("\n")

	maxVisible := 10
	if len(p.filtered) == 0 {
		b.WriteString("\n")
		b.WriteString(emptyStyle.Render("No matches"))
	} else {
		start := 0
		if p.cursor >= maxVisible {
			start = p.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(p.filtered) {
			end = len(p.filtered)
		}

		for vi := start; vi < end; vi++ {
			a := p.actions[p.filtered[vi]]

			indicator := "  "
			if vi == p.cursor {
				indicator = "> "
			}

			// Build plain row: "> Label          hint"
			gap := contentW - len(indicator) - len(a.label) - len(a.hint)
			if gap < 1 {
				gap = 1
			}
			row := indicator + a.label + strings.Repeat(" ", gap) + a.hint

			b.WriteString("\n")
			if vi == p.cursor {
				b.WriteString(selectedStyle.Render(row))
			} else {
				// Dim indicator + white label + dim hint
				b.WriteString(hintStyle.Render(indicator) + a.label +
					strings.Repeat(" ", gap) + hintStyle.Render(a.hint))
			}
		}
	}

	box := overlayBorderStyle.Render(b.String())
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}

// --- Messages emitted by palette actions ---

// PaletteNewVibespaceMsg tells the Vibespaces tab to enter create mode.
type PaletteNewVibespaceMsg struct{}

// PaletteNewSessionMsg tells the Sessions tab to enter new session mode.
type PaletteNewSessionMsg struct{}

// PaletteToggleHelpMsg tells the App to toggle the help overlay.
type PaletteToggleHelpMsg struct{}

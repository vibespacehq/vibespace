package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// VibespacesTab displays the vibespace list with inline expansion.
type VibespacesTab struct {
	shared        *SharedState
	width, height int
}

func NewVibespacesTab(shared *SharedState) *VibespacesTab {
	return &VibespacesTab{shared: shared}
}

func (t *VibespacesTab) Title() string { return TabNames[TabVibespaces] }

func (t *VibespacesTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "connect")),
	}
}

func (t *VibespacesTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *VibespacesTab) Init() tea.Cmd { return nil }

func (t *VibespacesTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return t, nil
}

func (t *VibespacesTab) View() string {
	status := t.statusLine()
	body := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Render("  Vibespaces tab coming soon.\n\n  Use CLI commands: vibespace create, vibespace list")
	return fmt.Sprintf("%s\n\n%s", status, body)
}

func (t *VibespacesTab) statusLine() string {
	daemon := lipgloss.NewStyle().Foreground(ui.ColorDim).Render("daemon: off")
	if t.shared != nil && t.shared.DaemonRunning {
		daemon = lipgloss.NewStyle().Foreground(ui.Teal).Render(fmt.Sprintf("daemon: pid %d", t.shared.DaemonPid))
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(daemon)
}

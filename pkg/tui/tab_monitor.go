package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// MonitorTab shows resource usage and agent activity.
type MonitorTab struct {
	shared        *SharedState
	width, height int
}

func NewMonitorTab(shared *SharedState) *MonitorTab {
	return &MonitorTab{shared: shared}
}

func (t *MonitorTab) Title() string { return TabNames[TabMonitor] }

func (t *MonitorTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pause")),
		key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "picker")),
	}
}

func (t *MonitorTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *MonitorTab) Init() tea.Cmd { return nil }

func (t *MonitorTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return t, nil
}

func (t *MonitorTab) View() string {
	return lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(1, 2).
		Render("Monitor tab coming soon.\n\nResource usage, agent activity, and sparklines.")
}

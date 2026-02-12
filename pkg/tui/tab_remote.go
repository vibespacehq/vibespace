package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// RemoteTab shows WireGuard remote connection status.
type RemoteTab struct {
	shared        *SharedState
	width, height int
}

func NewRemoteTab(shared *SharedState) *RemoteTab {
	return &RemoteTab{shared: shared}
}

func (t *RemoteTab) Title() string { return TabNames[TabRemote] }

func (t *RemoteTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
		key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "disconnect")),
	}
}

func (t *RemoteTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *RemoteTab) Init() tea.Cmd { return nil }

func (t *RemoteTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return t, nil
}

func (t *RemoteTab) View() string {
	return lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(1, 2).
		Render("Remote tab coming soon.\n\nWireGuard VPN connection, tunnel health, and server management.")
}

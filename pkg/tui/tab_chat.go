package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// ChatTab wraps the existing Model as a pass-through Tab.
type ChatTab struct {
	inner   *Model
	session *session.Session
	resume  bool
	width   int
	height  int
	inited  bool
}

// NewChatTab creates a ChatTab. If sess is nil, the tab shows an empty state.
func NewChatTab(sess *session.Session, resume bool) *ChatTab {
	ct := &ChatTab{
		session: sess,
		resume:  resume,
	}
	if sess != nil {
		ct.inner = NewModel(sess, resume)
	}
	return ct
}

func (ct *ChatTab) Title() string { return TabNames[TabChat] }

func (ct *ChatTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("ctrl+]"), key.WithHelp("ctrl+]", "exit to tabs")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		key.NewBinding(key.WithKeys("@"), key.WithHelp("@agent", "mention")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/cmd", "command")),
	}
}

func (ct *ChatTab) SetSize(width, height int) {
	ct.width = width
	ct.height = height
	if ct.inner != nil && ct.inited {
		ct.inner.Update(tea.WindowSizeMsg{Width: width, Height: height})
	}
}

func (ct *ChatTab) Init() tea.Cmd {
	if ct.inited || ct.inner == nil {
		return nil
	}
	ct.inited = true
	cmd := ct.inner.Init()
	if ct.width > 0 && ct.height > 0 {
		ct.inner.Update(tea.WindowSizeMsg{Width: ct.width, Height: ct.height})
	}
	return cmd
}

func (ct *ChatTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ct.inner == nil {
		return ct, nil
	}
	switch msg.(type) {
	case tea.WindowSizeMsg:
		// Already handled via SetSize; forward anyway so inner can react.
		_, cmd := ct.inner.Update(tea.WindowSizeMsg{Width: ct.width, Height: ct.height})
		return ct, cmd
	}
	_, cmd := ct.inner.Update(msg)
	return ct, cmd
}

func (ct *ChatTab) View() string {
	if ct.inner == nil {
		return lipgloss.NewStyle().
			Foreground(ui.ColorDim).
			Render("\n  No active session. Press n to start one, or switch to Sessions tab.")
	}
	return ct.inner.View()
}

// LoadSession replaces the inner model with a new session.
func (ct *ChatTab) LoadSession(sess *session.Session, resume bool) tea.Cmd {
	ct.session = sess
	ct.resume = resume
	ct.inner = NewModel(sess, resume)
	ct.inited = false
	return ct.Init()
}

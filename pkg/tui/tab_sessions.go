package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// sessionsMode represents the current UI mode of the Sessions tab.
type sessionsMode int

const (
	sessionsModeList   sessionsMode = iota
	sessionsModeDelete              // inline delete confirmation
	sessionsModeNew                 // inline new session form
)

// sessionsLoadedMsg delivers session data from the store.
type sessionsLoadedMsg struct {
	sessions []session.Session
	err      error
}

// sessionDeletedMsg signals a session was deleted.
type sessionDeletedMsg struct{ err error }

// sessionCreatedMsg signals a new session was created.
type sessionCreatedMsg struct {
	session *session.Session
	err     error
}

// sessionHistoryMsg delivers recent messages for the selected session.
type sessionHistoryMsg struct {
	sessionName string
	messages    []*Message
	totalCount  int
}

// SessionsTab manages multi-agent sessions.
type SessionsTab struct {
	shared   *SharedState
	sessions []session.Session
	selected int
	width    int
	height   int
	mode     sessionsMode
	nameInput       textinput.Model
	err             string     // transient error message
	previewName     string     // session name for cached preview messages
	previewMsgs     []*Message // last N messages for selected session
	previewTotal    int        // total message count for selected session
}

func NewSessionsTab(shared *SharedState) *SessionsTab {
	ti := textinput.New()
	ti.Placeholder = "session-name"
	ti.CharLimit = 64
	return &SessionsTab{
		shared:    shared,
		nameInput: ti,
	}
}

func (t *SessionsTab) Title() string { return TabNames[TabSessions] }

func (t *SessionsTab) ShortHelp() []key.Binding {
	switch t.mode {
	case sessionsModeDelete:
		return []key.Binding{
			key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm delete")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	case sessionsModeNew:
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "create")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	default:
		return []key.Binding{
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		}
	}
}

func (t *SessionsTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *SessionsTab) Init() tea.Cmd {
	return t.loadSessions()
}

func (t *SessionsTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case TabActivateMsg:
		return t, t.loadSessions()

	case sessionsLoadedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		t.sessions = msg.sessions
		t.clampSelected()
		t.err = ""
		return t, t.loadPreview()

	case sessionHistoryMsg:
		if t.selected < len(t.sessions) && t.sessions[t.selected].Name == msg.sessionName {
			t.previewName = msg.sessionName
			t.previewMsgs = msg.messages
			t.previewTotal = msg.totalCount
		}
		return t, nil

	case sessionDeletedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		t.mode = sessionsModeList
		return t, t.loadSessions()

	case sessionCreatedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			t.mode = sessionsModeList
			return t, nil
		}
		t.mode = sessionsModeList
		t.nameInput.SetValue("")
		return t, func() tea.Msg {
			return SwitchToChatMsg{Session: msg.session, Resume: false}
		}

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	// Forward to text input when in new mode
	if t.mode == sessionsModeNew {
		var cmd tea.Cmd
		t.nameInput, cmd = t.nameInput.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *SessionsTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch t.mode {
	case sessionsModeDelete:
		switch msg.String() {
		case "y":
			if t.selected < len(t.sessions) {
				name := t.sessions[t.selected].Name
				return t, t.deleteSession(name)
			}
			t.mode = sessionsModeList
		case "esc", "n", "q":
			t.mode = sessionsModeList
		}
		return t, nil

	case sessionsModeNew:
		switch msg.String() {
		case "enter":
			name := strings.TrimSpace(t.nameInput.Value())
			if name == "" {
				return t, nil
			}
			return t, t.createSession(name)
		case "esc":
			t.mode = sessionsModeList
			t.nameInput.SetValue("")
			t.nameInput.Blur()
		default:
			var cmd tea.Cmd
			t.nameInput, cmd = t.nameInput.Update(msg)
			return t, cmd
		}
		return t, nil

	default: // list mode
		switch msg.String() {
		case "j", "down":
			if len(t.sessions) > 0 {
				prev := t.selected
				t.selected = min(t.selected+1, len(t.sessions)-1)
				if t.selected != prev {
					return t, t.loadPreview()
				}
			}
		case "k", "up":
			if len(t.sessions) > 0 {
				prev := t.selected
				t.selected = max(t.selected-1, 0)
				if t.selected != prev {
					return t, t.loadPreview()
				}
			}
		case "g":
			prev := t.selected
			t.selected = 0
			if t.selected != prev {
				return t, t.loadPreview()
			}
		case "G":
			if len(t.sessions) > 0 {
				prev := t.selected
				t.selected = len(t.sessions) - 1
				if t.selected != prev {
					return t, t.loadPreview()
				}
			}
		case "enter":
			if t.selected < len(t.sessions) {
				sess := t.sessions[t.selected]
				return t, func() tea.Msg {
					return SwitchToChatMsg{Session: &sess, Resume: true}
				}
			}
		case "n":
			t.mode = sessionsModeNew
			t.nameInput.SetValue("")
			t.nameInput.Focus()
			t.err = ""
			return t, t.nameInput.Cursor.BlinkCmd()
		case "d":
			if len(t.sessions) > 0 {
				t.mode = sessionsModeDelete
				t.err = ""
			}
		}
		return t, nil
	}
}

func (t *SessionsTab) View() string {
	if t.err != "" && len(t.sessions) == 0 && t.mode == sessionsModeList {
		return lipgloss.NewStyle().
			Foreground(ui.ColorError).
			Padding(1, 2).
			Render(fmt.Sprintf("Error loading sessions: %s", t.err))
	}

	if len(t.sessions) == 0 && t.mode == sessionsModeList {
		return t.viewEmpty()
	}

	var top []string

	// Table (skip when no sessions exist, e.g. new-session form on empty state)
	if len(t.sessions) > 0 {
		top = append(top, t.viewTable())
	}

	// Inline prompt (delete confirmation or new session form)
	if prompt := t.viewPrompt(); prompt != "" {
		top = append(top, prompt)
	}

	// Error line
	if t.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(ui.ColorError).Padding(0, 2)
		top = append(top, errStyle.Render(t.err))
	}

	topBlock := strings.Join(top, "\n")

	// Detail pushed to bottom
	var bottom string
	if t.mode == sessionsModeList && t.selected < len(t.sessions) {
		bottom = t.viewDetail()
	}

	if bottom == "" {
		return topBlock
	}

	topH := lipgloss.Height(topBlock)
	bottomH := lipgloss.Height(bottom)
	gap := t.height - topH - bottomH
	if gap < 1 {
		gap = 1
	}

	return topBlock + strings.Repeat("\n", gap) + bottom
}

// --- View helpers ---

func (t *SessionsTab) viewEmpty() string {
	msg := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(2, 0).
		Render("No sessions yet.")

	hint := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Render("Press n to create a new session.")

	block := lipgloss.JoinVertical(lipgloss.Center, msg, hint)
	return lipgloss.Place(t.width, t.height, lipgloss.Center, lipgloss.Center, block)
}

func (t *SessionsTab) viewTable() string {
	rows := make([][]string, len(t.sessions))
	for i, sess := range t.sessions {
		name := "  " + sess.Name
		vs := sessVibespaceNames(sess)
		agents := sessAgentCount(sess)
		lastUsed := timeAgo(sess.LastUsed)

		if i == t.selected {
			cells := []string{"› " + sess.Name, vs, agents, lastUsed}
			colored := renderGradientRow(cells, brandGradient)
			rows[i] = colored
		} else {
			rows[i] = []string{name, vs, agents, lastUsed}
		}
	}

	sel := t.selected
	tbl := table.New().
		Headers("Session", "Vibespaces", "Agents", "Last Used").
		Rows(rows...).
		Border(lipgloss.NormalBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorMuted)).
		Width(t.width - 4).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(ui.ColorDim)
			}
			if row == sel {
				return s
			}
			return s.Foreground(ui.ColorDim)
		})

	noun := "sessions"
	if len(t.sessions) == 1 {
		noun = "session"
	}
	countText := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(fmt.Sprintf("(%d %s)", len(t.sessions), noun))
	count := lipgloss.NewStyle().Width(t.width - 4).Align(lipgloss.Right).
		PaddingTop(1).Render(countText)

	return lipgloss.NewStyle().Padding(1, 2).Render(tbl.Render() + "\n" + count)
}

func (t *SessionsTab) viewDetail() string {
	sess := t.sessions[t.selected]
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)

	// --- Metadata section ---
	var meta []string

	created := fmt.Sprintf("%s  %s",
		labelStyle.Render("Created"),
		dimStyle.Render(sess.CreatedAt.Format("2006-01-02 15:04:05")))
	meta = append(meta, created)

	if t.previewName == sess.Name {
		msgCount := fmt.Sprintf("%s  %s",
			labelStyle.Render("Messages"),
			dimStyle.Render(fmt.Sprintf("%d", t.previewTotal)))
		meta = append(meta, msgCount)
	}

	metaHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
		Render("Details")
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))
	metaBlock := metaHeader + "\n" + mutedLine + "\n" + strings.Join(meta, "\n") + "\n" + mutedLine

	// --- Messages section ---
	var msgBlock string
	if t.previewName == sess.Name && len(t.previewMsgs) > 0 {
		header := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
			Render("Latest Messages")

		contentStyle := dimStyle
		toolStyle := lipgloss.NewStyle().Foreground(ui.Orange)
		youStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Teal)

		// Assign colors to unique agent senders
		agentColorMap := make(map[string]lipgloss.Color)
		agentIdx := 0
		for _, msg := range t.previewMsgs {
			if msg.Type != MessageTypeUser && msg.Sender != "" {
				if _, ok := agentColorMap[msg.Sender]; !ok {
					agentColorMap[msg.Sender] = ui.GetAgentColor(agentIdx)
					agentIdx++
				}
			}
		}

		var lines []string
		for _, msg := range t.previewMsgs {
			ts := msg.Timestamp.Format("15:04")
			tsStr := lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(ts)
			agentStyle := lipgloss.NewStyle().Bold(true).Foreground(agentColorMap[msg.Sender])

			switch msg.Type {
			case MessageTypeUser:
				lines = append(lines, fmt.Sprintf("%s %s %s",
					tsStr,
					youStyle.Render("You →"),
					contentStyle.Render(truncate(msg.Content, 60))))
			case MessageTypeAssistant:
				lines = append(lines, fmt.Sprintf("%s %s %s",
					tsStr,
					agentStyle.Render(msg.Sender),
					contentStyle.Render(truncate(msg.Content, 60))))
			case MessageTypeToolUse:
				lines = append(lines, fmt.Sprintf("%s %s %s",
					tsStr,
					agentStyle.Render(msg.Sender),
					toolStyle.Render(fmt.Sprintf("[%s] %s", msg.ToolName, truncate(msg.ToolInput, 40)))))
			}
		}
		mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
			Render(strings.Repeat("─", t.width-4))
		msgBlock = header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	} else if t.previewName == sess.Name {
		msgBlock = lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
			Render("No messages yet.")
	}

	// Assemble with vertical spacing
	var sections []string
	sections = append(sections, metaBlock)
	if msgBlock != "" {
		sections = append(sections, msgBlock)
	}

	return lipgloss.NewStyle().Padding(0, 2).Render(
		strings.Join(sections, "\n\n"))
}

func (t *SessionsTab) viewPrompt() string {
	style := lipgloss.NewStyle().Padding(0, 2)

	switch t.mode {
	case sessionsModeDelete:
		if t.selected < len(t.sessions) {
			name := t.sessions[t.selected].Name
			prompt := lipgloss.NewStyle().Foreground(ui.ColorWarning).
				Render(fmt.Sprintf("Delete session %q? y to confirm, Esc to cancel", name))
			return style.Render(prompt)
		}
	case sessionsModeNew:
		label := lipgloss.NewStyle().Foreground(ui.Teal).Bold(true).
			Render("New session name: ")
		return style.Render(label + t.nameInput.View())
	}
	return ""
}

// --- Commands ---

func (t *SessionsTab) loadSessions() tea.Cmd {
	store := t.shared.SessionStore
	return func() tea.Msg {
		if store == nil {
			return sessionsLoadedMsg{err: fmt.Errorf("session store unavailable")}
		}
		sessions, err := store.List()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (t *SessionsTab) deleteSession(name string) tea.Cmd {
	store := t.shared.SessionStore
	return func() tea.Msg {
		if store == nil {
			return sessionDeletedMsg{err: fmt.Errorf("session store unavailable")}
		}
		return sessionDeletedMsg{err: store.Delete(name)}
	}
}

func (t *SessionsTab) loadPreview() tea.Cmd {
	if t.selected >= len(t.sessions) {
		return nil
	}
	name := t.sessions[t.selected].Name
	hs := t.shared.HistoryStore
	return func() tea.Msg {
		if hs == nil {
			return sessionHistoryMsg{sessionName: name}
		}
		allMsgs, _ := hs.Load(name)
		totalCount := len(allMsgs)

		// Take last 5 user/assistant/tool messages
		var filtered []*Message
		for _, m := range allMsgs {
			if m.Type == MessageTypeUser || m.Type == MessageTypeAssistant || m.Type == MessageTypeToolUse {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) > 5 {
			filtered = filtered[len(filtered)-5:]
		}
		return sessionHistoryMsg{sessionName: name, messages: filtered, totalCount: totalCount}
	}
}

func (t *SessionsTab) createSession(name string) tea.Cmd {
	store := t.shared.SessionStore
	return func() tea.Msg {
		if store == nil {
			return sessionCreatedMsg{err: fmt.Errorf("session store unavailable")}
		}
		sess, err := store.Create(name)
		return sessionCreatedMsg{session: sess, err: err}
	}
}

// --- Helpers ---

func (t *SessionsTab) clampSelected() {
	if t.selected >= len(t.sessions) {
		t.selected = max(len(t.sessions)-1, 0)
	}
}

func sessVibespaceNames(sess session.Session) string {
	if len(sess.Vibespaces) == 0 {
		return "-"
	}
	names := make([]string, len(sess.Vibespaces))
	for i, vs := range sess.Vibespaces {
		names[i] = vs.Name
	}
	return strings.Join(names, ", ")
}

func sessAgentCount(sess session.Session) string {
	seen := make(map[string]bool)
	for _, vs := range sess.Vibespaces {
		if len(vs.Agents) == 0 {
			return "all"
		}
		for _, a := range vs.Agents {
			seen[a] = true
		}
	}
	if len(seen) == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", len(seen))
}

func truncate(s string, maxLen int) string {
	// Truncate to first line, then to maxLen
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-1]) + "…"
	}
	return s
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

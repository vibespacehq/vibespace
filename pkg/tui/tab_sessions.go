package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/ui"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// sessionsMode represents the current UI mode of the Sessions tab.
type sessionsMode int

const (
	sessionsModeList          sessionsMode = iota
	sessionsModeDelete                     // inline delete confirmation
	sessionsModeNewName                    // step 1: name input
	sessionsModeNewVibespaces              // step 2: vibespace picker
	sessionsModeNewAgents                  // step 3: agent picker per vibespace
)

// sessionItemType distinguishes tree item kinds.
type sessionItemType int

const (
	sessionItemGroup  sessionItemType = iota // vibespace group header
	sessionItemSingle                        // single-agent session (from container)
	sessionItemMulti                         // multi-agent session (from store)
)

// treeItem is a flattened entry in the sessions tree.
type treeItem struct {
	Type        sessionItemType
	isLastChild bool // last child in its group (use └── instead of ├──)

	// Group fields
	VSName      string
	VSStatus    string
	Expanded    bool
	Loading     bool
	SingleCount int
	MultiCount  int

	// Single-agent fields
	Session   vsSessionInfo
	AgentName string
	AgentType agent.Type
	VSParent  string

	// Multi-agent fields
	MultiSession *session.Session
	CrossVSCount int // how many OTHER vibespaces this session spans
}

// groupLoadState tracks lazy-loaded data for an expanded vibespace group.
type groupLoadState struct {
	Agents        []vibespace.AgentInfo
	AgentsLoaded  bool
	AgentSessions map[string][]vsSessionInfo // agentName → sessions
	AgentConfigs  map[string]*agent.Config   // agentName → config
	Loading       bool
}

// --- Message types ---

// multiSessionsLoadedMsg delivers multi-agent sessions from the store.
type multiSessionsLoadedMsg struct {
	sessions []session.Session
	err      error
}

// groupAgentsLoadedMsg delivers agents + configs for a vibespace group.
type groupAgentsLoadedMsg struct {
	vsName  string
	agents  []vibespace.AgentInfo
	configs map[string]*agent.Config
	err     error
}

// vibespacesForTreeMsg delivers vibespaces for tree grouping.
type vibespacesForTreeMsg struct {
	vibespaces []*model.Vibespace
	err        error
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

// singleSessionPreviewMsg delivers recent messages for a single-agent session.
type singleSessionPreviewMsg struct {
	sessionID string
	messages  []singlePreviewEntry // last few user/assistant messages
}

// singlePreviewEntry is a parsed message from a container session JSONL.
type singlePreviewEntry struct {
	Role string // "user" or "assistant"
	Text string
	Time time.Time
}

// vibespacesForPickerMsg delivers vibespaces for the new-session wizard.
type vibespacesForPickerMsg struct {
	vibespaces []*model.Vibespace
	err        error
}

// agentsForPickerMsg delivers agents for a specific vibespace in the wizard.
type agentsForPickerMsg struct {
	vibespace string
	agents    []vibespace.AgentInfo
	err       error
}

// vsPickerItem is a vibespace entry in the picker.
type vsPickerItem struct {
	Name       string
	Status     string
	AgentCount int // -1 if unknown
}

// agentPickerItem is an agent entry in the picker.
type agentPickerItem struct {
	Name      string
	AgentType string
	Status    string
}

// --- SessionsTab ---

// SessionsTab shows a unified tree of all sessions grouped by vibespace.
type SessionsTab struct {
	shared *SharedState
	width  int
	height int
	mode   sessionsMode
	err    string

	// Tree state
	vibespaces    []*model.Vibespace
	multiSessions []session.Session
	groupStates   map[string]*groupLoadState
	expandedVS    map[string]bool
	flatTree      []treeItem
	cursor        int

	// Preview state (for multi-session message preview)
	previewSessionName string
	previewMsgs        []*Message
	previewTotal       int

	// Single-session preview (lazy-loaded messages from pod)
	singlePreviewID   string               // session ID currently previewed
	singlePreviewMsgs []singlePreviewEntry // recent messages

	// Creation wizard state
	nameInput        textinput.Model
	newSessionName   string
	newVibespaces    []vsPickerItem
	newVSSelected    []bool
	newVSCursor      int
	newAgents        []agentPickerItem
	newAgentSelected []bool
	newAgentCursor   int
	newAgentVSIndex  int
	newSelectedVS    []vsPickerItem
	newVSAgents      map[string][]string
}

func NewSessionsTab(shared *SharedState) *SessionsTab {
	ti := textinput.New()
	ti.Placeholder = "press enter to skip"
	ti.CharLimit = 64
	return &SessionsTab{
		shared:      shared,
		nameInput:   ti,
		groupStates: make(map[string]*groupLoadState),
		expandedVS:  make(map[string]bool),
		newVSAgents: make(map[string][]string),
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
	case sessionsModeNewName:
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm/skip")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	case sessionsModeNewVibespaces:
		return []key.Binding{
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
			key.NewBinding(key.WithKeys("x", " "), key.WithHelp("x/space", "toggle")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	case sessionsModeNewAgents:
		return []key.Binding{
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
			key.NewBinding(key.WithKeys("x", " "), key.WithHelp("x/space", "toggle")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	default:
		return []key.Binding{
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/resume")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		}
	}
}

func (t *SessionsTab) SetSize(w, h int) { t.width = w; t.height = h }

// --- Init / Update ---

func (t *SessionsTab) Init() tea.Cmd {
	return t.loadAllData()
}

func (t *SessionsTab) loadAllData() tea.Cmd {
	return tea.Batch(t.loadVibespacesForTree(), t.loadMultiSessions())
}

func (t *SessionsTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case TabActivateMsg:
		return t, t.loadAllData()

	case PaletteNewSessionMsg:
		t.mode = sessionsModeNewName
		t.nameInput.SetValue("")
		t.nameInput.Focus()
		t.err = ""
		return t, t.nameInput.Cursor.BlinkCmd()

	case vibespacesForTreeMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		t.vibespaces = msg.vibespaces
		t.rebuildTree()
		// Auto-expand first group if nothing is expanded yet
		if len(t.expandedVS) == 0 && len(t.flatTree) > 0 && t.flatTree[0].Type == sessionItemGroup {
			return t, t.toggleGroup(t.flatTree[0].VSName)
		}
		return t, nil

	case multiSessionsLoadedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		t.multiSessions = msg.sessions
		t.rebuildTree()
		return t, t.loadPreviewForCurrent()

	case groupAgentsLoadedMsg:
		gs := t.groupStates[msg.vsName]
		if gs == nil {
			return t, nil
		}
		if msg.err != nil {
			gs.Loading = false
			t.err = msg.err.Error()
			t.rebuildTree()
			return t, nil
		}
		gs.Agents = msg.agents
		gs.AgentsLoaded = true
		gs.AgentConfigs = msg.configs
		gs.Loading = false
		t.rebuildTree()
		// Load sessions for each session-capable agent
		var cmds []tea.Cmd
		for _, ag := range msg.agents {
			if ag.AgentType == agent.TypeClaudeCode || ag.AgentType == agent.TypeCodex {
				cmds = append(cmds, loadAgentSessionsCmd(msg.vsName, ag.AgentName, ag.AgentType))
			}
		}
		return t, tea.Batch(cmds...)

	case vsSessionsLoadedMsg:
		gs := t.groupStates[msg.vsName]
		if gs == nil {
			return t, nil
		}
		if msg.err != nil {
			return t, nil // non-fatal
		}
		if gs.AgentSessions == nil {
			gs.AgentSessions = make(map[string][]vsSessionInfo)
		}
		gs.AgentSessions[msg.agentName] = msg.sessions
		t.rebuildTree()
		return t, nil

	case vsConnectReadyMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		if msg.mode == vsConnectModeSessionResume {
			return t, execSessionResumeCmd(msg.sshPort, msg.agentName, msg.agentType, msg.sessionID, msg.agentConfig)
		}
		return t, nil

	case vsSessionResumeMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		return t, t.loadAllData()

	case sessionHistoryMsg:
		t.previewSessionName = msg.sessionName
		t.previewMsgs = msg.messages
		t.previewTotal = msg.totalCount
		return t, nil

	case singleSessionPreviewMsg:
		t.singlePreviewID = msg.sessionID
		t.singlePreviewMsgs = msg.messages
		return t, nil

	case sessionDeletedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		t.mode = sessionsModeList
		return t, t.loadMultiSessions()

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

	case vibespacesForPickerMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		t.newVibespaces = make([]vsPickerItem, len(msg.vibespaces))
		t.newVSSelected = make([]bool, len(msg.vibespaces))
		for i, vs := range msg.vibespaces {
			t.newVibespaces[i] = vsPickerItem{Name: vs.Name, Status: vs.Status, AgentCount: -1}
		}
		t.newVSCursor = 0
		return t, nil

	case agentsForPickerMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		t.newAgents = make([]agentPickerItem, len(msg.agents))
		t.newAgentSelected = make([]bool, len(msg.agents))
		for i, a := range msg.agents {
			t.newAgents[i] = agentPickerItem{Name: a.AgentName, AgentType: string(a.AgentType), Status: a.Status}
		}
		t.newAgentCursor = 0
		return t, nil

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	if t.mode == sessionsModeNewName {
		var cmd tea.Cmd
		t.nameInput, cmd = t.nameInput.Update(msg)
		return t, cmd
	}

	return t, nil
}

// --- Key handlers ---

func (t *SessionsTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch t.mode {
	case sessionsModeDelete:
		return t.handleKeyDelete(msg)
	case sessionsModeNewName:
		return t.handleKeyNewName(msg)
	case sessionsModeNewVibespaces:
		return t.handleKeyNewVibespaces(msg)
	case sessionsModeNewAgents:
		return t.handleKeyNewAgents(msg)
	default:
		return t.handleKeyList(msg)
	}
}

func (t *SessionsTab) handleKeyList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if t.cursor < len(t.flatTree)-1 {
			t.cursor++
			return t, t.loadPreviewForCurrent()
		}
	case "k", "up":
		if t.cursor > 0 {
			t.cursor--
			return t, t.loadPreviewForCurrent()
		}
	case "g":
		if t.cursor != 0 {
			t.cursor = 0
			return t, t.loadPreviewForCurrent()
		}
	case "G":
		if len(t.flatTree) > 0 && t.cursor != len(t.flatTree)-1 {
			t.cursor = len(t.flatTree) - 1
			return t, t.loadPreviewForCurrent()
		}
	case "enter":
		if t.cursor < len(t.flatTree) {
			item := t.flatTree[t.cursor]
			switch item.Type {
			case sessionItemGroup:
				return t, t.toggleGroup(item.VSName)
			case sessionItemSingle:
				gs := t.groupStates[item.VSParent]
				var cfg *agent.Config
				if gs != nil && gs.AgentConfigs != nil {
					cfg = gs.AgentConfigs[item.AgentName]
				}
				return t, prepareSessionResumeCmd(item.VSParent, item.AgentName, item.AgentType, item.Session.ID, cfg)
			case sessionItemMulti:
				sess := item.MultiSession
				return t, func() tea.Msg {
					return SwitchToChatMsg{Session: sess, Resume: true}
				}
			}
		}
	case "n":
		t.mode = sessionsModeNewName
		t.nameInput.SetValue("")
		t.nameInput.Focus()
		t.err = ""
		return t, t.nameInput.Cursor.BlinkCmd()
	case "d":
		if t.cursor < len(t.flatTree) && t.flatTree[t.cursor].Type == sessionItemMulti {
			t.mode = sessionsModeDelete
			t.err = ""
		}
	case "r":
		return t, t.loadAllData()
	}
	return t, nil
}

func (t *SessionsTab) handleKeyDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		if t.cursor < len(t.flatTree) && t.flatTree[t.cursor].Type == sessionItemMulti {
			name := t.flatTree[t.cursor].MultiSession.Name
			return t, t.deleteSession(name)
		}
		t.mode = sessionsModeList
	case "esc", "n", "q":
		t.mode = sessionsModeList
	}
	return t, nil
}

func (t *SessionsTab) handleKeyNewName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(t.nameInput.Value())
		if name == "" {
			name = uuid.New().String()[:8]
		}
		if err := session.ValidateSessionName(name); err != nil {
			t.err = err.Error()
			return t, nil
		}
		t.newSessionName = name
		t.nameInput.Blur()
		t.err = ""
		t.mode = sessionsModeNewVibespaces
		return t, t.loadVibespacesForPicker()
	case "esc":
		t.resetNewSession()
		t.mode = sessionsModeList
	default:
		var cmd tea.Cmd
		t.nameInput, cmd = t.nameInput.Update(msg)
		return t, cmd
	}
	return t, nil
}

func (t *SessionsTab) handleKeyNewVibespaces(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if len(t.newVibespaces) > 0 {
			t.newVSCursor = min(t.newVSCursor+1, len(t.newVibespaces)-1)
		}
	case "k", "up":
		if len(t.newVibespaces) > 0 {
			t.newVSCursor = max(t.newVSCursor-1, 0)
		}
	case "x", " ":
		if t.newVSCursor < len(t.newVSSelected) {
			t.newVSSelected[t.newVSCursor] = !t.newVSSelected[t.newVSCursor]
		}
	case "enter":
		t.newSelectedVS = nil
		for i, vs := range t.newVibespaces {
			if t.newVSSelected[i] {
				t.newSelectedVS = append(t.newSelectedVS, vs)
			}
		}
		t.newVSAgents = make(map[string][]string)
		if len(t.newSelectedVS) == 0 {
			return t, t.finalizeNewSession()
		}
		t.newAgentVSIndex = 0
		t.mode = sessionsModeNewAgents
		return t, t.loadAgentsForPicker(t.newSelectedVS[0].Name)
	case "esc":
		t.resetNewSession()
		t.mode = sessionsModeList
	}
	return t, nil
}

func (t *SessionsTab) handleKeyNewAgents(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if len(t.newAgents) > 0 {
			t.newAgentCursor = min(t.newAgentCursor+1, len(t.newAgents)-1)
		}
	case "k", "up":
		if len(t.newAgents) > 0 {
			t.newAgentCursor = max(t.newAgentCursor-1, 0)
		}
	case "x", " ":
		if t.newAgentCursor < len(t.newAgentSelected) {
			t.newAgentSelected[t.newAgentCursor] = !t.newAgentSelected[t.newAgentCursor]
		}
	case "enter":
		vsName := t.newSelectedVS[t.newAgentVSIndex].Name
		var selected []string
		selectedCount := 0
		for i, a := range t.newAgents {
			if t.newAgentSelected[i] {
				selected = append(selected, a.Name)
				selectedCount++
			}
		}
		if selectedCount == 0 || selectedCount == len(t.newAgents) {
			selected = nil
		}
		t.newVSAgents[vsName] = selected
		t.newAgentVSIndex++
		if t.newAgentVSIndex < len(t.newSelectedVS) {
			t.mode = sessionsModeNewAgents
			return t, t.loadAgentsForPicker(t.newSelectedVS[t.newAgentVSIndex].Name)
		}
		return t, t.finalizeNewSession()
	case "esc":
		t.resetNewSession()
		t.mode = sessionsModeList
	}
	return t, nil
}

// --- View ---

func (t *SessionsTab) View() string {
	// Wizard modes render full-width
	if t.mode == sessionsModeNewName || t.mode == sessionsModeNewVibespaces || t.mode == sessionsModeNewAgents {
		return t.viewWizard()
	}

	if len(t.flatTree) == 0 {
		if t.err != "" {
			return lipgloss.NewStyle().
				Foreground(ui.ColorError).
				Padding(1, 2).
				Render(fmt.Sprintf("Error: %s", t.err))
		}
		return t.viewEmpty()
	}

	// Split pane: left tree (~45%) + divider + right preview (~55%)
	leftWidth := t.width * 45 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := t.width - leftWidth - 1 // 1 for divider

	contentHeight := t.height
	if t.mode == sessionsModeDelete || t.err != "" {
		contentHeight -= 2 // reserve space for prompt/error
	}
	if contentHeight < 3 {
		contentHeight = 3
	}

	left := t.viewTree(leftWidth, contentHeight)
	right := t.viewPreview(rightWidth, contentHeight)

	// Build divider column
	var divLines []string
	for i := 0; i < contentHeight; i++ {
		divLines = append(divLines, "│")
	}
	divider := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Join(divLines, "\n"))

	main := lipgloss.JoinHorizontal(lipgloss.Top, left, divider, right)

	var parts []string
	parts = append(parts, main)

	if t.mode == sessionsModeDelete && t.cursor < len(t.flatTree) && t.flatTree[t.cursor].Type == sessionItemMulti {
		name := t.flatTree[t.cursor].MultiSession.Name
		parts = append(parts, lipgloss.NewStyle().Foreground(ui.ColorWarning).Padding(0, 2).
			Render(fmt.Sprintf("Delete session %q? y to confirm, Esc to cancel", name)))
	}

	if t.err != "" && t.mode == sessionsModeList {
		parts = append(parts, lipgloss.NewStyle().Foreground(ui.ColorError).Padding(0, 2).Render(t.err))
	}

	return strings.Join(parts, "\n")
}

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

func (t *SessionsTab) viewTree(width, height int) string {
	var lines []string
	for i, item := range t.flatTree {
		lines = append(lines, t.renderTreeItem(item, i == t.cursor, width-4))
	}

	// Legend
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("● single  ◆ multi"))

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(width).
		Height(height).
		Render(content)
}

func (t *SessionsTab) renderTreeItem(item treeItem, selected bool, _ int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)

	switch item.Type {
	case sessionItemGroup:
		arrow := "▶ "
		if item.Expanded {
			arrow = "▼ "
		}

		var badge string
		if item.Loading {
			badge = " …"
		} else if !item.Expanded {
			var parts []string
			if item.SingleCount > 0 {
				parts = append(parts, fmt.Sprintf("%d●", item.SingleCount))
			}
			if item.MultiCount > 0 {
				parts = append(parts, fmt.Sprintf("%d◆", item.MultiCount))
			}
			if len(parts) > 0 {
				badge = " " + strings.Join(parts, " ")
			}
		}

		if selected {
			return renderGradientText(arrow+item.VSName+badge, getBrandGradient())
		}
		return dimStyle.Render(arrow) +
			lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(item.VSName) +
			dimStyle.Render(badge)

	case sessionItemSingle:
		connector := "  ├── "
		if item.isLastChild {
			connector = "  └── "
		}
		marker := "● "
		age := formatSessionAge(item.Session.LastTime)
		shortID := item.Session.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		agentColor := t.agentColor(item.AgentName)

		if selected {
			return renderGradientText(connector+marker+item.AgentName+" "+shortID+" "+age, getBrandGradient())
		}
		return dimStyle.Render(connector) +
			lipgloss.NewStyle().Foreground(agentColor).Render(marker+item.AgentName) +
			" " + dimStyle.Render(shortID) +
			" " + lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(age)

	case sessionItemMulti:
		connector := "  ├── "
		if item.isLastChild {
			connector = "  └── "
		}
		marker := "◆ "
		name := item.MultiSession.Name
		agents := sessAgentCount(*item.MultiSession)
		age := timeAgo(item.MultiSession.LastUsed)

		var crossVS string
		if item.CrossVSCount > 0 {
			crossVS = fmt.Sprintf(" +%dvs", item.CrossVSCount)
		}

		if selected {
			return renderGradientText(connector+marker+name+" "+agents+" agt"+crossVS+" "+age, getBrandGradient())
		}
		return dimStyle.Render(connector) +
			lipgloss.NewStyle().Foreground(ui.Orange).Render(marker+name) +
			" " + dimStyle.Render(agents+" agt"+crossVS) +
			" " + lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(age)
	}

	return ""
}

func (t *SessionsTab) viewPreview(width, height int) string {
	if t.cursor >= len(t.flatTree) {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	item := t.flatTree[t.cursor]
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)

	var lines []string

	switch item.Type {
	case sessionItemGroup:
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(item.VSName))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Status"), dimStyle.Render(item.VSStatus)))
		gs := t.groupStates[item.VSName]
		if gs != nil && gs.AgentsLoaded {
			lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Agents"), dimStyle.Render(fmt.Sprintf("%d", len(gs.Agents)))))
			totalSingle := 0
			for _, sessions := range gs.AgentSessions {
				totalSingle += len(sessions)
			}
			lines = append(lines, fmt.Sprintf("%s  %s single, %d multi",
				labelStyle.Render("Sessions"),
				dimStyle.Render(fmt.Sprintf("%d", totalSingle)),
				item.MultiCount))
		} else if gs != nil && gs.Loading {
			lines = append(lines, dimStyle.Render("Loading agents..."))
		}

	case sessionItemSingle:
		sess := item.Session
		agentColor := t.agentColor(item.AgentName)
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render("Session"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("ID"), dimStyle.Render(sess.ID)))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Agent"),
			lipgloss.NewStyle().Foreground(agentColor).Render(item.AgentName)+" "+dimStyle.Render("("+string(item.AgentType)+")")))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Prompts"), dimStyle.Render(fmt.Sprintf("%d", sess.Prompts))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Last active"), dimStyle.Render(formatSessionAge(sess.LastTime))))

		if t.singlePreviewID == sess.ID && len(t.singlePreviewMsgs) > 0 {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("Recent Messages"))
			for _, m := range t.singlePreviewMsgs {
				ts := m.Time.Format("15:04")
				tsStr := lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(ts)
				text := truncate(m.Text, 60)
				if m.Role == "user" {
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(ui.Teal).Render("You →"),
						dimStyle.Render(text)))
				} else {
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(agentColor).Render(item.AgentName),
						dimStyle.Render(text)))
				}
			}
		} else if t.singlePreviewID == sess.ID {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("No messages yet."))
		}

	case sessionItemMulti:
		ms := item.MultiSession
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(ms.Name))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Vibespaces"), dimStyle.Render(sessVibespaceNames(*ms))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Agents"), dimStyle.Render(sessAgentCount(*ms))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Created"), dimStyle.Render(ms.CreatedAt.Format("2006-01-02 15:04:05"))))
		lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Last used"), dimStyle.Render(timeAgo(ms.LastUsed))))

		if t.previewSessionName == ms.Name && t.previewTotal > 0 {
			lines = append(lines, fmt.Sprintf("%s  %s", labelStyle.Render("Messages"), dimStyle.Render(fmt.Sprintf("%d", t.previewTotal))))
		}

		if t.previewSessionName == ms.Name && len(t.previewMsgs) > 0 {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("Recent Messages"))

			agentColorMap := make(map[string]lipgloss.Color)
			agentIdx := 0
			for _, m := range t.previewMsgs {
				if m.Type != MessageTypeUser && m.Sender != "" {
					if _, ok := agentColorMap[m.Sender]; !ok {
						agentColorMap[m.Sender] = ui.GetAgentColor(agentIdx)
						agentIdx++
					}
				}
			}

			for _, m := range t.previewMsgs {
				ts := m.Timestamp.Format("15:04")
				tsStr := lipgloss.NewStyle().Foreground(ui.ColorTimestamp).Render(ts)
				switch m.Type {
				case MessageTypeUser:
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(ui.Teal).Render("You →"),
						dimStyle.Render(truncate(m.Content, 60))))
				case MessageTypeAssistant:
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(agentColorMap[m.Sender]).Render(m.Sender),
						dimStyle.Render(truncate(m.Content, 60))))
				case MessageTypeToolUse:
					lines = append(lines, fmt.Sprintf("%s %s %s",
						tsStr,
						lipgloss.NewStyle().Bold(true).Foreground(agentColorMap[m.Sender]).Render(m.Sender),
						lipgloss.NewStyle().Foreground(ui.Orange).Render(fmt.Sprintf("[%s] %s", m.ToolName, truncate(m.ToolInput, 40)))))
				}
			}
		} else if t.previewSessionName == ms.Name {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).Render("No messages yet."))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(width).
		Height(height).
		Render(content)
}

func (t *SessionsTab) viewWizard() string {
	style := lipgloss.NewStyle().Padding(1, 2)
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.Teal).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted).Italic(true)

	var content string

	switch t.mode {
	case sessionsModeNewName:
		step := dimStyle.Render("Step 1/3")
		label := labelStyle.Render("Session name: ")
		hint := hintStyle.Render("  enter to skip (auto-generated)")
		content = step + "  " + label + t.nameInput.View() + hint
		if t.err != "" {
			content += "\n" + lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err)
		}

	case sessionsModeNewVibespaces:
		step := dimStyle.Render("Step 2/3")
		header := labelStyle.Render(fmt.Sprintf("Select vibespaces for %q", t.newSessionName))

		var lines []string
		lines = append(lines, step+"  "+header)

		if len(t.newVibespaces) == 0 {
			lines = append(lines, "  "+hintStyle.Render("No vibespaces found. Press enter to create an empty session."))
		} else {
			for i, vs := range t.newVibespaces {
				cursor := "  "
				if i == t.newVSCursor {
					cursor = renderGradientText("› ", getBrandGradient())
				}
				check := "[ ]"
				if t.newVSSelected[i] {
					check = "[x]"
				}
				status := dimStyle.Render(vs.Status)
				name := vs.Name
				if i == t.newVSCursor {
					name = lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render(name)
				} else {
					name = dimStyle.Render(name)
				}
				lines = append(lines, fmt.Sprintf("  %s%s %s  %s", cursor, check, name, status))
			}
		}

		lines = append(lines, "  "+hintStyle.Render("x toggle  enter confirm  enter with none to skip"))
		if t.err != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err))
		}
		content = strings.Join(lines, "\n")

	case sessionsModeNewAgents:
		vsName := ""
		if t.newAgentVSIndex < len(t.newSelectedVS) {
			vsName = t.newSelectedVS[t.newAgentVSIndex].Name
		}
		step := dimStyle.Render("Step 3/3")
		header := labelStyle.Render(fmt.Sprintf("Select agents from %q", vsName))

		var lines []string
		lines = append(lines, step+"  "+header)

		if len(t.newAgents) == 0 {
			lines = append(lines, "  "+hintStyle.Render("Loading agents..."))
		} else {
			for i, a := range t.newAgents {
				cursor := "  "
				if i == t.newAgentCursor {
					cursor = renderGradientText("› ", getBrandGradient())
				}
				check := "[ ]"
				if t.newAgentSelected[i] {
					check = "[x]"
				}
				status := dimStyle.Render(a.Status)
				agentType := dimStyle.Render(a.AgentType)
				name := a.Name
				if i == t.newAgentCursor {
					name = lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render(name)
				} else {
					name = dimStyle.Render(name)
				}
				lines = append(lines, fmt.Sprintf("  %s%s %s  %s  %s", cursor, check, name, agentType, status))
			}
		}

		lines = append(lines, "  "+hintStyle.Render("x toggle  enter confirm  enter with none for all agents"))
		if t.err != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err))
		}
		content = strings.Join(lines, "\n")
	}

	return style.Render(content)
}

// --- Tree building ---

func (t *SessionsTab) rebuildTree() {
	var items []treeItem

	// Track which multi-sessions get placed under at least one group
	placedMulti := make(map[string]bool)

	for _, vs := range t.vibespaces {
		expanded := t.expandedVS[vs.Name]
		gs := t.groupStates[vs.Name]

		singleCount := 0
		multiCount := 0
		loading := false

		if gs != nil {
			loading = gs.Loading
			for _, sessions := range gs.AgentSessions {
				singleCount += len(sessions)
			}
		}
		for _, ms := range t.multiSessions {
			for _, ve := range ms.Vibespaces {
				if ve.Name == vs.Name {
					multiCount++
					placedMulti[ms.Name] = true
					break
				}
			}
		}

		items = append(items, treeItem{
			Type:        sessionItemGroup,
			VSName:      vs.Name,
			VSStatus:    vs.Status,
			Expanded:    expanded,
			Loading:     loading,
			SingleCount: singleCount,
			MultiCount:  multiCount,
		})

		if !expanded {
			continue
		}

		// Single-agent sessions
		if gs != nil && gs.AgentsLoaded {
			for _, ag := range gs.Agents {
				if sessions, ok := gs.AgentSessions[ag.AgentName]; ok {
					for _, sess := range sessions {
						items = append(items, treeItem{
							Type:      sessionItemSingle,
							Session:   sess,
							AgentName: ag.AgentName,
							AgentType: ag.AgentType,
							VSParent:  vs.Name,
						})
					}
				}
			}
		}

		// Multi-agent sessions referencing this vibespace
		for i := range t.multiSessions {
			ms := &t.multiSessions[i]
			for _, ve := range ms.Vibespaces {
				if ve.Name == vs.Name {
					items = append(items, treeItem{
						Type:         sessionItemMulti,
						MultiSession: ms,
						CrossVSCount: len(ms.Vibespaces) - 1,
						VSParent:     vs.Name,
					})
					break
				}
			}
		}
	}

	// Orphan multi-sessions (no matching vibespace group)
	var orphans []treeItem
	for i := range t.multiSessions {
		ms := &t.multiSessions[i]
		if !placedMulti[ms.Name] {
			orphans = append(orphans, treeItem{
				Type:         sessionItemMulti,
				MultiSession: ms,
				CrossVSCount: 0,
			})
		}
	}
	if len(orphans) > 0 {
		items = append(items, treeItem{
			Type:       sessionItemGroup,
			VSName:     "Ungrouped",
			VSStatus:   "",
			Expanded:   true,
			MultiCount: len(orphans),
		})
		items = append(items, orphans...)
	}

	// Mark last child in each group
	for i := range items {
		if items[i].Type != sessionItemGroup {
			items[i].isLastChild = i == len(items)-1 || items[i+1].Type == sessionItemGroup
		}
	}

	t.flatTree = items
	if t.cursor >= len(t.flatTree) {
		t.cursor = max(len(t.flatTree)-1, 0)
	}
}

func (t *SessionsTab) toggleGroup(vsName string) tea.Cmd {
	if t.expandedVS[vsName] {
		t.expandedVS[vsName] = false
		t.rebuildTree()
		return nil
	}

	t.expandedVS[vsName] = true

	gs := t.groupStates[vsName]
	if gs != nil && gs.AgentsLoaded {
		t.rebuildTree()
		return nil
	}

	if gs == nil {
		gs = &groupLoadState{
			AgentSessions: make(map[string][]vsSessionInfo),
			AgentConfigs:  make(map[string]*agent.Config),
		}
		t.groupStates[vsName] = gs
	}
	gs.Loading = true
	t.rebuildTree()
	return t.loadGroupAgents(vsName)
}

// --- Commands ---

func (t *SessionsTab) loadVibespacesForTree() tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vibespacesForTreeMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		vs, err := svc.List(ctx)
		return vibespacesForTreeMsg{vibespaces: vs, err: err}
	}
}

func (t *SessionsTab) loadMultiSessions() tea.Cmd {
	store := t.shared.SessionStore
	return func() tea.Msg {
		if store == nil {
			return multiSessionsLoadedMsg{err: fmt.Errorf("session store unavailable")}
		}
		sessions, err := store.List()
		return multiSessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (t *SessionsTab) loadGroupAgents(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return groupAgentsLoadedMsg{vsName: vsName, err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		agents, err := svc.ListAgents(ctx, vsName)
		if err != nil {
			return groupAgentsLoadedMsg{vsName: vsName, err: err}
		}

		configs := make(map[string]*agent.Config)
		for _, ag := range agents {
			cfg, cerr := svc.GetAgentConfig(ctx, vsName, ag.AgentName)
			if cerr == nil && cfg != nil {
				configs[ag.AgentName] = cfg
			}
		}

		return groupAgentsLoadedMsg{vsName: vsName, agents: agents, configs: configs}
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

func (t *SessionsTab) loadPreviewForCurrent() tea.Cmd {
	if t.cursor >= len(t.flatTree) {
		return nil
	}
	item := t.flatTree[t.cursor]

	switch item.Type {
	case sessionItemSingle:
		sessID := item.Session.ID
		if sessID == t.singlePreviewID {
			return nil
		}
		vsName := item.VSParent
		agentName := item.AgentName
		agentType := item.AgentType
		return func() tea.Msg {
			entries := loadSingleSessionMessages(vsName, agentName, agentType, sessID, 5)
			return singleSessionPreviewMsg{sessionID: sessID, messages: entries}
		}

	case sessionItemMulti:
		if item.MultiSession == nil {
			return nil
		}
		name := item.MultiSession.Name
		if name == t.previewSessionName {
			return nil
		}
		hs := t.shared.HistoryStore
		return func() tea.Msg {
			if hs == nil {
				return sessionHistoryMsg{sessionName: name}
			}
			allMsgs, _ := hs.Load(name)
			totalCount := len(allMsgs)
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

	return nil
}

func (t *SessionsTab) loadVibespacesForPicker() tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vibespacesForPickerMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		vs, err := svc.List(ctx)
		return vibespacesForPickerMsg{vibespaces: vs, err: err}
	}
}

func (t *SessionsTab) loadAgentsForPicker(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return agentsForPickerMsg{vibespace: vsName, err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := svc.ListAgents(ctx, vsName)
		return agentsForPickerMsg{vibespace: vsName, agents: agents, err: err}
	}
}

func (t *SessionsTab) finalizeNewSession() tea.Cmd {
	store := t.shared.SessionStore
	name := t.newSessionName
	selectedVS := t.newSelectedVS
	vsAgents := t.newVSAgents
	return func() tea.Msg {
		if store == nil {
			return sessionCreatedMsg{err: fmt.Errorf("session store unavailable")}
		}
		sess, err := store.Create(name)
		if err != nil {
			return sessionCreatedMsg{err: err}
		}
		for _, vs := range selectedVS {
			agents := vsAgents[vs.Name]
			sess.AddVibespace(vs.Name, agents)
		}
		if err := store.Save(sess); err != nil {
			return sessionCreatedMsg{err: err}
		}
		return sessionCreatedMsg{session: sess}
	}
}

func (t *SessionsTab) resetNewSession() {
	t.nameInput.SetValue("")
	t.nameInput.Blur()
	t.newSessionName = ""
	t.newVibespaces = nil
	t.newVSSelected = nil
	t.newVSCursor = 0
	t.newAgents = nil
	t.newAgentSelected = nil
	t.newAgentCursor = 0
	t.newAgentVSIndex = 0
	t.newSelectedVS = nil
	t.newVSAgents = make(map[string][]string)
	t.err = ""
}

// agentColor returns a stable color for an agent name based on all known agents.
// Skips orange (reserved for ◆ multi-session markers) to avoid visual overlap.
func (t *SessionsTab) agentColor(agentName string) lipgloss.Color {
	// Collect all unique agent names in order from group states
	var allAgents []string
	seen := make(map[string]bool)
	for _, vs := range t.vibespaces {
		gs := t.groupStates[vs.Name]
		if gs == nil || !gs.AgentsLoaded {
			continue
		}
		for _, ag := range gs.Agents {
			if !seen[ag.AgentName] {
				seen[ag.AgentName] = true
				allAgents = append(allAgents, ag.AgentName)
			}
		}
	}
	// Build palette excluding orange (used for multi-session ◆)
	var palette []lipgloss.Color
	for _, c := range ui.AgentColors {
		if c != ui.Orange {
			palette = append(palette, c)
		}
	}
	if len(palette) == 0 {
		palette = ui.AgentColors
	}
	for i, name := range allAgents {
		if name == agentName {
			return palette[i%len(palette)]
		}
	}
	return palette[0]
}

// loadSingleSessionMessages SSHs into the agent pod and reads the last N
// user/assistant messages from the session's JSONL file.
func loadSingleSessionMessages(vsName, agentName string, agentType agent.Type, sessionID string, maxMsgs int) []singlePreviewEntry {
	sshPort, err := ensureSSHForwardForAgent(vsName, agentName)
	if err != nil {
		return nil
	}
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return nil
	}

	// Determine session file path based on agent type
	var sessionFile string
	switch agentType {
	case agent.TypeCodex:
		sessionFile = fmt.Sprintf("~/.codex/sessions/%s.jsonl", sessionID)
	default:
		sessionFile = fmt.Sprintf("~/.claude/projects/-vibespace/%s.jsonl", sessionID)
	}

	// Tail enough lines to get recent messages (each turn is ~1-2 lines)
	remoteCmd := fmt.Sprintf("tail -40 %s 2>/dev/null || true", sessionFile)

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=3",
		"user@localhost",
		remoteCmd,
	)
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return nil
	}

	// Parse JSONL lines for user/assistant text
	var entries []singlePreviewEntry
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		var raw struct {
			Type    string `json:"type"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}
		if raw.Type != "user" && raw.Type != "assistant" {
			continue
		}

		text := extractContentText(raw.Message.Content)
		if text == "" {
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, raw.Timestamp)
		entries = append(entries, singlePreviewEntry{
			Role: raw.Type,
			Text: text,
			Time: ts,
		})
	}

	if len(entries) > maxMsgs {
		entries = entries[len(entries)-maxMsgs:]
	}
	return entries
}

// extractContentText extracts text from a Claude JSONL content field.
// Content can be a plain string or an array of content blocks.
func extractContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try plain string first
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	// Try array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return strings.TrimSpace(b.Text)
			}
		}
	}
	return ""
}

// --- Helpers ---

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

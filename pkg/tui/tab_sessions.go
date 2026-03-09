package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/session"
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

	// Single session fields
	Session   vsSessionInfo
	AgentName string
	AgentType agent.Type
	VSParent  string

	// Multi session fields
	MultiSession *session.Session
	CrossVSCount int // how many other vibespaces this session spans
}

// groupLoadState tracks per-vibespace loading state for the sessions tree.
type groupLoadState struct {
	Loading       bool
	AgentsLoaded  bool
	Agents        []vibespace.AgentInfo
	AgentSessions map[string][]vsSessionInfo // agent name → sessions
	AgentConfigs  map[string]*agent.Config   // agent name → config
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

package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/ui"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// vibespacesMode represents the current UI mode of the Vibespaces tab.
type vibespacesMode int

const (
	vibespacesModeList          vibespacesMode = iota // table view
	vibespacesModeAgentView                           // full-screen agent detail
	vibespacesModeSessionList                         // session list for an agent
	vibespacesModeCreateForm                          // inline create form
	vibespacesModeDeleteConfirm                       // inline delete confirmation
	vibespacesModeAddAgent                            // inline add agent form (in agent view)
)

// vsConnectMode distinguishes connect action types.
type vsConnectMode int

const (
	vsConnectModeSessionResume vsConnectMode = iota // resume a session
	vsConnectModeShell                              // raw SSH shell
	vsConnectModeAgentCLI                           // SSH + agent interactive CLI
)

// vibespacesLoadedMsg delivers vibespace data from the service.
type vibespacesLoadedMsg struct {
	vibespaces []*model.Vibespace
	err        error
}

// agentInfoLoadedMsg delivers agent info (names + counts) for all vibespaces.
type agentInfoLoadedMsg struct {
	counts map[string]int      // vibespace ID → agent count
	names  map[string][]string // vibespace ID → agent names
}

// vsLogsLoadedMsg delivers recent logs for a vibespace.
type vsLogsLoadedMsg struct {
	vibespaceID string
	lines       []string
	err         error
}

// vsAgentsLoadedMsg delivers the full agent list for the expanded vibespace.
type vsAgentsLoadedMsg struct {
	vibespaceID string
	agents      []vibespace.AgentInfo
	err         error
}

// vsAgentConfigsLoadedMsg delivers agent configs for the expanded vibespace.
type vsAgentConfigsLoadedMsg struct {
	vibespaceID string
	configs     map[string]*agent.Config // agent name → config
}

// vsForwardsLoadedMsg delivers forward info from the daemon.
type vsForwardsLoadedMsg struct {
	vibespaceID string
	agents      []daemon.AgentStatus
}

// vsSessionInfo represents a parsed Claude Code session summary.
type vsSessionInfo struct {
	ID       string
	Title    string
	LastTime time.Time
	Prompts  int
}

// vsSessionsLoadedMsg delivers parsed sessions from inside the agent pod.
type vsSessionsLoadedMsg struct {
	agentName string
	sessions  []vsSessionInfo
	err       error
}

// vsConnectReadyMsg signals that SSH forward is ready for a connect action.
type vsConnectReadyMsg struct {
	sshPort   int
	agentName string
	agentType agent.Type
	sessionID string
	mode      vsConnectMode
	err       error
}

// vsSessionResumeMsg signals that a session resume process has completed.
type vsSessionResumeMsg struct {
	err error
}

// vsBrowserReadyMsg signals that a ttyd forward is ready for browser open.
type vsBrowserReadyMsg struct {
	ttydPort int
	err      error
}

// vsExecReturnMsg signals that a shell/agent CLI process has completed.
type vsExecReturnMsg struct {
	err error
}

// vsRefreshTickMsg triggers a periodic reload while vibespaces are in a transitional state.
type vsRefreshTickMsg struct{}

// vsCreateDoneMsg signals completion of a vibespace creation.
type vsCreateDoneMsg struct {
	err error
}

// vsDeleteDoneMsg signals completion of a vibespace deletion.
type vsDeleteDoneMsg struct {
	err error
}

// vsStartStopDoneMsg signals completion of a start/stop operation.
type vsStartStopDoneMsg struct {
	action string
	err    error
}

// vsAddAgentDoneMsg signals completion of an agent spawn.
type vsAddAgentDoneMsg struct {
	err error
}

// createFormField identifies which field is active in the create form.
type createFormField int

const (
	createFieldName      createFormField = iota
	createFieldAgentType                        // selector (j/k)
	createFieldCPU
	createFieldMemory
	createFieldStorage
	createFieldCount // sentinel
)

// addAgentFormField identifies which field is active in the add agent form.
type addAgentFormField int

const (
	addAgentFieldType  addAgentFormField = iota // selector (j/k)
	addAgentFieldName                           // text input, optional
	addAgentFieldCount                          // sentinel
)

// VibespacesTab displays the vibespace list with inline expansion.
type VibespacesTab struct {
	shared     *SharedState
	vibespaces []*model.Vibespace
	selected   int
	width      int
	height     int
	err        string
	mode       vibespacesMode

	agentCounts map[string]int      // vibespace ID → count
	agentNames  map[string][]string // vibespace ID → agent names

	logsID    string   // vibespace ID logs are for
	logsLines []string // cached recent log lines

	// Agent view state
	selectedVS   *model.Vibespace             // vibespace being viewed
	viewAgents   []vibespace.AgentInfo         // agents for the selected vibespace
	agentConfigs map[string]*agent.Config      // agent name → config
	forwards     []daemon.AgentStatus          // forward info from daemon
	agentCursor  int                           // cursor position within agents list

	// Session list state
	sessions         []vsSessionInfo // sessions for the selected agent
	sessionCursor    int             // cursor in session list
	sessionAgent     string          // agent name whose sessions are shown
	sessionAgentType agent.Type      // agent type whose sessions are shown

	// Create form state
	createField     createFormField
	createName      string
	createAgentType agent.Type
	createCPU       string
	createMemory    string
	createStorage   string

	// Delete confirm state
	deleteName  string
	deleteInput string

	// Add agent form state (agent view)
	addAgentField addAgentFormField
	addAgentType  agent.Type
	addAgentName  string
}

func NewVibespacesTab(shared *SharedState) *VibespacesTab {
	return &VibespacesTab{
		shared:      shared,
		agentCounts: make(map[string]int),
		agentNames:  make(map[string][]string),
	}
}

func (t *VibespacesTab) Title() string { return TabNames[TabVibespaces] }

func (t *VibespacesTab) ShortHelp() []key.Binding {
	switch t.mode {
	case vibespacesModeCreateForm:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "skip")),
			key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "submit")),
		}
	case vibespacesModeDeleteConfirm:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		}
	case vibespacesModeAddAgent:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "skip")),
			key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "submit")),
		}
	case vibespacesModeSessionList:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume")),
		}
	case vibespacesModeAgentView:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate agents")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "sessions")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add agent")),
			key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "connect")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "browser")),
		}
	default:
		return []key.Binding{
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "create")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
			key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "start/stop")),
			key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "connect")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "browser")),
		}
	}
}

func (t *VibespacesTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *VibespacesTab) Init() tea.Cmd {
	return t.loadVibespaces()
}

func (t *VibespacesTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TabActivateMsg:
		switch t.mode {
		case vibespacesModeSessionList:
			if t.selectedVS != nil && t.sessionAgent != "" {
				return t, t.loadSessions(t.selectedVS.Name, t.sessionAgent, t.sessionAgentType)
			}
		case vibespacesModeAgentView:
			if t.selectedVS != nil {
				return t, tea.Batch(
					t.loadAgentsForView(t.selectedVS.ID, t.selectedVS.Name),
					t.loadAgentConfigs(t.selectedVS.ID, t.selectedVS.Name),
					t.loadForwards(t.selectedVS.ID, t.selectedVS.Name),
				)
			}
		}
		return t, t.loadVibespaces()

	case vibespacesLoadedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		t.vibespaces = msg.vibespaces
		t.clampSelected()
		t.err = ""
		return t, tea.Batch(t.loadAgentInfo(), t.loadLogsForSelected(), t.scheduleRefreshIfNeeded())

	case vsRefreshTickMsg:
		return t, t.loadVibespaces()

	case agentInfoLoadedMsg:
		t.agentCounts = msg.counts
		t.agentNames = msg.names
		return t, nil

	case vsLogsLoadedMsg:
		if msg.err == nil && t.selectedID() == msg.vibespaceID {
			t.logsID = msg.vibespaceID
			t.logsLines = msg.lines
		}
		return t, nil

	case vsAgentsLoadedMsg:
		if msg.err == nil && t.selectedVS != nil && t.selectedVS.ID == msg.vibespaceID {
			t.viewAgents = msg.agents
			t.clampAgentCursor()
		}
		return t, nil

	case vsAgentConfigsLoadedMsg:
		if t.selectedVS != nil && t.selectedVS.ID == msg.vibespaceID {
			t.agentConfigs = msg.configs
		}
		return t, nil

	case vsForwardsLoadedMsg:
		if t.selectedVS != nil && t.selectedVS.ID == msg.vibespaceID {
			t.forwards = msg.agents
		}
		return t, nil

	case vsSessionsLoadedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		if t.sessionAgent == msg.agentName {
			t.sessions = msg.sessions
			t.sessionCursor = 0
			t.err = ""
		}
		return t, nil

	case vsConnectReadyMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		switch msg.mode {
		case vsConnectModeSessionResume:
			return t, t.execSessionResume(msg.sshPort, msg.agentName, msg.agentType, msg.sessionID)
		case vsConnectModeShell:
			return t, t.execShellConnect(msg.sshPort)
		case vsConnectModeAgentCLI:
			return t, t.execAgentConnect(msg.sshPort, msg.agentName, msg.agentType)
		}
		return t, nil

	case vsSessionResumeMsg:
		// Returned from tea.ExecProcess — refresh data
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		if t.selectedVS != nil && t.sessionAgent != "" {
			return t, t.loadSessions(t.selectedVS.Name, t.sessionAgent, t.sessionAgentType)
		}
		return t, nil

	case vsBrowserReadyMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		url := fmt.Sprintf("http://localhost:%d", msg.ttydPort)
		if err := openBrowserURL(url); err != nil {
			t.err = fmt.Sprintf("open browser: %s", err)
		}
		return t, nil

	case vsExecReturnMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		switch t.mode {
		case vibespacesModeAgentView:
			if t.selectedVS != nil {
				return t, tea.Batch(
					t.loadAgentsForView(t.selectedVS.ID, t.selectedVS.Name),
					t.loadForwards(t.selectedVS.ID, t.selectedVS.Name),
				)
			}
		case vibespacesModeList:
			return t, t.loadVibespaces()
		}
		return t, nil

	case vsCreateDoneMsg:
		t.mode = vibespacesModeList
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		return t, t.loadVibespaces()

	case vsDeleteDoneMsg:
		t.mode = vibespacesModeList
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		return t, t.loadVibespaces()

	case vsStartStopDoneMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		return t, t.loadVibespaces()

	case vsAddAgentDoneMsg:
		t.mode = vibespacesModeAgentView
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		if t.selectedVS != nil {
			return t, tea.Batch(
				t.loadAgentsForView(t.selectedVS.ID, t.selectedVS.Name),
				t.loadAgentConfigs(t.selectedVS.ID, t.selectedVS.Name),
				t.loadForwards(t.selectedVS.ID, t.selectedVS.Name),
			)
		}
		return t, nil

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	return t, nil
}

func (t *VibespacesTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch t.mode {
	case vibespacesModeSessionList:
		switch msg.String() {
		case "esc", "backspace":
			t.mode = vibespacesModeAgentView
			t.sessions = nil
			t.sessionCursor = 0
			t.sessionAgent = ""
			return t, nil
		case "j", "down":
			if len(t.sessions) > 0 {
				t.sessionCursor = min(t.sessionCursor+1, len(t.sessions)-1)
			}
		case "k", "up":
			if len(t.sessions) > 0 {
				t.sessionCursor = max(t.sessionCursor-1, 0)
			}
		case "g":
			t.sessionCursor = 0
		case "G":
			if len(t.sessions) > 0 {
				t.sessionCursor = len(t.sessions) - 1
			}
		case "enter":
			if t.sessionCursor < len(t.sessions) && t.selectedVS != nil {
				sess := t.sessions[t.sessionCursor]
				return t, t.prepareSessionResume(t.selectedVS.Name, t.sessionAgent, t.sessionAgentType, sess.ID)
			}
		}
		return t, nil

	case vibespacesModeAgentView:
		switch msg.String() {
		case "esc", "backspace":
			t.mode = vibespacesModeList
			t.selectedVS = nil
			t.viewAgents = nil
			t.agentConfigs = nil
			t.forwards = nil
			t.agentCursor = 0
			return t, t.loadLogsForSelected()
		case "j", "down":
			if len(t.viewAgents) > 0 {
				t.agentCursor = min(t.agentCursor+1, len(t.viewAgents)-1)
			}
		case "k", "up":
			if len(t.viewAgents) > 0 {
				t.agentCursor = max(t.agentCursor-1, 0)
			}
		case "g":
			t.agentCursor = 0
		case "G":
			if len(t.viewAgents) > 0 {
				t.agentCursor = len(t.viewAgents) - 1
			}
		case "enter":
			if t.agentCursor < len(t.viewAgents) && t.selectedVS != nil {
				ag := t.viewAgents[t.agentCursor]
				if ag.AgentType == agent.TypeClaudeCode || ag.AgentType == agent.TypeCodex {
					t.mode = vibespacesModeSessionList
					t.sessionAgent = ag.AgentName
					t.sessionAgentType = ag.AgentType
					t.sessions = nil
					t.sessionCursor = 0
					return t, t.loadSessions(t.selectedVS.Name, ag.AgentName, ag.AgentType)
				}
			}
		case "a":
			if t.selectedVS != nil {
				t.mode = vibespacesModeAddAgent
				t.addAgentField = addAgentFieldType
				t.addAgentType = agent.TypeClaudeCode
				t.addAgentName = ""
				t.err = ""
				return t, nil
			}
		case "x":
			if t.agentCursor < len(t.viewAgents) && t.selectedVS != nil {
				ag := t.viewAgents[t.agentCursor]
				return t, t.prepareAgentConnect(t.selectedVS.Name, ag.AgentName, ag.AgentType)
			}
		case "b":
			if t.agentCursor < len(t.viewAgents) && t.selectedVS != nil {
				ag := t.viewAgents[t.agentCursor]
				return t, t.prepareBrowserConnect(t.selectedVS.Name, ag.AgentName)
			}
		}
		return t, nil

	case vibespacesModeCreateForm:
		return t.handleCreateFormKey(msg)

	case vibespacesModeDeleteConfirm:
		return t.handleDeleteConfirmKey(msg)

	case vibespacesModeAddAgent:
		return t.handleAddAgentKey(msg)

	default: // list mode
		prev := t.selected
		switch msg.String() {
		case "j", "down":
			if len(t.vibespaces) > 0 {
				t.selected = min(t.selected+1, len(t.vibespaces)-1)
			}
		case "k", "up":
			if len(t.vibespaces) > 0 {
				t.selected = max(t.selected-1, 0)
			}
		case "g":
			t.selected = 0
		case "G":
			if len(t.vibespaces) > 0 {
				t.selected = len(t.vibespaces) - 1
			}
		case "enter":
			if t.selected < len(t.vibespaces) {
				vs := t.vibespaces[t.selected]
				t.mode = vibespacesModeAgentView
				t.selectedVS = vs
				t.viewAgents = nil
				t.agentConfigs = nil
				t.forwards = nil
				t.agentCursor = 0
				return t, tea.Batch(
					t.loadAgentsForView(vs.ID, vs.Name),
					t.loadAgentConfigs(vs.ID, vs.Name),
					t.loadForwards(vs.ID, vs.Name),
				)
			}
		case "x":
			if t.selected < len(t.vibespaces) {
				vs := t.vibespaces[t.selected]
				return t, t.prepareShellConnectPrimary(vs.Name)
			}
		case "b":
			if t.selected < len(t.vibespaces) {
				vs := t.vibespaces[t.selected]
				return t, t.prepareBrowserConnectPrimary(vs.Name)
			}
		case "n":
			t.mode = vibespacesModeCreateForm
			t.createField = createFieldName
			t.createName = ""
			t.createAgentType = agent.TypeClaudeCode
			t.createCPU = "250m"
			t.createMemory = "512Mi"
			t.createStorage = "10Gi"
			t.err = ""
			return t, nil
		case "d":
			if t.selected < len(t.vibespaces) {
				t.mode = vibespacesModeDeleteConfirm
				t.deleteName = t.vibespaces[t.selected].Name
				t.deleteInput = ""
				t.err = ""
				return t, nil
			}
		case "S":
			if t.selected < len(t.vibespaces) {
				vs := t.vibespaces[t.selected]
				return t, t.toggleStartStop(vs.Name, vs.Status)
			}
		}
		if t.selected != prev {
			return t, t.loadLogsForSelected()
		}
		return t, nil
	}
}

func (t *VibespacesTab) View() string {
	if t.err != "" && len(t.vibespaces) == 0 && t.mode == vibespacesModeList {
		return lipgloss.NewStyle().
			Foreground(ui.ColorError).
			Padding(1, 2).
			Render(fmt.Sprintf("Error loading vibespaces: %s", t.err))
	}

	switch t.mode {
	case vibespacesModeAgentView, vibespacesModeAddAgent:
		return t.viewAgentView()
	case vibespacesModeSessionList:
		return t.viewSessionList()
	}

	if len(t.vibespaces) == 0 && t.mode == vibespacesModeList {
		return t.viewEmpty()
	}

	topBlock := t.viewTable()

	var bottom string
	switch t.mode {
	case vibespacesModeCreateForm:
		bottom = t.viewCreateForm()
	case vibespacesModeDeleteConfirm:
		bottom = t.viewDeleteConfirm()
	default:
		bottom = t.viewDetail()
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

func (t *VibespacesTab) viewEmpty() string {
	msg := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(2, 0).
		Render("No vibespaces found.")

	hint := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Render("Create one with: vibespace create <name>")

	block := lipgloss.JoinVertical(lipgloss.Center, msg, hint)
	return lipgloss.Place(t.width, t.height, lipgloss.Center, lipgloss.Center, block)
}

func (t *VibespacesTab) viewTable() string {
	headers, rows := t.buildTableData()

	sel := t.selected
	tbl := table.New().
		Headers(headers...).
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

	noun := "vibespaces"
	if len(t.vibespaces) == 1 {
		noun = "vibespace"
	}
	countText := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(fmt.Sprintf("(%d %s)", len(t.vibespaces), noun))
	count := lipgloss.NewStyle().Width(t.width - 4).Align(lipgloss.Right).
		PaddingTop(1).Render(countText)

	return lipgloss.NewStyle().Padding(1, 2).Render(tbl.Render() + "\n" + count)
}

func (t *VibespacesTab) buildTableData() ([]string, [][]string) {
	w := t.width
	showCPU := w >= 61
	showMem := w >= 61
	showStorage := w >= 81
	showAge := w >= 81

	// Build headers
	headers := []string{"Name", "Status", "Agents"}
	if showCPU {
		headers = append(headers, "CPU (Reserved)")
	}
	if showMem {
		headers = append(headers, "Mem (Reserved)")
	}
	if showStorage {
		headers = append(headers, "Storage")
	}
	if showAge {
		headers = append(headers, "Age")
	}

	// Build rows
	rows := make([][]string, len(t.vibespaces))
	for i, vs := range t.vibespaces {
		agentCount := "-"
		if c, ok := t.agentCounts[vs.ID]; ok {
			agentCount = fmt.Sprintf("%d", c)
		}

		if i == t.selected {
			cells := []string{"› " + vs.Name, vs.Status, agentCount}
			if showCPU {
				cells = append(cells, vs.Resources.CPU)
			}
			if showMem {
				cells = append(cells, vs.Resources.Memory)
			}
			if showStorage {
				cells = append(cells, vs.Resources.Storage)
			}
			if showAge {
				cells = append(cells, vsTimeAgo(vs.CreatedAt))
			}
			rows[i] = renderGradientRow(cells, brandGradient)
		} else {
			cells := []string{"  " + vs.Name, vsStatusStyled(vs.Status), agentCount}
			if showCPU {
				cells = append(cells, vs.Resources.CPU)
			}
			if showMem {
				cells = append(cells, vs.Resources.Memory)
			}
			if showStorage {
				cells = append(cells, vs.Resources.Storage)
			}
			if showAge {
				cells = append(cells, vsTimeAgo(vs.CreatedAt))
			}
			rows[i] = cells
		}
	}

	return headers, rows
}

func (t *VibespacesTab) viewDetail() string {
	if t.selected >= len(t.vibespaces) {
		return ""
	}

	vs := t.vibespaces[t.selected]
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	var meta []string

	// ID (8-char prefix)
	idStr := vs.ID
	if len(idStr) > 8 {
		idStr = idStr[:8]
	}
	meta = append(meta, fmt.Sprintf("%s  %s",
		labelStyle.Render("ID"),
		dimStyle.Render(idStr)))

	// Limits (burst capacity)
	var limParts []string
	if vs.Resources.CPULimit != "" {
		limParts = append(limParts, "CPU "+vs.Resources.CPULimit)
	}
	if vs.Resources.MemoryLimit != "" {
		limParts = append(limParts, "Mem "+vs.Resources.MemoryLimit)
	}
	if len(limParts) > 0 {
		meta = append(meta, fmt.Sprintf("%s  %s",
			labelStyle.Render("Limits"),
			dimStyle.Render(strings.Join(limParts, ", "))))
	}

	// Agents (names, comma-separated)
	if names, ok := t.agentNames[vs.ID]; ok && len(names) > 0 {
		meta = append(meta, fmt.Sprintf("%s  %s",
			labelStyle.Render("Agents"),
			dimStyle.Render(strings.Join(names, ", "))))
	}

	// Image
	if vs.Image != "" {
		meta = append(meta, fmt.Sprintf("%s  %s",
			labelStyle.Render("Image"),
			dimStyle.Render(vs.Image)))
	}

	// PVC
	if vs.Persistent {
		meta = append(meta, fmt.Sprintf("%s  %s",
			labelStyle.Render("PVC"),
			dimStyle.Render(vsPVCName(vs.ID))))
	}

	// Mounts
	if len(vs.Mounts) > 0 {
		var mountStrs []string
		for _, m := range vs.Mounts {
			s := m.HostPath + " → " + m.ContainerPath
			if m.ReadOnly {
				s += " (ro)"
			}
			mountStrs = append(mountStrs, s)
		}
		meta = append(meta, fmt.Sprintf("%s  %s",
			labelStyle.Render("Mounts"),
			dimStyle.Render(strings.Join(mountStrs, ", "))))
	}

	// Created
	if vs.CreatedAt != "" {
		meta = append(meta, fmt.Sprintf("%s  %s",
			labelStyle.Render("Created"),
			dimStyle.Render(vs.CreatedAt)))
	}

	metaHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
		Render("Details")
	detailBlock := metaHeader + "\n" + mutedLine + "\n" + strings.Join(meta, "\n") + "\n" + mutedLine

	// Recent Logs section
	var logsBlock string
	if t.logsID == vs.ID && len(t.logsLines) > 0 {
		logsHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
			Render("Recent Logs")
		var logLines []string
		for _, line := range t.logsLines {
			logLines = append(logLines, dimStyle.Render(truncate(line, t.width-8)))
		}
		logsBlock = "\n\n" + logsHeader + "\n" + mutedLine + "\n" + strings.Join(logLines, "\n")
	}

	fullBlock := detailBlock + logsBlock + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

// viewAgentView renders the full-screen agent detail view.
func (t *VibespacesTab) viewAgentView() string {
	if t.selectedVS == nil {
		return ""
	}

	vs := t.selectedVS
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	// --- Top block: header + agent table ---
	var topParts []string

	// Header: ← name                                          status
	backArrow := renderGradientText("← ", brandGradient)
	nameText := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(vs.Name)
	statusText := vsStatusStyled(vs.Status)

	headerLeft := backArrow + nameText
	headerRight := statusText
	headerGap := t.width - 4 - lipgloss.Width(headerLeft) - lipgloss.Width(headerRight)
	if headerGap < 1 {
		headerGap = 1
	}
	header := headerLeft + strings.Repeat(" ", headerGap) + headerRight
	topParts = append(topParts, header, mutedLine)

	// Agent table
	if len(t.viewAgents) > 0 {
		topParts = append(topParts, t.viewAgentTable())
	} else {
		topParts = append(topParts, dimStyle.Render("Loading agents..."))
	}

	topBlock := lipgloss.NewStyle().Padding(1, 2).Render(
		strings.Join(topParts, "\n\n"))

	// --- Bottom block: agent detail or add-agent form ---
	var bottom string
	if t.mode == vibespacesModeAddAgent {
		bottom = t.viewAddAgentForm()
	} else {
		bottom = t.viewAgentDetail()
	}

	topH := lipgloss.Height(topBlock)
	bottomH := lipgloss.Height(bottom)
	gap := t.height - topH - bottomH
	if gap < 1 {
		gap = 1
	}

	return topBlock + strings.Repeat("\n", gap) + bottom
}

// viewAgentDetail renders the bottom detail panel scoped to the selected agent.
func (t *VibespacesTab) viewAgentDetail() string {
	vs := t.selectedVS
	if vs == nil || t.agentCursor >= len(t.viewAgents) {
		return ""
	}

	ag := t.viewAgents[t.agentCursor]
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	// --- Details section ---
	var details []string

	// Resources (vibespace-level)
	var resParts []string
	if vs.Resources.CPU != "" {
		cpuStr := "CPU " + vs.Resources.CPU
		if vs.Resources.CPULimit != "" {
			cpuStr += " (limit " + vs.Resources.CPULimit + ")"
		}
		resParts = append(resParts, cpuStr)
	}
	if vs.Resources.Memory != "" {
		memStr := "Mem " + vs.Resources.Memory
		if vs.Resources.MemoryLimit != "" {
			memStr += " (limit " + vs.Resources.MemoryLimit + ")"
		}
		resParts = append(resParts, memStr)
	}
	if len(resParts) > 0 {
		details = append(details, fmt.Sprintf("%s  %s",
			labelStyle.Render("Resources"),
			dimStyle.Render(strings.Join(resParts, "  "))))
	}

	// Storage
	if vs.Resources.Storage != "" {
		storageStr := vs.Resources.Storage
		if vs.Persistent {
			storageStr += " (PVC)"
		}
		details = append(details, fmt.Sprintf("%s  %s",
			labelStyle.Render("Storage"),
			dimStyle.Render(storageStr)))
	}

	// Mounts
	if len(vs.Mounts) > 0 {
		var mountStrs []string
		for _, m := range vs.Mounts {
			s := m.HostPath + " → " + m.ContainerPath
			if m.ReadOnly {
				s += " (ro)"
			} else {
				s += " (rw)"
			}
			mountStrs = append(mountStrs, s)
		}
		details = append(details, fmt.Sprintf("%s  %s",
			labelStyle.Render("Mounts"),
			dimStyle.Render(strings.Join(mountStrs, ", "))))
	}

	// Image (per-agent based on type)
	if img := agentImage(ag.AgentType); img != "" {
		details = append(details, fmt.Sprintf("%s  %s",
			labelStyle.Render("Image"),
			dimStyle.Render(img)))
	}

	detailsHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
		Render("Details")
	detailsBlock := detailsHeader + "\n" + mutedLine + "\n" +
		strings.Join(details, "\n") + "\n" + mutedLine

	// --- Configuration section (per-agent) ---
	var cfgLines []string
	cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
		labelStyle.Render("type"),
		dimStyle.Render(string(ag.AgentType))))

	isCodex := ag.AgentType == agent.TypeCodex

	if cfg, ok := t.agentConfigs[ag.AgentName]; ok && cfg != nil {
		// skip_permissions: codex → "always", else true/false
		skipStr := fmt.Sprintf("%v", cfg.SkipPermissions)
		if isCodex {
			skipStr = "always"
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("skip_permissions"),
			dimStyle.Render(skipStr)))

		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("share_credentials"),
			dimStyle.Render(fmt.Sprintf("%v", cfg.ShareCredentials))))

		// allowed_tools: codex → "all", skip=true → "all", custom → join, else → defaults + (default)
		var allowedStr string
		if isCodex || cfg.SkipPermissions {
			allowedStr = "all"
		} else if len(cfg.AllowedTools) > 0 {
			allowedStr = strings.Join(cfg.AllowedTools, ", ")
		} else {
			allowedStr = strings.Join(agent.DefaultAllowedTools(), ", ") + " (default)"
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("allowed_tools"),
			dimStyle.Render(allowedStr)))

		disallowedStr := "-"
		if !isCodex && len(cfg.DisallowedTools) > 0 {
			disallowedStr = strings.Join(cfg.DisallowedTools, ", ")
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("disallowed_tools"),
			dimStyle.Render(disallowedStr)))

		modelStr := "default"
		if cfg.Model != "" {
			modelStr = cfg.Model
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("model"),
			dimStyle.Render(modelStr)))

		maxTurnsStr := "unlimited"
		if cfg.MaxTurns > 0 {
			maxTurnsStr = fmt.Sprintf("%d", cfg.MaxTurns)
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("max_turns"),
			dimStyle.Render(maxTurnsStr)))
	} else {
		cfgLines = append(cfgLines, dimStyle.Render("Loading config..."))
	}

	cfgHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
		Render("Configuration")
	cfgBlock := cfgHeader + "\n" + mutedLine + "\n" +
		strings.Join(cfgLines, "\n") + "\n" + mutedLine

	// --- Forwards section (per-agent) ---
	var fwdBlock string
	agentForwards := t.forwardsForAgent(ag.AgentName)
	if len(agentForwards) > 0 {
		fwdHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
			Render("Forwards")
		// Compute column widths for alignment
		var maxRemote, maxLocal, maxType int
		for _, fwd := range agentForwards {
			if r := len(fmt.Sprintf(":%d", fwd.RemotePort)); r > maxRemote {
				maxRemote = r
			}
			if l := len(fmt.Sprintf(":%d", fwd.LocalPort)); l > maxLocal {
				maxLocal = l
			}
			if len(fwd.Type) > maxType {
				maxType = len(fwd.Type)
			}
		}
		var fwdLines []string
		for _, fwd := range agentForwards {
			remote := fmt.Sprintf(":%d", fwd.RemotePort)
			local := fmt.Sprintf(":%d", fwd.LocalPort)
			line := fmt.Sprintf("%s  %s  %s  %s",
				dimStyle.Render(fmt.Sprintf("%-*s", maxRemote, remote)),
				dimStyle.Render(fmt.Sprintf("→ %-*s", maxLocal, local)),
				dimStyle.Render(fmt.Sprintf("%-*s", maxType, fwd.Type)),
				dimStyle.Render(fwd.Status))
			fwdLines = append(fwdLines, line)
		}
		fwdBlock = "\n\n" + fwdHeader + "\n" + mutedLine + "\n" +
			strings.Join(fwdLines, "\n") + "\n" + mutedLine
	}

	fullBlock := detailsBlock + "\n\n" + cfgBlock + fwdBlock
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

// viewAgentTable renders agents as a table with gradient-highlighted selected row.
func (t *VibespacesTab) viewAgentTable() string {
	rows := make([][]string, len(t.viewAgents))
	for i, ag := range t.viewAgents {
		name := "  " + ag.AgentName
		agentType := string(ag.AgentType)
		status := ag.Status

		modelStr := "default"
		if cfg, ok := t.agentConfigs[ag.AgentName]; ok && cfg != nil && cfg.Model != "" {
			modelStr = cfg.Model
		}

		if i == t.agentCursor {
			cells := []string{"› " + ag.AgentName, agentType, modelStr, status}
			rows[i] = renderGradientRow(cells, brandGradient)
		} else {
			rows[i] = []string{name, agentType, modelStr, vsStatusStyled(status)}
		}
	}

	sel := t.agentCursor
	tbl := table.New().
		Headers("Name", "Type", "Model", "Status").
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

	noun := "agents"
	if len(t.viewAgents) == 1 {
		noun = "agent"
	}
	countText := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(fmt.Sprintf("(%d %s)", len(t.viewAgents), noun))
	count := lipgloss.NewStyle().Width(t.width - 4).Align(lipgloss.Right).
		PaddingTop(1).Render(countText)

	return tbl.Render() + "\n" + count
}

// forwardsForAgent returns the forwards matching the given agent name.
func (t *VibespacesTab) forwardsForAgent(agentName string) []daemon.ForwardInfo {
	for _, as := range t.forwards {
		if as.Name == agentName {
			return as.Forwards
		}
	}
	return nil
}

// viewSessionList renders the session list for a claude-code agent.
func (t *VibespacesTab) viewSessionList() string {
	if t.selectedVS == nil {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	// Header: ← agent-name sessions
	backArrow := renderGradientText("← ", brandGradient)
	nameText := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).
		Render(t.sessionAgent + " sessions")
	header := backArrow + nameText

	var topParts []string
	topParts = append(topParts, header, mutedLine)

	if t.err != "" {
		topParts = append(topParts, lipgloss.NewStyle().Foreground(ui.ColorError).
			Render("Error: "+t.err))
	} else if t.sessions == nil {
		topParts = append(topParts, dimStyle.Render("Loading sessions..."))
	} else if len(t.sessions) == 0 {
		topParts = append(topParts, dimStyle.Render("No sessions found."))
	} else {
		topParts = append(topParts, t.viewSessionTable())
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(
		strings.Join(topParts, "\n\n"))
}

// viewSessionTable renders sessions as a table with gradient-highlighted selected row.
func (t *VibespacesTab) viewSessionTable() string {
	rows := make([][]string, len(t.sessions))
	for i, s := range t.sessions {
		idShort := s.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}

		title := s.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		title = strings.ReplaceAll(title, "\n", " ")

		ago := formatSessionAge(s.LastTime)
		prompts := fmt.Sprintf("%d", s.Prompts)

		if i == t.sessionCursor {
			cells := []string{"› " + idShort, ago, prompts, title}
			rows[i] = renderGradientRow(cells, brandGradient)
		} else {
			rows[i] = []string{"  " + idShort, ago, prompts, title}
		}
	}

	sel := t.sessionCursor
	tbl := table.New().
		Headers("ID", "Last Active", "Turns", "Title").
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

	return tbl.Render() + "\n" + count
}

// --- Commands ---

func (t *VibespacesTab) loadVibespaces() tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vibespacesLoadedMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		vs, err := svc.List(ctx)
		return vibespacesLoadedMsg{vibespaces: vs, err: err}
	}
}

func (t *VibespacesTab) loadAgentInfo() tea.Cmd {
	svc := t.shared.Vibespace
	vibespaces := t.vibespaces
	return func() tea.Msg {
		counts := make(map[string]int)
		names := make(map[string][]string)
		if svc == nil {
			return agentInfoLoadedMsg{counts: counts, names: names}
		}
		for _, vs := range vibespaces {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			agents, err := svc.ListAgents(ctx, vs.ID)
			cancel()
			if err != nil {
				counts[vs.ID] = 1
			} else {
				counts[vs.ID] = len(agents)
				names[vs.ID] = agentNames(agents)
			}
		}
		return agentInfoLoadedMsg{counts: counts, names: names}
	}
}

func (t *VibespacesTab) loadLogsForSelected() tea.Cmd {
	if t.selected >= len(t.vibespaces) {
		return nil
	}
	vs := t.vibespaces[t.selected]
	return t.loadLogsForVibespace(vs.ID, vs.Name)
}

func (t *VibespacesTab) loadLogsForVibespace(vsID, vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vsLogsLoadedMsg{vibespaceID: vsID, err: fmt.Errorf("unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logs, err := svc.GetLogs(ctx, vsName, 8)
		if err != nil {
			return vsLogsLoadedMsg{vibespaceID: vsID, err: err}
		}
		lines := strings.Split(strings.TrimRight(logs, "\n"), "\n")
		return vsLogsLoadedMsg{vibespaceID: vsID, lines: lines}
	}
}

func (t *VibespacesTab) loadAgentsForView(vsID, vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vsAgentsLoadedMsg{vibespaceID: vsID, err: fmt.Errorf("unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := svc.ListAgents(ctx, vsName)
		return vsAgentsLoadedMsg{vibespaceID: vsID, agents: agents, err: err}
	}
}

func (t *VibespacesTab) loadAgentConfigs(vsID, vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	agents := t.viewAgents // may be nil on first load; configs will re-load when agents arrive
	// If we don't have agents yet, use agent names from the table view
	var names []string
	if len(agents) > 0 {
		for _, a := range agents {
			names = append(names, a.AgentName)
		}
	} else if n, ok := t.agentNames[vsID]; ok {
		names = n
	}
	return func() tea.Msg {
		configs := make(map[string]*agent.Config)
		if svc == nil || len(names) == 0 {
			return vsAgentConfigsLoadedMsg{vibespaceID: vsID, configs: configs}
		}
		for _, name := range names {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cfg, err := svc.GetAgentConfig(ctx, vsName, name)
			cancel()
			if err == nil && cfg != nil {
				configs[name] = cfg
			}
		}
		return vsAgentConfigsLoadedMsg{vibespaceID: vsID, configs: configs}
	}
}

func (t *VibespacesTab) loadForwards(vsID, vsName string) tea.Cmd {
	dc := t.shared.Daemon
	return func() tea.Msg {
		if dc == nil {
			return vsForwardsLoadedMsg{vibespaceID: vsID}
		}
		resp, err := dc.ListForwardsForVibespace(vsName)
		if err != nil || resp == nil {
			return vsForwardsLoadedMsg{vibespaceID: vsID}
		}
		return vsForwardsLoadedMsg{vibespaceID: vsID, agents: resp.Agents}
	}
}

func (t *VibespacesTab) loadSessions(vsName, agentName string, agentType agent.Type) tea.Cmd {
	return func() tea.Msg {
		sshPort, err := t.ensureSSHForward(vsName, agentName)
		if err != nil {
			return vsSessionsLoadedMsg{agentName: agentName, err: fmt.Errorf("SSH forward: %w", err)}
		}

		keyPath := vibespace.GetSSHPrivateKeyPath()
		if keyPath == "" {
			return vsSessionsLoadedMsg{agentName: agentName, err: fmt.Errorf("no SSH key found")}
		}

		// Build remote command based on agent type
		var remoteCmd string
		switch agentType {
		case agent.TypeCodex:
			remoteCmd = "cat ~/.codex/history.jsonl 2>/dev/null || true"
		default:
			remoteCmd = "cat ~/.claude/history.jsonl 2>/dev/null || true"
		}

		cmd := exec.Command("ssh",
			"-i", keyPath,
			"-p", strconv.Itoa(sshPort),
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
			"-o", "ConnectTimeout=5",
			"user@localhost",
			remoteCmd,
		)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			detail := stderr.String()
			if detail == "" {
				detail = err.Error()
			}
			return vsSessionsLoadedMsg{agentName: agentName, err: fmt.Errorf("read sessions: %s", strings.TrimSpace(detail))}
		}

		var sessions []vsSessionInfo
		switch agentType {
		case agent.TypeCodex:
			sessions = parseCodexHistory(output)
		default:
			sessions = parseHistoryJSONL(output, "/vibespace")
		}
		return vsSessionsLoadedMsg{agentName: agentName, sessions: sessions}
	}
}

// prepareSessionResume ensures the SSH forward is ready, then sends vsConnectReadyMsg.
func (t *VibespacesTab) prepareSessionResume(vsName, agentName string, agentType agent.Type, sessionID string) tea.Cmd {
	return func() tea.Msg {
		sshPort, err := t.ensureSSHForward(vsName, agentName)
		if err != nil {
			return vsConnectReadyMsg{agentName: agentName, agentType: agentType, sessionID: sessionID, mode: vsConnectModeSessionResume, err: err}
		}
		return vsConnectReadyMsg{sshPort: sshPort, agentName: agentName, agentType: agentType, sessionID: sessionID, mode: vsConnectModeSessionResume}
	}
}

// execSessionResume builds the SSH command and returns tea.ExecProcess to suspend the TUI.
func (t *VibespacesTab) execSessionResume(sshPort int, agentName string, agentType agent.Type, sessionID string) tea.Cmd {
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return func() tea.Msg {
			return vsSessionResumeMsg{err: fmt.Errorf("no SSH key found")}
		}
	}

	var cfg *agent.Config
	if c, ok := t.agentConfigs[agentName]; ok {
		cfg = c
	}

	agentImpl := agent.MustGet(agentType)
	agentCmd := agentImpl.BuildInteractiveCommand(sessionID, cfg)
	remoteCmd := fmt.Sprintf("bash -l -c 'cd /vibespace && %s'", agentCmd)

	slog.Debug("resuming session", "agent", agentName, "session", sessionID, "type", agentType)

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t",
		"user@localhost",
		remoteCmd,
	)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return vsSessionResumeMsg{err: err}
	})
}

// ensureSSHForward ensures daemon is running and an SSH forward exists for the agent.
// Returns the local port for SSH access.
func (t *VibespacesTab) ensureSSHForward(vsName, agentName string) (int, error) {
	if !daemon.IsDaemonRunning() {
		if err := daemon.SpawnDaemon(); err != nil {
			return 0, fmt.Errorf("start daemon: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	client, err := daemon.NewClient()
	if err != nil {
		return 0, fmt.Errorf("connect to daemon: %w", err)
	}

	// Try to find existing SSH forward
	if port, ok := findSSHForward(client, vsName, agentName); ok {
		return port, nil
	}

	// Refresh daemon state and retry
	_ = client.Refresh()
	time.Sleep(2 * time.Second)

	if port, ok := findSSHForward(client, vsName, agentName); ok {
		return port, nil
	}

	return 0, fmt.Errorf("no active SSH forward for %s/%s", vsName, agentName)
}

// findSSHForward queries the daemon for an active SSH forward.
func findSSHForward(client *daemon.Client, vsName, agentName string) (int, bool) {
	result, err := client.ListForwardsForVibespace(vsName)
	if err != nil || result == nil {
		return 0, false
	}
	for _, ag := range result.Agents {
		if ag.Name == agentName {
			for _, fwd := range ag.Forwards {
				if fwd.Type == "ssh" && fwd.Status == "active" {
					return fwd.LocalPort, true
				}
			}
		}
	}
	return 0, false
}

// prepareShellConnectPrimary finds the primary agent and ensures SSH forward for a raw shell.
func (t *VibespacesTab) prepareShellConnectPrimary(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := svc.ListAgents(ctx, vsName)
		if err != nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: fmt.Errorf("list agents: %w", err)}
		}
		primary := primaryAgent(agents)
		if primary == nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: fmt.Errorf("no agents in %s", vsName)}
		}
		sshPort, err := t.ensureSSHForward(vsName, primary.AgentName)
		if err != nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: err}
		}
		return vsConnectReadyMsg{sshPort: sshPort, mode: vsConnectModeShell}
	}
}

// prepareAgentConnect ensures SSH forward for the agent's interactive CLI.
func (t *VibespacesTab) prepareAgentConnect(vsName, agentName string, agentType agent.Type) tea.Cmd {
	return func() tea.Msg {
		sshPort, err := t.ensureSSHForward(vsName, agentName)
		if err != nil {
			return vsConnectReadyMsg{agentName: agentName, agentType: agentType, mode: vsConnectModeAgentCLI, err: err}
		}
		return vsConnectReadyMsg{sshPort: sshPort, agentName: agentName, agentType: agentType, mode: vsConnectModeAgentCLI}
	}
}

// prepareBrowserConnect ensures ttyd forward for the selected agent.
func (t *VibespacesTab) prepareBrowserConnect(vsName, agentName string) tea.Cmd {
	return func() tea.Msg {
		port, err := t.ensureTtydForward(vsName, agentName)
		if err != nil {
			return vsBrowserReadyMsg{err: err}
		}
		return vsBrowserReadyMsg{ttydPort: port}
	}
}

// prepareBrowserConnectPrimary finds the primary agent and ensures ttyd forward.
func (t *VibespacesTab) prepareBrowserConnectPrimary(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vsBrowserReadyMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := svc.ListAgents(ctx, vsName)
		if err != nil {
			return vsBrowserReadyMsg{err: fmt.Errorf("list agents: %w", err)}
		}
		primary := primaryAgent(agents)
		if primary == nil {
			return vsBrowserReadyMsg{err: fmt.Errorf("no agents in %s", vsName)}
		}
		port, err := t.ensureTtydForward(vsName, primary.AgentName)
		if err != nil {
			return vsBrowserReadyMsg{err: err}
		}
		return vsBrowserReadyMsg{ttydPort: port}
	}
}

// execShellConnect builds a plain SSH command (no remote command) and returns tea.ExecProcess.
func (t *VibespacesTab) execShellConnect(sshPort int) tea.Cmd {
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return func() tea.Msg {
			return vsExecReturnMsg{err: fmt.Errorf("no SSH key found")}
		}
	}

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t",
		"user@localhost",
	)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return vsExecReturnMsg{err: err}
	})
}

// execAgentConnect builds an SSH command with the agent's interactive CLI and returns tea.ExecProcess.
func (t *VibespacesTab) execAgentConnect(sshPort int, agentName string, agentType agent.Type) tea.Cmd {
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return func() tea.Msg {
			return vsExecReturnMsg{err: fmt.Errorf("no SSH key found")}
		}
	}

	var cfg *agent.Config
	if c, ok := t.agentConfigs[agentName]; ok {
		cfg = c
	}

	agentImpl := agent.MustGet(agentType)
	agentCmd := agentImpl.BuildInteractiveCommand("", cfg)
	remoteCmd := fmt.Sprintf("bash -l -c 'cd /vibespace && %s'", agentCmd)

	slog.Debug("agent connect", "agent", agentName, "type", agentType)

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t",
		"user@localhost",
		remoteCmd,
	)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return vsExecReturnMsg{err: err}
	})
}

// ensureTtydForward ensures daemon is running and a ttyd forward exists for the agent.
func (t *VibespacesTab) ensureTtydForward(vsName, agentName string) (int, error) {
	if !daemon.IsDaemonRunning() {
		if err := daemon.SpawnDaemon(); err != nil {
			return 0, fmt.Errorf("start daemon: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	client, err := daemon.NewClient()
	if err != nil {
		return 0, fmt.Errorf("connect to daemon: %w", err)
	}

	if port, ok := findTtydForward(client, vsName, agentName); ok {
		return port, nil
	}

	_ = client.Refresh()
	time.Sleep(2 * time.Second)

	if port, ok := findTtydForward(client, vsName, agentName); ok {
		return port, nil
	}

	return 0, fmt.Errorf("no active ttyd forward for %s/%s", vsName, agentName)
}

// findTtydForward queries the daemon for an active ttyd forward.
func findTtydForward(client *daemon.Client, vsName, agentName string) (int, bool) {
	result, err := client.ListForwardsForVibespace(vsName)
	if err != nil || result == nil {
		return 0, false
	}
	for _, ag := range result.Agents {
		if ag.Name == agentName {
			for _, fwd := range ag.Forwards {
				if fwd.Type == "ttyd" && fwd.Status == "active" {
					return fwd.LocalPort, true
				}
			}
		}
	}
	return 0, false
}

// primaryAgent returns the primary agent (AgentNum == 1) or the first agent.
func primaryAgent(agents []vibespace.AgentInfo) *vibespace.AgentInfo {
	for i := range agents {
		if agents[i].AgentNum == 1 {
			return &agents[i]
		}
	}
	if len(agents) > 0 {
		return &agents[0]
	}
	return nil
}

// openBrowserURL opens the URL in the default system browser.
func openBrowserURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Run()
}

// historyEntry is the JSON structure of each line in ~/.claude/history.jsonl.
type historyEntry struct {
	Display   string `json:"display"`
	Timestamp int64  `json:"timestamp"`
	Project   string `json:"project"`
	SessionID string `json:"sessionId"`
}

// parseHistoryJSONL parses Claude Code's history.jsonl and returns session summaries.
func parseHistoryJSONL(data []byte, project string) []vsSessionInfo {
	sessions := map[string]*vsSessionInfo{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		var entry historyEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Project != project || entry.SessionID == "" {
			continue
		}

		s, ok := sessions[entry.SessionID]
		if !ok {
			// Clean up display text for title
			title := strings.TrimSpace(entry.Display)
			title = strings.ReplaceAll(title, "\n", " ")
			s = &vsSessionInfo{
				ID:    entry.SessionID,
				Title: title,
			}
			sessions[entry.SessionID] = s
		}

		s.Prompts++
		ts := time.UnixMilli(entry.Timestamp)
		if ts.After(s.LastTime) {
			s.LastTime = ts
		}
	}

	// Sort by last activity, most recent first
	result := make([]vsSessionInfo, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTime.After(result[j].LastTime)
	})

	return result
}

// codexHistoryEntry is the JSON structure of each line in ~/.codex/history.jsonl.
type codexHistoryEntry struct {
	SessionID string `json:"session_id"`
	Timestamp int64  `json:"ts"`
	Text      string `json:"text"`
}

// parseCodexHistory parses Codex's history.jsonl and returns session summaries.
func parseCodexHistory(data []byte) []vsSessionInfo {
	sessions := map[string]*vsSessionInfo{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		var entry codexHistoryEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.SessionID == "" {
			continue
		}

		s, ok := sessions[entry.SessionID]
		if !ok {
			title := strings.TrimSpace(entry.Text)
			title = strings.ReplaceAll(title, "\n", " ")
			s = &vsSessionInfo{
				ID:    entry.SessionID,
				Title: title,
			}
			sessions[entry.SessionID] = s
		}

		s.Prompts++
		ts := time.Unix(entry.Timestamp, 0)
		if ts.After(s.LastTime) {
			s.LastTime = ts
		}
	}

	result := make([]vsSessionInfo, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTime.After(result[j].LastTime)
	})

	return result
}

func formatSessionAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	ago := time.Since(t)
	switch {
	case ago.Hours() >= 24*7:
		return fmt.Sprintf("%.0fw ago", ago.Hours()/(24*7))
	case ago.Hours() >= 24:
		return fmt.Sprintf("%.0fd ago", ago.Hours()/24)
	case ago.Hours() >= 1:
		return fmt.Sprintf("%.0fh ago", ago.Hours())
	default:
		return fmt.Sprintf("%.0fm ago", ago.Minutes())
	}
}

// --- Form key handlers ---

func (t *VibespacesTab) handleCreateFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeList
		return t, nil

	case k == "ctrl+s":
		if t.createName == "" {
			return t, nil
		}
		return t, t.submitCreateForm()

	case k == "tab":
		t.createField++
		if t.createField >= createFieldCount {
			if t.createName == "" {
				t.createField = createFieldName
				return t, nil
			}
			return t, t.submitCreateForm()
		}
		return t, nil

	case k == "enter":
		if t.createField == createFieldName && t.createName == "" {
			return t, nil
		}
		t.createField++
		if t.createField >= createFieldCount {
			return t, t.submitCreateForm()
		}
		return t, nil

	case k == "backspace":
		switch t.createField {
		case createFieldName:
			if len(t.createName) > 0 {
				t.createName = t.createName[:len(t.createName)-1]
			}
		case createFieldCPU:
			if len(t.createCPU) > 0 {
				t.createCPU = t.createCPU[:len(t.createCPU)-1]
			}
		case createFieldMemory:
			if len(t.createMemory) > 0 {
				t.createMemory = t.createMemory[:len(t.createMemory)-1]
			}
		case createFieldStorage:
			if len(t.createStorage) > 0 {
				t.createStorage = t.createStorage[:len(t.createStorage)-1]
			}
		}
		return t, nil

	default:
		if t.createField == createFieldAgentType {
			if k == "j" || k == "k" {
				if t.createAgentType == agent.TypeClaudeCode {
					t.createAgentType = agent.TypeCodex
				} else {
					t.createAgentType = agent.TypeClaudeCode
				}
			}
			return t, nil
		}

		if len(k) == 1 {
			switch t.createField {
			case createFieldName:
				t.createName += k
			case createFieldCPU:
				t.createCPU += k
			case createFieldMemory:
				t.createMemory += k
			case createFieldStorage:
				t.createStorage += k
			}
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeList
		return t, nil

	case k == "enter":
		if t.deleteInput == t.deleteName {
			return t, t.submitDelete()
		}
		return t, nil

	case k == "backspace":
		if len(t.deleteInput) > 0 {
			t.deleteInput = t.deleteInput[:len(t.deleteInput)-1]
		}
		return t, nil

	default:
		if len(k) == 1 {
			t.deleteInput += k
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleAddAgentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	switch {
	case k == "esc":
		t.mode = vibespacesModeAgentView
		return t, nil

	case k == "ctrl+s":
		return t, t.submitAddAgent()

	case k == "tab", k == "enter":
		t.addAgentField++
		if t.addAgentField >= addAgentFieldCount {
			return t, t.submitAddAgent()
		}
		return t, nil

	case k == "backspace":
		if t.addAgentField == addAgentFieldName && len(t.addAgentName) > 0 {
			t.addAgentName = t.addAgentName[:len(t.addAgentName)-1]
		}
		return t, nil

	default:
		if t.addAgentField == addAgentFieldType {
			if k == "j" || k == "k" {
				if t.addAgentType == agent.TypeClaudeCode {
					t.addAgentType = agent.TypeCodex
				} else {
					t.addAgentType = agent.TypeClaudeCode
				}
			}
			return t, nil
		}

		if len(k) == 1 && t.addAgentField == addAgentFieldName {
			t.addAgentName += k
		}
		return t, nil
	}
}

// --- Form views ---

func (t *VibespacesTab) viewCreateForm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
		Render("Create vibespace")

	type formField struct {
		label    string
		field    createFormField
		value    string
		isSelect bool
	}

	fields := []formField{
		{"Name", createFieldName, t.createName, false},
		{"Agent type", createFieldAgentType, string(t.createAgentType), true},
		{"CPU", createFieldCPU, t.createCPU, false},
		{"Memory", createFieldMemory, t.createMemory, false},
		{"Storage", createFieldStorage, t.createStorage, false},
	}

	var lines []string
	for _, f := range fields {
		label := fmt.Sprintf("%-12s", f.label)
		isActive := f.field == t.createField

		var val string
		if isActive {
			if f.isSelect {
				val = activeStyle.Render("["+f.value+"]") + "  " + dimStyle.Render("j/k to change")
			} else {
				val = activeStyle.Render(f.value+"█")
			}
		} else {
			val = dimStyle.Render(f.value)
		}

		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(label), val))
	}

	if t.err != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err))
	}

	fullBlock := header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewDeleteConfirm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorError).
		Render(fmt.Sprintf("Delete \"%s\"?", t.deleteName))

	prompt := fmt.Sprintf("  Type %s to confirm: %s",
		dimStyle.Render(t.deleteName),
		activeStyle.Render(t.deleteInput+"█"))

	var errLine string
	if t.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err)
	}

	fullBlock := header + "\n" + mutedLine + "\n" + prompt + errLine + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

func (t *VibespacesTab) viewAddAgentForm() string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	activeStyle := lipgloss.NewStyle().Foreground(ui.ColorText)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	header := lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
		Render("Add agent")

	var lines []string

	// Agent type field
	typeLabel := fmt.Sprintf("%-12s", "Agent type")
	if t.addAgentField == addAgentFieldType {
		val := activeStyle.Render("["+string(t.addAgentType)+"]") + "  " + dimStyle.Render("j/k to change")
		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(typeLabel), val))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(typeLabel), dimStyle.Render(string(t.addAgentType))))
	}

	// Name field
	nameLabel := fmt.Sprintf("%-12s", "Name")
	if t.addAgentField == addAgentFieldName {
		val := activeStyle.Render(t.addAgentName + "█")
		if t.addAgentName == "" {
			val += "  " + dimStyle.Render("optional, auto-generated if empty")
		}
		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(nameLabel), val))
	} else {
		nameVal := t.addAgentName
		if nameVal == "" {
			nameVal = "(auto)"
		}
		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render(nameLabel), dimStyle.Render(nameVal)))
	}

	if t.err != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(ui.ColorError).Render("  "+t.err))
	}

	fullBlock := header + "\n" + mutedLine + "\n" + strings.Join(lines, "\n") + "\n" + mutedLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

// --- Form submit commands ---

func (t *VibespacesTab) submitCreateForm() tea.Cmd {
	svc := t.shared.Vibespace
	name := t.createName
	agentType := t.createAgentType
	cpu := t.createCPU
	memory := t.createMemory
	storage := t.createStorage

	return func() tea.Msg {
		if svc == nil {
			return vsCreateDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &model.CreateVibespaceRequest{
			Name:       name,
			Persistent: true,
			AgentType:  agentType,
			Resources: &model.Resources{
				CPU:         cpu,
				CPULimit:    "1000m",
				Memory:      memory,
				MemoryLimit: "1Gi",
				Storage:     storage,
			},
		}

		_, err := svc.Create(ctx, req)
		return vsCreateDoneMsg{err: err}
	}
}

func (t *VibespacesTab) submitDelete() tea.Cmd {
	svc := t.shared.Vibespace
	name := t.deleteName

	return func() tea.Msg {
		if svc == nil {
			return vsDeleteDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := svc.Delete(ctx, name, &vibespace.DeleteOptions{})
		return vsDeleteDoneMsg{err: err}
	}
}

func (t *VibespacesTab) toggleStartStop(name, status string) tea.Cmd {
	svc := t.shared.Vibespace

	return func() tea.Msg {
		if svc == nil {
			return vsStartStopDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if status == "running" {
			return vsStartStopDoneMsg{action: "stop", err: svc.Stop(ctx, name)}
		}
		return vsStartStopDoneMsg{action: "start", err: svc.Start(ctx, name)}
	}
}

func (t *VibespacesTab) submitAddAgent() tea.Cmd {
	svc := t.shared.Vibespace
	if t.selectedVS == nil {
		return nil
	}
	vsName := t.selectedVS.Name
	agentType := t.addAgentType
	agentName := t.addAgentName

	return func() tea.Msg {
		if svc == nil {
			return vsAddAgentDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		opts := &vibespace.SpawnAgentOptions{
			Name:      agentName,
			AgentType: agentType,
		}

		_, err := svc.SpawnAgent(ctx, vsName, opts)
		return vsAddAgentDoneMsg{err: err}
	}
}

// scheduleRefreshIfNeeded returns a tick command if any vibespace is in a transitional state.
func (t *VibespacesTab) scheduleRefreshIfNeeded() tea.Cmd {
	for _, vs := range t.vibespaces {
		if vs.Status == "creating" {
			return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return vsRefreshTickMsg{}
			})
		}
	}
	return nil
}

// --- Helpers ---

func (t *VibespacesTab) selectedID() string {
	if t.selected < len(t.vibespaces) {
		return t.vibespaces[t.selected].ID
	}
	return ""
}

func (t *VibespacesTab) clampSelected() {
	if t.selected >= len(t.vibespaces) {
		t.selected = max(len(t.vibespaces)-1, 0)
	}
}

func (t *VibespacesTab) clampAgentCursor() {
	if t.agentCursor >= len(t.viewAgents) {
		t.agentCursor = max(len(t.viewAgents)-1, 0)
	}
}

func agentImage(agentType agent.Type) string {
	switch agentType {
	case agent.TypeClaudeCode:
		return "ghcr.io/vibespacehq/vibespace/claude-code:latest"
	case agent.TypeCodex:
		return "ghcr.io/vibespacehq/vibespace/codex:latest"
	default:
		return ""
	}
}

func agentNames(agents []vibespace.AgentInfo) []string {
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.AgentName
	}
	return names
}

func vsStatusStyled(status string) string {
	switch status {
	case "running":
		return lipgloss.NewStyle().Foreground(ui.Teal).Render(status)
	case "stopped":
		return lipgloss.NewStyle().Foreground(ui.ColorDim).Render(status)
	case "error":
		return lipgloss.NewStyle().Foreground(ui.ColorError).Render(status)
	case "creating":
		return lipgloss.NewStyle().Foreground(ui.Yellow).Render(status)
	default:
		return lipgloss.NewStyle().Foreground(ui.ColorDim).Render(status)
	}
}

// vsPVCName returns the PVC name with the redundant "vibespace-" prefix stripped.
func vsPVCName(id string) string {
	short := id
	if len(short) > 8 {
		short = short[:8]
	}
	return short + "-pvc"
}

func vsTimeAgo(createdAt string) string {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return createdAt
	}
	return timeAgo(t)
}

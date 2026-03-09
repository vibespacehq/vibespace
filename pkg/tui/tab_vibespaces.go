package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/config"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/github"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/ui"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// vibespacesMode represents the current UI mode of the Vibespaces tab.
type vibespacesMode int

const (
	vibespacesModeList               vibespacesMode = iota // table view
	vibespacesModeAgentView                                // full-screen agent detail
	vibespacesModeSessionList                              // session list for an agent
	vibespacesModeCreateForm                               // inline create form
	vibespacesModeDeleteConfirm                            // inline delete confirmation
	vibespacesModeAddAgent                                 // inline add agent form (in agent view)
	vibespacesModeDeleteAgentConfirm                       // inline delete agent confirmation (in agent view)
	vibespacesModeEditConfig                               // inline edit config form (in agent view)
	vibespacesModeForwardManager                           // inline forward manager (in agent view)
	vibespacesModeGithubAuth                               // waiting for GitHub device flow authorization
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
	vsName    string // vibespace name (used by SessionsTab tree)
	agentName string
	sessions  []vsSessionInfo
	err       error
}

// vsConnectReadyMsg signals that SSH forward is ready for a connect action.
type vsConnectReadyMsg struct {
	sshPort     int
	agentName   string
	agentType   agent.Type
	sessionID   string
	mode        vsConnectMode
	agentConfig *agent.Config // carried for SessionsTab (no t.agentConfigs access)
	err         error
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

// vsGithubDeviceCodeMsg carries the device code response from GitHub.
type vsGithubDeviceCodeMsg struct {
	resp *github.DeviceCodeResponse
	err  error
}

// vsGithubTokenMsg carries the token response from GitHub polling.
type vsGithubTokenMsg struct {
	token *github.TokenResponse
	err   error
}

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

// vsEditConfigDoneMsg signals completion of an agent config update.
type vsEditConfigDoneMsg struct {
	err error
}

// vsAddForwardDoneMsg signals completion of adding a forward.
type vsAddForwardDoneMsg struct {
	err     error
	dnsName string // non-empty if DNS hosts entry still needed
}

// vsRemoveForwardDoneMsg signals completion of removing a forward.
type vsRemoveForwardDoneMsg struct {
	err     error
	dnsName string // non-empty if DNS hosts entry removal still needed
}

// vsToggleDNSDoneMsg signals completion of toggling DNS on a forward.
type vsToggleDNSDoneMsg struct {
	err     error
	dnsName string // non-empty = DNS was added (needs AddHostEntry); empty = DNS was removed
	oldDNS  string // non-empty = DNS was removed (needs RemoveHostEntry)
}

// vsDeleteAgentDoneMsg signals completion of an agent deletion (kill).
type vsDeleteAgentDoneMsg struct {
	err error
}

// vsAgentStartStopDoneMsg signals completion of an agent start/stop operation.
type vsAgentStartStopDoneMsg struct {
	action string
	err    error
}

// vsAgentStatusClearMsg clears the transient agent status message after a delay.
type vsAgentStatusClearMsg struct{}

// vsAgentRefreshTickMsg triggers a periodic agent list reload after start/stop.
type vsAgentRefreshTickMsg struct{}

// vsSudoDoneMsg signals completion of sudo password validation.
type vsSudoDoneMsg struct {
	ok       bool
	password string
}

// createFormField identifies which field is active in the create form.
type createFormField int

const (
	createFieldName      createFormField = iota
	createFieldAgentType                 // selector (j/k)
	createFieldRepo                      // text input, optional
	createFieldWorktree                  // toggle (j/k), only shown when repo is set
	createFieldBranch                    // text input, only shown when worktree is enabled
	createFieldCPU
	createFieldMemory
	createFieldStorage
	createFieldCount // sentinel
)

// addAgentFormField identifies which field is active in the add agent form.
type addAgentFormField int

const (
	addAgentFieldType            addAgentFormField = iota // selector (j/k)
	addAgentFieldName                                     // text input, optional
	addAgentFieldModel                                    // text input, optional
	addAgentFieldMaxTurns                                 // text input, optional
	addAgentFieldShareCreds                               // toggle (j/k)
	addAgentFieldSkipPerms                                // toggle (j/k)
	addAgentFieldBranch                                   // text input, only shown in worktree mode
	addAgentFieldAllowedTools                             // multi-select (j/k navigate, space toggle)
	addAgentFieldDisallowedTools                          // multi-select (j/k navigate, space toggle)
	addAgentFieldCount                                    // sentinel
)

// editConfigFormField identifies which field is active in the edit config form.
type editConfigFormField int

const (
	editConfigFieldModel           editConfigFormField = iota // text input
	editConfigFieldMaxTurns                                   // text input
	editConfigFieldSkipPerms                                  // toggle (j/k)
	editConfigFieldAllowedTools                               // multi-select
	editConfigFieldDisallowedTools                            // multi-select
	editConfigFieldCount                                      // sentinel
)

// fwdManagerAddField identifies which field is active in the add forward sub-form.
type fwdManagerAddField int

const (
	fwdManagerAddFieldRemote  fwdManagerAddField = iota // text input (remote port)
	fwdManagerAddFieldLocal                             // text input (local port, 0 = auto)
	fwdManagerAddFieldDNS                               // bool toggle (enable DNS)
	fwdManagerAddFieldDNSName                           // text input (custom DNS name, optional)
	fwdManagerAddFieldCount                             // sentinel
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
	selectedVS   *model.Vibespace         // vibespace being viewed
	viewAgents   []vibespace.AgentInfo    // agents for the selected vibespace
	agentConfigs map[string]*agent.Config // agent name → config
	forwards     []daemon.AgentStatus     // forward info from daemon
	agentCursor  int                      // cursor position within agents list

	// Session list state
	sessions         []vsSessionInfo // sessions for the selected agent
	sessionCursor    int             // cursor in session list
	sessionAgent     string          // agent name whose sessions are shown
	sessionAgentType agent.Type      // agent type whose sessions are shown

	// Create form state
	createField     createFormField
	createName      string
	createAgentType agent.Type
	createRepo      string
	createWorktree  bool
	createBranch    string
	createCPU       string
	createMemory    string
	createStorage   string

	// GitHub auth state (device flow)
	githubUserCode     string
	githubVerifyURI    string
	githubDevCode      string
	githubInterval     int
	githubAccessToken  string
	githubRefreshToken string

	// Delete confirm state
	deleteName  string
	deleteInput string

	// Delete agent confirm state (agent view)
	deleteAgentName  string
	deleteAgentInput string

	// Agent status feedback (transient message shown in agent view)
	agentStatusMsg      string
	agentRefreshPending int // remaining refresh ticks after start/stop

	// Add agent form state (agent view)
	addAgentField         addAgentFormField
	addAgentType          agent.Type
	addAgentName          string
	addAgentBranch        string
	addAgentModel         string
	addAgentMaxTurns      string
	addAgentShareCreds    bool
	addAgentSkipPerms     bool
	addAgentToolsList     []string        // available tools for current agent type
	addAgentAllowedSet    map[string]bool // selected allowed tools
	addAgentDisallowedSet map[string]bool // selected disallowed tools
	addAgentToolsCursor   int             // cursor within tools list for multi-select

	// Edit config form state (agent view)
	editConfigField         editConfigFormField
	editConfigAgentName     string
	editConfigModel         string
	editConfigMaxTurns      string
	editConfigSkipPerms     bool
	editConfigToolsList     []string        // available tools for current agent type
	editConfigAllowedSet    map[string]bool // selected allowed tools
	editConfigDisallowedSet map[string]bool // selected disallowed tools
	editConfigToolsCursor   int             // cursor within tools list for multi-select

	// Forward manager state (agent view)
	fwdManagerCursor     int
	fwdManagerAdding     bool               // true when add forward sub-form is active
	fwdManagerAddRemote  string             // remote port input
	fwdManagerAddLocal   string             // local port input
	fwdManagerAddDNS     bool               // enable DNS resolution
	fwdManagerAddDNSName string             // custom DNS name (optional)
	fwdManagerAddField   fwdManagerAddField // which add-form field is active

	// Sudo state for DNS hosts entry management
	sudoPassword     string // cached sudo password for sudo -S
	sudoPromptActive bool   // true when showing password prompt
	sudoInput        string // password input buffer
	sudoPendingDNS   string // DNS name waiting for sudo to complete
	sudoPendingOp    string // "add" or "remove"

	// Welcome screen state
	welcomeClusterStatus clusterStatus
	welcomeClusterMode   string
	welcomeLoaded        bool
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
	case vibespacesModeGithubAuth:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	case vibespacesModeDeleteConfirm, vibespacesModeDeleteAgentConfirm:
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
	case vibespacesModeEditConfig:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "skip")),
			key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
		}
	case vibespacesModeForwardManager:
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "remove")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "dns")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
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
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
			key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit config")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "forwards")),
			key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "start/stop")),
			key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "connect")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "browser")),
		}
	default:
		if len(t.vibespaces) == 0 {
			return []key.Binding{
				key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "init")),
				key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "create")),
				key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
				key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
				key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "palette")),
			}
		}
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
	return tea.Batch(t.loadVibespaces(), detectClusterStatus())
}

func (t *VibespacesTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TabActivateMsg:
		return t.handleTabActivate()
	case PaletteNewVibespaceMsg:
		return t.handlePaletteNewVibespace()
	case vibespacesLoadedMsg:
		return t.handleVibespacesLoaded(msg)
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
		return t.handleSessionsLoaded(msg)
	case vsConnectReadyMsg:
		return t.handleConnectReady(msg)
	case vsSessionResumeMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		if t.selectedVS != nil && t.sessionAgent != "" {
			return t, t.loadSessions(t.selectedVS.Name, t.sessionAgent, t.sessionAgentType)
		}
		return t, nil
	case vsBrowserReadyMsg:
		return t.handleBrowserReady(msg)
	case vsExecReturnMsg:
		return t.handleExecReturn(msg)
	case vsGithubDeviceCodeMsg:
		return t.handleGithubDeviceCode(msg)
	case vsGithubTokenMsg:
		return t.handleGithubToken(msg)
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
	case welcomeClusterStatusMsg:
		t.welcomeClusterStatus = msg.status
		if msg.clusterMode != "" {
			t.welcomeClusterMode = msg.clusterMode
		}
		t.welcomeLoaded = true
		return t, nil
	case vsInitDoneMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		return t, tea.Batch(t.loadVibespaces(), detectClusterStatus(), refreshSharedState(t.shared))
	case vsDeleteAgentDoneMsg:
		return t.handleDeleteAgentDone(msg)
	case vsAgentStartStopDoneMsg:
		return t.handleAgentStartStopDone(msg)
	case vsAgentStatusClearMsg:
		t.agentStatusMsg = ""
		return t, nil
	case vsAgentRefreshTickMsg:
		return t.handleAgentRefreshTick()
	case vsAddAgentDoneMsg:
		return t.handleAddAgentDone(msg)
	case vsEditConfigDoneMsg:
		t.mode = vibespacesModeAgentView
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		if t.selectedVS != nil {
			return t, t.loadAgentConfigs(t.selectedVS.ID, t.selectedVS.Name)
		}
		return t, nil
	case vsAddForwardDoneMsg:
		return t.handleAddForwardDone(msg)
	case vsRemoveForwardDoneMsg:
		return t.handleRemoveForwardDone(msg)
	case vsToggleDNSDoneMsg:
		return t.handleToggleDNSDone(msg)
	case vsSudoDoneMsg:
		t.sudoPromptActive = false
		if msg.ok {
			t.sudoPassword = msg.password
			return t, t.retryDNSHostEntry()
		}
		t.err = "sudo authentication failed — password is required for DNS configuration"
		return t, nil
	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	return t, nil
}

func (t *VibespacesTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if t.sudoPromptActive {
		return t.handleSudoPromptKey(msg)
	}
	switch t.mode {
	case vibespacesModeSessionList:
		return t.handleSessionListKey(msg)
	case vibespacesModeAgentView:
		return t.handleAgentViewKey(msg)
	case vibespacesModeCreateForm:
		return t.handleCreateFormKey(msg)
	case vibespacesModeGithubAuth:
		if msg.String() == "esc" {
			t.mode = vibespacesModeCreateForm
			t.err = ""
			return t, nil
		}
		return t, nil
	case vibespacesModeDeleteConfirm:
		return t.handleDeleteConfirmKey(msg)
	case vibespacesModeDeleteAgentConfirm:
		return t.handleDeleteAgentConfirmKey(msg)
	case vibespacesModeAddAgent:
		return t.handleAddAgentKey(msg)
	case vibespacesModeEditConfig:
		return t.handleEditConfigKey(msg)
	case vibespacesModeForwardManager:
		return t.handleForwardManagerKey(msg)
	default:
		return t.handleListKey(msg)
	}
}

func (t *VibespacesTab) handleTabActivate() (tea.Model, tea.Cmd) {
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
}

func (t *VibespacesTab) handlePaletteNewVibespace() (tea.Model, tea.Cmd) {
	t.mode = vibespacesModeCreateForm
	t.createField = createFieldName
	t.createName = ""
	t.createAgentType = agent.TypeClaudeCode
	t.createRepo = ""
	t.createWorktree = false
	t.createBranch = ""
	t.createCPU = config.Global().Resources.CPU
	t.createMemory = config.Global().Resources.Memory
	t.createStorage = config.Global().Resources.Storage
	t.err = ""
	return t, nil
}

func (t *VibespacesTab) handleVibespacesLoaded(msg vibespacesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.err = errorWithHint(msg.err)
		return t, nil
	}
	t.vibespaces = msg.vibespaces
	t.clampSelected()
	t.err = ""
	return t, tea.Batch(t.loadAgentInfo(), t.loadLogsForSelected(), t.scheduleRefreshIfNeeded())
}

func (t *VibespacesTab) handleSessionsLoaded(msg vsSessionsLoadedMsg) (tea.Model, tea.Cmd) {
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
}

func (t *VibespacesTab) handleConnectReady(msg vsConnectReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.err = msg.err.Error()
		return t, nil
	}
	switch msg.mode {
	case vsConnectModeSessionResume:
		var cfg *agent.Config
		if c, ok := t.agentConfigs[msg.agentName]; ok {
			cfg = c
		}
		return t, execSessionResumeCmd(msg.sshPort, msg.agentName, msg.agentType, msg.sessionID, cfg)
	case vsConnectModeShell:
		return t, t.execShellConnect(msg.sshPort)
	case vsConnectModeAgentCLI:
		return t, t.execAgentConnect(msg.sshPort, msg.agentName, msg.agentType)
	}
	return t, nil
}

func (t *VibespacesTab) handleBrowserReady(msg vsBrowserReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.err = msg.err.Error()
		return t, nil
	}
	url := fmt.Sprintf("http://localhost:%d", msg.ttydPort)
	if err := openBrowserURL(url); err != nil {
		t.err = fmt.Sprintf("open browser: %s", err)
	}
	return t, nil
}

func (t *VibespacesTab) handleExecReturn(msg vsExecReturnMsg) (tea.Model, tea.Cmd) {
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
}

func (t *VibespacesTab) handleGithubDeviceCode(msg vsGithubDeviceCodeMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.mode = vibespacesModeCreateForm
		t.err = fmt.Sprintf("GitHub auth failed: %s", msg.err)
		return t, nil
	}
	t.mode = vibespacesModeGithubAuth
	t.githubUserCode = msg.resp.UserCode
	t.githubVerifyURI = msg.resp.VerificationURI
	t.githubDevCode = msg.resp.DeviceCode
	t.githubInterval = msg.resp.Interval
	_ = openBrowserURL(msg.resp.VerificationURI)
	clientID := config.Global().GitHub.ClientID
	devCode := msg.resp.DeviceCode
	interval := msg.resp.Interval
	expiresIn := msg.resp.ExpiresIn
	return t, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(expiresIn)*time.Second)
		defer cancel()
		token, err := github.PollForToken(ctx, clientID, devCode, interval)
		return vsGithubTokenMsg{token: token, err: err}
	}
}

func (t *VibespacesTab) handleGithubToken(msg vsGithubTokenMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.mode = vibespacesModeCreateForm
		t.err = fmt.Sprintf("GitHub auth failed: %s", msg.err)
		return t, nil
	}
	t.githubAccessToken = msg.token.AccessToken
	t.githubRefreshToken = msg.token.RefreshToken
	t.mode = vibespacesModeCreateForm
	return t, t.submitCreateForm()
}

func (t *VibespacesTab) handleDeleteAgentDone(msg vsDeleteAgentDoneMsg) (tea.Model, tea.Cmd) {
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
}

func (t *VibespacesTab) handleAgentStartStopDone(msg vsAgentStartStopDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.agentStatusMsg = ""
		t.agentRefreshPending = 0
		t.err = msg.err.Error()
	} else if msg.action == "stop" {
		t.agentStatusMsg = "Agent stopped"
	} else {
		t.agentStatusMsg = "Agent started"
	}
	t.agentRefreshPending = 5
	refreshTick := tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return vsAgentRefreshTickMsg{}
	})
	clearTick := tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return vsAgentStatusClearMsg{}
	})
	if t.selectedVS != nil {
		return t, tea.Batch(t.loadAgentsForView(t.selectedVS.ID, t.selectedVS.Name), refreshTick, clearTick)
	}
	return t, tea.Batch(refreshTick, clearTick)
}

func (t *VibespacesTab) handleAgentRefreshTick() (tea.Model, tea.Cmd) {
	t.agentRefreshPending--
	if t.agentRefreshPending <= 0 || t.mode != vibespacesModeAgentView {
		t.agentRefreshPending = 0
		return t, nil
	}
	var cmds []tea.Cmd
	if t.selectedVS != nil {
		cmds = append(cmds, t.loadAgentsForView(t.selectedVS.ID, t.selectedVS.Name))
	}
	cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return vsAgentRefreshTickMsg{}
	}))
	return t, tea.Batch(cmds...)
}

func (t *VibespacesTab) handleAddAgentDone(msg vsAddAgentDoneMsg) (tea.Model, tea.Cmd) {
	t.mode = vibespacesModeAgentView
	if msg.err != nil {
		t.err = msg.err.Error()
	}
	t.agentRefreshPending = 5
	refreshTick := tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return vsAgentRefreshTickMsg{}
	})
	if t.selectedVS != nil {
		return t, tea.Batch(
			t.loadAgentsForView(t.selectedVS.ID, t.selectedVS.Name),
			t.loadAgentConfigs(t.selectedVS.ID, t.selectedVS.Name),
			t.loadForwards(t.selectedVS.ID, t.selectedVS.Name),
			refreshTick,
		)
	}
	return t, refreshTick
}

func (t *VibespacesTab) handleAddForwardDone(msg vsAddForwardDoneMsg) (tea.Model, tea.Cmd) {
	t.fwdManagerAdding = false
	if msg.err != nil {
		t.err = msg.err.Error()
	} else if msg.dnsName != "" {
		t.sudoPromptActive = true
		t.sudoInput = ""
		t.sudoPendingDNS = msg.dnsName
		t.sudoPendingOp = "add"
	}
	if t.selectedVS != nil {
		return t, t.loadForwards(t.selectedVS.ID, t.selectedVS.Name)
	}
	return t, nil
}

func (t *VibespacesTab) handleRemoveForwardDone(msg vsRemoveForwardDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.err = msg.err.Error()
	} else if msg.dnsName != "" {
		t.sudoPromptActive = true
		t.sudoInput = ""
		t.sudoPendingDNS = msg.dnsName
		t.sudoPendingOp = "remove"
	}
	if t.selectedVS != nil {
		return t, t.loadForwards(t.selectedVS.ID, t.selectedVS.Name)
	}
	return t, nil
}

func (t *VibespacesTab) handleToggleDNSDone(msg vsToggleDNSDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		t.err = msg.err.Error()
	} else if msg.dnsName != "" {
		t.sudoPromptActive = true
		t.sudoInput = ""
		t.sudoPendingDNS = msg.dnsName
		t.sudoPendingOp = "add"
	} else if msg.oldDNS != "" {
		t.sudoPromptActive = true
		t.sudoInput = ""
		t.sudoPendingDNS = msg.oldDNS
		t.sudoPendingOp = "remove"
	}
	if t.selectedVS != nil {
		return t, t.loadForwards(t.selectedVS.ID, t.selectedVS.Name)
	}
	return t, nil
}

func (t *VibespacesTab) handleSudoPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.sudoPromptActive = false
		t.sudoInput = ""
		t.sudoPendingDNS = ""
		t.sudoPendingOp = ""
		return t, nil
	case "enter":
		pw := t.sudoInput
		t.sudoInput = ""
		if pw == "" {
			t.sudoPromptActive = false
			return t, nil
		}
		return t, t.validateSudo(pw)
	case "backspace":
		if len(t.sudoInput) > 0 {
			t.sudoInput = t.sudoInput[:len(t.sudoInput)-1]
		}
		return t, nil
	default:
		if len(msg.String()) == 1 {
			t.sudoInput += msg.String()
		}
		return t, nil
	}
}

func (t *VibespacesTab) handleSessionListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
}

func (t *VibespacesTab) handleAgentViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			t.addAgentBranch = ""
			agentCfg := config.Global().Agent
			t.addAgentModel = agentCfg.Model
			if agentCfg.MaxTurns > 0 {
				t.addAgentMaxTurns = strconv.Itoa(agentCfg.MaxTurns)
			} else {
				t.addAgentMaxTurns = ""
			}
			t.addAgentShareCreds = agentCfg.ShareCredentials
			t.addAgentSkipPerms = agentCfg.SkipPermissions
			t.addAgentToolsList = agentSupportedTools(agent.TypeClaudeCode)
			t.addAgentAllowedSet = make(map[string]bool)
			t.addAgentDisallowedSet = make(map[string]bool)
			t.addAgentToolsCursor = 0
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
	case "e":
		if t.agentCursor < len(t.viewAgents) && t.selectedVS != nil {
			ag := t.viewAgents[t.agentCursor]
			t.mode = vibespacesModeEditConfig
			t.editConfigAgentName = ag.AgentName
			t.editConfigField = editConfigFieldModel
			t.editConfigToolsList = agentSupportedTools(ag.AgentType)
			t.editConfigToolsCursor = 0
			t.err = ""
			if cfg, ok := t.agentConfigs[ag.AgentName]; ok && cfg != nil {
				t.editConfigModel = cfg.Model
				t.editConfigMaxTurns = ""
				if cfg.MaxTurns > 0 {
					t.editConfigMaxTurns = strconv.Itoa(cfg.MaxTurns)
				}
				t.editConfigSkipPerms = cfg.SkipPermissions
				t.editConfigAllowedSet = make(map[string]bool)
				for _, tool := range cfg.AllowedTools {
					t.editConfigAllowedSet[tool] = true
				}
				t.editConfigDisallowedSet = make(map[string]bool)
				for _, tool := range cfg.DisallowedTools {
					t.editConfigDisallowedSet[tool] = true
				}
			} else {
				t.editConfigModel = ""
				t.editConfigMaxTurns = ""
				t.editConfigSkipPerms = false
				t.editConfigAllowedSet = make(map[string]bool)
				t.editConfigDisallowedSet = make(map[string]bool)
			}
			return t, nil
		}
	case "f":
		if t.selectedVS != nil {
			t.mode = vibespacesModeForwardManager
			t.fwdManagerCursor = 0
			t.fwdManagerAdding = false
			t.err = ""
			return t, nil
		}
	case "d":
		if t.agentCursor < len(t.viewAgents) && t.selectedVS != nil {
			ag := t.viewAgents[t.agentCursor]
			if !ag.IsPrimary {
				t.mode = vibespacesModeDeleteAgentConfirm
				t.deleteAgentName = ag.AgentName
				t.deleteAgentInput = ""
				t.err = ""
				return t, nil
			}
		}
	case "S":
		if t.agentCursor < len(t.viewAgents) && t.selectedVS != nil {
			ag := t.viewAgents[t.agentCursor]
			if ag.Status == "running" {
				t.agentStatusMsg = fmt.Sprintf("Stopping %s…", ag.AgentName)
			} else {
				t.agentStatusMsg = fmt.Sprintf("Starting %s…", ag.AgentName)
			}
			return t, t.toggleAgentStartStop(t.selectedVS.Name, ag.AgentName, ag.Status)
		}
	}
	return t, nil
}

func (t *VibespacesTab) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		t.createCPU = config.Global().Resources.CPU
		t.createMemory = config.Global().Resources.Memory
		t.createStorage = config.Global().Resources.Storage
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
	case "i":
		if len(t.vibespaces) == 0 {
			return t, execInit()
		}
	case "r":
		return t, tea.Batch(t.loadVibespaces(), detectClusterStatus(), refreshSharedState(t.shared))
	}
	if t.selected != prev {
		return t, t.loadLogsForSelected()
	}
	return t, nil
}

func (t *VibespacesTab) View() string {
	if t.err != "" && len(t.vibespaces) == 0 && t.mode == vibespacesModeList {
		return lipgloss.NewStyle().
			Foreground(ui.ColorError).
			Padding(1, 2).
			Render(fmt.Sprintf("Error loading vibespaces: %s", t.err))
	}

	switch t.mode {
	case vibespacesModeAgentView, vibespacesModeAddAgent, vibespacesModeDeleteAgentConfirm, vibespacesModeEditConfig, vibespacesModeForwardManager:
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
	case vibespacesModeGithubAuth:
		bottom = t.viewGithubAuth()
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

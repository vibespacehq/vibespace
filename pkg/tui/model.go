package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/permission"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/vibespace"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// LayoutMode represents the TUI layout mode
type LayoutMode int

const (
	LayoutChat LayoutMode = iota // Unified chat view
)

// Model is the main Bubble Tea model for the TUI
type Model struct {
	// Session
	session     *session.Session
	sessionName string // Name for persistence (can be empty for ad-hoc)
	resume      bool   // If true, resume existing Claude sessions; if false, start fresh

	// Connections
	agents     map[string]*AgentConn // "claude-1@projectA" → connection
	agentOrder []string              // display order
	agentMu    sync.RWMutex

	// Agent states (thinking indicators, session IDs)
	agentStates map[string]*AgentState

	// Claude session management (--session-id vs --resume)
	sessionManager *AgentSessionManager

	// Global daemon client
	daemonClient *daemon.Client

	// UI components
	input    textinput.Model
	viewport viewport.Model // Scrollable chat area
	history  *ChatHistory   // Unified chat history (still used for message storage)
	styles   Styles

	// Persistence
	historyStore *HistoryStore
	sessionStore *session.Store // For saving session changes

	// Layout
	layout        LayoutMode
	width         int
	height        int
	viewportReady bool // True once viewport is sized
	contentDirty  bool // True when messages changed and viewport needs update

	// State
	ctx       context.Context
	cancel    context.CancelFunc
	ready     bool
	quitting  bool
	err       error
	statusMsg string
	tickCount int // For animations

	// Default vibespace for short commands
	defaultVibespace string

	// Channel for receiving messages from agent goroutines (proper Bubble Tea pattern)
	incomingMsgs chan RichMessageMsg

	// Permission handling
	permissionServer *permission.Server
	pendingPerms     []*permission.Request // Queue of pending permission requests
	permissionPrompt *PermissionPrompt     // Current permission prompt (nil if none showing)

	// Reconnection tracking
	reconnecting map[string]bool // Agents currently being reconnected
}

// NewModel creates a new TUI model
// resume indicates whether to resume existing Claude sessions (true) or start fresh (false)
func NewModel(sess *session.Session, resume bool) *Model {
	ti := textinput.New()
	ti.Prompt = "" // We render our own prompt
	ti.Placeholder = "@agent message or /help (Tab for autocomplete)"
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80
	ti.ShowSuggestions = true

	ctx, cancel := context.WithCancel(context.Background())

	// Set default vibespace to first one if available
	defaultVS := ""
	if len(sess.Vibespaces) > 0 {
		defaultVS = sess.Vibespaces[0].Name
	}

	// Session name is used for persistence
	sessionName := sess.Name

	// Try to create history store
	historyStore, _ := NewHistoryStore()

	// Try to create session store for saving session changes
	sessionStore, _ := session.NewStore()

	// Create Claude session manager for --session-id vs --resume logic
	sessionManager := NewAgentSessionManager()

	// Create permission server
	permServer := permission.NewServer(permission.DefaultPermissionPort())

	return &Model{
		session:          sess,
		sessionName:      sessionName,
		resume:           resume,
		agents:           make(map[string]*AgentConn),
		agentOrder:       make([]string, 0),
		agentStates:      make(map[string]*AgentState),
		sessionManager:   sessionManager,
		input:            ti,
		history:          NewChatHistory(1000),
		historyStore:     historyStore,
		sessionStore:     sessionStore,
		styles:           NewStyles(),
		layout:           LayoutChat, // Default to unified chat view
		ctx:              ctx,
		cancel:           cancel,
		defaultVibespace: defaultVS,
		incomingMsgs:     make(chan RichMessageMsg, 100), // Buffered channel for agent messages
		permissionServer: permServer,
		pendingPerms:     make([]*permission.Request, 0),
		reconnecting:     make(map[string]bool),
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.initConnections(),
		m.loadHistory(),
		m.startPermissionServer(),
	)
}

// startPermissionServer starts the permission server and returns a command to listen for requests.
func (m *Model) startPermissionServer() tea.Cmd {
	return func() tea.Msg {
		if err := m.permissionServer.Start(); err != nil {
			// Log error but don't fail - permission prompts just won't work
			return nil
		}
		return m.listenForPermissions()()
	}
}

// listenForPermissions returns a command that waits for permission requests.
func (m *Model) listenForPermissions() tea.Cmd {
	return func() tea.Msg {
		select {
		case req := <-m.permissionServer.RequestChan():
			return PermissionRequestMsg{Request: req}
		case <-m.ctx.Done():
			return nil
		}
	}
}

// loadHistory loads chat history from persistence
func (m *Model) loadHistory() tea.Cmd {
	return func() tea.Msg {
		if m.historyStore == nil || m.sessionName == "" {
			return nil
		}

		messages, err := m.historyStore.Load(m.sessionName)
		if err != nil {
			return nil // Silently ignore load errors
		}

		if len(messages) > 0 {
			m.history.SetMessages(messages)
		}

		return nil
	}
}

// initConnections starts connecting to all agents
func (m *Model) initConnections() tea.Cmd {
	return func() tea.Msg {
		var errors []error

		// Handle empty session - this is valid, user will add vibespaces with /add
		if len(m.session.Vibespaces) == 0 {
			return InitCompleteMsg{Errors: nil}
		}

		// Ensure daemon is running
		if err := m.ensureDaemon(); err != nil {
			return InitCompleteMsg{Errors: []error{fmt.Errorf("daemon: %w", err)}}
		}

		// Connect to agents for each vibespace
		for _, vs := range m.session.Vibespaces {
			// Get agents for this vibespace
			agents, err := m.getAgentsForVibespace(vs)
			if err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", vs.Name, err))
				continue
			}

			// Connect to each agent
			for _, agentName := range agents {
				addr := session.AgentAddress{Agent: agentName, Vibespace: vs.Name}
				if err := m.connectAgent(addr); err != nil {
					errors = append(errors, fmt.Errorf("%s: %w", addr.String(), err))
				}
			}
		}

		return InitCompleteMsg{Errors: errors}
	}
}

// ensureDaemon ensures the daemon is running
func (m *Model) ensureDaemon() error {
	// Check if already have client
	if m.daemonClient != nil {
		return nil
	}

	// Check if daemon is running, start if not
	if !daemon.IsDaemonRunning() {
		if err := daemon.SpawnDaemon(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
		// Wait for daemon to be ready
		if err := daemon.WaitForDaemonReady(10 * time.Second); err != nil {
			return fmt.Errorf("daemon failed to start: %w", err)
		}
	}

	// Create client
	client, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create daemon client: %w", err)
	}

	m.daemonClient = client
	return nil
}

// getAgentsForVibespace returns the list of agents to connect to for a vibespace
func (m *Model) getAgentsForVibespace(vs session.VibespaceEntry) ([]string, error) {
	// If specific agents are listed, use those
	if len(vs.Agents) > 0 {
		return vs.Agents, nil
	}

	// Otherwise, get all agents from the daemon
	if m.daemonClient == nil {
		return nil, fmt.Errorf("daemon client not initialized")
	}

	forwards, err := m.daemonClient.ListForwardsForVibespace(vs.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get agents: %w", err)
	}

	agents := make([]string, 0, len(forwards.Agents))
	for _, a := range forwards.Agents {
		agents = append(agents, a.Name)
	}

	return agents, nil
}

// connectAgent connects to an agent
func (m *Model) connectAgent(addr session.AgentAddress) error {
	// Get SSH port from daemon
	if m.daemonClient == nil {
		return fmt.Errorf("daemon client not initialized")
	}

	forwards, err := m.daemonClient.ListForwardsForVibespace(addr.Vibespace)
	if err != nil {
		return fmt.Errorf("failed to list forwards: %w", err)
	}

	// Find SSH port for this agent
	var sshPort int
	for _, agent := range forwards.Agents {
		if agent.Name == addr.Agent {
			for _, fwd := range agent.Forwards {
				if fwd.Type == "ssh" && fwd.Status == "active" {
					sshPort = fwd.LocalPort
					break
				}
			}
		}
	}

	if sshPort == 0 {
		return fmt.Errorf("no active SSH forward for %s", addr.String())
	}

	// Get agent info and config from vibespace service
	var agentConfig *agent.Config
	var agentType agent.Type = agent.TypeClaudeCode // Default
	svc := vibespace.NewService(nil)                // Service will initialize k8s client on first use

	// Get agent type from service
	agentInfos, err := svc.ListAgents(m.ctx, addr.Vibespace)
	if err != nil {
		slog.Warn("failed to get agent info, using default type", "agent", addr.String(), "error", err)
	} else {
		for _, info := range agentInfos {
			if info.AgentName == addr.Agent {
				agentType = info.AgentType
				break
			}
		}
	}

	config, err := svc.GetAgentConfig(m.ctx, addr.Vibespace, addr.Agent)
	if err != nil {
		slog.Warn("failed to get agent config, using defaults", "agent", addr.String(), "error", err)
	} else {
		agentConfig = config
	}

	// Create agent connection with shared session manager
	conn := NewAgentConn(AgentConnOptions{
		Addr:            addr,
		LocalPort:       sshPort,
		SessionMgr:      m.sessionManager,
		MultiSessionID:  m.sessionName,
		Resume:          m.resume,
		AgentType:       agentType,
		Config:          agentConfig,
		PermissionToken: m.permissionServer.AuthToken(),
	})
	if err := conn.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Register agent
	m.agentMu.Lock()
	key := addr.String()
	m.agents[key] = conn
	m.agentOrder = append(m.agentOrder, key)
	m.agentStates[key] = NewAgentState(addr)
	m.agentMu.Unlock()

	return nil
}

// GetAgentCount returns the number of connected agents
func (m *Model) GetAgentCount() int {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()
	return len(m.agents)
}

// GetAgentColor returns the color for an agent
func (m *Model) GetAgentColor(addr string) lipgloss.Color {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	for i, key := range m.agentOrder {
		if key == addr {
			return GetAgentColor(i)
		}
	}
	return GetAgentColor(0)
}

// Available slash commands for autocomplete
var slashCommands = []struct {
	cmd  string
	args string // hint for what comes after
}{
	{"help", ""},
	{"list", ""},
	{"add", "<vibespace>"},
	{"remove", "<agent>"},
	{"focus", "<agent>"},
	{"clear", ""},
	{"quit", ""},
	{"session", "@agent"},
	{"scroll", "<top|bottom>"},
}

// UpdateSuggestions updates the autocomplete suggestions based on current input
func (m *Model) UpdateSuggestions() {
	input := m.input.Value()

	// Handle @mentions
	if strings.HasPrefix(input, "@") {
		m.updateMentionSuggestions(input)
		return
	}

	// Handle /commands
	if strings.HasPrefix(input, "/") {
		m.updateCommandSuggestions(input)
		return
	}

	m.input.SetSuggestions(nil)
}

// updateMentionSuggestions handles @agent autocomplete
func (m *Model) updateMentionSuggestions(input string) {
	mention := strings.TrimPrefix(input, "@")
	if idx := strings.Index(mention, " "); idx != -1 {
		// Already have a space, no more suggestions needed
		m.input.SetSuggestions(nil)
		return
	}

	var suggestions []string

	// Add "all" option
	if strings.HasPrefix("all", strings.ToLower(mention)) {
		suggestions = append(suggestions, "@all ")
	}

	// Add connected agents
	m.agentMu.RLock()
	for _, key := range m.agentOrder {
		if strings.HasPrefix(strings.ToLower(key), strings.ToLower(mention)) {
			suggestions = append(suggestions, "@"+key+" ")
		}
	}
	m.agentMu.RUnlock()

	m.input.SetSuggestions(suggestions)
}

// updateCommandSuggestions handles /command autocomplete
func (m *Model) updateCommandSuggestions(input string) {
	cmdPart := strings.TrimPrefix(input, "/")
	parts := strings.SplitN(cmdPart, " ", 2)
	cmd := parts[0]

	// If no space yet, suggest commands
	if len(parts) == 1 {
		var suggestions []string
		for _, c := range slashCommands {
			if strings.HasPrefix(c.cmd, strings.ToLower(cmd)) {
				if c.args != "" {
					suggestions = append(suggestions, "/"+c.cmd+" ")
				} else {
					suggestions = append(suggestions, "/"+c.cmd)
				}
			}
		}
		m.input.SetSuggestions(suggestions)
		return
	}

	// Command already typed, suggest args based on command
	arg := parts[1]
	switch cmd {
	case "focus", "session":
		// Suggest agents
		var suggestions []string
		m.agentMu.RLock()
		for _, key := range m.agentOrder {
			// For /focus, suggest agent names
			// For /session, suggest @agent format
			if cmd == "session" {
				if strings.HasPrefix("@"+strings.ToLower(key), strings.ToLower(arg)) {
					suggestions = append(suggestions, "/"+cmd+" @"+key+" ")
				}
			} else {
				if strings.HasPrefix(strings.ToLower(key), strings.ToLower(arg)) {
					suggestions = append(suggestions, "/"+cmd+" "+key)
				}
			}
		}
		m.agentMu.RUnlock()
		m.input.SetSuggestions(suggestions)

	case "scroll":
		var suggestions []string
		for _, opt := range []string{"top", "bottom"} {
			if strings.HasPrefix(opt, strings.ToLower(arg)) {
				suggestions = append(suggestions, "/scroll "+opt)
			}
		}
		m.input.SetSuggestions(suggestions)

	default:
		m.input.SetSuggestions(nil)
	}
}

// AddMessage adds a message to history and persists it
func (m *Model) AddMessage(msg *Message) {
	m.history.Add(msg)
	m.contentDirty = true // Mark for viewport refresh

	// Persist to history store
	if m.historyStore != nil && m.sessionName != "" {
		go m.historyStore.Append(m.sessionName, msg)
	}
}

// updateViewportContent renders all messages and updates the viewport
func (m *Model) updateViewportContent() {
	if !m.viewportReady {
		return
	}

	content := m.renderAllMessages()
	m.viewport.SetContent(content)

	// Auto-scroll to bottom when new content arrives (if already at bottom)
	if m.history.IsAtBottom() {
		m.viewport.GotoBottom()
	}

	m.contentDirty = false
}

// renderAllMessages renders all messages as a single string for the viewport
func (m *Model) renderAllMessages() string {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	const leftPad = "  " // Left padding for all content
	const rightPad = 4   // Right margin
	wrapWidth := m.width - len(leftPad) - rightPad
	if wrapWidth < 40 {
		wrapWidth = 40
	}

	if len(m.agentOrder) == 0 {
		return leftPad + m.styles.Dim.Render("No agents connected. Use /add <vibespace> to add agents.")
	}

	messages := m.history.GetAll()
	if len(messages) == 0 {
		return leftPad + m.styles.Dim.Render("(no messages yet - type @all <message> to send to all agents)")
	}

	var lines []string

	// Render each message with padding and word wrapping
	for i, msg := range messages {
		rendered := m.renderMessageForViewport(msg)
		// Word wrap the rendered message
		wrapped := wordwrap.String(rendered, wrapWidth)
		// Add left padding to each line of the message
		msgLines := strings.Split(wrapped, "\n")
		for _, line := range msgLines {
			lines = append(lines, leftPad+line)
		}
		// Add bottom spacing between messages (empty line)
		if i < len(messages)-1 {
			lines = append(lines, "")
		}
	}

	// Add thinking indicators for agents that are thinking
	for _, state := range m.agentStates {
		if state.IsThinking {
			lines = append(lines, "") // Space before thinking
			line := m.renderThinkingForViewport(state)
			lines = append(lines, leftPad+line)
		}
	}

	return strings.Join(lines, "\n")
}

// renderMessageForViewport renders a single message (may be multiple lines)
func (m *Model) renderMessageForViewport(msg *Message) string {
	// Format timestamp
	ts := m.styles.Timestamp.Render(msg.Timestamp.Format("15:04"))

	var label, content string

	switch msg.Type {
	case MessageTypeUser:
		label = UserLabelWithTarget(msg.Target)
		content = msg.Content

	case MessageTypeAssistant:
		color := m.GetAgentColor(msg.Sender)
		label = AgentLabelStyle(color).Render(fmt.Sprintf("[%s]", msg.Sender))
		content = renderMarkdown(msg.Content)

	case MessageTypeToolUse:
		content = m.renderToolUse(msg)
		// Tool use has its own label formatting
		return fmt.Sprintf("%s %s", ts, content)

	case MessageTypeError:
		color := m.GetAgentColor(msg.Sender)
		label = AgentLabelStyle(color).Render(fmt.Sprintf("[%s]", msg.Sender))
		content = m.styles.Error.Render(msg.Content)

	case MessageTypeSystem:
		label = m.styles.Dim.Render("[system]")
		content = m.styles.Dim.Render(msg.Content)

	default:
		label = m.styles.Dim.Render("[unknown]")
		content = msg.Content
	}

	if content != "" {
		return fmt.Sprintf("%s %s %s", ts, label, content)
	}
	return fmt.Sprintf("%s %s", ts, label)
}

// renderThinkingForViewport renders a thinking indicator
func (m *Model) renderThinkingForViewport(state *AgentState) string {
	ts := m.styles.Timestamp.Render("     ") // Blank timestamp for alignment
	color := m.GetAgentColor(state.Address.String())
	label := AgentLabelStyle(color).Render(fmt.Sprintf("[%s]", state.Address.String()))
	spinner := state.ThinkingIndicatorText()
	// Color the spinner to match the agent's color
	indicator := lipgloss.NewStyle().Foreground(color).Render(spinner)

	return fmt.Sprintf("%s %s %s", ts, label, indicator)
}

// renderToolUse renders a tool use message with improved formatting
// Format: [agent] [colored_symbol] ToolName → details
func (m *Model) renderToolUse(msg *Message) string {
	agentColor := m.GetAgentColor(msg.Sender)
	agentLabel := AgentLabelStyle(agentColor).Render(fmt.Sprintf("[%s]", msg.Sender))

	// Get tool icon and color
	icon, toolColor := getToolIconAndColor(msg.ToolName)
	coloredIcon := lipgloss.NewStyle().Foreground(toolColor).Render(fmt.Sprintf("[%s]", icon))
	toolName := lipgloss.NewStyle().Foreground(toolColor).Bold(true).Render(msg.ToolName)

	// Format details based on tool type
	var details string
	switch msg.ToolName {
	case "Read":
		details = msg.ToolInput
	case "Write":
		details = msg.ToolInput
	case "Edit":
		details = msg.ToolInput
	case "Bash":
		cmd := msg.ToolInput
		if len(cmd) > 60 {
			cmd = cmd[:57] + "..."
		}
		details = cmd
	case "Glob", "Grep":
		details = msg.ToolInput
	case "WebSearch":
		details = msg.ToolInput
	case "WebFetch":
		details = msg.ToolInput
	case "Task":
		details = msg.ToolInput
	case "EnterPlanMode":
		details = "entering plan mode"
	case "ExitPlanMode":
		details = "plan complete"
	case "AskUserQuestion":
		details = "awaiting input"
	case "TodoWrite":
		details = "updating tasks"
	default:
		details = msg.ToolInput
	}

	// Format: [agent] [symbol] Tool → details
	arrow := m.styles.Dim.Render("→")
	if details != "" {
		detailsStyled := m.styles.Dim.Render(details)
		return fmt.Sprintf("%s %s %s %s %s", agentLabel, coloredIcon, toolName, arrow, detailsStyled)
	}
	return fmt.Sprintf("%s %s %s", agentLabel, coloredIcon, toolName)
}

// getToolIconAndColor returns an icon and color for a tool type
func getToolIconAndColor(toolName string) (string, lipgloss.Color) {
	switch toolName {
	case "Read":
		return "◀", lipgloss.Color("#FFB800") // Yellow - reading in
	case "Write":
		return "▶", lipgloss.Color("#00FF9F") // Green - writing out
	case "Edit":
		return "✎", lipgloss.Color("#FF8C42") // Orange - modifying
	case "Bash":
		return "$", lipgloss.Color("#00D9FF") // Cyan - terminal
	case "Glob":
		return "⊛", lipgloss.Color("#7B61FF") // Purple - pattern
	case "Grep":
		return "⌕", lipgloss.Color("#7B61FF") // Purple - search
	case "WebSearch":
		return "◎", lipgloss.Color("#4ECDC4") // Teal - web
	case "WebFetch":
		return "⇣", lipgloss.Color("#4ECDC4") // Teal - download
	case "Task":
		return "◈", lipgloss.Color("#FF6B9D") // Pink - task
	case "EnterPlanMode":
		return "▣", lipgloss.Color("#FFB800") // Yellow - planning
	case "ExitPlanMode":
		return "✓", lipgloss.Color("#00FF9F") // Green - done
	case "AskUserQuestion":
		return "?", lipgloss.Color("#FF6B9D") // Pink - question
	case "TodoWrite":
		return "☐", lipgloss.Color("#00D9FF") // Cyan - todo
	default:
		return "●", lipgloss.Color("#888888") // Gray - unknown
	}
}

// renderMarkdown renders content as terminal-styled markdown using glamour.
func renderMarkdown(content string) string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(0), // outer wordwrap handles line length
	)
	if err != nil {
		return content
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.Trim(rendered, "\n")
}

// SetAgentThinking sets the thinking state for an agent
func (m *Model) SetAgentThinking(agentKey string, thinking bool) {
	m.agentMu.Lock()
	defer m.agentMu.Unlock()

	if state, ok := m.agentStates[agentKey]; ok {
		state.SetThinking(thinking)
	}
}

// GetThinkingAgents returns agents that are currently thinking
func (m *Model) GetThinkingAgents() []*AgentState {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	var thinking []*AgentState
	for _, state := range m.agentStates {
		if state.IsThinking {
			thinking = append(thinking, state)
		}
	}
	return thinking
}

// Close cleans up all connections
func (m *Model) Close() {
	m.cancel()

	// Stop permission server
	if m.permissionServer != nil {
		m.permissionServer.Stop()
	}

	m.agentMu.Lock()
	defer m.agentMu.Unlock()

	for _, conn := range m.agents {
		conn.Close()
	}
}

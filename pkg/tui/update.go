package tui

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/permission"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/vibespace"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Compile-time check that vibespace package is imported (used for GetSSHPrivateKeyPath)
var _ = vibespace.GetSSHPrivateKeyPath

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If permission prompt is showing and input is empty, handle permission keys
		if m.permissionPrompt != nil && m.input.Value() == "" {
			switch msg.String() {
			case "a", "A":
				return m, m.handlePermissionDecision(permission.DecisionAllow)
			case "d", "D":
				return m, m.handlePermissionDecision(permission.DecisionDeny)
			}
		}

		// Handle special keys
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			return m.handleInput()

		// Scrolling keys - handled by viewport
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
			// Let viewport handle scrolling
			if m.viewportReady {
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
				// Track if user scrolled away from bottom
				m.history.SetScrollPosition(!m.viewport.AtBottom())
			}
			// Don't fall through - these are scroll-only keys
			return m, tea.Batch(cmds...)
		}
		// Fall through to update text input below

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4

		// Update permission prompt size if showing
		if m.permissionPrompt != nil {
			m.permissionPrompt.SetSize(msg.Width, msg.Height)
		}

		// Calculate viewport height by measuring actual rendered heights
		// like the official chat example does
		headerRendered := m.renderHeader()
		inputRendered := m.renderInput()
		statusRendered := " " // minimum status height

		headerH := lipgloss.Height(headerRendered)
		inputH := lipgloss.Height(inputRendered)
		statusH := lipgloss.Height(statusRendered)
		gaps := 4 // newlines between sections (extra blank line before input)

		viewportHeight := msg.Height - headerH - inputH - statusH - gaps
		if viewportHeight < 3 {
			viewportHeight = 3
		}

		if !m.viewportReady {
			// First time sizing
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.Style = m.styles.OutputArea
			m.viewportReady = true
			m.contentDirty = true
		} else {
			// Resize existing viewport
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}

	case InitCompleteMsg:
		m.ready = true
		if len(msg.Errors) > 0 {
			errMsgs := make([]string, len(msg.Errors))
			for i, e := range msg.Errors {
				errMsgs[i] = e.Error()
			}
			m.statusMsg = fmt.Sprintf("Errors: %s", strings.Join(errMsgs, "; "))
		}
		// Start listening to all agent outputs
		cmds = append(cmds, m.listenToAgents())
		// Start tick for animations
		cmds = append(cmds, m.tick())
		// Initial content render
		m.contentDirty = true

	case RichMessageMsg:
		needsReconnect, addr := m.handleRichMessage(msg)
		if needsReconnect {
			key := addr.String()
			// Only schedule reconnect if not already reconnecting
			if !m.reconnecting[key] {
				m.reconnecting[key] = true
				m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s disconnected, will retry...", key)))
				cmds = append(cmds, m.scheduleReconnect(addr, 1, 5))
			}
		}
		// Continue listening for more messages
		cmds = append(cmds, m.waitForAgentMessage())

	case AgentConnectedMsg:
		m.statusMsg = fmt.Sprintf("Connected: %s", msg.Address.String())
		m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s connected", msg.Address.String())))

	case AgentDisconnectedMsg:
		key := msg.Address.String()
		m.statusMsg = fmt.Sprintf("Disconnected: %s", key)
		if msg.Error != nil {
			m.statusMsg += fmt.Sprintf(" (%s)", msg.Error.Error())
		}
		// Clear thinking state
		m.SetAgentThinking(key, false)
		// Only schedule reconnect if not already reconnecting
		if !m.reconnecting[key] {
			m.reconnecting[key] = true
			m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s disconnected, will retry...", key)))
			cmds = append(cmds, m.scheduleReconnect(msg.Address, 1, 5))
		}

	case AgentReconnectMsg:
		cmds = append(cmds, m.handleReconnect(msg))

	case AgentErrorMsg:
		m.statusMsg = fmt.Sprintf("Error from %s: %s", msg.Address.String(), msg.Error.Error())

	case TickMsg:
		m.tickCount++
		// Update viewport content if dirty or if thinking (for animation)
		if m.contentDirty || len(m.GetThinkingAgents()) > 0 {
			m.updateViewportContent()
		}
		// Periodic tick for animations
		cmds = append(cmds, m.tick())

	case FocusReturnMsg:
		// Returned from interactive focus mode
		m.statusMsg = fmt.Sprintf("Returned from %s", msg.Address.String())

	case ThinkingStartMsg:
		m.SetAgentThinking(msg.AgentKey, true)
		m.contentDirty = true

	case ThinkingEndMsg:
		m.SetAgentThinking(msg.AgentKey, false)
		m.contentDirty = true

	case PermissionRequestMsg:
		// Add to queue
		m.pendingPerms = append(m.pendingPerms, msg.Request)
		// Show prompt if not already showing one
		if m.permissionPrompt == nil && len(m.pendingPerms) > 0 {
			m.permissionPrompt = NewPermissionPrompt(m.pendingPerms[0])
			m.permissionPrompt.SetSize(m.width, m.height)
		}
		// Continue listening for more permission requests
		cmds = append(cmds, m.listenForPermissions())

	case PermissionDecisionMsg:
		// This is handled via handlePermissionDecision which returns a command
		// to send the decision to the server
	}

	// Update text input (for all non-scroll key events)
	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)

	// Update autocomplete suggestions based on current input
	m.UpdateSuggestions()

	return m, tea.Batch(cmds...)
}

// handleInput processes user input
func (m *Model) handleInput() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.input.Value())
	m.input.Reset()

	if input == "" {
		return m, nil
	}

	action, err := ParseInput(input, m.defaultVibespace)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}

	if action == nil {
		return m, nil
	}

	return m.executeAction(action)
}

// executeAction executes a parsed action
func (m *Model) executeAction(action Action) (tea.Model, tea.Cmd) {
	switch a := action.(type) {
	case SendAction:
		return m.executeSend(a)

	case CommandAction:
		return m.executeCommand(a)
	}

	return m, nil
}

// executeSend sends a message to agents
func (m *Model) executeSend(action SendAction) (tea.Model, tea.Cmd) {
	sentTo := []string{}

	// Determine target string for user message
	var targetStr string
	if len(action.Targets) == 1 && action.Targets[0].Agent == "all" {
		targetStr = "all"
	} else if len(action.Targets) == 1 {
		targetStr = action.Targets[0].String()
	} else {
		var targetNames []string
		for _, t := range action.Targets {
			targetNames = append(targetNames, t.String())
		}
		targetStr = strings.Join(targetNames, ", ")
	}

	// Add user message to history
	userMsg := NewUserMessage(targetStr, action.Message)
	m.AddMessage(userMsg)

	// Collect agents to send to (avoid holding lock during send)
	type sendTarget struct {
		key  string
		conn *AgentConn
	}
	var targets []sendTarget

	m.agentMu.RLock()
	for _, target := range action.Targets {
		if target.Agent == "all" {
			// Broadcast to all agents (optionally filtered by vibespace)
			for key, conn := range m.agents {
				if target.Vibespace == "" || conn.address.Vibespace == target.Vibespace {
					targets = append(targets, sendTarget{key: key, conn: conn})
				}
			}
		} else {
			// Send to specific agent
			key := target.String()
			if conn, ok := m.agents[key]; ok {
				targets = append(targets, sendTarget{key: key, conn: conn})
			} else {
				m.statusMsg = fmt.Sprintf("Agent not found: %s", key)
			}
		}
	}
	m.agentMu.RUnlock()

	// Now send to each target (no lock held)
	var reconnectCmds []tea.Cmd
	for _, t := range targets {
		// Mark agent as thinking
		m.SetAgentThinking(t.key, true)

		if err := t.conn.SendAndReconnect(action.Message); err != nil {
			m.statusMsg = fmt.Sprintf("Failed to send to %s: %s", t.key, err.Error())
			m.SetAgentThinking(t.key, false)
			// Trigger reconnect if agent is not connected
			if !m.reconnecting[t.key] {
				m.reconnecting[t.key] = true
				m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s disconnected, will retry...", t.key)))
				reconnectCmds = append(reconnectCmds, m.scheduleReconnect(t.conn.Address(), 1, 5))
			}
		} else {
			sentTo = append(sentTo, t.key)
		}
	}

	if len(sentTo) > 0 {
		m.statusMsg = fmt.Sprintf("Sent to %s", strings.Join(sentTo, ", "))
	}

	// Scroll to bottom when sending
	m.history.ScrollToBottom()
	if m.viewportReady {
		m.updateViewportContent()
		m.viewport.GotoBottom()
	}

	// Return reconnect commands if any
	if len(reconnectCmds) > 0 {
		return m, tea.Batch(reconnectCmds...)
	}
	return m, nil
}

// executeCommand executes a TUI command
func (m *Model) executeCommand(cmd CommandAction) (tea.Model, tea.Cmd) {
	switch cmd.Cmd {
	case "quit", "exit", "q":
		m.quitting = true
		return m, tea.Quit

	case "help", "h", "?":
		m.statusMsg = m.renderHelp()

	case "list", "ls":
		m.statusMsg = m.listAgents()

	case "focus":
		if len(cmd.Args) == 0 {
			m.statusMsg = "Usage: /focus <agent>[@vibespace] (Ctrl+B D to detach)"
			return m, nil
		}
		addr := session.ParseAgentAddress(cmd.Args[0], m.defaultVibespace)
		key := addr.String()
		m.agentMu.RLock()
		conn, exists := m.agents[key]
		m.agentMu.RUnlock()
		if !exists {
			m.statusMsg = fmt.Sprintf("Agent not found: %s", key)
			return m, nil
		}

		// Build agent command with agent's config (returns []string for safe quoting)
		sessionID := m.sessionManager.GetSession(m.sessionName, key)
		agentArgs := BuildInteractiveAgentCommand(conn.AgentType(), conn.Config(), sessionID)

		// tmux session name based on agent (sanitize for tmux)
		tmuxSession := fmt.Sprintf("agent-%s", addr.Agent)

		// Build properly shell-quoted tmux command
		tmuxCmd := agent.WrapForTmuxSSH(tmuxSession, agentArgs)

		// Debug logging
		sshKeyPath := vibespace.GetSSHPrivateKeyPath()
		sshPort := fmt.Sprintf("%d", conn.LocalPort())
		slog.Debug("focus: launching",
			"agent", key,
			"agentType", conn.AgentType(),
			"sessionID", sessionID,
			"agentArgs", agentArgs,
			"tmuxSession", tmuxSession,
			"tmuxCmd", tmuxCmd,
			"sshKeyPath", sshKeyPath,
			"sshPort", sshPort,
		)

		// Launch interactive Claude session via tmux
		// User can detach with Ctrl+B D to return to TUI without killing Claude
		m.statusMsg = fmt.Sprintf("Launching %s (Ctrl+B D to detach)...", key)
		return m, tea.ExecProcess(
			exec.Command("ssh",
				"-i", sshKeyPath,
				"-p", sshPort,
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"-t",
				"user@localhost",
				tmuxCmd,
			),
			func(err error) tea.Msg {
				slog.Debug("focus: process returned", "agent", key, "error", err)
				if err != nil {
					return AgentErrorMsg{Address: addr, Error: err}
				}
				return FocusReturnMsg{Address: addr}
			},
		)

	case "save":
		// Sessions are automatically saved on creation with a UUID name
		// /save allows renaming to a more memorable name
		name := ""
		if len(cmd.Args) > 0 {
			name = cmd.Args[0]
		}
		if name == "" {
			m.statusMsg = "Usage: /save <name> (rename session)"
			return m, nil
		}
		if err := m.saveSession(name); err != nil {
			m.statusMsg = fmt.Sprintf("Failed to save: %s", err.Error())
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Session saved as '%s'", name)
		m.sessionName = name
		m.session.Name = name

	case "add":
		if len(cmd.Args) == 0 {
			m.statusMsg = "Usage: /add <vibespace> or /add <agent>@<vibespace>"
			return m, nil
		}
		return m.executeAddCommand(cmd.Args)

	case "remove", "rm":
		if len(cmd.Args) == 0 {
			m.statusMsg = "Usage: /remove <agent>[@vibespace]"
			return m, nil
		}
		addr := session.ParseAgentAddress(cmd.Args[0], m.defaultVibespace)
		key := addr.String()
		m.agentMu.Lock()
		if conn, ok := m.agents[key]; ok {
			conn.Close()
			delete(m.agents, key)
			delete(m.agentStates, key)
			// Remove from order
			for i, k := range m.agentOrder {
				if k == key {
					m.agentOrder = append(m.agentOrder[:i], m.agentOrder[i+1:]...)
					break
				}
			}
			m.statusMsg = fmt.Sprintf("Removed %s", key)
			m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s removed", key)))
		} else {
			m.statusMsg = fmt.Sprintf("Agent not found: %s", key)
		}
		m.agentMu.Unlock()

	case "clear":
		// Clear chat history
		m.history.Clear()
		// Also clear persisted history
		if m.historyStore != nil && m.sessionName != "" {
			go m.historyStore.Clear(m.sessionName)
		}
		// Clear viewport content
		if m.viewportReady {
			m.viewport.SetContent("")
			m.viewport.GotoTop()
		}
		m.contentDirty = true
		m.statusMsg = "Cleared chat history"

	case "scroll":
		if len(cmd.Args) == 0 {
			m.statusMsg = "Usage: /scroll <top|bottom>"
			return m, nil
		}
		switch cmd.Args[0] {
		case "top":
			m.history.ScrollToTop()
			m.statusMsg = "Scrolled to top"
		case "bottom":
			m.history.ScrollToBottom()
			m.statusMsg = "Scrolled to bottom"
		default:
			m.statusMsg = "Usage: /scroll <top|bottom>"
		}

	case "session":
		// /session @agent <action> [args]
		// /session @agent new        - Start fresh Claude session
		// /session @agent list       - List session history
		// /session @agent resume <id> - Resume specific session
		// /session @agent info       - Show current session info
		if len(cmd.Args) < 2 {
			m.statusMsg = "Usage: /session @agent <new|list|resume|info> [session-id]"
			return m, nil
		}

		// Parse agent address (first arg should be @agent)
		agentArg := cmd.Args[0]
		if !strings.HasPrefix(agentArg, "@") {
			m.statusMsg = "Usage: /session @agent <new|list|resume|info> [session-id]"
			return m, nil
		}
		addr := session.ParseAgentAddress(agentArg[1:], m.defaultVibespace)
		key := addr.String()

		// Verify agent exists
		m.agentMu.RLock()
		conn, exists := m.agents[key]
		m.agentMu.RUnlock()
		if !exists {
			m.statusMsg = fmt.Sprintf("Agent not found: %s", key)
			return m, nil
		}

		action := cmd.Args[1]
		slog.Debug("session command", "action", action, "agent", key, "multiSession", m.sessionName)
		switch action {
		case "new":
			// Create new session for this agent within current multi-session
			newID := m.sessionManager.NewSession(m.sessionName, key)
			shortID := newID
			if len(shortID) > 13 {
				shortID = shortID[:13]
			}
			slog.Debug("created new session", "agent", key, "sessionID", newID)

			// Check if this is a Codex agent (which auto-generates session IDs)
			isCodex := conn.AgentType() == agent.TypeCodex

			// Reconnect agent with new session (force --session-id to create it on pod)
			// Do this synchronously so the agent is ready for the next message
			conn.SetForceNewSession(true)
			slog.Debug("reconnecting agent", "agent", key, "forceNewSession", true)
			if err := conn.Reconnect(); err != nil {
				slog.Error("reconnect failed", "agent", key, "error", err)
				m.statusMsg = fmt.Sprintf("Failed to start new session: %s", err.Error())
				m.AddMessage(NewSystemMessage(fmt.Sprintf("Failed to start new session for %s: %s", key, err.Error())))
			} else {
				slog.Debug("reconnect successful", "agent", key, "sessionID", newID)
				if isCodex {
					// Codex auto-generates session IDs - will be captured from thread.started
					m.statusMsg = fmt.Sprintf("New session for %s (ID assigned on first message)", key)
					m.AddMessage(NewSystemMessage(fmt.Sprintf("New session started for %s (send a message to get session ID)", key)))
				} else {
					m.statusMsg = fmt.Sprintf("New session for %s: %s", key, shortID)
					m.AddMessage(NewSystemMessage(fmt.Sprintf("New session started for %s (ID: %s)", key, shortID)))
				}
			}

		case "list":
			// List sessions for this agent within current multi-session
			sessions := m.sessionManager.ListSessions(m.sessionName, key)
			currentID := m.sessionManager.GetCurrentSessionID(m.sessionName, key)
			if len(sessions) == 0 {
				m.statusMsg = fmt.Sprintf("No sessions for %s", key)
			} else {
				m.statusMsg = fmt.Sprintf("Sessions for %s:\n%s", key, FormatSessionList(sessions, currentID))
			}

		case "resume":
			if len(cmd.Args) < 3 {
				m.statusMsg = "Usage: /session @agent resume <session-id>"
				return m, nil
			}
			sessionID := cmd.Args[2]
			if err := m.sessionManager.ResumeSession(m.sessionName, key, sessionID); err != nil {
				m.statusMsg = fmt.Sprintf("Failed to resume session: %s", err.Error())
			} else {
				// Reconnect agent with resumed session (use --resume)
				// Do this synchronously so the agent is ready for the next message
				if err := conn.Reconnect(); err != nil {
					m.statusMsg = fmt.Sprintf("Failed to resume session: %s", err.Error())
					m.AddMessage(NewSystemMessage(fmt.Sprintf("Failed to resume session for %s: %s", key, err.Error())))
				} else {
					shortID := sessionID
					if len(shortID) > 13 {
						shortID = shortID[:13]
					}
					m.statusMsg = fmt.Sprintf("Resumed session %s for %s", shortID, key)
					m.AddMessage(NewSystemMessage(fmt.Sprintf("Resumed session %s for %s", shortID, key)))
				}
			}

		case "info":
			// Show current session info
			currentID := m.sessionManager.GetCurrentSessionID(m.sessionName, key)
			if currentID == "" {
				m.statusMsg = fmt.Sprintf("No active session for %s", key)
			} else {
				shortID := currentID
				if len(shortID) > 13 {
					shortID = shortID[:13]
				}
				m.statusMsg = fmt.Sprintf("Session for %s: %s", key, shortID)
			}

		default:
			m.statusMsg = "Usage: /session @agent <new|list|resume|info> [session-id]"
		}

	default:
		m.statusMsg = fmt.Sprintf("Unknown command: /%s. Type /help for available commands.", cmd.Cmd)
	}

	return m, nil
}

// listAgents returns a string listing all connected agents
func (m *Model) listAgents() string {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	if len(m.agentOrder) == 0 {
		return "No agents connected"
	}

	var parts []string
	for _, key := range m.agentOrder {
		conn := m.agents[key]
		status := "connected"
		if !conn.IsConnected() {
			status = "disconnected"
		}
		// Check if thinking
		if state, ok := m.agentStates[key]; ok && state.IsThinking {
			status = "thinking"
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", key, status))
	}

	return "Agents: " + strings.Join(parts, ", ")
}

// saveSession saves the current ad-hoc session
func (m *Model) saveSession(name string) error {
	store, err := session.NewStore()
	if err != nil {
		return err
	}

	m.session.Name = name
	return store.Save(m.session)
}

// handleRichMessage processes a rich message from an agent
func (m *Model) handleRichMessage(msg RichMessageMsg) (needsReconnect bool, addr session.AgentAddress) {
	// Clear thinking state when we receive a message
	m.SetAgentThinking(msg.AgentKey, false)

	// Check if we should auto-scroll (before adding message)
	wasAtBottom := m.history.IsAtBottom()

	// Check for disconnection errors - only trigger reconnect if agent is actually disconnected
	// Note: "Connection to localhost closed" is normal in print mode (SSH closes after each message)
	// Only reconnect for "Connection refused" which indicates the port-forward is down
	if msg.Message.Type == MessageTypeError {
		content := msg.Message.Content
		if strings.Contains(content, "Connection refused") ||
			strings.Contains(content, "Connection reset") {
			// Parse agent address from the key - only if not already connected
			m.agentMu.RLock()
			if conn, ok := m.agents[msg.AgentKey]; ok && !conn.IsConnected() {
				addr = conn.Address()
				needsReconnect = true
			}
			m.agentMu.RUnlock()
			if needsReconnect {
				// Don't add disconnect error to history - we'll show reconnect message instead
				return needsReconnect, addr
			}
		}
	}

	// Add the message to history (marks content as dirty)
	m.AddMessage(msg.Message)

	// Auto-scroll viewport to bottom if we were already at the bottom
	if wasAtBottom && m.viewportReady {
		m.viewport.GotoBottom()
	}

	return false, session.AgentAddress{}
}

// listenToAgents starts listening to all agent output channels
func (m *Model) listenToAgents() tea.Cmd {
	// Get current agents under lock
	m.agentMu.RLock()
	agents := make([]*AgentConn, 0, len(m.agents))
	for _, conn := range m.agents {
		agents = append(agents, conn)
	}
	m.agentMu.RUnlock()

	// Start a goroutine for each agent to forward messages to the channel
	// These goroutines send to m.incomingMsgs, NOT directly modify model state
	for _, conn := range agents {
		go func(c *AgentConn) {
			for msg := range c.OutputChan() {
				select {
				case m.incomingMsgs <- RichMessageMsg{
					AgentKey: c.Address().String(),
					Message:  msg,
				}:
				case <-m.ctx.Done():
					return
				}
			}
		}(conn)
	}

	// Return command to start listening to the incoming messages channel
	return m.waitForAgentMessage()
}

// waitForAgentMessage returns a command that waits for the next message from any agent
func (m *Model) waitForAgentMessage() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-m.incomingMsgs:
			return msg
		case <-m.ctx.Done():
			return nil
		}
	}
}

// tick returns a command that sends a tick message after a delay
func (m *Model) tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// executeAddCommand handles the /add command to add a vibespace or agent
func (m *Model) executeAddCommand(args []string) (tea.Model, tea.Cmd) {
	arg := args[0]

	// Check if it's an agent@vibespace format or just a vibespace
	var vsName string
	var specificAgent string

	if strings.Contains(arg, "@") {
		addr := session.ParseAgentAddress(arg, "")
		if addr.Vibespace == "" {
			m.statusMsg = "Invalid format. Use: /add <vibespace> or /add <agent>@<vibespace>"
			return m, nil
		}
		vsName = addr.Vibespace
		specificAgent = addr.Agent
	} else {
		vsName = arg
	}

	// Ensure daemon is running
	if err := m.ensureDaemon(); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to connect to daemon: %s", err.Error())
		return m, nil
	}

	// Get agents to connect
	var agentsToConnect []string
	if specificAgent != "" {
		agentsToConnect = []string{specificAgent}
	} else {
		// Get all agents from daemon
		forwards, err := m.daemonClient.ListForwardsForVibespace(vsName)
		if err != nil {
			m.statusMsg = fmt.Sprintf("Failed to get agents for %s: %s", vsName, err.Error())
			return m, nil
		}
		for _, a := range forwards.Agents {
			agentsToConnect = append(agentsToConnect, a.Name)
		}
	}

	// Connect to agents
	connected := []string{}
	for _, agentName := range agentsToConnect {
		addr := session.AgentAddress{Agent: agentName, Vibespace: vsName}
		key := addr.String()

		// Skip if already connected
		m.agentMu.RLock()
		_, exists := m.agents[key]
		m.agentMu.RUnlock()
		if exists {
			continue
		}

		if err := m.connectAgent(addr); err != nil {
			m.AddMessage(NewSystemMessage(fmt.Sprintf("Failed to connect to %s: %s", key, err.Error())))
			continue
		}
		connected = append(connected, key)
	}

	// Update session state
	if specificAgent != "" {
		m.session.AddVibespace(vsName, []string{specificAgent})
	} else {
		m.session.AddVibespace(vsName, nil)
	}

	// Set default vibespace if not set
	if m.defaultVibespace == "" {
		m.defaultVibespace = vsName
	}

	// Save session
	if m.sessionStore != nil && m.session.Name != "" {
		go m.sessionStore.Save(m.session)
	}

	// Start listening to new agents
	if len(connected) > 0 {
		m.AddMessage(NewSystemMessage(fmt.Sprintf("Added: %s", strings.Join(connected, ", "))))
		m.statusMsg = fmt.Sprintf("Added %d agent(s) from %s", len(connected), vsName)

		// Start listening to the new agents' output channels
		m.agentMu.RLock()
		for _, key := range connected {
			if conn, ok := m.agents[key]; ok {
				go func(c *AgentConn) {
					for msg := range c.OutputChan() {
						select {
						case m.incomingMsgs <- RichMessageMsg{
							AgentKey: c.Address().String(),
							Message:  msg,
						}:
						case <-m.ctx.Done():
							return
						}
					}
				}(conn)
			}
		}
		m.agentMu.RUnlock()
	} else {
		m.statusMsg = fmt.Sprintf("No new agents to add from %s", vsName)
	}

	return m, nil
}

// handlePermissionDecision sends a permission decision and advances the queue.
func (m *Model) handlePermissionDecision(decision permission.Decision) tea.Cmd {
	if m.permissionPrompt == nil {
		return nil
	}

	req := m.permissionPrompt.Request()
	slog.Debug("permission decision", "id", req.ID, "agent", req.AgentKey, "tool", req.ToolName, "decision", decision)

	// Send decision to server
	if err := m.permissionServer.Respond(req.ID, decision); err != nil {
		slog.Error("failed to send permission decision", "error", err)
	}

	// Add system message about the decision
	decisionStr := "denied"
	if decision == permission.DecisionAllow {
		decisionStr = "allowed"
	}
	m.AddMessage(NewSystemMessage(fmt.Sprintf("Permission %s: %s for %s", decisionStr, req.ToolName, req.AgentKey)))

	// Remove from queue
	if len(m.pendingPerms) > 0 {
		m.pendingPerms = m.pendingPerms[1:]
	}

	// Show next prompt or clear
	if len(m.pendingPerms) > 0 {
		m.permissionPrompt = NewPermissionPrompt(m.pendingPerms[0])
		m.permissionPrompt.SetSize(m.width, m.height)
	} else {
		m.permissionPrompt = nil
	}

	return nil
}

// scheduleReconnect returns a command that triggers a reconnect after a delay
func (m *Model) scheduleReconnect(addr session.AgentAddress, attempt, maxRetry int) tea.Cmd {
	// Exponential backoff: 2s, 4s, 8s, 16s, 32s
	delay := time.Duration(1<<uint(attempt)) * time.Second
	if delay > 32*time.Second {
		delay = 32 * time.Second
	}

	slog.Debug("scheduling reconnect", "agent", addr.String(), "attempt", attempt, "delay", delay)

	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return AgentReconnectMsg{
			Address:  addr,
			Attempt:  attempt,
			MaxRetry: maxRetry,
		}
	})
}

// handleReconnect attempts to reconnect to a disconnected agent
func (m *Model) handleReconnect(msg AgentReconnectMsg) tea.Cmd {
	key := msg.Address.String()
	slog.Debug("attempting reconnect", "agent", key, "attempt", msg.Attempt)

	// Check if agent still exists in our map
	m.agentMu.RLock()
	conn, exists := m.agents[key]
	m.agentMu.RUnlock()

	if !exists {
		slog.Debug("agent no longer in session, skipping reconnect", "agent", key)
		delete(m.reconnecting, key)
		return nil
	}

	// Check if already connected
	if conn.IsConnected() {
		slog.Debug("agent already connected, skipping reconnect", "agent", key)
		delete(m.reconnecting, key)
		return nil
	}

	// Always get fresh port info from daemon (port changes after pod restart)
	if m.daemonClient != nil {
		forwards, err := m.daemonClient.ListForwardsForVibespace(msg.Address.Vibespace)
		if err != nil {
			slog.Debug("failed to get forwards from daemon", "agent", key, "error", err)
		} else {
			for _, agentFwd := range forwards.Agents {
				if agentFwd.Name == msg.Address.Agent {
					for _, fwd := range agentFwd.Forwards {
						if fwd.Type == "ssh" && fwd.Status == "active" {
							slog.Debug("found SSH forward, creating new connection",
								"agent", key, "old_port", conn.LocalPort(), "new_port", fwd.LocalPort)
							// Always close old connection and create new one with fresh port
							conn.Close()
							newConn := NewAgentConn(AgentConnOptions{
								Addr:            msg.Address,
								LocalPort:       fwd.LocalPort,
								SessionMgr:      m.sessionManager,
								MultiSessionID:  m.sessionName,
								Resume:          true,
								AgentType:       conn.AgentType(),
								Config:          conn.Config(),
								PermissionToken: m.permissionServer.AuthToken(),
							})
							if err := newConn.Connect(); err == nil {
								m.agentMu.Lock()
								m.agents[key] = newConn
								m.agentMu.Unlock()
								delete(m.reconnecting, key)
								m.statusMsg = fmt.Sprintf("Reconnected: %s", key)
								m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s reconnected", key)))
								// Start listening to the new connection's output
								go func(c *AgentConn) {
									for msg := range c.OutputChan() {
										select {
										case m.incomingMsgs <- RichMessageMsg{
											AgentKey: c.Address().String(),
											Message:  msg,
										}:
										case <-m.ctx.Done():
											return
										}
									}
								}(newConn)
								return nil
							} else {
								slog.Debug("new connection failed", "agent", key, "port", fwd.LocalPort, "error", err)
							}
							break
						}
					}
					break
				}
			}
		}
	}

	// Try to reconnect using existing connection
	if err := conn.Reconnect(); err != nil {
		slog.Debug("reconnect failed", "agent", key, "attempt", msg.Attempt, "error", err)

		if msg.Attempt < msg.MaxRetry {
			m.statusMsg = fmt.Sprintf("Reconnect failed for %s (attempt %d/%d), retrying...", key, msg.Attempt, msg.MaxRetry)
			return m.scheduleReconnect(msg.Address, msg.Attempt+1, msg.MaxRetry)
		}

		delete(m.reconnecting, key)
		m.statusMsg = fmt.Sprintf("Failed to reconnect to %s after %d attempts", key, msg.MaxRetry)
		m.AddMessage(NewSystemMessage(fmt.Sprintf("Failed to reconnect to %s after %d attempts", key, msg.MaxRetry)))
		return nil
	}

	delete(m.reconnecting, key)
	m.statusMsg = fmt.Sprintf("Reconnected: %s", key)
	m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s reconnected", key)))
	return nil
}

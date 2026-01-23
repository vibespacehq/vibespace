package tui

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/yagizdagabak/vibespace/pkg/session"
	"github.com/yagizdagabak/vibespace/pkg/vibespace"

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

		// Calculate viewport height by measuring actual rendered heights
		// like the official chat example does
		headerRendered := m.renderHeader()
		inputRendered := m.renderInput()
		statusRendered := " " // minimum status height

		headerH := lipgloss.Height(headerRendered)
		inputH := lipgloss.Height(inputRendered)
		statusH := lipgloss.Height(statusRendered)
		gaps := 3 // newlines between sections

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
		m.handleRichMessage(msg)
		// Continue listening for more messages
		cmds = append(cmds, m.waitForAgentMessage())

	case AgentConnectedMsg:
		m.statusMsg = fmt.Sprintf("Connected: %s", msg.Address.String())
		m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s connected", msg.Address.String())))

	case AgentDisconnectedMsg:
		m.statusMsg = fmt.Sprintf("Disconnected: %s", msg.Address.String())
		if msg.Error != nil {
			m.statusMsg += fmt.Sprintf(" (%s)", msg.Error.Error())
		}
		// Clear thinking state
		m.SetAgentThinking(msg.Address.String(), false)
		m.AddMessage(NewSystemMessage(fmt.Sprintf("Agent %s disconnected", msg.Address.String())))

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
	for _, t := range targets {
		// Mark agent as thinking
		m.SetAgentThinking(t.key, true)

		if err := t.conn.SendAndReconnect(action.Message); err != nil {
			m.statusMsg = fmt.Sprintf("Failed to send to %s: %s", t.key, err.Error())
			m.SetAgentThinking(t.key, false)
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

		// Build Claude command based on resume flag and existing session
		var claudeCmd string
		sessionID := m.sessionManager.GetSession(m.sessionName, key)
		if m.resume && sessionID != "" {
			// Resume existing session
			claudeCmd = fmt.Sprintf("claude --resume %s", sessionID)
		} else if sessionID != "" {
			// Session exists but we're not resuming - use session-id to continue
			claudeCmd = fmt.Sprintf("claude --session-id %s", sessionID)
		} else {
			// No session yet, just start claude
			claudeCmd = "claude"
		}

		// tmux session name based on agent (sanitize for tmux)
		tmuxSession := fmt.Sprintf("claude-%s", addr.Agent)

		// Use tmux: attach to existing session or create new one with Claude
		// tmux new-session -A: attach if exists, create if not
		tmuxCmd := fmt.Sprintf("tmux new-session -A -s %s '%s'", tmuxSession, claudeCmd)

		// Launch interactive Claude session via tmux
		// User can detach with Ctrl+B D to return to TUI without killing Claude
		m.statusMsg = fmt.Sprintf("Launching %s (Ctrl+B D to detach)...", key)
		return m, tea.ExecProcess(
			exec.Command("ssh",
				"-i", vibespace.GetSSHPrivateKeyPath(),
				"-p", fmt.Sprintf("%d", conn.LocalPort()),
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"-t",
				"user@localhost",
				tmuxCmd,
			),
			func(err error) tea.Msg {
				if err != nil {
					return AgentErrorMsg{Address: addr, Error: err}
				}
				return FocusReturnMsg{Address: addr}
			},
		)

	case "ports":
		m.statusMsg = m.listPorts()

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
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			slog.Debug("created new session", "agent", key, "sessionID", newID)

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
				m.statusMsg = fmt.Sprintf("New session for %s: %s", key, shortID)
				m.AddMessage(NewSystemMessage(fmt.Sprintf("New Claude session started for %s (ID: %s)", key, shortID)))
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
					m.statusMsg = fmt.Sprintf("Resumed session %s for %s", sessionID, key)
					m.AddMessage(NewSystemMessage(fmt.Sprintf("Resumed Claude session %s for %s", sessionID, key)))
				}
			}

		case "info":
			// Show current session info
			currentID := m.sessionManager.GetCurrentSessionID(m.sessionName, key)
			if currentID == "" {
				m.statusMsg = fmt.Sprintf("No active session for %s", key)
			} else {
				shortID := currentID
				if len(shortID) > 8 {
					shortID = shortID[:8]
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

// listPorts returns a string listing all forwarded ports
func (m *Model) listPorts() string {
	var parts []string

	for vsName, client := range m.daemons {
		forwards, err := client.ListForwards()
		if err != nil {
			parts = append(parts, fmt.Sprintf("%s: error", vsName))
			continue
		}

		for _, agent := range forwards.Agents {
			for _, fwd := range agent.Forwards {
				parts = append(parts, fmt.Sprintf(
					"%s@%s: localhost:%d → %d (%s)",
					agent.Name, vsName, fwd.LocalPort, fwd.RemotePort, fwd.Type,
				))
			}
		}
	}

	if len(parts) == 0 {
		return "No ports forwarded"
	}

	return "Ports:\n  " + strings.Join(parts, "\n  ")
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
func (m *Model) handleRichMessage(msg RichMessageMsg) {
	// Clear thinking state when we receive a message
	m.SetAgentThinking(msg.AgentKey, false)

	// Check if we should auto-scroll (before adding message)
	wasAtBottom := m.history.IsAtBottom()

	// Add the message to history (marks content as dirty)
	m.AddMessage(msg.Message)

	// Auto-scroll viewport to bottom if we were already at the bottom
	if wasAtBottom && m.viewportReady {
		m.viewport.GotoBottom()
	}
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

	// Ensure daemon is running - this will fail if vibespace doesn't exist or isn't running
	if err := m.ensureDaemon(vsName); err != nil {
		m.statusMsg = fmt.Sprintf("Failed to connect to %s: %s", vsName, err.Error())
		return m, nil
	}

	// Get agents to connect
	var agentsToConnect []string
	if specificAgent != "" {
		agentsToConnect = []string{specificAgent}
	} else {
		// Get all agents from daemon
		client := m.daemons[vsName]
		status, err := client.Status()
		if err != nil {
			m.statusMsg = fmt.Sprintf("Failed to get status for %s: %s", vsName, err.Error())
			return m, nil
		}
		for _, a := range status.Agents {
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

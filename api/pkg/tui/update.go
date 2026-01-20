package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"vibespace/pkg/session"
	"vibespace/pkg/vibespace"

	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle special keys, but let others pass through to text input
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			return m.handleInput()
		case tea.KeyEsc:
			if m.layout == LayoutFocus {
				m.layout = LayoutSplit
				m.focusAgent = ""
				m.statusMsg = "Returned to split view"
			}
		}
		// Fall through to update text input below

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4

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

	case AgentOutputMsg:
		m.handleAgentOutput(msg)

	case AgentConnectedMsg:
		m.statusMsg = fmt.Sprintf("Connected: %s", msg.Address.String())

	case AgentDisconnectedMsg:
		m.statusMsg = fmt.Sprintf("Disconnected: %s", msg.Address.String())
		if msg.Error != nil {
			m.statusMsg += fmt.Sprintf(" (%s)", msg.Error.Error())
		}

	case AgentErrorMsg:
		m.statusMsg = fmt.Sprintf("Error from %s: %s", msg.Address.String(), msg.Error.Error())

	case TickMsg:
		// Periodic tick for updates
		cmds = append(cmds, m.tick())

	case FocusReturnMsg:
		// Returned from interactive focus mode
		m.statusMsg = fmt.Sprintf("Returned from %s", msg.Address.String())
	}

	// Update text input
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
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	for _, target := range action.Targets {
		if target.Agent == "all" {
			// Broadcast to all agents (optionally filtered by vibespace)
			for key, conn := range m.agents {
				if target.Vibespace == "" || conn.address.Vibespace == target.Vibespace {
					if err := conn.Send(action.Message); err != nil {
						m.statusMsg = fmt.Sprintf("Failed to send to %s: %s", key, err.Error())
					}
				}
			}
		} else {
			// Send to specific agent
			key := target.String()
			conn, ok := m.agents[key]
			if !ok {
				m.statusMsg = fmt.Sprintf("Agent not found: %s", key)
				continue
			}
			if err := conn.Send(action.Message); err != nil {
				m.statusMsg = fmt.Sprintf("Failed to send to %s: %s", key, err.Error())
			}
		}
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
			m.statusMsg = "Usage: /focus <agent>[@vibespace]"
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
		// Launch interactive Claude session
		// This temporarily exits the TUI and hands control to Claude
		m.statusMsg = fmt.Sprintf("Launching interactive session with %s...", key)
		return m, tea.ExecProcess(
			exec.Command("ssh",
				"-i", vibespace.GetSSHPrivateKeyPath(),
				"-p", fmt.Sprintf("%d", conn.localPort),
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
				"-t",
				"user@localhost",
				"bash", "-l", "-c", "claude",
			),
			func(err error) tea.Msg {
				if err != nil {
					return AgentErrorMsg{Address: addr, Error: err}
				}
				return FocusReturnMsg{Address: addr}
			},
		)

	case "split":
		m.layout = LayoutSplit
		m.focusAgent = ""
		m.statusMsg = "Returned to split view"

	case "ports":
		m.statusMsg = m.listPorts()

	case "save":
		if !m.isAdHoc {
			m.statusMsg = "This is already a saved session"
			return m, nil
		}
		name := ""
		if len(cmd.Args) > 0 {
			name = cmd.Args[0]
		}
		if name == "" {
			m.statusMsg = "Usage: /save <name>"
			return m, nil
		}
		if err := m.saveSession(name); err != nil {
			m.statusMsg = fmt.Sprintf("Failed to save: %s", err.Error())
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Session saved as '%s'", name)
		m.isAdHoc = false
		m.session.Name = name

	case "add":
		if len(cmd.Args) == 0 {
			m.statusMsg = "Usage: /add <vibespace> [agent]"
			return m, nil
		}
		// Adding vibespaces at runtime is complex, show message
		m.statusMsg = "Adding vibespaces at runtime is not yet supported. Create a session with 'vibespace session add'"

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
			// Remove from order
			for i, k := range m.agentOrder {
				if k == key {
					m.agentOrder = append(m.agentOrder[:i], m.agentOrder[i+1:]...)
					break
				}
			}
			m.statusMsg = fmt.Sprintf("Removed %s", key)
		} else {
			m.statusMsg = fmt.Sprintf("Agent not found: %s", key)
		}
		m.agentMu.Unlock()

	case "clear":
		// Clear output for all agents
		m.agentMu.Lock()
		for _, output := range m.outputs {
			output.mu.Lock()
			output.lines = nil
			output.mu.Unlock()
		}
		m.agentMu.Unlock()
		m.statusMsg = "Cleared all output"

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

// handleAgentOutput processes output from an agent
func (m *Model) handleAgentOutput(msg AgentOutputMsg) {
	key := msg.Address.String()

	m.agentMu.RLock()
	output, ok := m.outputs[key]
	m.agentMu.RUnlock()

	if ok {
		output.Add(msg.Output)
	}
}

// listenToAgents starts listening to all agent output channels
func (m *Model) listenToAgents() tea.Cmd {
	return func() tea.Msg {
		m.agentMu.RLock()
		agents := make([]*AgentConn, 0, len(m.agents))
		for _, conn := range m.agents {
			agents = append(agents, conn)
		}
		m.agentMu.RUnlock()

		// This is a simplified approach - in practice you'd want a more
		// sophisticated multiplexing strategy
		for _, conn := range agents {
			go func(c *AgentConn) {
				for output := range c.OutputChan() {
					// We can't send messages directly from here,
					// so we buffer in the output and let the tick pick it up
					m.agentMu.RLock()
					if out, ok := m.outputs[c.Address().String()]; ok {
						out.Add(output)
					}
					m.agentMu.RUnlock()
				}
			}(conn)
		}

		return TickMsg{}
	}
}

// tick returns a command that sends a tick message after a delay
func (m *Model) tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

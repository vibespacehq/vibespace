package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return m.styles.Error.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if !m.ready {
		return m.renderLoading()
	}

	// Simple layout like official example: header + viewport + input + status
	// The viewport internally handles padding to its set Height
	header := m.renderHeader()
	input := m.renderInput()

	var status string
	if m.statusMsg != "" {
		status = m.styles.StatusBar.Render(m.statusMsg)
	} else {
		status = " " // Keep consistent height
	}

	// Viewport handles its own height padding
	return fmt.Sprintf("%s\n%s\n%s\n%s", header, m.viewport.View(), input, status)
}

// renderLoading renders the loading state
func (m *Model) renderLoading() string {
	var sb strings.Builder

	sb.WriteString(m.styles.Title.Render("vibespace multi-session"))
	sb.WriteString("\n\n")

	if m.isAdHoc {
		sb.WriteString(m.styles.Dim.Render("Starting ad-hoc session..."))
	} else {
		sb.WriteString(m.styles.Dim.Render(fmt.Sprintf("Starting session '%s'...", m.session.Name)))
	}
	sb.WriteString("\n\n")

	// Show connection progress
	sb.WriteString("Connecting to agents:\n")
	for _, vs := range m.session.Vibespaces {
		sb.WriteString(fmt.Sprintf("  %s\n", vs.Name))
	}

	return sb.String()
}

// renderHeader renders the header section
func (m *Model) renderHeader() string {
	var title string
	if m.isAdHoc {
		title = "vibespace multi-session"
	} else {
		title = fmt.Sprintf("vibespace session: %s", m.session.Name)
	}

	agentCount := m.GetAgentCount()
	agentLabel := "agent"
	if agentCount != 1 {
		agentLabel = "agents"
	}

	statusStr := fmt.Sprintf("%d %s connected", agentCount, agentLabel)

	// Add scroll indicator if not at bottom (check viewport)
	if m.viewportReady && !m.viewport.AtBottom() {
		scrollPercent := m.viewport.ScrollPercent() * 100
		statusStr += fmt.Sprintf(" | %.0f%% ", scrollPercent) + m.styles.Dim.Render("↑↓ PgUp/PgDn End")
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.styles.Title.Render(title),
		"  ",
		m.styles.Dim.Render(statusStr),
	)

	return m.styles.Header.Width(m.width).Render(header)
}

// renderContent renders the main content area with agent outputs
func (m *Model) renderContent() string {
	switch m.layout {
	case LayoutFocus:
		if m.focusAgent != "" {
			return m.renderFocusedAgent()
		}
		fallthrough
	case LayoutChat:
		return m.renderChatView()
	default:
		return m.renderChatView()
	}
}

// renderChatView renders the unified chronological chat view
func (m *Model) renderChatView() string {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	if len(m.agentOrder) == 0 {
		return m.styles.Dim.Render("No agents connected. Waiting for connections...")
	}

	// Calculate available height for messages
	availableHeight := m.height - 8 // header + input + status + padding
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Get visible messages
	messages := m.history.GetVisible(availableHeight)

	var lines []string

	// Render each message
	for _, msg := range messages {
		line := m.renderMessage(msg)
		lines = append(lines, line)
	}

	// Add thinking indicators for agents that are thinking
	for _, state := range m.agentStates {
		if state.IsThinking {
			line := m.renderThinking(state)
			lines = append(lines, line)
		}
	}

	// If no messages yet, show placeholder
	if len(lines) == 0 {
		lines = append(lines, m.styles.Dim.Render("(no messages yet - type @all <message> to send to all agents)"))
	}

	return strings.Join(lines, "\n")
}

// renderMessage renders a single message in the chat view
func (m *Model) renderMessage(msg *Message) string {
	// Format timestamp
	ts := m.styles.Timestamp.Render(msg.Timestamp.Format("15:04"))

	var label, content string

	switch msg.Type {
	case MessageTypeUser:
		// User message: [You → target]
		label = UserLabelWithTarget(msg.Target)
		content = msg.Content

	case MessageTypeAssistant:
		// Agent message: [agent@vibespace]
		color := m.GetAgentColor(msg.Sender)
		label = AgentLabelStyle(color).Render(fmt.Sprintf("[%s]", msg.Sender))
		content = m.styleContent(msg.Content)

	case MessageTypeToolUse:
		// Tool use: [agent: Tool] input
		color := m.GetAgentColor(msg.Sender)
		agentPart := AgentLabelStyle(color).Render(msg.Sender)
		toolPart := m.styles.ToolLabel.Render(msg.ToolName)
		label = fmt.Sprintf("[%s: %s]", agentPart, toolPart)
		if msg.ToolInput != "" {
			content = m.styles.Dim.Render(msg.ToolInput)
		}

	case MessageTypeError:
		// Error message
		color := m.GetAgentColor(msg.Sender)
		label = AgentLabelStyle(color).Render(fmt.Sprintf("[%s]", msg.Sender))
		content = m.styles.Error.Render(msg.Content)

	case MessageTypeSystem:
		// System message
		label = m.styles.Dim.Render("[system]")
		content = m.styles.Dim.Render(msg.Content)

	default:
		label = m.styles.Dim.Render("[unknown]")
		content = msg.Content
	}

	// Combine timestamp, label, and content
	if content != "" {
		return fmt.Sprintf("%s %s %s", ts, label, content)
	}
	return fmt.Sprintf("%s %s", ts, label)
}

// renderThinking renders a thinking indicator for an agent
func (m *Model) renderThinking(state *AgentState) string {
	ts := m.styles.Timestamp.Render("     ") // Blank timestamp for alignment
	color := m.GetAgentColor(state.Address.String())
	label := AgentLabelStyle(color).Render(fmt.Sprintf("[%s]", state.Address.String()))
	spinner := state.ThinkingIndicatorText()
	indicator := m.styles.Thinking.Render(spinner)

	return fmt.Sprintf("%s %s %s", ts, label, indicator)
}

// styleContent applies styling to message content, including code blocks
func (m *Model) styleContent(content string) string {
	// Detect code blocks and style them
	// Simple approach: look for triple backticks
	codeBlockRegex := regexp.MustCompile("(?s)```([a-z]*)\n?(.*?)```")

	result := codeBlockRegex.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the code content
		parts := codeBlockRegex.FindStringSubmatch(match)
		if len(parts) >= 3 {
			code := parts[2]
			return m.styles.CodeBlock.Render(code)
		}
		return match
	})

	return result
}

// renderFocusedAgent renders a single agent in focus mode
func (m *Model) renderFocusedAgent() string {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	// Show messages only from the focused agent
	color := m.GetAgentColor(m.focusAgent)
	label := AgentLabelStyle(color).Render(fmt.Sprintf("[%s] (focused - /split to return)", m.focusAgent))

	// Filter messages for this agent
	availableHeight := m.height - 8
	allMessages := m.history.GetVisible(availableHeight * 2) // Get more to filter

	var lines []string
	lines = append(lines, label)

	count := 0
	for _, msg := range allMessages {
		if msg.Sender == m.focusAgent || msg.Type == MessageTypeUser {
			lines = append(lines, m.renderMessage(msg))
			count++
			if count >= availableHeight-1 {
				break
			}
		}
	}

	if count == 0 {
		lines = append(lines, m.styles.Dim.Render("(waiting for output...)"))
	}

	// Add thinking indicator if focused agent is thinking
	if state, ok := m.agentStates[m.focusAgent]; ok && state.IsThinking {
		lines = append(lines, m.renderThinking(state))
	}

	return strings.Join(lines, "\n")
}

// renderInput renders the input area
func (m *Model) renderInput() string {
	prompt := m.styles.Prompt.Render("> ")

	// Render input with syntax coloring for commands and mentions
	input := m.renderColoredInput()

	// Add contextual hint below input
	var hint string
	inputVal := m.input.Value()

	if strings.HasPrefix(inputVal, "/") {
		// Command hint
		parts := strings.SplitN(inputVal[1:], " ", 2)
		cmd := parts[0]
		if cmd != "" {
			hint = m.styles.Dim.Render("command")
		}
	} else if strings.HasPrefix(inputVal, "@") {
		// Parse target from input
		parts := strings.SplitN(inputVal[1:], " ", 2)
		if len(parts) >= 1 && parts[0] != "" {
			hint = m.styles.Dim.Render(fmt.Sprintf("Sending to %s", parts[0]))
		}
	} else if inputVal != "" {
		hint = m.styles.Dim.Render("Sending to all agents (use @agent to target specific)")
	}

	inputLine := lipgloss.JoinHorizontal(lipgloss.Left, prompt, input)

	if hint != "" {
		return m.styles.InputArea.Width(m.width).Render(
			lipgloss.JoinVertical(lipgloss.Left, inputLine, hint),
		)
	}

	return m.styles.InputArea.Width(m.width).Render(inputLine)
}

// renderColoredInput renders the input text with syntax coloring
func (m *Model) renderColoredInput() string {
	val := m.input.Value()
	pos := m.input.Position()

	// Style definitions
	cmdStyle := lipgloss.NewStyle().Foreground(warningColor).Bold(true)
	mentionStyle := lipgloss.NewStyle().Foreground(successColor).Bold(true)
	cursorStyle := lipgloss.NewStyle().Reverse(true)
	suggestionStyle := lipgloss.NewStyle().Foreground(dimColor)

	// Get suggestion if any
	suggestion := ""
	if suggestions := m.input.AvailableSuggestions(); len(suggestions) > 0 {
		// Show first suggestion as ghost text
		if len(suggestions[0]) > len(val) {
			suggestion = suggestions[0][len(val):]
		}
	}

	// If empty, just show cursor and suggestion
	if val == "" {
		cursor := cursorStyle.Render(" ")
		if suggestion != "" {
			return cursor + suggestionStyle.Render(suggestion)
		}
		return cursor
	}

	// Build colored output
	var result strings.Builder

	if strings.HasPrefix(val, "/") {
		// Color the command part
		parts := strings.SplitN(val, " ", 2)
		cmd := parts[0]

		// Render command with color, handling cursor position
		if pos <= len(cmd) {
			// Cursor is in the command
			before := cmd[:pos]
			after := cmd[pos:]
			if pos < len(cmd) {
				result.WriteString(cmdStyle.Render(before))
				result.WriteString(cursorStyle.Render(string(cmd[pos])))
				result.WriteString(cmdStyle.Render(after[1:]))
			} else {
				result.WriteString(cmdStyle.Render(before))
				if len(parts) > 1 {
					result.WriteString(cursorStyle.Render(" "))
					result.WriteString(parts[1][1:])
				} else {
					result.WriteString(cursorStyle.Render(" "))
				}
			}
		} else {
			// Cursor is after the command
			result.WriteString(cmdStyle.Render(cmd))
			if len(parts) > 1 {
				rest := " " + parts[1]
				restPos := pos - len(cmd)
				if restPos < len(rest) {
					result.WriteString(rest[:restPos])
					result.WriteString(cursorStyle.Render(string(rest[restPos])))
					result.WriteString(rest[restPos+1:])
				} else {
					result.WriteString(rest)
					result.WriteString(cursorStyle.Render(" "))
				}
			} else {
				result.WriteString(cursorStyle.Render(" "))
			}
		}
	} else if strings.HasPrefix(val, "@") {
		// Color the mention part
		spaceIdx := strings.Index(val, " ")
		var mention, rest string
		if spaceIdx == -1 {
			mention = val
			rest = ""
		} else {
			mention = val[:spaceIdx]
			rest = val[spaceIdx:]
		}

		if pos <= len(mention) {
			// Cursor is in the mention
			if pos < len(mention) {
				result.WriteString(mentionStyle.Render(mention[:pos]))
				result.WriteString(cursorStyle.Render(string(mention[pos])))
				result.WriteString(mentionStyle.Render(mention[pos+1:]))
			} else {
				result.WriteString(mentionStyle.Render(mention))
				if rest != "" {
					result.WriteString(cursorStyle.Render(string(rest[0])))
					result.WriteString(rest[1:])
				} else {
					result.WriteString(cursorStyle.Render(" "))
				}
			}
			result.WriteString(rest)
		} else {
			result.WriteString(mentionStyle.Render(mention))
			restPos := pos - len(mention)
			if restPos < len(rest) {
				result.WriteString(rest[:restPos])
				result.WriteString(cursorStyle.Render(string(rest[restPos])))
				result.WriteString(rest[restPos+1:])
			} else {
				result.WriteString(rest)
				result.WriteString(cursorStyle.Render(" "))
			}
		}
	} else {
		// Regular text
		if pos < len(val) {
			result.WriteString(val[:pos])
			result.WriteString(cursorStyle.Render(string(val[pos])))
			result.WriteString(val[pos+1:])
		} else {
			result.WriteString(val)
			result.WriteString(cursorStyle.Render(" "))
		}
	}

	// Add ghost suggestion
	if suggestion != "" {
		result.WriteString(suggestionStyle.Render(suggestion))
	}

	return result.String()
}

// renderHelp renders the help text in a compact multi-line format for the status area
func (m *Model) renderHelp() string {
	// Use different colors for different categories
	msgStyle := lipgloss.NewStyle().Foreground(successColor).Bold(true)
	cmdStyle := lipgloss.NewStyle().Foreground(warningColor).Bold(true)
	sessionStyle := lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
	descStyle := m.styles.HelpDesc

	// Line 1: Messaging
	line1 := fmt.Sprintf("%s %s  %s %s",
		msgStyle.Render("@agent <msg>"),
		descStyle.Render("send to agent"),
		msgStyle.Render("@all <msg>"),
		descStyle.Render("broadcast to all"),
	)

	// Line 2: Basic commands
	line2 := fmt.Sprintf("%s %s  %s %s  %s %s",
		cmdStyle.Render("/list"),
		descStyle.Render("show agents"),
		cmdStyle.Render("/clear"),
		descStyle.Render("clear history"),
		cmdStyle.Render("/quit"),
		descStyle.Render("exit"),
	)

	// Line 3: Add/remove agents
	line3 := fmt.Sprintf("%s %s  %s %s",
		cmdStyle.Render("/add <vibespace>"),
		descStyle.Render("add agents (or agent@vibespace)"),
		cmdStyle.Render("/remove <agent>"),
		descStyle.Render("remove agent"),
	)

	// Line 4: Focus mode
	line4 := fmt.Sprintf("%s %s",
		cmdStyle.Render("/focus <agent>"),
		descStyle.Render("open interactive Claude (exit with /exit to return)"),
	)

	// Line 5: Session management
	line5 := fmt.Sprintf("%s %s",
		sessionStyle.Render("/session @agent"),
		descStyle.Render("new | list | resume <id> | info"),
	)

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", line1, line2, line3, line4, line5)
}

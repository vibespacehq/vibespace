package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// errorWithHint appends a hint to an error message if one is available.
func errorWithHint(err error) string {
	msg := err.Error()
	if hint := vserrors.GetErrorHint(err); hint != "" {
		msg += "\n  Hint: " + hint
	}
	return msg
}

// View implements tea.Model
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return m.styles.Error.Render(fmt.Sprintf("Error: %s", errorWithHint(m.err)))
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

	// If permission prompt is showing, render it inline above the status bar
	if m.permissionPrompt != nil {
		permBar := m.renderPermissionBar()
		return fmt.Sprintf("%s\n%s\n\n%s\n%s", header, m.viewport.View(), input, permBar)
	}

	return fmt.Sprintf("%s\n%s\n\n%s\n%s", header, m.viewport.View(), input, status)
}

// renderLoading renders the loading state
func (m *Model) renderLoading() string {
	var sb strings.Builder

	sb.WriteString(m.styles.Title.Render("vibespace multi-session"))
	sb.WriteString("\n\n")

	sb.WriteString(m.styles.Dim.Render(fmt.Sprintf("Starting session '%s'...", m.sessionName)))
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
	title := fmt.Sprintf("vibespace session: %s", m.sessionName)

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
	cmdStyle := lipgloss.NewStyle().Foreground(ui.Orange).Bold(true)
	mentionStyle := lipgloss.NewStyle().Foreground(ui.Teal).Bold(true)
	cursorStyle := lipgloss.NewStyle().Reverse(true)
	suggestionStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)

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

// renderPermissionBar renders the permission prompt as a compact inline bar
func (m *Model) renderPermissionBar() string {
	req := m.permissionPrompt.Request()

	labelStyle := lipgloss.NewStyle().Foreground(ui.Orange).Bold(true)
	agentStyle := lipgloss.NewStyle().Foreground(ui.Pink)
	toolStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Bold(true)
	inputStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	allowStyle := lipgloss.NewStyle().Foreground(ui.Teal).Bold(true)
	denyStyle := lipgloss.NewStyle().Foreground(ui.ColorError).Bold(true)

	// Format tool input
	toolInput := m.permissionPrompt.formatToolInput()
	if len(toolInput) > 50 {
		toolInput = toolInput[:47] + "..."
	}

	// Build the bar: ⚠ Permission: claude-1@test wants Bash "ls -la"  [a]llow [d]eny
	bar := fmt.Sprintf(" %s %s %s %s",
		labelStyle.Render("Permission:"),
		agentStyle.Render(req.AgentKey),
		toolStyle.Render(req.ToolName),
		inputStyle.Render(toolInput),
	)

	queueInfo := ""
	if len(m.pendingPerms) > 1 {
		queueInfo = lipgloss.NewStyle().Foreground(ui.ColorDim).Render(
			fmt.Sprintf(" (+%d)", len(m.pendingPerms)-1))
	}

	keys := fmt.Sprintf("  %s  %s%s",
		allowStyle.Render("[a]llow"),
		denyStyle.Render("[d]eny"),
		queueInfo,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("#1a1a2e")).
		Render(bar + keys)
}

// renderHelp renders the help text in a compact multi-line format for the status area
func (m *Model) renderHelp() string {
	// Use different colors for different categories
	msgStyle := lipgloss.NewStyle().Foreground(ui.Teal).Bold(true)
	cmdStyle := lipgloss.NewStyle().Foreground(ui.Orange).Bold(true)
	sessionStyle := lipgloss.NewStyle().Foreground(ui.Pink).Bold(true)
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
		descStyle.Render("interactive agent session (Ctrl+B D to detach)"),
	)

	// Line 5: Session management
	line5 := fmt.Sprintf("%s %s",
		sessionStyle.Render("/session @agent"),
		descStyle.Render("new | list | resume <id> | info"),
	)

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s", line1, line2, line3, line4, line5)
}

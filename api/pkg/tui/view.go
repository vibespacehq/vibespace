package tui

import (
	"fmt"
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

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Main content area (agents output)
	sections = append(sections, m.renderContent())

	// Input area
	sections = append(sections, m.renderInput())

	// Status bar
	if m.statusMsg != "" {
		sections = append(sections, m.styles.StatusBar.Render(m.statusMsg))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
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
	if m.layout == LayoutFocus && m.focusAgent != "" {
		return m.renderFocusedAgent()
	}
	return m.renderSplitView()
}

// renderSplitView renders all agents in split view
func (m *Model) renderSplitView() string {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	if len(m.agentOrder) == 0 {
		return m.styles.Dim.Render("No agents connected. Waiting for connections...")
	}

	var sections []string

	// Calculate how many lines per agent
	availableHeight := m.height - 8 // header + input + status
	linesPerAgent := availableHeight / len(m.agentOrder)
	if linesPerAgent < 3 {
		linesPerAgent = 3
	}

	for _, key := range m.agentOrder {
		conn := m.agents[key]
		output := m.outputs[key]
		color := m.GetAgentColor(key)

		section := m.renderAgentSection(conn.address.String(), output, color, linesPerAgent)
		sections = append(sections, section)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderAgentSection renders a single agent's output section
func (m *Model) renderAgentSection(addr string, output *OutputBuffer, color lipgloss.Color, maxLines int) string {
	label := AgentLabelStyle(color).Render(fmt.Sprintf("[%s]", addr))

	// Get recent output lines
	lines := output.GetLines(maxLines - 1) // -1 for label line

	var outputLines []string
	for _, line := range lines {
		outputLines = append(outputLines, line.Text)
	}

	if len(outputLines) == 0 {
		outputLines = append(outputLines, m.styles.Dim.Render("(waiting for output...)"))
	}

	content := strings.Join(outputLines, "\n")

	return m.styles.AgentSection.Render(
		lipgloss.JoinVertical(lipgloss.Left, label, content),
	)
}

// renderFocusedAgent renders a single agent in focus mode
func (m *Model) renderFocusedAgent() string {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	output, ok := m.outputs[m.focusAgent]
	if !ok {
		return m.styles.Error.Render(fmt.Sprintf("Agent %s not found", m.focusAgent))
	}

	color := m.GetAgentColor(m.focusAgent)
	label := AgentLabelStyle(color).Render(fmt.Sprintf("[%s] (focused - /split to return)", m.focusAgent))

	// Use all available height
	availableHeight := m.height - 8
	lines := output.GetLines(availableHeight)

	var outputLines []string
	for _, line := range lines {
		outputLines = append(outputLines, line.Text)
	}

	if len(outputLines) == 0 {
		outputLines = append(outputLines, m.styles.Dim.Render("(waiting for output...)"))
	}

	content := strings.Join(outputLines, "\n")

	return lipgloss.JoinVertical(lipgloss.Left, label, content)
}

// renderInput renders the input area
func (m *Model) renderInput() string {
	prompt := m.styles.Prompt.Render("> ")
	input := m.input.View()

	return m.styles.InputArea.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, prompt, input),
	)
}

// renderHelp renders the help text
func (m *Model) renderHelp() string {
	help := []struct {
		key  string
		desc string
	}{
		{"@agent msg", "send to agent"},
		{"@all msg", "broadcast"},
		{"/help", "show help"},
		{"/list", "list agents"},
		{"/focus agent", "focus view"},
		{"/split", "split view"},
		{"/quit", "exit"},
	}

	var parts []string
	for _, h := range help {
		parts = append(parts, fmt.Sprintf("%s %s",
			m.styles.HelpKey.Render(h.key),
			m.styles.HelpDesc.Render(h.desc),
		))
	}

	return strings.Join(parts, "  ")
}

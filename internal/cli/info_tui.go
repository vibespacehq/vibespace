package cli

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yagizdagabak/vibespace/pkg/agent"
	"github.com/yagizdagabak/vibespace/pkg/ui"
	"github.com/yagizdagabak/vibespace/pkg/vibespace"
)

// Tab indices
const (
	tabOverview  = 0
	tabAgents    = 1
	tabForwards  = 2
	numTabs      = 3
)

var tabNames = []string{"Overview", "Agents", "Forwards"}

// infoModel is the bubbletea model for the interactive info TUI.
type infoModel struct {
	info         InfoOutput
	agents       []vibespace.AgentInfo
	agentConfigs map[string]*agent.Config
	forwards     []AgentForwardInfo

	activeTab int
	width     int
	height    int
	ready     bool
}

func newInfoModel(info InfoOutput, agents []vibespace.AgentInfo, agentConfigs map[string]*agent.Config, forwards []AgentForwardInfo) infoModel {
	return infoModel{
		info:         info,
		agents:       agents,
		agentConfigs: agentConfigs,
		forwards:     forwards,
		activeTab:    tabOverview,
	}
}

func (m infoModel) Init() tea.Cmd {
	return nil
}

func (m infoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % numTabs
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + numTabs) % numTabs
		case "1":
			m.activeTab = tabOverview
		case "2":
			m.activeTab = tabAgents
		case "3":
			m.activeTab = tabForwards
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}
	return m, nil
}

func (m infoModel) View() string {
	if !m.ready {
		return ""
	}

	contentWidth := m.width
	if contentWidth > 80 {
		contentWidth = 80
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	var sb strings.Builder

	// Header: vibespace name + status
	sb.WriteString(m.renderHeader(contentWidth))
	sb.WriteString("\n")

	// Tab bar
	sb.WriteString(m.renderTabs(contentWidth))
	sb.WriteString("\n\n")

	// Tab content
	switch m.activeTab {
	case tabOverview:
		sb.WriteString(m.renderOverview(contentWidth))
	case tabAgents:
		sb.WriteString(m.renderAgents(contentWidth))
	case tabForwards:
		sb.WriteString(m.renderForwards(contentWidth))
	}

	// Footer hint
	sb.WriteString("\n")
	sb.WriteString(m.renderFooter(contentWidth))

	return sb.String()
}

// --- Header ---

func (m infoModel) renderHeader(width int) string {
	nameStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Teal)

	statusDot, statusColor := statusIndicator(m.info.Status)
	statusStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true)

	name := nameStyle.Render(m.info.Name)
	status := statusStyle.Render(statusDot + " " + m.info.Status)

	// Space between name and status
	gap := width - lipgloss.Width(name) - lipgloss.Width(status)
	if gap < 2 {
		gap = 2
	}

	return name + strings.Repeat(" ", gap) + status
}

// --- Tab Bar ---

func (m infoModel) renderTabs(width int) string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Teal).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ui.Teal).
		Padding(0, 2)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(0, 2)

	var tabs []string
	for i, name := range tabNames {
		if i == m.activeTab {
			tabs = append(tabs, activeStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	// Underline spanning the full width
	lineWidth := width - lipgloss.Width(row)
	if lineWidth < 0 {
		lineWidth = 0
	}
	line := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", lineWidth))

	return lipgloss.JoinHorizontal(lipgloss.Bottom, row, line)
}

// --- Overview Tab ---

func (m infoModel) renderOverview(width int) string {
	var sb strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Width(14)
	valueStyle := lipgloss.NewStyle().
		Foreground(ui.ColorWhite)

	// Main info
	sb.WriteString(field(labelStyle, valueStyle, "ID", m.info.ID))
	sb.WriteString(field(labelStyle, valueStyle, "PVC", m.info.PVC))
	sb.WriteString(field(labelStyle, valueStyle, "Created", m.info.CreatedAt))
	sb.WriteString("\n")

	// Resources box
	boxWidth := width - 4
	if boxWidth > 50 {
		boxWidth = 50
	}

	resContent := strings.Join([]string{
		field(labelStyle, valueStyle, "CPU", m.info.CPU),
		field(labelStyle, valueStyle, "Memory", m.info.Memory),
		field(labelStyle, valueStyle, "Storage", m.info.Storage),
	}, "")
	resContent = strings.TrimRight(resContent, "\n")

	resBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorMuted).
		Padding(0, 1).
		Width(boxWidth).
		Render(resContent)

	resHeader := lipgloss.NewStyle().
		Foreground(ui.Orange).
		Bold(true).
		Render(" Resources ")

	sb.WriteString(overlayTitle(resBox, resHeader))
	sb.WriteString("\n")

	// Mounts box
	if len(m.info.Mounts) > 0 {
		sb.WriteString("\n")
		var mountLines []string
		arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
		hostStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		containerStyle := lipgloss.NewStyle().Foreground(ui.Teal)
		modeStyle := lipgloss.NewStyle().Foreground(ui.Orange)

		for _, mt := range m.info.Mounts {
			mode := "rw"
			if mt.ReadOnly {
				mode = "ro"
			}
			line := fmt.Sprintf("%s %s %s %s",
				hostStyle.Render(shortenPath(mt.HostPath)),
				arrowStyle.Render("→"),
				containerStyle.Render(mt.ContainerPath),
				modeStyle.Render("("+mode+")"),
			)
			mountLines = append(mountLines, line)
		}

		mountContent := strings.Join(mountLines, "\n")
		mountBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorMuted).
			Padding(0, 1).
			Width(boxWidth).
			Render(mountContent)

		mountHeader := lipgloss.NewStyle().
			Foreground(ui.Orange).
			Bold(true).
			Render(" Mounts ")

		sb.WriteString(overlayTitle(mountBox, mountHeader))
		sb.WriteString("\n")
	}

	return sb.String()
}

// --- Agents Tab ---

func (m infoModel) renderAgents(width int) string {
	if len(m.agents) == 0 {
		dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
		return dim.Render("  No agents configured")
	}

	boxWidth := width - 4
	if boxWidth > 70 {
		boxWidth = 70
	}

	var sb strings.Builder

	for i, a := range m.agents {
		config := m.agentConfigs[a.AgentName]
		if config == nil {
			config = &agent.Config{}
		}

		content := m.renderAgentCard(a, config)

		cardBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorMuted).
			Padding(0, 1).
			Width(boxWidth).
			Render(content)

		// Agent name + status as title
		dot, color := statusIndicator(a.Status)
		agentTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Teal).
			Render(" ⬡ " + a.AgentName + " ")
		statusBadge := lipgloss.NewStyle().
			Foreground(color).
			Render(dot)

		titleStr := agentTitle + statusBadge + " "
		sb.WriteString(overlayTitle(cardBox, titleStr))

		if i < len(m.agents)-1 {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m infoModel) renderAgentCard(a vibespace.AgentInfo, config *agent.Config) string {
	isCodex := a.AgentType == agent.TypeCodex

	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorDim).Width(20)
	enabledStyle := lipgloss.NewStyle().Foreground(ui.ColorSuccess)
	disabledStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	normalStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

	row := func(icon, label, value string, style lipgloss.Style) string {
		iconStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
		return fmt.Sprintf("%s %s%s\n",
			iconStyle.Render(icon),
			labelStyle.Render(label),
			style.Render(value),
		)
	}

	var sb strings.Builder

	// permissions
	if isCodex {
		sb.WriteString(row("◉", "permissions", "always (yolo)", enabledStyle))
	} else if config.SkipPermissions {
		sb.WriteString(row("◉", "permissions", "skip", enabledStyle))
	} else {
		sb.WriteString(row("○", "permissions", "default", disabledStyle))
	}

	// credentials
	if config.ShareCredentials {
		sb.WriteString(row("◉", "credentials", "shared", enabledStyle))
	} else {
		sb.WriteString(row("○", "credentials", "isolated", disabledStyle))
	}

	// model
	if config.Model != "" {
		sb.WriteString(row("◈", "model", config.Model, normalStyle))
	} else {
		sb.WriteString(row("◈", "model", "default", disabledStyle))
	}

	// max_turns
	if config.MaxTurns > 0 {
		sb.WriteString(row("↻", "max_turns", strconv.Itoa(config.MaxTurns), normalStyle))
	} else {
		sb.WriteString(row("↻", "max_turns", "unlimited", disabledStyle))
	}

	// tools
	if isCodex {
		sb.WriteString(row("⚙", "tools", "all (no restrictions)", enabledStyle))
	} else if len(config.AllowedTools) > 0 {
		tools := strings.Join(config.AllowedTools, ", ")
		if len(tools) > 40 {
			tools = tools[:37] + "..."
		}
		sb.WriteString(row("⚙", "tools", tools, normalStyle))
	} else if config.SkipPermissions {
		sb.WriteString(row("⚙", "tools", "all", enabledStyle))
	} else {
		tools := strings.Join(agent.DefaultAllowedTools(), ", ")
		defaultLabel := lipgloss.NewStyle().Foreground(ui.ColorDim).Render(" (default)")
		sb.WriteString(row("⚙", "tools", tools+defaultLabel, normalStyle))
	}

	// disallowed
	if len(config.DisallowedTools) > 0 {
		redStyle := lipgloss.NewStyle().Foreground(ui.ColorError)
		sb.WriteString(row("⊘", "disallowed", strings.Join(config.DisallowedTools, ", "), redStyle))
	}

	// system_prompt
	if config.SystemPrompt != "" {
		prompt := config.SystemPrompt
		if len(prompt) > 36 {
			prompt = prompt[:33] + "..."
		}
		sb.WriteString(row("✎", "system_prompt", prompt, normalStyle))
	}

	// reasoning_effort
	if config.ReasoningEffort != "" {
		sb.WriteString(row("◆", "reasoning", config.ReasoningEffort, normalStyle))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// --- Forwards Tab ---

func (m infoModel) renderForwards(width int) string {
	// Collect all forwards
	var hasForwards bool
	for _, a := range m.forwards {
		if len(a.Forwards) > 0 {
			hasForwards = true
			break
		}
	}

	if !hasForwards {
		dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
		return dim.Render("  No active port forwards\n\n") +
			dim.Render("  Use: vibespace " + m.info.Name + " forward add PORT")
	}

	// Build table rows
	var rows [][]string
	for _, a := range m.forwards {
		for _, fwd := range a.Forwards {
			fwdType := fwd.Type
			if fwdType == "" {
				fwdType = "manual"
			}
			dot, _ := fwdStatusIndicator(fwd.Status)
			status := dot + " " + fwd.Status
			if fwd.Error != "" {
				status += " (" + fwd.Error + ")"
			}
			rows = append(rows, []string{
				a.Name,
				strconv.Itoa(fwd.LocalPort),
				strconv.Itoa(fwd.RemotePort),
				fwdType,
				status,
			})
		}
	}

	headers := []string{"Agent", "Local", "Remote", "Type", "Status"}
	tableStr := ui.NewTable(headers, rows, false)

	return tableStr
}

// --- Footer ---

func (m infoModel) renderFooter(width int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	keyStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)

	hint := fmt.Sprintf("%s navigate   %s quit",
		keyStyle.Render("←/→/tab"),
		keyStyle.Render("q"),
	)

	return dimStyle.Render(strings.Repeat("─", width)) + "\n" + hint
}

// --- Helpers ---

func field(labelStyle, valueStyle lipgloss.Style, label, value string) string {
	return fmt.Sprintf("  %s %s\n", labelStyle.Render(label), valueStyle.Render(value))
}

func statusIndicator(status string) (string, lipgloss.Color) {
	switch status {
	case "running":
		return "●", ui.ColorSuccess
	case "stopped":
		return "○", ui.ColorWarning
	case "creating":
		return "◐", ui.Teal
	default:
		return "●", ui.ColorError
	}
}

func fwdStatusIndicator(status string) (string, lipgloss.Color) {
	switch status {
	case "active":
		return "●", ui.ColorSuccess
	case "error", "failed":
		return "●", ui.ColorError
	default:
		return "○", ui.ColorDim
	}
}

// overlayTitle renders a titled box by placing the title above the box content.
func overlayTitle(box, title string) string {
	return title + "\n" + box
}

// runInfoTUI launches the interactive tabbed info TUI.
func runInfoTUI(info InfoOutput, agents []vibespace.AgentInfo, agentConfigs map[string]*agent.Config, forwards []AgentForwardInfo) error {
	m := newInfoModel(info, agents, agentConfigs, forwards)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

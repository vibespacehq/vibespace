package tui

import (
	"context"
	"fmt"
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
	vibespacesModeList      vibespacesMode = iota // table view
	vibespacesModeAgentView                       // full-screen agent detail
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
	if t.mode == vibespacesModeAgentView {
		return []key.Binding{
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
			key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate agents")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "view")),
	}
}

func (t *VibespacesTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *VibespacesTab) Init() tea.Cmd {
	return t.loadVibespaces()
}

func (t *VibespacesTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TabActivateMsg:
		if t.mode == vibespacesModeAgentView && t.selectedVS != nil {
			return t, tea.Batch(
				t.loadAgentsForView(t.selectedVS.ID, t.selectedVS.Name),
				t.loadAgentConfigs(t.selectedVS.ID, t.selectedVS.Name),
				t.loadForwards(t.selectedVS.ID, t.selectedVS.Name),
			)
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
		return t, tea.Batch(t.loadAgentInfo(), t.loadLogsForSelected())

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

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	return t, nil
}

func (t *VibespacesTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch t.mode {
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
		}
		return t, nil

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

	if t.mode == vibespacesModeAgentView {
		return t.viewAgentView()
	}

	if len(t.vibespaces) == 0 {
		return t.viewEmpty()
	}

	topBlock := t.viewTable()

	bottom := t.viewDetail()

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
	nameText := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite).Render(vs.Name)
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

	// --- Bottom block: vibespace info + selected agent config ---
	bottom := t.viewAgentDetail()

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

	if cfg, ok := t.agentConfigs[ag.AgentName]; ok && cfg != nil {
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("skip_permissions"),
			dimStyle.Render(fmt.Sprintf("%v", cfg.SkipPermissions))))

		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("share_credentials"),
			dimStyle.Render(fmt.Sprintf("%v", cfg.ShareCredentials))))

		allowedStr := "all"
		if len(cfg.AllowedTools) > 0 {
			allowedStr = strings.Join(cfg.AllowedTools, ", ")
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("allowed_tools"),
			dimStyle.Render(allowedStr)))

		disallowedStr := "-"
		if len(cfg.DisallowedTools) > 0 {
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
		var fwdLines []string
		for _, fwd := range agentForwards {
			line := fmt.Sprintf("%s  %s  %s  %s",
				dimStyle.Render(fmt.Sprintf(":%d", fwd.LocalPort)),
				dimStyle.Render(fmt.Sprintf("→ :%d", fwd.RemotePort)),
				dimStyle.Render(fwd.Type),
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

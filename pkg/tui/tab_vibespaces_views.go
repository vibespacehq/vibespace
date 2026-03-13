package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

func (t *VibespacesTab) viewEmpty() string {
	// This is a fallback — the App renders the full-screen welcome cover
	// when no vibespaces exist, bypassing this method entirely.
	msg := lipgloss.NewStyle().
		Foreground(ui.ColorDim).
		Padding(2, 0).
		Render("No vibespaces found. Press n to create one.")
	return lipgloss.Place(t.width, t.height, lipgloss.Center, lipgloss.Center, msg)
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
			rows[i] = renderGradientRow(cells, getBrandGradient())
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

func (t *VibespacesTab) viewAgentView() string {
	if t.selectedVS == nil {
		return ""
	}

	vs := t.selectedVS
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	var topParts []string

	// Header: ← name                                          status
	backArrow := renderGradientText("← ", getBrandGradient())
	nameText := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).Render(vs.Name)
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

	var bottom string
	switch t.mode {
	case vibespacesModeAddAgent:
		bottom = t.viewAddAgentForm()
	case vibespacesModeDeleteAgentConfirm:
		bottom = t.viewDeleteAgentConfirm()
	case vibespacesModeEditConfig:
		bottom = t.viewEditConfigForm()
	case vibespacesModeForwardManager:
		bottom = t.viewForwardManager()
	default:
		bottom = t.viewAgentDetail()
	}

	topH := lipgloss.Height(topBlock)
	bottomH := lipgloss.Height(bottom)
	gap := t.height - topH - bottomH
	if gap < 1 {
		gap = 1
	}

	return topBlock + strings.Repeat("\n", gap) + bottom
}

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

	var cfgLines []string
	cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
		labelStyle.Render("type"),
		dimStyle.Render(string(ag.AgentType))))

	isCodex := ag.AgentType == agent.TypeCodex

	if cfg, ok := t.agentConfigs[ag.AgentName]; ok && cfg != nil {
		// skip_permissions: codex → "always", else true/false
		skipStr := fmt.Sprintf("%v", cfg.SkipPermissions)
		if isCodex {
			skipStr = "always"
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("skip_permissions"),
			dimStyle.Render(skipStr)))

		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("share_credentials"),
			dimStyle.Render(fmt.Sprintf("%v", cfg.ShareCredentials))))

		// allowed_tools: codex → "all", skip=true → "all", custom → join, else → defaults + (default)
		var allowedStr string
		if isCodex || cfg.SkipPermissions {
			allowedStr = "all"
		} else if len(cfg.AllowedTools) > 0 {
			allowedStr = strings.Join(cfg.AllowedTools, ", ")
		} else {
			// Show defaults minus any disallowed tools
			defaults := agent.DefaultAllowedTools()
			if len(cfg.DisallowedTools) > 0 {
				disallowed := make(map[string]bool, len(cfg.DisallowedTools))
				for _, t := range cfg.DisallowedTools {
					disallowed[t] = true
				}
				var filtered []string
				for _, t := range defaults {
					if !disallowed[t] {
						filtered = append(filtered, t)
					}
				}
				if len(filtered) == 0 {
					allowedStr = "none (all defaults disallowed)"
				} else {
					allowedStr = strings.Join(filtered, ", ") + " (default)"
				}
			} else {
				allowedStr = strings.Join(defaults, ", ") + " (default)"
			}
		}
		cfgLines = append(cfgLines, fmt.Sprintf("%s  %s",
			labelStyle.Render("allowed_tools"),
			dimStyle.Render(allowedStr)))

		disallowedStr := "-"
		if !isCodex {
			if len(cfg.DisallowedTools) > 0 {
				disallowedStr = strings.Join(cfg.DisallowedTools, ", ")
			} else if len(cfg.AllowedTools) > 0 {
				excluded := excludedTools(ag.AgentType, cfg.AllowedTools)
				if len(excluded) > 0 {
					disallowedStr = strings.Join(excluded, ", ") + " (excluded)"
				}
			}
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

	var fwdBlock string
	agentForwards := t.forwardsForAgent(ag.AgentName)
	if len(agentForwards) > 0 {
		fwdHeader := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted).
			Render("Forwards")
		// Compute column widths for alignment
		var maxRemote, maxLocal, maxType int
		for _, fwd := range agentForwards {
			if r := len(fmt.Sprintf(":%d", fwd.RemotePort)); r > maxRemote {
				maxRemote = r
			}
			if l := len(fmt.Sprintf(":%d", fwd.LocalPort)); l > maxLocal {
				maxLocal = l
			}
			if len(fwd.Type) > maxType {
				maxType = len(fwd.Type)
			}
		}
		var fwdLines []string
		for _, fwd := range agentForwards {
			remote := fmt.Sprintf(":%d", fwd.RemotePort)
			local := fmt.Sprintf(":%d", fwd.LocalPort)
			line := fmt.Sprintf("%s  %s  %s  %s",
				dimStyle.Render(fmt.Sprintf("%-*s", maxRemote, remote)),
				dimStyle.Render(fmt.Sprintf("→ %-*s", maxLocal, local)),
				dimStyle.Render(fmt.Sprintf("%-*s", maxType, fwd.Type)),
				dimStyle.Render(fwd.Status))
			if fwd.DNSName != "" {
				line += "  " + dimStyle.Render(fmt.Sprintf("%s.vibespace.internal:%d", fwd.DNSName, fwd.LocalPort))
			}
			fwdLines = append(fwdLines, line)
		}
		fwdBlock = "\n\n" + fwdHeader + "\n" + mutedLine + "\n" +
			strings.Join(fwdLines, "\n") + "\n" + mutedLine
	}

	var statusLine string
	if t.agentStatusMsg != "" {
		statusLine = "\n\n" + lipgloss.NewStyle().Italic(true).Foreground(ui.Orange).
			Render(t.agentStatusMsg)
	}

	fullBlock := detailsBlock + "\n\n" + cfgBlock + fwdBlock + statusLine
	return lipgloss.NewStyle().Padding(0, 2).Render(fullBlock)
}

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
			rows[i] = renderGradientRow(cells, getBrandGradient())
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

func (t *VibespacesTab) forwardsForAgent(agentName string) []daemon.ForwardInfo {
	for _, as := range t.forwards {
		if as.Name == agentName {
			return as.Forwards
		}
	}
	return nil
}

func (t *VibespacesTab) viewSessionList() string {
	if t.selectedVS == nil {
		return ""
	}

	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	// Header: ← agent-name interactive sessions
	backArrow := renderGradientText("← ", getBrandGradient())
	nameText := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorText).
		Render(t.sessionAgent + " interactive sessions")
	header := backArrow + nameText

	var topParts []string
	topParts = append(topParts, header, mutedLine)

	if t.err != "" {
		topParts = append(topParts, lipgloss.NewStyle().Foreground(ui.ColorError).
			Render("Error: "+t.err))
	} else if t.sessions == nil {
		topParts = append(topParts, dimStyle.Render("Loading sessions..."))
	} else if len(t.sessions) == 0 {
		topParts = append(topParts, dimStyle.Render("No sessions found."))
	} else {
		topParts = append(topParts, t.viewSessionTable())
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(
		strings.Join(topParts, "\n\n"))
}

func (t *VibespacesTab) viewSessionTable() string {
	rows := make([][]string, len(t.sessions))
	for i, s := range t.sessions {
		idShort := s.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}

		title := s.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		title = strings.ReplaceAll(title, "\n", " ")

		ago := formatSessionAge(s.LastTime)
		prompts := fmt.Sprintf("%d", s.Prompts)

		if i == t.sessionCursor {
			cells := []string{"› " + idShort, ago, prompts, title}
			rows[i] = renderGradientRow(cells, getBrandGradient())
		} else {
			rows[i] = []string{"  " + idShort, ago, prompts, title}
		}
	}

	sel := t.sessionCursor
	tbl := table.New().
		Headers("ID", "Last Active", "Turns", "First Prompt").
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

	noun := "sessions"
	if len(t.sessions) == 1 {
		noun = "session"
	}
	countText := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(fmt.Sprintf("(%d %s)", len(t.sessions), noun))
	count := lipgloss.NewStyle().Width(t.width - 4).Align(lipgloss.Right).
		PaddingTop(1).Render(countText)

	return tbl.Render() + "\n" + count
}

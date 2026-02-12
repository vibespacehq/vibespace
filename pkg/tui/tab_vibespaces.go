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
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// vibespacesLoadedMsg delivers vibespace data from the service.
type vibespacesLoadedMsg struct {
	vibespaces []*model.Vibespace
	err        error
}

// agentCountsLoadedMsg delivers agent counts for all vibespaces.
type agentCountsLoadedMsg struct {
	counts map[string]int // vibespace ID → agent count
}

// VibespacesTab displays the vibespace list with inline expansion.
type VibespacesTab struct {
	shared      *SharedState
	vibespaces  []*model.Vibespace
	selected    int
	width       int
	height      int
	err         string
	agentCounts map[string]int // vibespace ID → count (async loaded)
}

func NewVibespacesTab(shared *SharedState) *VibespacesTab {
	return &VibespacesTab{
		shared:      shared,
		agentCounts: make(map[string]int),
	}
}

func (t *VibespacesTab) Title() string { return TabNames[TabVibespaces] }

func (t *VibespacesTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("j", "k"), key.WithHelp("j/k", "navigate")),
	}
}

func (t *VibespacesTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *VibespacesTab) Init() tea.Cmd {
	return t.loadVibespaces()
}

func (t *VibespacesTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TabActivateMsg:
		return t, t.loadVibespaces()

	case vibespacesLoadedMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
			return t, nil
		}
		t.vibespaces = msg.vibespaces
		t.clampSelected()
		t.err = ""
		return t, t.loadAgentCounts()

	case agentCountsLoadedMsg:
		t.agentCounts = msg.counts
		return t, nil

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	return t, nil
}

func (t *VibespacesTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	}
	return t, nil
}

func (t *VibespacesTab) View() string {
	if t.err != "" && len(t.vibespaces) == 0 {
		return lipgloss.NewStyle().
			Foreground(ui.ColorError).
			Padding(1, 2).
			Render(fmt.Sprintf("Error loading vibespaces: %s", t.err))
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
	// Build headers and rows based on responsive width
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
	showVolumes := w >= 81
	showAge := w >= 81

	// Build headers
	headers := []string{"Name", "Status", "Agents"}
	if showCPU {
		headers = append(headers, "CPU")
	}
	if showMem {
		headers = append(headers, "Mem")
	}
	if showVolumes {
		headers = append(headers, "Volumes")
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

		volumes := vsVolumeSummary(vs)

		if i == t.selected {
			cells := []string{"› " + vs.Name, vs.Status, agentCount}
			if showCPU {
				cells = append(cells, vs.Resources.CPU)
			}
			if showMem {
				cells = append(cells, vs.Resources.Memory)
			}
			if showVolumes {
				cells = append(cells, volumes)
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
			if showVolumes {
				cells = append(cells, volumes)
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

	var meta []string

	// ID (8-char prefix)
	idStr := vs.ID
	if len(idStr) > 8 {
		idStr = idStr[:8]
	}
	meta = append(meta, fmt.Sprintf("%s  %s",
		labelStyle.Render("ID"),
		dimStyle.Render(idStr)))

	// Status (colored)
	meta = append(meta, fmt.Sprintf("%s  %s",
		labelStyle.Render("Status"),
		vsStatusStyled(vs.Status)))

	// Resources — requests
	var reqParts []string
	if vs.Resources.CPU != "" {
		reqParts = append(reqParts, "CPU "+vs.Resources.CPU)
	}
	if vs.Resources.Memory != "" {
		reqParts = append(reqParts, "Mem "+vs.Resources.Memory)
	}
	if vs.Resources.Storage != "" {
		reqParts = append(reqParts, "Storage "+vs.Resources.Storage)
	}
	if len(reqParts) > 0 {
		meta = append(meta, fmt.Sprintf("%s  %s",
			labelStyle.Render("Reserved"),
			dimStyle.Render(strings.Join(reqParts, ", "))))
	}

	// Resources — limits
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

	// Agents
	agentStr := "-"
	if c, ok := t.agentCounts[vs.ID]; ok {
		noun := "agents"
		if c == 1 {
			noun = "agent"
		}
		agentStr = fmt.Sprintf("%d %s", c, noun)
	}
	meta = append(meta, fmt.Sprintf("%s  %s",
		labelStyle.Render("Agents"),
		dimStyle.Render(agentStr)))

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
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))
	metaBlock := metaHeader + "\n" + mutedLine + "\n" + strings.Join(meta, "\n") + "\n" + mutedLine

	return lipgloss.NewStyle().Padding(0, 2).Render(metaBlock)
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

func (t *VibespacesTab) loadAgentCounts() tea.Cmd {
	svc := t.shared.Vibespace
	vibespaces := t.vibespaces
	return func() tea.Msg {
		counts := make(map[string]int)
		if svc == nil {
			return agentCountsLoadedMsg{counts: counts}
		}
		for _, vs := range vibespaces {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			agents, err := svc.ListAgents(ctx, vs.ID)
			cancel()
			if err != nil {
				counts[vs.ID] = 1 // fallback, matches CLI list.go behavior
			} else {
				counts[vs.ID] = len(agents)
			}
		}
		return agentCountsLoadedMsg{counts: counts}
	}
}

// --- Helpers ---

func (t *VibespacesTab) clampSelected() {
	if t.selected >= len(t.vibespaces) {
		t.selected = max(len(t.vibespaces)-1, 0)
	}
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

// vsVolumeSummary returns a short summary for the Volumes table column.
// e.g. "PVC + 2 mounts", "PVC", "1 mount", "-"
func vsVolumeSummary(vs *model.Vibespace) string {
	var parts []string
	if vs.Persistent {
		parts = append(parts, "PVC")
	}
	if n := len(vs.Mounts); n == 1 {
		parts = append(parts, "1 mount")
	} else if n > 1 {
		parts = append(parts, fmt.Sprintf("%d mounts", n))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " + ")
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
		return createdAt // return raw string if not parseable
	}
	return timeAgo(t)
}

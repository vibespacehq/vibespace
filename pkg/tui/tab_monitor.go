package tui

import (
	"context"
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/k8s"
	"github.com/vibespacehq/vibespace/pkg/metrics"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// MonitorTab shows resource usage and agent activity.
type MonitorTab struct {
	shared        *SharedState
	width, height int

	// Data
	pods        []metrics.PodMetrics
	nodes       []metrics.NodeMetrics
	vibespaces  []*model.Vibespace
	available   bool
	err         string
	lastRefresh time.Time

	// State
	paused       bool
	filterVS     string // "" = all, else vibespace name
	pickerOpen   bool
	pickerCursor int
	pickerItems  []string // ["all", "test", "test9", ...]

	// Charts (ntcharts streaming line charts)
	cpuChart    streamlinechart.Model
	memChart    streamlinechart.Model
	chartsReady bool
	cpuHistory  []float64 // keep raw data to rebuild charts on resize
	memHistory  []float64
}

const (
	monitorRefreshInterval = 5 * time.Second
	monitorMaxHistory      = 60 // 5min at 5s refresh
)

var monitorHeaderStyle = lipgloss.NewStyle().
	Foreground(ui.ColorDim).
	Underline(true)

var monitorTextStyle = lipgloss.NewStyle().Foreground(ui.ColorText)

// --- Private message types ---

type monitorMetricsMsg struct {
	pods      []metrics.PodMetrics
	nodes     []metrics.NodeMetrics
	available bool
	err       string
}

type monitorVSListMsg struct {
	vibespaces []*model.Vibespace
}

type monitorTickMsg struct{}

func NewMonitorTab(shared *SharedState) *MonitorTab {
	return &MonitorTab{
		shared:    shared,
		available: true,
	}
}

func (t *MonitorTab) Title() string { return TabNames[TabMonitor] }

func (t *MonitorTab) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pause")),
		key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "picker")),
	}
}

func (t *MonitorTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *MonitorTab) Init() tea.Cmd {
	return tea.Batch(t.loadMetrics(), t.loadVibespaces(), t.scheduleTick())
}

func (t *MonitorTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TabActivateMsg:
		return t, tea.Batch(t.loadMetrics(), t.loadVibespaces())

	case TabDeactivateMsg:
		return t, nil

	case monitorMetricsMsg:
		t.available = msg.available
		t.err = msg.err
		if msg.available {
			t.pods = msg.pods
			t.nodes = msg.nodes
			t.lastRefresh = time.Now()
			t.appendHistory()
		}
		return t, nil

	case monitorVSListMsg:
		t.vibespaces = msg.vibespaces
		t.rebuildPickerItems()
		return t, nil

	case monitorTickMsg:
		var cmds []tea.Cmd
		if !t.paused {
			cmds = append(cmds, t.loadMetrics())
		}
		cmds = append(cmds, t.scheduleTick())
		return t, tea.Batch(cmds...)

	case tea.KeyMsg:
		return t.handleKey(msg)

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		if t.chartsReady {
			t.ensureCharts() // resize charts if needed
		}
		return t, nil
	}
	return t, nil
}

func (t *MonitorTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if t.pickerOpen {
		return t.handlePickerKey(msg)
	}

	switch msg.String() {
	case "R":
		return t, t.loadMetrics()
	case "p":
		t.paused = !t.paused
		return t, nil
	case "v":
		t.pickerOpen = true
		t.pickerCursor = 0
		return t, nil
	}
	return t, nil
}

func (t *MonitorTab) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if t.pickerCursor < len(t.pickerItems)-1 {
			t.pickerCursor++
		}
	case "k", "up":
		if t.pickerCursor > 0 {
			t.pickerCursor--
		}
	case "enter":
		if t.pickerCursor < len(t.pickerItems) {
			selected := t.pickerItems[t.pickerCursor]
			if selected == "all" {
				t.filterVS = ""
			} else {
				t.filterVS = selected
			}
		}
		t.pickerOpen = false
		return t, t.loadMetrics()
	case "esc", "v":
		t.pickerOpen = false
	}
	return t, nil
}

// --- View ---

func (t *MonitorTab) View() string {
	if t.pickerOpen {
		return t.renderPicker()
	}
	if !t.available {
		return t.renderUnavailable()
	}
	return t.renderDashboard()
}

func (t *MonitorTab) renderUnavailable() string {
	warn := lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(warn.Render("  ⚠ Metrics server not available"))
	sb.WriteString("\n\n")
	sb.WriteString(dim.Render("  The Kubernetes metrics API is not responding."))
	sb.WriteString("\n")
	sb.WriteString(dim.Render("  This usually resolves within 60 seconds after cluster startup."))
	sb.WriteString("\n\n")
	if t.err != "" {
		sb.WriteString(dim.Render("  " + t.err))
		sb.WriteString("\n\n")
	}
	sb.WriteString(dim.Render("  Retrying in 10s..."))
	sb.WriteString("\n")
	return sb.String()
}

func (t *MonitorTab) renderPicker() string {
	selected := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite)
	normal := lipgloss.NewStyle().Foreground(ui.ColorDim)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  " + renderGradientText("Select Vibespace", brandGradient))
	sb.WriteString("\n\n")

	for i, item := range t.pickerItems {
		cursor := "  "
		if i == t.pickerCursor {
			cursor = "▸ "
		}
		label := item
		if item == "all" {
			label = "all vibespaces"
		}
		if i == t.pickerCursor {
			sb.WriteString(selected.Render("  " + cursor + label))
		} else {
			sb.WriteString(normal.Render("  " + cursor + label))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(ui.ColorDim).Render("  ↑/↓ navigate  enter select  esc close"))
	sb.WriteString("\n")
	return sb.String()
}

func (t *MonitorTab) renderDashboard() string {
	var sb strings.Builder

	// Top padding + header
	sb.WriteString("\n")
	sb.WriteString(t.renderHeader())
	sb.WriteString("\n")

	showAll := t.filterVS == ""
	barW := t.barWidth()

	// Node Resources (only in "all" view)
	if showAll && len(t.nodes) > 0 {
		sb.WriteString("\n")
		sb.WriteString(t.renderNodeTable(barW))
	}

	// Agent Resources
	filteredPods := t.filteredPods()
	if len(filteredPods) > 0 {
		sb.WriteString("\n")
		sb.WriteString(t.renderAgentTable(filteredPods, showAll, barW))
	}

	// Totals line (only in single-vibespace view)
	if !showAll && len(filteredPods) > 0 {
		sb.WriteString("\n")
		sb.WriteString(t.renderTotals(filteredPods))
	}

	// Charts (if height permits)
	sb.WriteString(t.renderCharts(showAll))

	return sb.String()
}

func (t *MonitorTab) renderHeader() string {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)

	vsLabel := "all"
	if t.filterVS != "" {
		vsLabel = t.filterVS
	}
	left := bold.Render("  Vibespace: ") + renderGradientText(vsLabel+" ▾", brandGradient)

	var right string
	if t.paused {
		right = dim.Render("⏸ paused")
	} else {
		right = dim.Render("↻ refreshing 5s")
	}

	rightMargin := 2
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	padding := t.width - leftWidth - rightWidth - rightMargin
	if padding < 1 {
		padding = 1
	}

	return left + strings.Repeat(" ", padding) + right
}

func (t *MonitorTab) renderNodeTable(barW int) string {
	sectionTitle := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	var sb strings.Builder
	sb.WriteString("  " + sectionTitle.Render("Node Resources") + "\n")
	sb.WriteString("  " + mutedLine + "\n")

	headers := []string{"Node", "Cpu", "Memory"}
	rows := make([][]string, 0, len(t.nodes))
	for _, n := range t.nodes {
		cpuBar := monitorBar(n.CPUMillis, n.CPUAllocatableMillis, barW, formatCPU(n.CPUMillis)+"/"+formatCPU(n.CPUAllocatableMillis))
		memBar := monitorBar(n.MemoryBytes, n.MemoryAllocatableBytes, barW, formatMemory(n.MemoryBytes)+"/"+formatMemory(n.MemoryAllocatableBytes))
		rows = append(rows, []string{monitorTextStyle.Render(n.Name), cpuBar, memBar})
	}

	table := ui.NewTableWithOptions(headers, rows, ui.TableOptions{HeaderStyle: &monitorHeaderStyle})
	sb.WriteString(indentTableWithHeaderGap(table))
	sb.WriteString("  " + mutedLine + "\n")
	return sb.String()
}

func (t *MonitorTab) renderAgentTable(pods []metrics.PodMetrics, showAll bool, barW int) string {
	sectionTitle := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))
	showVS := showAll && t.width >= 80

	var sb strings.Builder
	sb.WriteString("  " + sectionTitle.Render("Agent Resources") + "\n")
	sb.WriteString("  " + mutedLine + "\n")

	var headers []string
	if showVS {
		headers = []string{"Agent", "Vibespace", "Cpu", "Memory"}
	} else {
		headers = []string{"Agent", "Cpu", "Memory"}
	}

	rows := make([][]string, 0, len(pods))
	for _, p := range pods {
		// CPU: show used/total. Memory: show only used (per mockup).
		cpuBar := monitorBar(p.CPUMillis, p.CPULimitMillis, barW, formatCPU(p.CPUMillis)+"/"+formatCPU(p.CPULimitMillis))
		memBar := monitorBar(p.MemoryBytes, p.MemoryLimitBytes, barW, formatMemory(p.MemoryBytes))

		name := p.AgentName
		if name == "" {
			name = p.Name
		}

		styledName := monitorTextStyle.Render(name)
		if showVS {
			rows = append(rows, []string{styledName, monitorTextStyle.Render(p.VibspaceName), cpuBar, memBar})
		} else {
			rows = append(rows, []string{styledName, cpuBar, memBar})
		}
	}

	table := ui.NewTableWithOptions(headers, rows, ui.TableOptions{HeaderStyle: &monitorHeaderStyle})
	sb.WriteString(indentTableWithHeaderGap(table))
	sb.WriteString("  " + mutedLine + "\n")
	return sb.String()
}

func (t *MonitorTab) renderTotals(pods []metrics.PodMetrics) string {
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
	var totalCPU, totalMem int64
	for _, p := range pods {
		totalCPU += p.CPUMillis
		totalMem += p.MemoryBytes
	}
	return dim.Render(fmt.Sprintf("  Totals: %d agents  %s CPU  %s memory",
		len(pods), formatCPU(totalCPU), formatMemory(totalMem)))
}

func (t *MonitorTab) renderCharts(showAll bool) string {
	if t.height < 30 || len(t.cpuHistory) < 2 {
		return ""
	}

	t.ensureCharts()

	sectionTitle := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted)
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
	mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", t.width-4))

	var sb strings.Builder

	// CPU chart
	var label string
	if showAll {
		label = "CPU History (cluster)"
	} else {
		label = "CPU History"
	}

	sb.WriteString("\n")
	sb.WriteString("  " + sectionTitle.Render(label) + "\n")
	sb.WriteString("  " + mutedLine + "\n")
	sb.WriteString(indentBlock(t.cpuChart.View(), "  "))
	if len(t.cpuHistory) > 0 {
		sb.WriteString(dim.Render(fmt.Sprintf("  now %d%%", int(t.cpuHistory[len(t.cpuHistory)-1]))))
		sb.WriteString("\n")
	}
	sb.WriteString("  " + mutedLine + "\n")

	// Memory chart (only if enough vertical space)
	if t.height > 40 {
		var memLabel string
		if showAll {
			memLabel = "Memory History (cluster)"
		} else {
			memLabel = "Memory History"
		}

		sb.WriteString("\n")
		sb.WriteString("  " + sectionTitle.Render(memLabel) + "\n")
		sb.WriteString("  " + mutedLine + "\n")
		sb.WriteString(indentBlock(t.memChart.View(), "  "))
		if len(t.memHistory) > 0 {
			sb.WriteString(dim.Render(fmt.Sprintf("  now %d%%", int(t.memHistory[len(t.memHistory)-1]))))
			sb.WriteString("\n")
		}
		sb.WriteString("  " + mutedLine + "\n")
	}

	return sb.String()
}

// indentBlock adds a prefix to each line of a multi-line string.
func indentBlock(s, prefix string) string {
	lines := strings.Split(s, "\n")
	var sb strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		sb.WriteString(prefix + line + "\n")
	}
	return sb.String()
}

// --- Helpers ---

func (t *MonitorTab) barWidth() int {
	switch {
	case t.width > 120:
		return 15
	case t.width > 100:
		return 12
	default:
		return 10
	}
}

func (t *MonitorTab) filteredPods() []metrics.PodMetrics {
	if t.filterVS == "" {
		return t.pods
	}
	filtered := make([]metrics.PodMetrics, 0)
	for _, p := range t.pods {
		if p.VibspaceName == t.filterVS {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (t *MonitorTab) rebuildPickerItems() {
	items := []string{"all"}
	seen := make(map[string]bool)
	for _, vs := range t.vibespaces {
		if !seen[vs.Name] {
			items = append(items, vs.Name)
			seen[vs.Name] = true
		}
	}
	t.pickerItems = items
}

func (t *MonitorTab) appendHistory() {
	cpuPct, memPct := t.computePercentages()
	t.cpuHistory = append(t.cpuHistory, cpuPct)
	t.memHistory = append(t.memHistory, memPct)
	if len(t.cpuHistory) > monitorMaxHistory {
		t.cpuHistory = t.cpuHistory[len(t.cpuHistory)-monitorMaxHistory:]
	}
	if len(t.memHistory) > monitorMaxHistory {
		t.memHistory = t.memHistory[len(t.memHistory)-monitorMaxHistory:]
	}

	// Push to charts
	if t.chartsReady {
		t.cpuChart.Push(cpuPct)
		t.cpuChart.DrawAll()
		applyChartGradient(&t.cpuChart)

		t.memChart.Push(memPct)
		t.memChart.DrawAll()
		applyChartGradient(&t.memChart)
	}
}

// chartSize returns the width and height for charts based on terminal dimensions.
func (t *MonitorTab) chartSize() (w, h int) {
	w = t.width / 2
	if w < 20 {
		w = 20
	}
	if w > 50 {
		w = 50
	}
	h = 5
	return
}

// ensureCharts creates or resizes the streaming line charts.
func (t *MonitorTab) ensureCharts() {
	w, h := t.chartSize()

	if !t.chartsReady {
		chartLineStyle := lipgloss.NewStyle().Foreground(ui.Teal)
		axisStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)

		t.cpuChart = streamlinechart.New(w, h,
			streamlinechart.WithYRange(0, 100),
			streamlinechart.WithAxesStyles(axisStyle, labelStyle),
			streamlinechart.WithStyles(runes.ArcLineStyle, chartLineStyle),
		)
		t.cpuChart.SetViewYRange(0, 100)
		t.cpuChart.DrawXYAxisAndLabel()

		t.memChart = streamlinechart.New(w, h,
			streamlinechart.WithYRange(0, 100),
			streamlinechart.WithAxesStyles(axisStyle, labelStyle),
			streamlinechart.WithStyles(runes.ArcLineStyle, chartLineStyle),
		)
		t.memChart.SetViewYRange(0, 100)
		t.memChart.DrawXYAxisAndLabel()

		// Replay history into charts, pre-filling so line starts from left edge
		replayIntoChart(&t.cpuChart, t.cpuHistory, w)
		applyChartGradient(&t.cpuChart)
		replayIntoChart(&t.memChart, t.memHistory, w)
		applyChartGradient(&t.memChart)

		t.chartsReady = true
		return
	}

	// Resize if dimensions changed
	if t.cpuChart.Width() != w || t.cpuChart.Height() != h {
		t.cpuChart.Resize(w, h)
		t.cpuChart.DrawXYAxisAndLabel()
		t.cpuChart.ClearAllData()
		replayIntoChart(&t.cpuChart, t.cpuHistory, w)
		applyChartGradient(&t.cpuChart)

		t.memChart.Resize(w, h)
		t.memChart.DrawXYAxisAndLabel()
		t.memChart.ClearAllData()
		replayIntoChart(&t.memChart, t.memHistory, w)
		applyChartGradient(&t.memChart)
	}
}

// replayIntoChart pre-fills a chart with padding so the line starts from the left edge,
// then pushes the actual data points.
func replayIntoChart(chart *streamlinechart.Model, data []float64, chartWidth int) {
	// The graph area is smaller than canvas width (axis labels take space).
	// Use GraphWidth for the actual plotting area.
	graphW := chart.GraphWidth()
	if graphW <= 0 {
		graphW = chartWidth
	}

	// Pre-fill with the first value so the line starts from the left
	padValue := 0.0
	if len(data) > 0 {
		padValue = data[0]
	}
	padCount := graphW - len(data)
	if padCount < 0 {
		padCount = 0
	}
	for i := 0; i < padCount; i++ {
		chart.Push(padValue)
	}
	for _, v := range data {
		chart.Push(v)
	}
	chart.DrawAll()
}

// applyChartGradient recolors chart line cells with the brand gradient (left→right).
// Axis and label cells (which use ColorMuted/ColorDim) are left unchanged.
func applyChartGradient(chart *streamlinechart.Model) {
	w := chart.Canvas.Width()
	h := chart.Canvas.Height()
	if w <= 0 {
		return
	}

	gradColors := buildGradient(w, brandGradient)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := chart.Canvas.Cell(image.Point{X: x, Y: y})
			if cell.Rune == 0 || cell.Rune == ' ' {
				continue
			}
			// Skip axis/label cells: check if it's a digit or axis border
			// The Y axis uses │ at x=0 area, X axis uses ─ at bottom
			// We only recolor arc/line runes that belong to the data line
			switch cell.Rune {
			case '╭', '╮', '╰', '╯', '─', '│',
				'┌', '┐', '└', '┘',
				'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█':
				// Could be axis or data — check foreground color
				fg := cell.Style.GetForeground()
				if fg == ui.ColorMuted || fg == ui.ColorDim {
					continue // axis/label cell, skip
				}
				chart.Canvas.SetCellStyle(image.Point{X: x, Y: y},
					lipgloss.NewStyle().Foreground(gradColors[x]))
			}
		}
	}
}

func (t *MonitorTab) computePercentages() (cpuPct, memPct float64) {
	if t.filterVS == "" {
		var totalCPU, totalMem, allocCPU, allocMem int64
		for _, n := range t.nodes {
			totalCPU += n.CPUMillis
			totalMem += n.MemoryBytes
			allocCPU += n.CPUAllocatableMillis
			allocMem += n.MemoryAllocatableBytes
		}
		if allocCPU > 0 {
			cpuPct = float64(totalCPU) / float64(allocCPU) * 100
		}
		if allocMem > 0 {
			memPct = float64(totalMem) / float64(allocMem) * 100
		}
	} else {
		var totalCPU, totalMem, limitCPU, limitMem int64
		for _, p := range t.pods {
			if p.VibspaceName == t.filterVS {
				totalCPU += p.CPUMillis
				totalMem += p.MemoryBytes
				limitCPU += p.CPULimitMillis
				limitMem += p.MemoryLimitBytes
			}
		}
		if limitCPU > 0 {
			cpuPct = float64(totalCPU) / float64(limitCPU) * 100
		}
		if limitMem > 0 {
			memPct = float64(totalMem) / float64(limitMem) * 100
		}
	}
	return
}

// indentTableWithHeaderGap indents each line of a table string with 2 spaces
// and inserts a blank line after the header row (first line).
func indentTableWithHeaderGap(table string) string {
	lines := strings.Split(strings.TrimRight(table, "\n"), "\n")
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString("  " + line + "\n")
		if i == 0 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// --- Rendering helpers ---

// monitorBar draws a colored progress bar: ██░░░░  XX% (detail)
// Under 60% uses the brand gradient; 60-79% orange; >=80% red.
func monitorBar(used, total int64, width int, detail string) string {
	if total <= 0 {
		return detail
	}

	pct := float64(used) / float64(total) * 100
	if pct > 100 {
		pct = 100
	}

	filled := int(pct / 100 * float64(width))
	if filled == 0 && used > 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	emptyStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)

	// Gradient across full bar width: teal→pink
	gradColors := buildGradient(width, brandGradient)
	var b strings.Builder
	for i := 0; i < filled; i++ {
		b.WriteString(lipgloss.NewStyle().Foreground(gradColors[i]).Render("█"))
	}
	filledStr := b.String()

	bar := filledStr + emptyStyle.Render(strings.Repeat("░", empty))

	pctStr := fmt.Sprintf("%d%%", int(pct))
	if int(pct) == 0 && used > 0 {
		pctStr = "<1%"
	}

	return fmt.Sprintf("%s %s", bar, monitorTextStyle.Render(fmt.Sprintf("%s (%s)", pctStr, detail)))
}

func formatCPU(millis int64) string {
	return fmt.Sprintf("%dm", millis)
}

func formatMemory(bytes int64) string {
	mi := bytes / (1024 * 1024)
	return fmt.Sprintf("%dMi", mi)
}

// --- Commands ---

func (t *MonitorTab) loadMetrics() tea.Cmd {
	fetcher := t.shared.Metrics
	return func() tea.Msg {
		if fetcher == nil {
			return monitorMetricsMsg{available: false, err: "metrics service unavailable"}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if !fetcher.Available(ctx) {
			return monitorMetricsMsg{available: false, err: "metrics API not responding"}
		}

		pods, podErr := fetcher.FetchPodMetrics(ctx, k8s.VibespaceNamespace)
		nodes, nodeErr := fetcher.FetchNodeMetrics(ctx)

		if podErr != nil {
			return monitorMetricsMsg{available: true, err: podErr.Error()}
		}
		if nodeErr != nil {
			return monitorMetricsMsg{available: true, err: nodeErr.Error(), pods: pods}
		}

		sort.Slice(pods, func(i, j int) bool {
			if pods[i].VibspaceName != pods[j].VibspaceName {
				return pods[i].VibspaceName < pods[j].VibspaceName
			}
			return pods[i].AgentName < pods[j].AgentName
		})

		return monitorMetricsMsg{
			pods:      pods,
			nodes:     nodes,
			available: true,
		}
	}
}

func (t *MonitorTab) loadVibespaces() tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return monitorVSListMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		vs, _ := svc.List(ctx)
		return monitorVSListMsg{vibespaces: vs}
	}
}

func (t *MonitorTab) scheduleTick() tea.Cmd {
	return tea.Tick(monitorRefreshInterval, func(_ time.Time) tea.Msg {
		return monitorTickMsg{}
	})
}

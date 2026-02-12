package tui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/harmonica"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	colorful "github.com/lucasb-eyer/go-colorful"
	zone "github.com/lrstanley/bubblezone"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/ui"
	"golang.org/x/term"
)

// App is the root Bubble Tea model for the tab-based TUI.
type App struct {
	tabs      []Tab
	tabInited []bool
	activeTab int
	width     int
	height    int
	shared    *SharedState
	help      *HelpOverlay
	palette   *PaletteOverlay
	ready     bool

	// Tab transition animation
	spring       harmonica.Spring
	highlightX   float64 // current animated X position of highlight
	highlightVel float64 // current velocity
	targetX      float64 // target X position
	animating    bool
	tabOffsets   []int // left X offset of each tab label
	tabWidths    []int // rendered width of each tab label
}

// springTickMsg drives the animation loop.
type springTickMsg struct{}

// NewApp creates an App starting on the Vibespaces tab.
func NewApp() *App {
	shared := NewSharedState()
	return newApp(shared, TabVibespaces, nil, false)
}

// NewAppWithChat creates an App starting on the Chat tab with a session.
func NewAppWithChat(sess *session.Session, resume bool) *App {
	shared := NewSharedState()
	return newApp(shared, TabChat, sess, resume)
}

func newApp(shared *SharedState, startTab int, sess *session.Session, resume bool) *App {
	a := &App{
		tabs: []Tab{
			NewVibespacesTab(shared),
			NewChatTab(sess, resume),
			NewMonitorTab(shared),
			NewSessionsTab(shared),
			NewRemoteTab(shared),
		},
		tabInited: make([]bool, tabCount),
		activeTab: startTab,
		shared:    shared,
		help:      NewHelpOverlay(),
		palette:   NewPaletteOverlay(),
		spring:    harmonica.NewSpring(harmonica.FPS(60), 14.0, 1.0),
	}
	return a
}

// --- tea.Model ---

func (a *App) Init() tea.Cmd {
	zone.NewGlobal()
	cmds := []tea.Cmd{refreshSharedState(a.shared)}

	// Init the starting tab.
	cmd := a.tabs[a.activeTab].Init()
	a.tabInited[a.activeTab] = true
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.computeTabLayout()
		// Snap highlight to active tab on first render / resize
		if len(a.tabOffsets) > a.activeTab {
			a.highlightX = float64(a.tabOffsets[a.activeTab])
			a.targetX = a.highlightX
		}
		iw := a.innerWidth()
		contentH := a.contentHeight()
		a.help.SetSize(a.width, a.height)
		a.palette.SetSize(a.width, a.height)
		a.tabs[a.activeTab].SetSize(iw, contentH)
		// Forward adjusted size to active tab
		_, cmd := a.tabs[a.activeTab].Update(tea.WindowSizeMsg{
			Width: iw, Height: contentH,
		})
		return a, cmd

	case springTickMsg:
		if !a.animating {
			return a, nil
		}
		a.highlightX, a.highlightVel = a.spring.Update(
			a.highlightX, a.highlightVel, a.targetX,
		)
		// Stop when close enough and velocity is low
		if math.Abs(a.highlightX-a.targetX) < 0.5 && math.Abs(a.highlightVel) < 0.5 {
			a.highlightX = a.targetX
			a.highlightVel = 0
			a.animating = false
			return a, nil
		}
		return a, a.springTick()

	case SharedStateRefreshedMsg:
		return a, nil

	case SwitchTabMsg:
		return a, a.switchTab(msg.Tab)

	case SwitchToChatMsg:
		chat := a.tabs[TabChat].(*ChatTab)
		cmd := chat.LoadSession(msg.Session, msg.Resume)
		a.tabInited[TabChat] = true
		return a, tea.Batch(a.switchTab(TabChat), cmd)

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease {
			// Click anywhere in the tab bar area (label row + underline row)
			if msg.Y < tabBarHeight && len(a.tabOffsets) > 0 {
				x := msg.X
				for i := tabCount - 1; i >= 0; i-- {
					if x >= a.tabOffsets[i] {
						return a, a.switchTab(i)
					}
				}
			}
		}

	case tea.KeyMsg:
		// Overlays intercept keys first
		if a.help.Visible() {
			cmd := a.help.Update(msg)
			return a, cmd
		}
		if a.palette.Visible() {
			cmd := a.palette.Update(msg)
			return a, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit

		// Tab/Shift+Tab and Ctrl+1..5 always switch tabs (even from Chat)
		case "tab":
			return a, a.switchTab((a.activeTab + 1) % tabCount)
		case "shift+tab":
			return a, a.switchTab((a.activeTab + tabCount - 1) % tabCount)
		case "ctrl+1":
			return a, a.switchTab(0)
		case "ctrl+2":
			return a, a.switchTab(1)
		case "ctrl+3":
			return a, a.switchTab(2)
		case "ctrl+4":
			return a, a.switchTab(3)
		case "ctrl+5":
			return a, a.switchTab(4)
		}

		// Number keys and overlays only outside Chat tab
		if a.activeTab != TabChat {
			switch msg.String() {
			case "1":
				return a, a.switchTab(0)
			case "2":
				return a, a.switchTab(1)
			case "3":
				return a, a.switchTab(2)
			case "4":
				return a, a.switchTab(3)
			case "5":
				return a, a.switchTab(4)
			case "?":
				a.help.Toggle()
				return a, nil
			case ":":
				a.palette.Toggle()
				return a, nil
			}
		}
	}

	// Delegate to active tab
	_, cmd := a.tabs[a.activeTab].Update(msg)
	return a, cmd
}

func (a *App) View() string {
	if !a.ready {
		return "\n  Loading..."
	}

	tabBar := a.renderTabBar()
	content := a.tabs[a.activeTab].View()
	statusBar := a.renderStatusBar()

	// Assemble: tabBar + content + statusBar
	contentHeight := a.contentHeight()
	styledContent := lipgloss.NewStyle().
		Height(contentHeight).
		MaxHeight(contentHeight).
		Render(content)

	inner := lipgloss.JoinVertical(lipgloss.Left, tabBar, styledContent, statusBar)

	base := appBorderStyle.
		Width(a.innerWidth()).
		Height(a.height - borderH).
		Render(inner)

	// Overlays render on top
	if a.help.Visible() {
		base = a.help.View()
	} else if a.palette.Visible() {
		base = a.palette.View()
	}

	return zone.Scan(base)
}

// --- Tab switching ---

func (a *App) switchTab(idx int) tea.Cmd {
	if idx < 0 || idx >= tabCount || idx == a.activeTab {
		return nil
	}

	var cmds []tea.Cmd

	// Deactivate old
	_, cmd := a.tabs[a.activeTab].Update(TabDeactivateMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	a.activeTab = idx

	// Start highlight animation
	if idx < len(a.tabOffsets) {
		a.targetX = float64(a.tabOffsets[idx])
		a.animating = true
		cmds = append(cmds, a.springTick())
	}

	// Lazy init
	if !a.tabInited[idx] {
		cmd = a.tabs[idx].Init()
		a.tabInited[idx] = true
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Resize and activate
	iw := a.innerWidth()
	a.tabs[idx].SetSize(iw, a.contentHeight())
	_, cmd = a.tabs[idx].Update(TabActivateMsg{})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Send size to new tab
	_, cmd = a.tabs[idx].Update(tea.WindowSizeMsg{
		Width: iw, Height: a.contentHeight(),
	})
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// --- Animation ---

func (a *App) springTick() tea.Cmd {
	return tea.Tick(time.Second/60, func(time.Time) tea.Msg {
		return springTickMsg{}
	})
}

func (a *App) computeTabLayout() {
	a.tabOffsets = make([]int, tabCount)
	a.tabWidths = make([]int, tabCount)
	x := 0
	for i := 0; i < tabCount; i++ {
		label := fmt.Sprintf("  %d %s  ", i+1, TabNames[i]) // matches renderTabBar format
		w := len([]rune(label))
		a.tabOffsets[i] = x
		a.tabWidths[i] = w
		x += w
	}
}

// --- Rendering helpers ---

func (a *App) contentHeight() int {
	h := a.height - tabBarHeight - statusBarHeight - borderH
	if h < 1 {
		h = 1
	}
	return h
}

// innerWidth returns the usable width inside the border.
func (a *App) innerWidth() int {
	w := a.width - borderW
	if w < 1 {
		w = 1
	}
	return w
}

func (a *App) renderTabBar() string {
	iw := a.innerWidth()

	// Build flat text content: "  1 Vibespaces    2 Chat  ..." with padding per tab
	var fullText []rune
	for i := 0; i < tabCount; i++ {
		label := fmt.Sprintf("  %d %s  ", i+1, TabNames[i])
		fullText = append(fullText, []rune(label)...)
	}
	// Pad to full width
	for len(fullText) < iw {
		fullText = append(fullText, ' ')
	}
	if len(fullText) > iw {
		fullText = fullText[:iw]
	}

	// Highlight region (animated position)
	activeW := 0
	if a.activeTab < len(a.tabWidths) {
		activeW = a.tabWidths[a.activeTab]
	}
	hlStart := int(math.Round(a.highlightX))
	hlEnd := hlStart + activeW
	if hlStart < 0 {
		hlStart = 0
	}
	if hlEnd > iw {
		hlEnd = iw
	}

	// Pre-compute gradient colors for the highlight segment
	segLen := hlEnd - hlStart
	var gradColors []lipgloss.Color
	if segLen > 0 {
		gradColors = buildGradient(segLen, brandGradient)
	}

	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	mutedStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)

	// Row 1: tab labels — gradient foreground text for active, dim for inactive
	var labelRow strings.Builder
	for i, r := range fullText {
		if i >= hlStart && i < hlEnd {
			gc := gradColors[i-hlStart]
			labelRow.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(gc).
				Render(string(r)))
		} else {
			labelRow.WriteString(dimStyle.Render(string(r)))
		}
	}

	// Row 2: underline — gradient ─ under active tab, dim ─ elsewhere
	var underline strings.Builder
	for i := 0; i < iw; i++ {
		if i >= hlStart && i < hlEnd {
			gc := gradColors[i-hlStart]
			underline.WriteString(lipgloss.NewStyle().
				Foreground(gc).
				Render("─"))
		} else {
			underline.WriteString(mutedStyle.Render("─"))
		}
	}

	return labelRow.String() + "\n" + underline.String()
}

// renderGradientText renders a string with per-character gradient colors + bold.
func renderGradientText(s string, stops []lipgloss.Color) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}
	colors := buildGradient(len(runes), stops)

	var b strings.Builder
	for i, r := range runes {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colors[i]).Render(string(r)))
	}
	return b.String()
}

// renderGradientRow applies a single gradient across all cells of a table row.
// Each cell gets the gradient slice corresponding to its position in the full row.
func renderGradientRow(cells []string, stops []lipgloss.Color) []string {
	// Count total runes across all cells
	var allRunes [][]rune
	total := 0
	for _, c := range cells {
		r := []rune(c)
		allRunes = append(allRunes, r)
		total += len(r)
	}
	if total == 0 {
		return cells
	}

	colors := buildGradient(total, stops)

	result := make([]string, len(cells))
	offset := 0
	for i, runes := range allRunes {
		var b strings.Builder
		for j, r := range runes {
			b.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(colors[offset+j]).
				Render(string(r)))
		}
		result[i] = b.String()
		offset += len(runes)
	}
	return result
}

// buildGradient pre-computes gradient colors for n characters across the given stops.
// Uses OKLab interpolation via go-colorful for perceptually smooth transitions.
func buildGradient(n int, stops []lipgloss.Color) []lipgloss.Color {
	if n <= 0 {
		return nil
	}

	// Parse all stop colors once
	parsed := make([]colorful.Color, len(stops))
	for i, c := range stops {
		parsed[i], _ = colorful.Hex(string(c))
	}

	colors := make([]lipgloss.Color, n)
	segments := float64(len(stops) - 1)

	for i := 0; i < n; i++ {
		t := float64(i) / math.Max(float64(n-1), 1)
		scaled := t * segments
		idx := int(scaled)
		if idx >= len(stops)-1 {
			idx = len(stops) - 2
		}
		frac := scaled - float64(idx)

		blended := parsed[idx].BlendOkLab(parsed[idx+1], frac)
		colors[i] = lipgloss.Color(blended.Hex())
	}

	return colors
}

func (a *App) renderStatusBar() string {
	iw := a.innerWidth()

	// Dim top border
	border := lipgloss.NewStyle().Foreground(ui.ColorMuted).
		Render(strings.Repeat("─", iw))

	// Collect just the key strings for gradient
	bindings := a.tabs[a.activeTab].ShortHelp()
	var keys []string
	for _, b := range bindings {
		keys = append(keys, b.Help().Key)
	}

	// Build gradient across all keys combined
	totalKeyRunes := 0
	for _, k := range keys {
		totalKeyRunes += len([]rune(k))
	}
	gradColors := buildGradient(totalKeyRunes, brandGradient)

	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	var parts []string
	offset := 0
	for i, b := range bindings {
		h := b.Help()
		keyRunes := []rune(keys[i])

		// Gradient key
		var kb strings.Builder
		for j, r := range keyRunes {
			kb.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(gradColors[offset+j]).
				Render(string(r)))
		}
		offset += len(keyRunes)

		parts = append(parts, kb.String()+" "+dimStyle.Render(h.Desc))
	}

	text := " " + strings.Join(parts, dimStyle.Render("  |  "))

	return border + "\n" + text
}

// --- Public entry points ---

// RunApp starts the tab-based TUI on the Vibespaces tab.
func RunApp() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("TUI requires an interactive terminal (stdin is not a TTY); use --json for non-interactive mode")
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("TUI requires an interactive terminal (stdout is not a TTY); use --json for non-interactive mode")
	}

	a := NewApp()
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// RunAppWithChat starts the tab-based TUI on the Chat tab with a session.
func RunAppWithChat(sess *session.Session, resume bool) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("TUI requires an interactive terminal (stdin is not a TTY); use --json for non-interactive mode")
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("TUI requires an interactive terminal (stdout is not a TTY); use --json for non-interactive mode")
	}

	a := NewAppWithChat(sess, resume)
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

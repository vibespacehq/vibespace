package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/internal/platform"
	"github.com/vibespacehq/vibespace/pkg/remote"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// clusterStatus represents the detected state of the cluster.
type clusterStatus int

const (
	clusterStatusUnknown      clusterStatus = iota // detection not yet complete
	clusterStatusNotInstalled                      // no cluster.json found
	clusterStatusStopped                           // cluster exists but not running
	clusterStatusRunning                           // cluster is running
)

// welcomeClusterStatusMsg delivers the result of async cluster detection.
type welcomeClusterStatusMsg struct {
	status      clusterStatus
	clusterMode string
	err         error
}

// vsInitDoneMsg signals that `vibespace init` has completed.
type vsInitDoneMsg struct {
	err error
}

// detectClusterStatus checks the cluster state asynchronously.
func detectClusterStatus() tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return welcomeClusterStatusMsg{status: clusterStatusUnknown, err: err}
		}
		vsHome := home + "/.vibespace"

		state, err := platform.LoadClusterState(vsHome)
		if err != nil {
			// No cluster.json → not installed
			return welcomeClusterStatusMsg{status: clusterStatusNotInstalled}
		}

		p := platform.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
		mgr, err := platform.NewClusterManager(p, vsHome, platform.ClusterManagerOptions{})
		if err != nil {
			return welcomeClusterStatusMsg{
				status:      clusterStatusStopped,
				clusterMode: string(state.Mode),
				err:         err,
			}
		}

		running, err := mgr.IsRunning()
		if err != nil || !running {
			return welcomeClusterStatusMsg{
				status:      clusterStatusStopped,
				clusterMode: string(state.Mode),
			}
		}

		return welcomeClusterStatusMsg{
			status:      clusterStatusRunning,
			clusterMode: string(state.Mode),
		}
	}
}

// execInit runs `vibespace init` via tea.ExecProcess.
func execInit() tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg {
			return vsInitDoneMsg{err: fmt.Errorf("cannot find vibespace binary: %w", err)}
		}
	}
	cmd := exec.Command(bin, "init")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return vsInitDoneMsg{err: err}
	})
}

// renderLogoArt renders the vibespace logo as colored block characters.
// The logo is a 2×2 bento grid: teal (top-left, big), orange (top-right, small),
// yellow (bottom-left, small), pink (bottom-right, big).
func renderLogoArt() string {
	rounded := lipgloss.RoundedBorder()

	makeBox := func(color lipgloss.TerminalColor, w, h int) string {
		return lipgloss.NewStyle().
			Width(w).
			Height(h).
			Border(rounded).
			BorderForeground(color).
			Render("")
	}

	tealBox := makeBox(ui.Teal, 7, 4)     // 6 rows (big)
	orangeBox := makeBox(ui.Orange, 7, 2) // 4 rows (small)
	yellowBox := makeBox(ui.Yellow, 7, 1) // 3 rows (small)
	pinkBox := makeBox(ui.Pink, 7, 3)     // 5 rows (big)
	// Left: 6+3=9, Right: 4+5=9 ✓
	// Width: 9+1+9=19 chars ≈ 2×9 rows → square

	// Bento grid: stack each column, then join horizontally.
	// Constraint: teal + yellow rows == orange + pink rows (5+2 == 3+4)
	leftCol := lipgloss.JoinVertical(lipgloss.Left, tealBox, yellowBox)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, orangeBox, pinkBox)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, " ", rightCol)
}

// renderWelcome produces the complete welcome screen layout.
// Pure function: all state is passed in as parameters.
//
// Responsive modes:
//   - Minimal (height < 18): hero + system status lines only
//   - Compact (height < 30): hero + system + quick start (no cards, no pills)
//   - Full (height >= 30):   hero + cards stacked
//
// Width < 60 = narrow: hero vertical, cards stacked, no pills.
func renderWelcome(width, height int, cluster clusterStatus,
	daemonRunning bool, blinkOn bool, updateAvailable bool, latestVersion string) string {

	narrow := width < 60
	minimal := height < 18
	full := height >= 30 && !narrow

	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDim)
	mutedStyle := lipgloss.NewStyle().Foreground(ui.ColorMuted)
	greenStyle := lipgloss.NewStyle().Foreground(ui.ColorSuccess)
	sectionTitle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite)

	// --- Hero section ---
	heroSection := renderLogoArt()

	// --- System status lines ---
	// Active dots blink: alternate between ● and ◦
	activeDot := func(color lipgloss.TerminalColor) string {
		if blinkOn {
			return lipgloss.NewStyle().Foreground(color).Render("●")
		}
		return mutedStyle.Render("◦")
	}
	offDot := mutedStyle.Render("○")

	// Status line helper: fixed-width columns so lines form a uniform block.
	statusLine := func(label string, dot string, status string) string {
		return fmt.Sprintf("%-10s %s %-15s", label, dot, status)
	}

	var clusterText string
	switch cluster {
	case clusterStatusRunning:
		clusterText = statusLine("Cluster", activeDot(ui.ColorSuccess), "Running")
	case clusterStatusStopped:
		clusterText = statusLine("Cluster", activeDot(ui.Yellow), "Stopped")
	case clusterStatusNotInstalled:
		clusterText = statusLine("Cluster", offDot, "Not installed")
	default:
		clusterText = statusLine("Cluster", offDot, "Checking…")
	}

	var daemonText string
	if daemonRunning {
		daemonText = statusLine("Daemon", activeDot(ui.ColorSuccess), "Running")
	} else {
		daemonText = statusLine("Daemon", offDot, "Not running")
	}

	remoteConnected := remote.IsConnected()
	var remoteText string
	if remoteConnected {
		remoteText = statusLine("Remote", activeDot(ui.ColorSuccess), "Connected")
	} else {
		remoteText = statusLine("Remote", offDot, "Not connected")
	}

	statusLines := []string{clusterText, daemonText, remoteText}
	if updateAvailable && latestVersion != "" {
		updateText := statusLine("Update", activeDot(ui.Orange), latestVersion+" available")
		statusLines = append(statusLines, updateText)
	}
	systemLines := strings.Join(statusLines, "\n")

	// --- Minimal mode: hero + system status only ---
	if minimal {
		content := lipgloss.JoinVertical(lipgloss.Center,
			heroSection, "",
			sectionTitle.Render("System"),
			systemLines,
		)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
	}

	// --- Quick Start steps ---
	// A cluster is available if running locally OR connected via remote.
	clusterReady := cluster == clusterStatusRunning || remoteConnected
	var step1Prefix, step2Prefix, step3Prefix string
	if clusterReady {
		step1Prefix = greenStyle.Render("✓")
		step2Prefix = lipgloss.NewStyle().Foreground(ui.Teal).Render("→")
		step3Prefix = dimStyle.Render(" ")
	} else {
		step1Prefix = lipgloss.NewStyle().Foreground(ui.Teal).Render("→")
		step2Prefix = dimStyle.Render(" ")
		step3Prefix = dimStyle.Render(" ")
	}
	// Step line helper: fixed-width columns so lines form a uniform block.
	stepLine := func(prefix, desc, hint string) string {
		return fmt.Sprintf(" %s %-25s %s", prefix, desc, dimStyle.Render(fmt.Sprintf("%-32s", hint)))
	}

	quickStartLines := strings.Join([]string{
		stepLine(step1Prefix, "1. Connect to a cluster", "press i to init · 5 for remote"),
		stepLine(step2Prefix, "2. Create a vibespace", "press n"),
		stepLine(step3Prefix, "3. Add agents & connect", "press enter → a / x"),
	}, "\n")

	// --- Compact mode: hero + system + quick start (no cards) ---
	if !full {
		content := lipgloss.JoinVertical(lipgloss.Center,
			heroSection, "",
			sectionTitle.Render("System"),
			systemLines, "",
			sectionTitle.Render("Quick Start"),
			quickStartLines,
		)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
	}

	// --- Full mode: hero + cards ---

	// Cards: system and quick start stacked vertically
	cardW := width - 4
	if cardW > 72 {
		cardW = 72
	}

	systemCardBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Teal).
		Padding(1, 2)

	qsCardBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Orange).
		Padding(1, 2)

	systemTitle := lipgloss.NewStyle().Bold(true).Foreground(ui.Teal)
	qsTitle := lipgloss.NewStyle().Bold(true).Foreground(ui.Orange)

	// Card inner width: cardW minus border (2) minus padding (2*2)
	cardInnerW := cardW - 6

	systemBody := systemLines
	systemCardContent := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.PlaceHorizontal(cardInnerW, lipgloss.Center, systemTitle.Render("System")),
		lipgloss.PlaceHorizontal(cardInnerW, lipgloss.Center, systemBody),
	)
	systemCard := systemCardBorder.Width(cardW).Render(systemCardContent)

	qsBody := strings.Join([]string{
		stepLine(step1Prefix, "1. Connect to a cluster", "press i to init · 5 for remote"),
		stepLine(step2Prefix, "2. Create a vibespace", "press n"),
		stepLine(step3Prefix, "3. Add agents & connect", "press enter → a / x"),
	}, "\n")
	qsCardContent := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.PlaceHorizontal(cardInnerW, lipgloss.Center, qsTitle.Render("Quick Start")),
		lipgloss.PlaceHorizontal(cardInnerW, lipgloss.Center, qsBody),
	)
	quickStartCard := qsCardBorder.Width(cardW).Render(qsCardContent)

	cardsStack := lipgloss.JoinVertical(lipgloss.Center, systemCard, "", quickStartCard)

	// Links: horizontal with separator
	linkStyle := lipgloss.NewStyle().Foreground(ui.Teal)
	sep := mutedStyle.Render(" · ")
	webIcon := lipgloss.NewStyle().Foreground(ui.Orange).Render("⊕")
	ghIcon := lipgloss.NewStyle().Foreground(ui.Pink).Render("⊙")
	links := webIcon + " " + linkStyle.Render("vibespace.build") + sep +
		ghIcon + " " + linkStyle.Render("github.com/vibespacehq/vibespace")

	content := lipgloss.JoinVertical(lipgloss.Center,
		heroSection, "",
		cardsStack, "",
		links,
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/vibespacehq/vibespace/pkg/remote"
	"github.com/vibespacehq/vibespace/pkg/ui"
)

// remoteMode represents the current state of the Remote tab.
type remoteMode int

const (
	remoteModeDisconnected remoteMode = iota
	remoteModeConnected
	remoteModeServing
	remoteModeTokenInput // user is typing a token to connect
	remoteModeSudoPrompt // user is typing sudo password for diagnostics
)

const remoteRefreshInterval = 10 * time.Second

// --- Private message types ---

type remoteStateMsg struct {
	remoteState *remote.RemoteState
	serverState *remote.ServerState
	ifaceStatus string
	diagnostics []remote.DiagnosticResult
	needsSudo   bool // sudo -n failed; prompt user
	err         error
}

type remoteTickMsg struct{}

type remoteExecReturnMsg struct {
	err error
}

type remoteSudoResultMsg struct {
	ok       bool
	password string
}

// RemoteTab shows WireGuard remote connection status.
type RemoteTab struct {
	shared        *SharedState
	width, height int

	// State
	mode        remoteMode
	remoteState *remote.RemoteState
	serverState *remote.ServerState
	ifaceStatus string
	diagnostics []remote.DiagnosticResult
	err         string
	lastRefresh time.Time

	// Token input
	tokenInput textinput.Model

	// Sudo password input (for diagnostics)
	sudoInput     textinput.Model
	sudoPassword  string // stored in memory, piped via sudo -S
	sudoDismissed bool   // user pressed Esc on sudo prompt, don't auto-prompt again

	// Confirm disconnect
	confirmDisconnect bool
}

func NewRemoteTab(shared *SharedState) *RemoteTab {
	ti := textinput.New()
	ti.Placeholder = "vs-eyJrIjoiYWJjMTI..."
	ti.CharLimit = 2048
	ti.Width = 60

	si := textinput.New()
	si.Placeholder = "password"
	si.EchoMode = textinput.EchoPassword
	si.EchoCharacter = '•'
	si.CharLimit = 256
	si.Width = 40

	return &RemoteTab{
		shared:     shared,
		tokenInput: ti,
		sudoInput:  si,
	}
}

func (t *RemoteTab) Title() string { return TabNames[TabRemote] }

func (t *RemoteTab) ShortHelp() []key.Binding {
	switch t.mode {
	case remoteModeConnected:
		return []key.Binding{
			key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "disconnect")),
			key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		}
	case remoteModeServing:
		return []key.Binding{
			key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "generate token")),
			key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		}
	case remoteModeTokenInput, remoteModeSudoPrompt:
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	default:
		return []key.Binding{
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "connect")),
		}
	}
}

func (t *RemoteTab) SetSize(w, h int) { t.width = w; t.height = h }

func (t *RemoteTab) Init() tea.Cmd {
	return tea.Batch(t.loadRemoteState(true), t.scheduleTick())
}

func (t *RemoteTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TabActivateMsg:
		return t, t.loadRemoteState(true)

	case TabDeactivateMsg:
		return t, nil

	case remoteStateMsg:
		t.applyState(msg)
		if t.mode == remoteModeSudoPrompt {
			return t, t.sudoInput.Cursor.BlinkCmd()
		}
		return t, nil

	case remoteTickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, t.loadRemoteState(false))
		cmds = append(cmds, t.scheduleTick())
		return t, tea.Batch(cmds...)

	case remoteSudoResultMsg:
		if msg.ok {
			t.sudoPassword = msg.password
		} else {
			t.sudoPassword = ""
			t.err = "sudo authentication failed"
		}
		t.mode = t.detectMode()
		return t, t.loadRemoteState(true)

	case remoteExecReturnMsg:
		if msg.err != nil {
			t.err = msg.err.Error()
		}
		t.confirmDisconnect = false
		return t, t.loadRemoteState(true)

	case tea.KeyMsg:
		return t.handleKey(msg)

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		return t, nil
	}
	return t, nil
}

func (t *RemoteTab) applyState(msg remoteStateMsg) {
	if msg.err != nil {
		t.err = msg.err.Error()
		return
	}
	t.err = ""
	t.remoteState = msg.remoteState
	t.serverState = msg.serverState
	t.ifaceStatus = msg.ifaceStatus
	t.diagnostics = msg.diagnostics
	t.lastRefresh = time.Now()

	// Don't override interactive input modes
	if t.mode == remoteModeTokenInput || t.mode == remoteModeSudoPrompt {
		return
	}

	// If sudo is needed and user hasn't dismissed the prompt, show it
	if msg.needsSudo && !t.sudoDismissed {
		t.mode = remoteModeSudoPrompt
		t.sudoInput.SetValue("")
		t.sudoInput.Focus()
		return
	}

	t.mode = t.detectMode()
}

func (t *RemoteTab) detectMode() remoteMode {
	if t.serverState != nil && t.serverState.Running {
		return remoteModeServing
	}
	if t.remoteState != nil && t.remoteState.Connected {
		return remoteModeConnected
	}
	return remoteModeDisconnected
}

func (t *RemoteTab) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Token input mode — delegate to textinput
	if t.mode == remoteModeTokenInput {
		return t.handleTokenInputKey(msg)
	}

	// Sudo password input mode
	if t.mode == remoteModeSudoPrompt {
		return t.handleSudoInputKey(msg)
	}

	// Disconnect confirmation
	if t.confirmDisconnect {
		switch msg.String() {
		case "y", "Y":
			t.confirmDisconnect = false
			return t, t.execDisconnect()
		default:
			t.confirmDisconnect = false
			return t, nil
		}
	}

	switch t.detectMode() {
	case remoteModeDisconnected:
		switch msg.String() {
		case "c":
			t.mode = remoteModeTokenInput
			t.tokenInput.SetValue("")
			t.tokenInput.Focus()
			return t, t.tokenInput.Cursor.BlinkCmd()
		}

	case remoteModeConnected:
		switch msg.String() {
		case "D":
			t.confirmDisconnect = true
			return t, nil
		case "R":
			t.sudoDismissed = false // allow re-prompt on manual refresh
			return t, t.loadRemoteState(true)
		}

	case remoteModeServing:
		switch msg.String() {
		case "g":
			return t, t.execGenerateToken()
		case "R":
			return t, t.loadRemoteState(true)
		}
	}

	return t, nil
}

func (t *RemoteTab) handleTokenInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.mode = t.detectMode()
		t.tokenInput.Blur()
		return t, nil
	case "enter":
		token := strings.TrimSpace(t.tokenInput.Value())
		if token == "" {
			return t, nil
		}
		t.tokenInput.Blur()
		t.mode = t.detectMode()
		return t, t.execConnect(token)
	}

	var cmd tea.Cmd
	t.tokenInput, cmd = t.tokenInput.Update(msg)
	return t, cmd
}

func (t *RemoteTab) handleSudoInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.sudoDismissed = true
		t.mode = t.detectMode()
		t.sudoInput.Blur()
		t.sudoInput.SetValue("")
		return t, nil
	case "enter":
		pw := t.sudoInput.Value()
		t.sudoInput.Blur()
		t.sudoInput.SetValue("")
		if pw == "" {
			t.sudoDismissed = true
			t.mode = t.detectMode()
			return t, nil
		}
		return t, t.cacheSudo(pw)
	}

	var cmd tea.Cmd
	t.sudoInput, cmd = t.sudoInput.Update(msg)
	return t, cmd
}

// --- View ---

func (t *RemoteTab) View() string {
	if t.mode == remoteModeTokenInput {
		return t.renderTokenInput()
	}
	if t.mode == remoteModeSudoPrompt {
		return t.renderSudoPrompt()
	}
	if t.confirmDisconnect {
		return t.renderDisconnectConfirm()
	}

	switch t.detectMode() {
	case remoteModeConnected:
		return t.renderConnected()
	case remoteModeServing:
		return t.renderServing()
	default:
		return t.renderDisconnected()
	}
}

func (t *RemoteTab) renderHeader(modeLabel string) string {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)

	left := bold.Render("  Remote Mode  ") + renderGradientText(modeLabel, getBrandGradient())

	right := dim.Render(fmt.Sprintf("↻ %ds", int(remoteRefreshInterval.Seconds())))

	rightMargin := 2
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	padding := t.width - leftWidth - rightWidth - rightMargin
	if padding < 1 {
		padding = 1
	}

	return left + strings.Repeat(" ", padding) + right
}

func (t *RemoteTab) renderConnected() string {
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
	label := lipgloss.NewStyle().Foreground(ui.ColorDim)
	value := lipgloss.NewStyle().Foreground(ui.ColorText)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(t.renderHeader("connected"))
	sb.WriteString("\n\n")

	// Connection details
	rs := t.remoteState
	if rs != nil {
		details := []struct{ k, v string }{
			{"Server", extractHost(rs.ServerEndpoint)},
			{"Local IP", stripCIDR(rs.LocalIP)},
			{"Server IP", stripCIDR(rs.ServerIP)},
			{"Connected", timeAgo(rs.ConnectedAt)},
		}
		for _, d := range details {
			if d.v == "" || d.v == "-" {
				continue
			}
			sb.WriteString("  " + label.Render("• "+d.k) + "  " + value.Render(d.v) + "\n")
		}
	}

	// Health checks
	if len(t.diagnostics) > 0 {
		sb.WriteString("\n")
		sectionTitle := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted)
		mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
			Render(strings.Repeat("─", t.width-4))

		sb.WriteString("  " + sectionTitle.Render("Health Checks") + "\n")
		sb.WriteString("  " + mutedLine + "\n")

		pass := lipgloss.NewStyle().Foreground(ui.ColorSuccess)
		fail := lipgloss.NewStyle().Foreground(ui.ColorError)

		for _, d := range t.diagnostics {
			var icon string
			if d.Status {
				icon = pass.Render("✓")
			} else {
				icon = fail.Render("✗")
			}
			checkName := lipgloss.NewStyle().Foreground(ui.ColorText).Width(24).Render(d.Check)
			msg := dim.Render(d.Message)
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", icon, checkName, msg))
		}

		sb.WriteString("  " + mutedLine + "\n")
	}

	// Error
	if t.err != "" {
		sb.WriteString("\n")
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err) + "\n")
	}

	return sb.String()
}

func (t *RemoteTab) renderDisconnected() string {
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)

	var sb strings.Builder
	sb.WriteString("\n")

	bold := lipgloss.NewStyle().Bold(true)
	sb.WriteString(bold.Render("  Remote Mode  ") + renderGradientText("disconnected", getBrandGradient()))
	sb.WriteString("\n\n")

	sb.WriteString("  " + dim.Render("Not connected to any remote server.") + "\n")
	sb.WriteString("\n")

	// WireGuard status
	wgInstalled := remote.IsWireGuardInstalled()
	if wgInstalled {
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render("✓") +
			" " + dim.Render("WireGuard installed") + "\n")
	} else {
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorWarning).Render("⚠") +
			" " + dim.Render("WireGuard not installed (will auto-install on connect)") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + dim.Render("Press c to enter an invite token and connect.") + "\n")

	// Error
	if t.err != "" {
		sb.WriteString("\n")
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err) + "\n")
	}

	return sb.String()
}

func (t *RemoteTab) renderTokenInput() string {
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)

	var sb strings.Builder
	sb.WriteString("\n")

	bold := lipgloss.NewStyle().Bold(true)
	sb.WriteString(bold.Render("  Remote Mode  ") + renderGradientText("connect", getBrandGradient()))
	sb.WriteString("\n\n")

	sb.WriteString("  " + dim.Render("Paste an invite token to connect:") + "\n\n")
	sb.WriteString("  " + t.tokenInput.View() + "\n\n")
	sb.WriteString("  " + dim.Render("enter connect  esc cancel") + "\n")

	return sb.String()
}

func (t *RemoteTab) renderDisconnectConfirm() string {
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
	warn := lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(t.renderHeader("connected"))
	sb.WriteString("\n\n")
	sb.WriteString("  " + warn.Render("Disconnect from remote server?") + "\n\n")
	sb.WriteString("  " + dim.Render("This will tear down the WireGuard tunnel and remove the kubeconfig.") + "\n\n")
	sb.WriteString("  " + dim.Render("y confirm  any other key cancel") + "\n")
	return sb.String()
}

func (t *RemoteTab) renderServing() string {
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
	label := lipgloss.NewStyle().Foreground(ui.ColorDim)
	value := lipgloss.NewStyle().Foreground(ui.ColorText)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(t.renderHeader("serving"))
	sb.WriteString("\n\n")

	ss := t.serverState
	if ss != nil {
		// Parse endpoint from server IP + port
		endpoint := fmt.Sprintf(":%d", ss.ListenPort)
		// Server info
		details := []struct{ k, v string }{
			{"Server IP", stripCIDR(ss.ServerIP)},
			{"Listen Port", fmt.Sprintf("%d", ss.ListenPort)},
			{"Endpoint", endpoint},
			{"Clients", fmt.Sprintf("%d", len(ss.Clients))},
		}
		for _, d := range details {
			sb.WriteString("  " + label.Render("• "+d.k) + "  " + value.Render(d.v) + "\n")
		}

		// Client table
		if len(ss.Clients) > 0 {
			sb.WriteString("\n")
			sectionTitle := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorMuted)
			mutedLine := lipgloss.NewStyle().Foreground(ui.ColorMuted).
				Render(strings.Repeat("─", t.width-4))

			sb.WriteString("  " + sectionTitle.Render("Registered Clients") + "\n")
			sb.WriteString("  " + mutedLine + "\n")

			rows := make([][]string, len(ss.Clients))
			for i, c := range ss.Clients {
				rows[i] = []string{
					c.Name,
					stripCIDR(c.AssignedIP),
					c.Hostname,
					timeAgo(c.RegisteredAt),
				}
			}

			tbl := table.New().
				Headers("Name", "IP", "Hostname", "Registered").
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
					return s.Foreground(ui.ColorText)
				})

			for _, line := range strings.Split(tbl.Render(), "\n") {
				sb.WriteString("  " + line + "\n")
			}

			sb.WriteString("  " + mutedLine + "\n")
		}
	}

	// Interface status
	if t.ifaceStatus != "" {
		sb.WriteString("\n")
		var statusText string
		switch t.ifaceStatus {
		case "up":
			statusText = lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render("✓") +
				" " + dim.Render("WireGuard interface is up")
		case "down":
			statusText = lipgloss.NewStyle().Foreground(ui.ColorError).Render("✗") +
				" " + dim.Render("WireGuard interface is down")
		default:
			statusText = lipgloss.NewStyle().Foreground(ui.ColorWarning).Render("?") +
				" " + dim.Render("WireGuard interface status unknown")
		}
		sb.WriteString("  " + statusText + "\n")
	}

	// Error
	if t.err != "" {
		sb.WriteString("\n")
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorError).Render(t.err) + "\n")
	}

	return sb.String()
}

// --- Commands ---

// loadRemoteState returns a tea.Cmd that loads remote and server state.
// If withDiagnostics is true and the client is connected, diagnostics are also run.
// When diagnostics need sudo but no password is stored, it returns needsSudo=true
// so the TUI can prompt for the password inline.
func (t *RemoteTab) loadRemoteState(withDiagnostics bool) tea.Cmd {
	sudoPass := t.sudoPassword
	return func() tea.Msg {
		rs, err := remote.LoadRemoteState()
		if err != nil {
			return remoteStateMsg{err: err}
		}

		ss, err := remote.LoadServerState()
		if err != nil {
			// Non-fatal — serve.json may not exist
			ss = nil
		}

		ifaceStatus := remote.InterfaceStatus()

		var diag []remote.DiagnosticResult
		var needsSudo bool
		if withDiagnostics && rs.Connected {
			if sudoPass != "" {
				// Have password — pipe it via sudo -S to diagnostics
				diag = remote.RunDiagnostics(rs, sudoPass)
			} else {
				// No password yet — ask the TUI to prompt
				needsSudo = true
			}
		}

		return remoteStateMsg{
			remoteState: rs,
			serverState: ss,
			ifaceStatus: ifaceStatus,
			diagnostics: diag,
			needsSudo:   needsSudo,
		}
	}
}

func (t *RemoteTab) scheduleTick() tea.Cmd {
	return tea.Tick(remoteRefreshInterval, func(_ time.Time) tea.Msg {
		return remoteTickMsg{}
	})
}

// execConnect runs `vibespace remote connect <token>` via tea.ExecProcess.
func (t *RemoteTab) execConnect(token string) tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg {
			return remoteExecReturnMsg{err: fmt.Errorf("cannot find vibespace binary: %w", err)}
		}
	}
	cmd := exec.Command("sudo", bin, "remote", "connect", token)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return remoteExecReturnMsg{err: err}
	})
}

// execDisconnect runs `vibespace remote disconnect` via tea.ExecProcess.
func (t *RemoteTab) execDisconnect() tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg {
			return remoteExecReturnMsg{err: fmt.Errorf("cannot find vibespace binary: %w", err)}
		}
	}
	cmd := exec.Command("sudo", bin, "remote", "disconnect")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return remoteExecReturnMsg{err: err}
	})
}

// execGenerateToken runs `vibespace serve --generate-token` via tea.ExecProcess.
func (t *RemoteTab) execGenerateToken() tea.Cmd {
	bin, err := os.Executable()
	if err != nil {
		return func() tea.Msg {
			return remoteExecReturnMsg{err: fmt.Errorf("cannot find vibespace binary: %w", err)}
		}
	}
	cmd := exec.Command(bin, "serve", "--generate-token")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return remoteExecReturnMsg{err: err}
	})
}

// --- Helpers ---

// cacheSudo validates the password via `sudo -S true`, then returns
// a remoteSudoResultMsg with the password on success.
func (t *RemoteTab) cacheSudo(pw string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sudo", "-S", "true")
		cmd.Stdin = strings.NewReader(pw + "\n")
		err := cmd.Run()
		return remoteSudoResultMsg{ok: err == nil, password: pw}
	}
}

func (t *RemoteTab) renderSudoPrompt() string {
	dim := lipgloss.NewStyle().Foreground(ui.ColorDim)
	warn := lipgloss.NewStyle().Foreground(ui.ColorWarning)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(t.renderHeader("connected"))
	sb.WriteString("\n\n")
	sb.WriteString("  " + warn.Render("sudo required") + " " + dim.Render("for WireGuard diagnostics") + "\n\n")
	sb.WriteString("  " + dim.Render("Password:") + " " + t.sudoInput.View() + "\n\n")
	sb.WriteString("  " + dim.Render("enter submit  esc skip") + "\n")
	return sb.String()
}

// extractHost returns the host part from a "host:port" endpoint string.
func extractHost(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	if idx := strings.LastIndex(endpoint, ":"); idx > 0 {
		return endpoint[:idx]
	}
	return endpoint
}

// stripCIDR removes the /prefix from an IP address (e.g., "10.100.0.2/32" → "10.100.0.2").
func stripCIDR(ip string) string {
	if idx := strings.Index(ip, "/"); idx > 0 {
		return ip[:idx]
	}
	return ip
}

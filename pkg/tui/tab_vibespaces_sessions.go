package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// ensureSSHForwardForAgent ensures daemon is running and an SSH forward exists for the agent.
// Returns the local port for SSH access.
func ensureSSHForwardForAgent(vsName, agentName string) (int, error) {
	if !daemon.IsDaemonRunning() {
		if err := daemon.SpawnDaemon(); err != nil {
			return 0, fmt.Errorf("start daemon: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	client, err := daemon.NewClient()
	if err != nil {
		return 0, fmt.Errorf("connect to daemon: %w", err)
	}

	if port, ok := findSSHForward(client, vsName, agentName); ok {
		return port, nil
	}

	_ = client.Refresh()
	time.Sleep(2 * time.Second)

	if port, ok := findSSHForward(client, vsName, agentName); ok {
		return port, nil
	}

	return 0, fmt.Errorf("no active SSH forward for %s/%s", vsName, agentName)
}

func loadAgentSessionsCmd(vsName, agentName string, agentType agent.Type) tea.Cmd {
	return func() tea.Msg {
		sshPort, err := ensureSSHForwardForAgent(vsName, agentName)
		if err != nil {
			return vsSessionsLoadedMsg{vsName: vsName, agentName: agentName, err: fmt.Errorf("SSH forward: %w", err)}
		}

		keyPath := vibespace.GetSSHPrivateKeyPath()
		if keyPath == "" {
			return vsSessionsLoadedMsg{vsName: vsName, agentName: agentName, err: fmt.Errorf("no SSH key found")}
		}

		var remoteCmd string
		switch agentType {
		case agent.TypeCodex:
			remoteCmd = "cat ~/.codex/history.jsonl 2>/dev/null || true"
		default:
			remoteCmd = "cat ~/.claude/history.jsonl 2>/dev/null || true"
		}

		cmd := exec.Command("ssh",
			"-i", keyPath,
			"-p", strconv.Itoa(sshPort),
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
			"-o", "ConnectTimeout=5",
			"user@localhost",
			remoteCmd,
		)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			detail := stderr.String()
			if detail == "" {
				detail = err.Error()
			}
			return vsSessionsLoadedMsg{vsName: vsName, agentName: agentName, err: fmt.Errorf("read sessions: %s", strings.TrimSpace(detail))}
		}

		var sessions []vsSessionInfo
		switch agentType {
		case agent.TypeCodex:
			sessions = parseCodexHistory(output)
		default:
			sessions = parseHistoryJSONL(output, "/vibespace")
		}
		return vsSessionsLoadedMsg{vsName: vsName, agentName: agentName, sessions: sessions}
	}
}

func prepareSessionResumeCmd(vsName, agentName string, agentType agent.Type, sessionID string, cfg *agent.Config) tea.Cmd {
	return func() tea.Msg {
		sshPort, err := ensureSSHForwardForAgent(vsName, agentName)
		if err != nil {
			return vsConnectReadyMsg{agentName: agentName, agentType: agentType, sessionID: sessionID, mode: vsConnectModeSessionResume, err: err}
		}
		return vsConnectReadyMsg{sshPort: sshPort, agentName: agentName, agentType: agentType, sessionID: sessionID, mode: vsConnectModeSessionResume, agentConfig: cfg}
	}
}

func execSessionResumeCmd(sshPort int, agentName string, agentType agent.Type, sessionID string, cfg *agent.Config) tea.Cmd {
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return func() tea.Msg {
			return vsSessionResumeMsg{err: fmt.Errorf("no SSH key found")}
		}
	}

	agentImpl := agent.MustGet(agentType)
	agentArgs := agentImpl.BuildInteractiveCommand(sessionID, cfg)
	remoteCmd := agent.WrapForSSHRemote(agentArgs)

	slog.Debug("resuming session", "agent", agentName, "session", sessionID, "type", agentType)

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t",
		"user@localhost",
		remoteCmd,
	)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return vsSessionResumeMsg{err: err}
	})
}

func findSSHForward(client *daemon.Client, vsName, agentName string) (int, bool) {
	result, err := client.ListForwardsForVibespace(vsName)
	if err != nil || result == nil {
		return 0, false
	}
	for _, ag := range result.Agents {
		if ag.Name == agentName {
			for _, fwd := range ag.Forwards {
				if fwd.Type == "ssh" && fwd.Status == "active" {
					return fwd.LocalPort, true
				}
			}
		}
	}
	return 0, false
}

func (t *VibespacesTab) prepareShellConnectPrimary(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := svc.ListAgents(ctx, vsName)
		if err != nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: fmt.Errorf("list agents: %w", err)}
		}
		primary := primaryAgent(agents)
		if primary == nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: fmt.Errorf("no agents in %s", vsName)}
		}
		sshPort, err := ensureSSHForwardForAgent(vsName, primary.AgentName)
		if err != nil {
			return vsConnectReadyMsg{mode: vsConnectModeShell, err: err}
		}
		return vsConnectReadyMsg{sshPort: sshPort, mode: vsConnectModeShell}
	}
}

func (t *VibespacesTab) prepareAgentConnect(vsName, agentName string, agentType agent.Type) tea.Cmd {
	return func() tea.Msg {
		sshPort, err := ensureSSHForwardForAgent(vsName, agentName)
		if err != nil {
			return vsConnectReadyMsg{agentName: agentName, agentType: agentType, mode: vsConnectModeAgentCLI, err: err}
		}
		return vsConnectReadyMsg{sshPort: sshPort, agentName: agentName, agentType: agentType, mode: vsConnectModeAgentCLI}
	}
}

func (t *VibespacesTab) prepareBrowserConnect(vsName, agentName string) tea.Cmd {
	return func() tea.Msg {
		port, err := t.ensureTtydForward(vsName, agentName)
		if err != nil {
			return vsBrowserReadyMsg{err: err}
		}
		return vsBrowserReadyMsg{ttydPort: port}
	}
}

func (t *VibespacesTab) prepareBrowserConnectPrimary(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vsBrowserReadyMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := svc.ListAgents(ctx, vsName)
		if err != nil {
			return vsBrowserReadyMsg{err: fmt.Errorf("list agents: %w", err)}
		}
		primary := primaryAgent(agents)
		if primary == nil {
			return vsBrowserReadyMsg{err: fmt.Errorf("no agents in %s", vsName)}
		}
		port, err := t.ensureTtydForward(vsName, primary.AgentName)
		if err != nil {
			return vsBrowserReadyMsg{err: err}
		}
		return vsBrowserReadyMsg{ttydPort: port}
	}
}

func (t *VibespacesTab) execShellConnect(sshPort int) tea.Cmd {
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return func() tea.Msg {
			return vsExecReturnMsg{err: fmt.Errorf("no SSH key found")}
		}
	}

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t",
		"user@localhost",
	)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return vsExecReturnMsg{err: err}
	})
}

func (t *VibespacesTab) execAgentConnect(sshPort int, agentName string, agentType agent.Type) tea.Cmd {
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return func() tea.Msg {
			return vsExecReturnMsg{err: fmt.Errorf("no SSH key found")}
		}
	}

	var cfg *agent.Config
	if c, ok := t.agentConfigs[agentName]; ok {
		cfg = c
	}

	agentImpl := agent.MustGet(agentType)
	agentArgs := agentImpl.BuildInteractiveCommand("", cfg)
	remoteCmd := agent.WrapForSSHRemote(agentArgs)

	slog.Debug("agent connect", "agent", agentName, "type", agentType)

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t",
		"user@localhost",
		remoteCmd,
	)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return vsExecReturnMsg{err: err}
	})
}

func (t *VibespacesTab) ensureTtydForward(vsName, agentName string) (int, error) {
	if !daemon.IsDaemonRunning() {
		if err := daemon.SpawnDaemon(); err != nil {
			return 0, fmt.Errorf("start daemon: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	client, err := daemon.NewClient()
	if err != nil {
		return 0, fmt.Errorf("connect to daemon: %w", err)
	}

	if port, ok := findTtydForward(client, vsName, agentName); ok {
		return port, nil
	}

	_ = client.Refresh()
	time.Sleep(2 * time.Second)

	if port, ok := findTtydForward(client, vsName, agentName); ok {
		return port, nil
	}

	return 0, fmt.Errorf("no active ttyd forward for %s/%s", vsName, agentName)
}

func findTtydForward(client *daemon.Client, vsName, agentName string) (int, bool) {
	result, err := client.ListForwardsForVibespace(vsName)
	if err != nil || result == nil {
		return 0, false
	}
	for _, ag := range result.Agents {
		if ag.Name == agentName {
			for _, fwd := range ag.Forwards {
				if fwd.Type == "ttyd" && fwd.Status == "active" {
					return fwd.LocalPort, true
				}
			}
		}
	}
	return 0, false
}

func primaryAgent(agents []vibespace.AgentInfo) *vibespace.AgentInfo {
	for i := range agents {
		if agents[i].AgentNum == 1 {
			return &agents[i]
		}
	}
	if len(agents) > 0 {
		return &agents[0]
	}
	return nil
}

func openBrowserURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Run()
}

type historyEntry struct {
	Display   string `json:"display"`
	Timestamp int64  `json:"timestamp"`
	Project   string `json:"project"`
	SessionID string `json:"sessionId"`
}

func parseHistoryJSONL(data []byte, project string) []vsSessionInfo {
	sessions := map[string]*vsSessionInfo{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		var entry historyEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Project != project || entry.SessionID == "" {
			continue
		}

		s, ok := sessions[entry.SessionID]
		if !ok {
			// Clean up display text for title
			title := strings.TrimSpace(entry.Display)
			title = strings.ReplaceAll(title, "\n", " ")
			s = &vsSessionInfo{
				ID:    entry.SessionID,
				Title: title,
			}
			sessions[entry.SessionID] = s
		}

		s.Prompts++
		ts := time.UnixMilli(entry.Timestamp)
		if ts.After(s.LastTime) {
			s.LastTime = ts
		}
	}

	// Sort by last activity, most recent first
	result := make([]vsSessionInfo, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTime.After(result[j].LastTime)
	})

	return result
}

type codexHistoryEntry struct {
	SessionID string `json:"session_id"`
	Timestamp int64  `json:"ts"`
	Text      string `json:"text"`
}

func parseCodexHistory(data []byte) []vsSessionInfo {
	sessions := map[string]*vsSessionInfo{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	for scanner.Scan() {
		var entry codexHistoryEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.SessionID == "" {
			continue
		}

		s, ok := sessions[entry.SessionID]
		if !ok {
			title := strings.TrimSpace(entry.Text)
			title = strings.ReplaceAll(title, "\n", " ")
			s = &vsSessionInfo{
				ID:    entry.SessionID,
				Title: title,
			}
			sessions[entry.SessionID] = s
		}

		s.Prompts++
		ts := time.Unix(entry.Timestamp, 0)
		if ts.After(s.LastTime) {
			s.LastTime = ts
		}
	}

	result := make([]vsSessionInfo, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTime.After(result[j].LastTime)
	})

	return result
}

func formatSessionAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	ago := time.Since(t)
	switch {
	case ago.Hours() >= 24*7:
		return fmt.Sprintf("%.0fw ago", ago.Hours()/(24*7))
	case ago.Hours() >= 24:
		return fmt.Sprintf("%.0fd ago", ago.Hours()/24)
	case ago.Hours() >= 1:
		return fmt.Sprintf("%.0fh ago", ago.Hours())
	default:
		return fmt.Sprintf("%.0fm ago", ago.Minutes())
	}
}

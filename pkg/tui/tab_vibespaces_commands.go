package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/config"
	vsdns "github.com/vibespacehq/vibespace/pkg/dns"
	"github.com/vibespacehq/vibespace/pkg/github"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

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
		// Fetch extra lines so we have enough after filtering noise
		logs, err := svc.GetLogs(ctx, vsName, 100)
		if err != nil {
			return vsLogsLoadedMsg{vibespaceID: vsID, err: err}
		}
		raw := strings.Split(strings.TrimRight(logs, "\n"), "\n")
		lines := filterContainerLogs(raw, 8)
		return vsLogsLoadedMsg{vibespaceID: vsID, lines: lines}
	}
}

// filterContainerLogs keeps only meaningful log lines, dropping supervisord noise.
// It returns at most maxLines lines.
func filterContainerLogs(raw []string, maxLines int) []string {
	var filtered []string
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		lower := strings.ToLower(line)

		// Skip known noise lines
		if strings.Contains(lower, "syslogin_perform_logout") ||
			strings.Contains(lower, "received disconnect from") {
			continue
		}

		switch {
		case strings.Contains(line, "[vibespace]"):
			// Entrypoint logs (clone, credentials, startup)
		case strings.Contains(line, "[github-refresh]"):
			// Token refresh daemon
		case strings.Contains(lower, "accepted publickey") ||
			strings.Contains(lower, "session opened") ||
			strings.Contains(lower, "session closed") ||
			strings.Contains(lower, "disconnected from user"):
			// SSH access logs — strip verbose key info
			if idx := strings.Index(line, " ssh2:"); idx > 0 {
				line = line[:idx]
			}
		case strings.Contains(lower, "error"):
			// Any error
		case strings.Contains(lower, "warn"):
			// Any warning
		case strings.Contains(lower, "fatal"):
			// Fatal errors
		case strings.Contains(lower, "panic"):
			// Go panics
		case strings.Contains(lower, "oom"):
			// Out of memory
		default:
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) > maxLines {
		filtered = filtered[len(filtered)-maxLines:]
	}
	return filtered
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

func (t *VibespacesTab) loadSessions(vsName, agentName string, agentType agent.Type) tea.Cmd {
	return loadAgentSessionsCmd(vsName, agentName, agentType)
}

func (t *VibespacesTab) prepareSessionResume(vsName, agentName string, agentType agent.Type, sessionID string) tea.Cmd {
	var cfg *agent.Config
	if c, ok := t.agentConfigs[agentName]; ok {
		cfg = c
	}
	return prepareSessionResumeCmd(vsName, agentName, agentType, sessionID, cfg)
}

func (t *VibespacesTab) submitCreateForm() tea.Cmd {
	repo := t.createRepo

	// If HTTPS repo and no GitHub tokens yet, start device flow first
	if repo != "" && strings.HasPrefix(repo, "https://") && t.githubAccessToken == "" {
		clientID := config.Global().GitHub.ClientID
		return func() tea.Msg {
			resp, err := github.RequestDeviceCode(context.Background(), clientID, "repo")
			return vsGithubDeviceCodeMsg{resp: resp, err: err}
		}
	}

	svc := t.shared.Vibespace
	name := t.createName
	agentType := t.createAgentType
	cpu := t.createCPU
	memory := t.createMemory
	storage := t.createStorage
	worktree := t.createWorktree
	branch := t.createBranch
	accessToken := t.githubAccessToken
	refreshToken := t.githubRefreshToken

	// Reset GitHub auth state for next create
	t.githubAccessToken = ""
	t.githubRefreshToken = ""

	return func() tea.Msg {
		if svc == nil {
			return vsCreateDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cfg := config.Global()
		req := &model.CreateVibespaceRequest{
			Name:               name,
			Persistent:         true,
			AgentType:          agentType,
			GithubRepo:         repo,
			Worktree:           worktree,
			WorktreeBranch:     branch,
			GithubAccessToken:  accessToken,
			GithubRefreshToken: refreshToken,
			ShareCredentials:   cfg.Agent.ShareCredentials,
			Resources: &model.Resources{
				CPU:         cpu,
				CPULimit:    cfg.Resources.CPULimit,
				Memory:      memory,
				MemoryLimit: cfg.Resources.MemoryLimit,
				Storage:     storage,
			},
		}
		agentCfg := &agent.Config{
			SkipPermissions: cfg.Agent.SkipPermissions,
			Model:           cfg.Agent.Model,
			MaxTurns:        cfg.Agent.MaxTurns,
		}
		if !agentCfg.IsEmpty() {
			req.AgentConfig = agentCfg
		}

		_, err := svc.Create(ctx, req)
		return vsCreateDoneMsg{err: err}
	}
}

func (t *VibespacesTab) submitDelete() tea.Cmd {
	svc := t.shared.Vibespace
	name := t.deleteName

	return func() tea.Msg {
		if svc == nil {
			return vsDeleteDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := svc.Delete(ctx, name, &vibespace.DeleteOptions{})
		return vsDeleteDoneMsg{err: err}
	}
}

func (t *VibespacesTab) toggleStartStop(name, status string) tea.Cmd {
	svc := t.shared.Vibespace

	return func() tea.Msg {
		if svc == nil {
			return vsStartStopDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if status == "running" {
			return vsStartStopDoneMsg{action: "stop", err: svc.Stop(ctx, name)}
		}
		return vsStartStopDoneMsg{action: "start", err: svc.Start(ctx, name)}
	}
}

func (t *VibespacesTab) deleteAgent(vsName, agentName string) tea.Cmd {
	svc := t.shared.Vibespace

	return func() tea.Msg {
		if svc == nil {
			return vsDeleteAgentDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := svc.KillAgent(ctx, vsName, agentName)
		return vsDeleteAgentDoneMsg{err: err}
	}
}

func (t *VibespacesTab) toggleAgentStartStop(vsName, agentName, status string) tea.Cmd {
	svc := t.shared.Vibespace

	return func() tea.Msg {
		if svc == nil {
			return vsAgentStartStopDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if status == "running" {
			return vsAgentStartStopDoneMsg{action: "stop", err: svc.StopAgent(ctx, vsName, agentName)}
		}
		return vsAgentStartStopDoneMsg{action: "start", err: svc.StartAgent(ctx, vsName, agentName)}
	}
}

func (t *VibespacesTab) submitAddAgent() tea.Cmd {
	svc := t.shared.Vibespace
	if t.selectedVS == nil {
		return nil
	}
	vsName := t.selectedVS.Name
	agentType := t.addAgentType
	agentName := t.addAgentName
	agentBranch := t.addAgentBranch
	shareCreds := t.addAgentShareCreds
	skipPerms := t.addAgentSkipPerms
	modelName := t.addAgentModel
	maxTurnsStr := t.addAgentMaxTurns

	// Collect selected tools preserving order from the tools list
	var allowedTools, disallowedTools []string
	for _, tool := range t.addAgentToolsList {
		if t.addAgentAllowedSet[tool] {
			allowedTools = append(allowedTools, tool)
		}
		if t.addAgentDisallowedSet[tool] {
			disallowedTools = append(disallowedTools, tool)
		}
	}

	return func() tea.Msg {
		if svc == nil {
			return vsAddAgentDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		opts := &vibespace.SpawnAgentOptions{
			Name:             agentName,
			AgentType:        agentType,
			ShareCredentials: shareCreds,
			Branch:           agentBranch,
		}

		// Build agent config if any config flags are set
		maxTurns, _ := strconv.Atoi(maxTurnsStr)
		if skipPerms || len(allowedTools) > 0 || len(disallowedTools) > 0 || modelName != "" || maxTurns > 0 {
			cfg := &agent.Config{
				SkipPermissions: skipPerms,
				Model:           modelName,
				MaxTurns:        maxTurns,
				AllowedTools:    allowedTools,
				DisallowedTools: disallowedTools,
			}
			opts.Config = cfg
		}

		_, err := svc.SpawnAgent(ctx, vsName, opts)
		return vsAddAgentDoneMsg{err: err}
	}
}

func (t *VibespacesTab) submitEditConfig() tea.Cmd {
	svc := t.shared.Vibespace
	if t.selectedVS == nil {
		return nil
	}
	vsName := t.selectedVS.Name
	agentName := t.editConfigAgentName
	modelName := t.editConfigModel
	maxTurnsStr := t.editConfigMaxTurns
	skipPerms := t.editConfigSkipPerms

	// Collect selected tools preserving order from the tools list
	var allowedTools, disallowedTools []string
	for _, tool := range t.editConfigToolsList {
		if t.editConfigAllowedSet[tool] {
			allowedTools = append(allowedTools, tool)
		}
		if t.editConfigDisallowedSet[tool] {
			disallowedTools = append(disallowedTools, tool)
		}
	}

	return func() tea.Msg {
		if svc == nil {
			return vsEditConfigDoneMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		maxTurns, _ := strconv.Atoi(maxTurnsStr)
		cfg := &agent.Config{
			SkipPermissions: skipPerms,
			Model:           modelName,
			MaxTurns:        maxTurns,
			AllowedTools:    allowedTools,
			DisallowedTools: disallowedTools,
		}

		err := svc.UpdateAgentConfig(ctx, vsName, agentName, cfg)
		return vsEditConfigDoneMsg{err: err}
	}
}

func (t *VibespacesTab) submitAddForward() tea.Cmd {
	dc := t.shared.Daemon
	if t.selectedVS == nil || t.agentCursor >= len(t.viewAgents) {
		return nil
	}
	vsName := t.selectedVS.Name
	agentName := t.viewAgents[t.agentCursor].AgentName
	remoteStr := t.fwdManagerAddRemote
	localStr := t.fwdManagerAddLocal
	enableDNS := t.fwdManagerAddDNS
	dnsName := t.fwdManagerAddDNSName

	sudoPass := t.sudoPassword

	return func() tea.Msg {
		if dc == nil {
			return vsAddForwardDoneMsg{err: fmt.Errorf("daemon not available")}
		}
		remotePort, err := strconv.Atoi(remoteStr)
		if err != nil {
			return vsAddForwardDoneMsg{err: fmt.Errorf("invalid remote port: %s", remoteStr)}
		}
		localPort := 0
		if localStr != "" {
			localPort, err = strconv.Atoi(localStr)
			if err != nil {
				return vsAddForwardDoneMsg{err: fmt.Errorf("invalid local port: %s", localStr)}
			}
		}

		result, err := dc.AddForwardForVibespace(vsName, agentName, remotePort, localPort, enableDNS, dnsName)
		if err != nil {
			return vsAddForwardDoneMsg{err: err}
		}
		if result != nil && result.DNSName != "" {
			if hostErr := vsdns.AddHostEntry(result.DNSName, sudoPass); errors.Is(hostErr, vsdns.ErrSudoRequired) {
				return vsAddForwardDoneMsg{dnsName: result.DNSName}
			}
		}
		return vsAddForwardDoneMsg{}
	}
}

func (t *VibespacesTab) submitRemoveForward(remotePort int) tea.Cmd {
	dc := t.shared.Daemon
	if t.selectedVS == nil || t.agentCursor >= len(t.viewAgents) {
		return nil
	}
	vsName := t.selectedVS.Name
	agentName := t.viewAgents[t.agentCursor].AgentName
	sudoPass := t.sudoPassword

	return func() tea.Msg {
		if dc == nil {
			return vsRemoveForwardDoneMsg{err: fmt.Errorf("daemon not available")}
		}
		// Look up DNS name before removing so we can clean up /etc/hosts
		var dnsName string
		if list, err := dc.ListForwardsForVibespace(vsName); err == nil {
			for _, a := range list.Agents {
				if a.Name == agentName {
					for _, fwd := range a.Forwards {
						if fwd.RemotePort == remotePort && fwd.DNSName != "" {
							dnsName = fwd.DNSName
						}
					}
				}
			}
		}
		err := dc.RemoveForwardForVibespace(vsName, agentName, remotePort)
		if err != nil {
			return vsRemoveForwardDoneMsg{err: err}
		}
		if dnsName != "" {
			if hostErr := vsdns.RemoveHostEntry(dnsName, sudoPass); errors.Is(hostErr, vsdns.ErrSudoRequired) {
				return vsRemoveForwardDoneMsg{dnsName: dnsName}
			}
		}
		return vsRemoveForwardDoneMsg{}
	}
}

func (t *VibespacesTab) submitToggleDNS(remotePort int, currentDNS string) tea.Cmd {
	dc := t.shared.Daemon
	if t.selectedVS == nil || t.agentCursor >= len(t.viewAgents) {
		return nil
	}
	vsName := t.selectedVS.Name
	agentName := t.viewAgents[t.agentCursor].AgentName
	sudoPass := t.sudoPassword

	return func() tea.Msg {
		if dc == nil {
			return vsToggleDNSDoneMsg{err: fmt.Errorf("daemon not available")}
		}
		result, err := dc.UpdateForwardDNS(vsName, agentName, remotePort, "")
		if err != nil {
			return vsToggleDNSDoneMsg{err: err}
		}
		if result.DNSName != "" {
			// DNS was added — update /etc/hosts
			if hostErr := vsdns.AddHostEntry(result.DNSName, sudoPass); errors.Is(hostErr, vsdns.ErrSudoRequired) {
				return vsToggleDNSDoneMsg{dnsName: result.DNSName}
			}
			return vsToggleDNSDoneMsg{}
		}
		// DNS was removed — clean up /etc/hosts
		if currentDNS != "" {
			if hostErr := vsdns.RemoveHostEntry(currentDNS, sudoPass); errors.Is(hostErr, vsdns.ErrSudoRequired) {
				return vsToggleDNSDoneMsg{oldDNS: currentDNS}
			}
		}
		return vsToggleDNSDoneMsg{}
	}
}

func (t *VibespacesTab) validateSudo(pw string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sudo", "-S", "true")
		cmd.Stdin = strings.NewReader(pw + "\n")
		err := cmd.Run()
		return vsSudoDoneMsg{ok: err == nil, password: pw}
	}
}

func (t *VibespacesTab) retryDNSHostEntry() tea.Cmd {
	op := t.sudoPendingOp
	name := t.sudoPendingDNS
	pass := t.sudoPassword
	t.sudoPendingDNS = ""
	t.sudoPendingOp = ""

	return func() tea.Msg {
		switch op {
		case "add":
			vsdns.AddHostEntry(name, pass)
		case "remove":
			vsdns.RemoveHostEntry(name, pass)
		}
		return nil
	}
}

func (t *VibespacesTab) scheduleRefreshIfNeeded() tea.Cmd {
	for _, vs := range t.vibespaces {
		if vs.Status == "creating" {
			return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return vsRefreshTickMsg{}
			})
		}
	}
	return nil
}

func agentSupportedTools(agentType agent.Type) []string {
	impl, err := agent.Get(agentType)
	if err != nil {
		return nil
	}
	return impl.SupportedTools()
}

// excludedTools returns tools from SupportedTools that are not in the allowed list.
// Handles parameterized tools like "Bash(npm run *)" by comparing base names.
func excludedTools(agentType agent.Type, allowed []string) []string {
	supported := agentSupportedTools(agentType)
	allowedBase := make(map[string]bool, len(allowed))
	for _, t := range allowed {
		base := t
		if idx := strings.Index(t, "("); idx >= 0 {
			base = t[:idx]
		}
		allowedBase[base] = true
	}
	var excluded []string
	for _, t := range supported {
		base := t
		if idx := strings.Index(t, "("); idx >= 0 {
			base = t[:idx]
		}
		if !allowedBase[base] {
			excluded = append(excluded, t)
		}
	}
	return excluded
}

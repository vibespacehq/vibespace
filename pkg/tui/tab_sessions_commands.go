package tui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/ui"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// --- Tree building ---

func (t *SessionsTab) rebuildTree() {
	var items []treeItem

	// Track which multi-sessions get placed under at least one group
	placedMulti := make(map[string]bool)

	for _, vs := range t.vibespaces {
		expanded := t.expandedVS[vs.Name]
		gs := t.groupStates[vs.Name]

		singleCount := 0
		multiCount := 0
		loading := false

		if gs != nil {
			loading = gs.Loading
			for _, sessions := range gs.AgentSessions {
				singleCount += len(sessions)
			}
		}
		for _, ms := range t.multiSessions {
			for _, ve := range ms.Vibespaces {
				if ve.Name == vs.Name {
					multiCount++
					placedMulti[ms.Name] = true
					break
				}
			}
		}

		items = append(items, treeItem{
			Type:        sessionItemGroup,
			VSName:      vs.Name,
			VSStatus:    vs.Status,
			Expanded:    expanded,
			Loading:     loading,
			SingleCount: singleCount,
			MultiCount:  multiCount,
		})

		if !expanded {
			continue
		}

		// Single-agent sessions
		if gs != nil && gs.AgentsLoaded {
			for _, ag := range gs.Agents {
				if sessions, ok := gs.AgentSessions[ag.AgentName]; ok {
					for _, sess := range sessions {
						items = append(items, treeItem{
							Type:      sessionItemSingle,
							Session:   sess,
							AgentName: ag.AgentName,
							AgentType: ag.AgentType,
							VSParent:  vs.Name,
						})
					}
				}
			}
		}

		// Multi-agent sessions referencing this vibespace
		for i := range t.multiSessions {
			ms := &t.multiSessions[i]
			for _, ve := range ms.Vibespaces {
				if ve.Name == vs.Name {
					items = append(items, treeItem{
						Type:         sessionItemMulti,
						MultiSession: ms,
						CrossVSCount: len(ms.Vibespaces) - 1,
						VSParent:     vs.Name,
					})
					break
				}
			}
		}
	}

	// Orphan multi-sessions (no matching vibespace group)
	var orphans []treeItem
	for i := range t.multiSessions {
		ms := &t.multiSessions[i]
		if !placedMulti[ms.Name] {
			orphans = append(orphans, treeItem{
				Type:         sessionItemMulti,
				MultiSession: ms,
				CrossVSCount: 0,
			})
		}
	}
	if len(orphans) > 0 {
		items = append(items, treeItem{
			Type:       sessionItemGroup,
			VSName:     "Ungrouped",
			VSStatus:   "",
			Expanded:   true,
			MultiCount: len(orphans),
		})
		items = append(items, orphans...)
	}

	// Mark last child in each group
	for i := range items {
		if items[i].Type != sessionItemGroup {
			items[i].isLastChild = i == len(items)-1 || items[i+1].Type == sessionItemGroup
		}
	}

	t.flatTree = items
	if t.cursor >= len(t.flatTree) {
		t.cursor = max(len(t.flatTree)-1, 0)
	}
}

func (t *SessionsTab) toggleGroup(vsName string) tea.Cmd {
	if t.expandedVS[vsName] {
		t.expandedVS[vsName] = false
		t.rebuildTree()
		return nil
	}

	t.expandedVS[vsName] = true

	gs := t.groupStates[vsName]
	if gs != nil && gs.AgentsLoaded {
		t.rebuildTree()
		return nil
	}

	if gs == nil {
		gs = &groupLoadState{
			AgentSessions: make(map[string][]vsSessionInfo),
			AgentConfigs:  make(map[string]*agent.Config),
		}
		t.groupStates[vsName] = gs
	}
	gs.Loading = true
	t.rebuildTree()
	return t.loadGroupAgents(vsName)
}

// --- Commands ---

func (t *SessionsTab) loadVibespacesForTree() tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vibespacesForTreeMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		vs, err := svc.List(ctx)
		return vibespacesForTreeMsg{vibespaces: vs, err: err}
	}
}

func (t *SessionsTab) loadMultiSessions() tea.Cmd {
	store := t.shared.SessionStore
	return func() tea.Msg {
		if store == nil {
			return multiSessionsLoadedMsg{err: fmt.Errorf("session store unavailable")}
		}
		sessions, err := store.List()
		return multiSessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (t *SessionsTab) loadGroupAgents(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return groupAgentsLoadedMsg{vsName: vsName, err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		agents, err := svc.ListAgents(ctx, vsName)
		if err != nil {
			return groupAgentsLoadedMsg{vsName: vsName, err: err}
		}

		configs := make(map[string]*agent.Config)
		for _, ag := range agents {
			cfg, cerr := svc.GetAgentConfig(ctx, vsName, ag.AgentName)
			if cerr == nil && cfg != nil {
				configs[ag.AgentName] = cfg
			}
		}

		return groupAgentsLoadedMsg{vsName: vsName, agents: agents, configs: configs}
	}
}

func (t *SessionsTab) deleteSession(name string) tea.Cmd {
	store := t.shared.SessionStore
	return func() tea.Msg {
		if store == nil {
			return sessionDeletedMsg{err: fmt.Errorf("session store unavailable")}
		}
		return sessionDeletedMsg{err: store.Delete(name)}
	}
}

func (t *SessionsTab) loadPreviewForCurrent() tea.Cmd {
	if t.cursor >= len(t.flatTree) {
		return nil
	}
	item := t.flatTree[t.cursor]

	switch item.Type {
	case sessionItemSingle:
		sessID := item.Session.ID
		if sessID == t.singlePreviewID {
			return nil
		}
		vsName := item.VSParent
		agentName := item.AgentName
		agentType := item.AgentType
		return func() tea.Msg {
			entries := loadSingleSessionMessages(vsName, agentName, agentType, sessID, 5)
			return singleSessionPreviewMsg{sessionID: sessID, messages: entries}
		}

	case sessionItemMulti:
		if item.MultiSession == nil {
			return nil
		}
		name := item.MultiSession.Name
		if name == t.previewSessionName {
			return nil
		}
		hs := t.shared.HistoryStore
		return func() tea.Msg {
			if hs == nil {
				return sessionHistoryMsg{sessionName: name}
			}
			allMsgs, _ := hs.Load(name)
			totalCount := len(allMsgs)
			var filtered []*Message
			for _, m := range allMsgs {
				if m.Type == MessageTypeUser || m.Type == MessageTypeAssistant || m.Type == MessageTypeToolUse {
					filtered = append(filtered, m)
				}
			}
			if len(filtered) > 5 {
				filtered = filtered[len(filtered)-5:]
			}
			return sessionHistoryMsg{sessionName: name, messages: filtered, totalCount: totalCount}
		}
	}

	return nil
}

func (t *SessionsTab) loadVibespacesForPicker() tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return vibespacesForPickerMsg{err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		vs, err := svc.List(ctx)
		return vibespacesForPickerMsg{vibespaces: vs, err: err}
	}
}

func (t *SessionsTab) loadAgentsForPicker(vsName string) tea.Cmd {
	svc := t.shared.Vibespace
	return func() tea.Msg {
		if svc == nil {
			return agentsForPickerMsg{vibespace: vsName, err: fmt.Errorf("vibespace service unavailable")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := svc.ListAgents(ctx, vsName)
		return agentsForPickerMsg{vibespace: vsName, agents: agents, err: err}
	}
}

func (t *SessionsTab) finalizeNewSession() tea.Cmd {
	store := t.shared.SessionStore
	name := t.newSessionName
	selectedVS := t.newSelectedVS
	vsAgents := t.newVSAgents
	return func() tea.Msg {
		if store == nil {
			return sessionCreatedMsg{err: fmt.Errorf("session store unavailable")}
		}
		sess, err := store.Create(name)
		if err != nil {
			return sessionCreatedMsg{err: err}
		}
		for _, vs := range selectedVS {
			agents := vsAgents[vs.Name]
			sess.AddVibespace(vs.Name, agents)
		}
		if err := store.Save(sess); err != nil {
			return sessionCreatedMsg{err: err}
		}
		return sessionCreatedMsg{session: sess}
	}
}

func (t *SessionsTab) resetNewSession() {
	t.nameInput.SetValue("")
	t.nameInput.Blur()
	t.newSessionName = ""
	t.newVibespaces = nil
	t.newVSSelected = nil
	t.newVSCursor = 0
	t.newAgents = nil
	t.newAgentSelected = nil
	t.newAgentCursor = 0
	t.newAgentVSIndex = 0
	t.newSelectedVS = nil
	t.newVSAgents = make(map[string][]string)
	t.err = ""
}

// agentColor returns a stable color for an agent name based on all known agents.
// Skips orange (reserved for ◆ multi-session markers) to avoid visual overlap.
func (t *SessionsTab) agentColor(agentName string) lipgloss.Color {
	// Collect all unique agent names in order from group states
	var allAgents []string
	seen := make(map[string]bool)
	for _, vs := range t.vibespaces {
		gs := t.groupStates[vs.Name]
		if gs == nil || !gs.AgentsLoaded {
			continue
		}
		for _, ag := range gs.Agents {
			if !seen[ag.AgentName] {
				seen[ag.AgentName] = true
				allAgents = append(allAgents, ag.AgentName)
			}
		}
	}
	// Build palette excluding orange (used for multi-session ◆)
	var palette []lipgloss.Color
	for _, c := range ui.AgentColors {
		if c != ui.Orange {
			palette = append(palette, c)
		}
	}
	if len(palette) == 0 {
		palette = ui.AgentColors
	}
	for i, name := range allAgents {
		if name == agentName {
			return palette[i%len(palette)]
		}
	}
	return palette[0]
}

// loadSingleSessionMessages SSHs into the agent pod and reads the last N
// user/assistant messages from the session's JSONL file.
func loadSingleSessionMessages(vsName, agentName string, agentType agent.Type, sessionID string, maxMsgs int) []singlePreviewEntry {
	sshPort, err := ensureSSHForwardForAgent(vsName, agentName)
	if err != nil {
		return nil
	}
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return nil
	}

	// Determine session file path based on agent type
	var sessionFile string
	switch agentType {
	case agent.TypeCodex:
		sessionFile = fmt.Sprintf("~/.codex/sessions/%s.jsonl", sessionID)
	default:
		sessionFile = fmt.Sprintf("~/.claude/projects/-vibespace/%s.jsonl", sessionID)
	}

	// Tail enough lines to get recent messages (each turn is ~1-2 lines)
	remoteCmd := fmt.Sprintf("tail -40 %s 2>/dev/null || true", sessionFile)

	cmd := exec.Command("ssh",
		"-i", keyPath,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=3",
		"user@localhost",
		remoteCmd,
	)
	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		return nil
	}

	// Parse JSONL lines for user/assistant text
	var entries []singlePreviewEntry
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		var raw struct {
			Type    string `json:"type"`
			Message struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}
		if raw.Type != "user" && raw.Type != "assistant" {
			continue
		}

		text := extractContentText(raw.Message.Content)
		if text == "" {
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, raw.Timestamp)
		entries = append(entries, singlePreviewEntry{
			Role: raw.Type,
			Text: text,
			Time: ts,
		})
	}

	if len(entries) > maxMsgs {
		entries = entries[len(entries)-maxMsgs:]
	}
	return entries
}

// extractContentText extracts text from a Claude JSONL content field.
// Content can be a plain string or an array of content blocks.
func extractContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try plain string first
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	// Try array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return strings.TrimSpace(b.Text)
			}
		}
	}
	return ""
}

// --- Helpers ---

func sessVibespaceNames(sess session.Session) string {
	if len(sess.Vibespaces) == 0 {
		return "-"
	}
	names := make([]string, len(sess.Vibespaces))
	for i, vs := range sess.Vibespaces {
		names[i] = vs.Name
	}
	return strings.Join(names, ", ")
}

func sessAgentCount(sess session.Session) string {
	seen := make(map[string]bool)
	for _, vs := range sess.Vibespaces {
		if len(vs.Agents) == 0 {
			return "all"
		}
		for _, a := range vs.Agents {
			seen[a] = true
		}
	}
	if len(seen) == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", len(seen))
}

func truncate(s string, maxLen int) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-1]) + "…"
	}
	return s
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

func newTestVibespacesTab() *VibespacesTab {
	shared := &SharedState{}
	tab := NewVibespacesTab(shared)
	tab.SetSize(120, 40)
	return tab
}

func TestVsStatusStyled(t *testing.T) {
	tests := []struct {
		status string
		expect string // text content after stripping ANSI
	}{
		{"running", "running"},
		{"stopped", "stopped"},
		{"error", "error"},
		{"creating", "creating"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		got := stripAnsi(vsStatusStyled(tt.status))
		if got != tt.expect {
			t.Errorf("vsStatusStyled(%q) = %q, want %q", tt.status, got, tt.expect)
		}
	}
}

func TestVsTimeAgo(t *testing.T) {
	// Valid RFC3339
	ts := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	got := vsTimeAgo(ts)
	if got != "5m ago" {
		t.Errorf("expected '5m ago', got %q", got)
	}

	// Invalid date returns raw string
	got = vsTimeAgo("not-a-date")
	if got != "not-a-date" {
		t.Errorf("expected raw string for invalid date, got %q", got)
	}
}

func TestVsPVCName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"12345678-abcd", "12345678-pvc"},
		{"short", "short-pvc"},
		{"12345678901234567890", "12345678-pvc"},
	}
	for _, tt := range tests {
		got := vsPVCName(tt.id)
		if got != tt.want {
			t.Errorf("vsPVCName(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestAgentImage(t *testing.T) {
	tests := []struct {
		agentType agent.Type
		expect    string
	}{
		{agent.TypeClaudeCode, "ghcr.io/vibespacehq/vibespace/claude-code:latest"},
		{agent.TypeCodex, "ghcr.io/vibespacehq/vibespace/codex:latest"},
		{agent.Type("unknown"), ""},
	}
	for _, tt := range tests {
		got := agentImage(tt.agentType)
		if got != tt.expect {
			t.Errorf("agentImage(%q) = %q, want %q", tt.agentType, got, tt.expect)
		}
	}
}

func TestAgentNames(t *testing.T) {
	agents := []vibespace.AgentInfo{
		{AgentName: "claude-1"},
		{AgentName: "claude-2"},
	}
	names := agentNames(agents)
	if len(names) != 2 {
		t.Fatalf("expected 2, got %d", len(names))
	}
	if names[0] != "claude-1" || names[1] != "claude-2" {
		t.Fatalf("expected [claude-1, claude-2], got %v", names)
	}

	// Empty
	names = agentNames(nil)
	if len(names) != 0 {
		t.Fatalf("expected 0, got %d", len(names))
	}
}

func TestVibespacesTabEmptyListView(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = nil

	// Tab's viewEmpty is a simple fallback; the full welcome cover is
	// rendered at the App level via showWelcomeCover().
	view := stripAnsi(tab.View())
	if !strings.Contains(view, "No vibespaces found") {
		t.Errorf("empty view should contain fallback message, got: %s", view)
	}
}

func TestVibespacesTabListView(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{
		{ID: "id-1", Name: "project-a", Status: "running", CreatedAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339)},
		{ID: "id-2", Name: "project-b", Status: "stopped", CreatedAt: time.Now().Format(time.RFC3339)},
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "project-a") {
		t.Error("list view should contain vibespace names")
	}
	if !strings.Contains(view, "project-b") {
		t.Error("list view should contain vibespace names")
	}
}

func TestVibespacesTabNavigation(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{
		{ID: "1", Name: "a"},
		{ID: "2", Name: "b"},
		{ID: "3", Name: "c"},
	}
	tab.selected = 0

	// j moves down
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.selected != 1 {
		t.Fatalf("expected selected=1, got %d", tab.selected)
	}

	// k moves up
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.selected != 0 {
		t.Fatalf("expected selected=0, got %d", tab.selected)
	}

	// k at top clamps
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.selected != 0 {
		t.Fatalf("expected selected=0 (clamped), got %d", tab.selected)
	}
}

func TestVibespacesTabCreateFormMode(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{
		{ID: "1", Name: "existing"},
	}

	// Key "n" enters create mode
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if tab.mode != vibespacesModeCreateForm {
		t.Fatalf("expected createForm mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Name") {
		t.Error("create form should contain 'Name'")
	}
}

func TestVibespacesTabCreateFormFieldNav(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeCreateForm
	tab.createField = createFieldName

	// Tab cycles through fields
	tab.Update(tea.KeyMsg{Type: tea.KeyTab})
	if tab.createField != createFieldAgentType {
		t.Fatalf("expected agentType field after tab, got %d", tab.createField)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyTab})
	if tab.createField != createFieldRepo {
		t.Fatalf("expected Repo field after second tab, got %d", tab.createField)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyTab})
	if tab.createField != createFieldCPU {
		t.Fatalf("expected CPU field after third tab, got %d", tab.createField)
	}
}

func TestVibespacesTabDeleteConfirmMode(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{
		{ID: "1", Name: "to-delete", Status: "stopped"},
	}
	tab.selected = 0

	// Key "d" enters delete confirm
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if tab.mode != vibespacesModeDeleteConfirm {
		t.Fatalf("expected deleteConfirm mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "delete") || !strings.Contains(view, "to-delete") {
		t.Errorf("delete confirm should contain 'delete' and name, got: %s", view)
	}
}

func TestVibespacesTabAgentViewMode(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test-vs", Status: "running"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode, Status: "running"},
		{AgentName: "claude-2", AgentType: agent.TypeClaudeCode, Status: "running"},
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "claude-1") {
		t.Error("agent view should contain agent names")
	}
	if !strings.Contains(view, "claude-2") {
		t.Error("agent view should contain agent names")
	}
}

func TestVibespacesTabAgentNavigation(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "a1"},
		{AgentName: "a2"},
		{AgentName: "a3"},
	}
	tab.agentCursor = 0

	// j moves down
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.agentCursor != 1 {
		t.Fatalf("expected agentCursor=1, got %d", tab.agentCursor)
	}

	// k moves up
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.agentCursor != 0 {
		t.Fatalf("expected agentCursor=0, got %d", tab.agentCursor)
	}
}

func TestVibespacesTabAddAgentMode(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{{AgentName: "existing"}}
	tab.agentCursor = 0

	// Key "a" enters add agent mode
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if tab.mode != vibespacesModeAddAgent {
		t.Fatalf("expected addAgent mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Agent") {
		t.Error("add agent form should contain 'Agent'")
	}
}

func TestVibespacesTabEditConfigMode(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeEditConfig
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.editConfigAgentName = "claude-1"
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode},
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "claude-1") {
		t.Error("edit config view should contain agent name")
	}
}

func TestVibespacesTabForwardManagerMode(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeForwardManager
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode},
	}
	tab.agentCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Forward") || !strings.Contains(view, "claude-1") {
		t.Errorf("forward manager should contain 'Forward' and agent name, got: %s", view)
	}
}

func TestVibespacesTabEscReturnsFromModes(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{{AgentName: "a1"}}

	// AgentView → List
	tab.mode = vibespacesModeAgentView
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeList {
		t.Fatalf("expected list mode from agentView, got %d", tab.mode)
	}

	// AddAgent → AgentView
	tab.mode = vibespacesModeAddAgent
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeAgentView {
		t.Fatalf("expected agentView mode from addAgent, got %d", tab.mode)
	}

	// EditConfig → AgentView
	tab.mode = vibespacesModeEditConfig
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeAgentView {
		t.Fatalf("expected agentView mode from editConfig, got %d", tab.mode)
	}

	// ForwardManager → AgentView
	tab.mode = vibespacesModeForwardManager
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeAgentView {
		t.Fatalf("expected agentView mode from forwardManager, got %d", tab.mode)
	}

	// CreateForm → List
	tab.mode = vibespacesModeCreateForm
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeList {
		t.Fatalf("expected list mode from createForm, got %d", tab.mode)
	}

	// DeleteConfirm → List
	tab.mode = vibespacesModeDeleteConfirm
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeList {
		t.Fatalf("expected list mode from deleteConfirm, got %d", tab.mode)
	}

	// SessionList → AgentView
	tab.mode = vibespacesModeSessionList
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeAgentView {
		t.Fatalf("expected agentView mode from sessionList, got %d", tab.mode)
	}
}

func TestVibespacesTabSessionListMode(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeSessionList
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.sessionAgent = "claude-1"
	tab.sessions = []vsSessionInfo{
		{ID: "sess-1", Title: "First session", LastTime: time.Now(), Prompts: 5},
		{ID: "sess-2", Title: "Second session", LastTime: time.Now(), Prompts: 10},
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "sess-1") && !strings.Contains(view, "First session") {
		t.Error("session list should contain session IDs or titles")
	}
}

func TestVibespacesTabSessionListNavigation(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeSessionList
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.sessions = []vsSessionInfo{
		{ID: "s1"}, {ID: "s2"}, {ID: "s3"},
	}
	tab.sessionCursor = 0

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.sessionCursor != 1 {
		t.Fatalf("expected sessionCursor=1, got %d", tab.sessionCursor)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.sessionCursor != 0 {
		t.Fatalf("expected sessionCursor=0, got %d", tab.sessionCursor)
	}
}

func TestVibespacesTabClampSelected(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{{ID: "1"}, {ID: "2"}}
	tab.selected = 5

	tab.clampSelected()
	if tab.selected != 1 {
		t.Fatalf("expected selected=1, got %d", tab.selected)
	}

	tab.vibespaces = nil
	tab.selected = 5
	tab.clampSelected()
	if tab.selected != 0 {
		t.Fatalf("expected selected=0 for empty, got %d", tab.selected)
	}
}

func TestVibespacesTabClampAgentCursor(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.viewAgents = []vibespace.AgentInfo{{AgentName: "a1"}}
	tab.agentCursor = 5

	tab.clampAgentCursor()
	if tab.agentCursor != 0 {
		t.Fatalf("expected agentCursor=0, got %d", tab.agentCursor)
	}
}

func TestVibespacesTabSelectedID(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{{ID: "abc"}, {ID: "def"}}
	tab.selected = 1

	if got := tab.selectedID(); got != "def" {
		t.Fatalf("expected 'def', got %q", got)
	}

	// Out of bounds
	tab.selected = 5
	if got := tab.selectedID(); got != "" {
		t.Fatalf("expected empty for out of bounds, got %q", got)
	}
}

func TestVibespacesTabErrorView(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = nil
	tab.err = "k8s connection failed"

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Error") || !strings.Contains(view, "k8s connection failed") {
		t.Errorf("error view should contain error message, got: %s", view)
	}
}

func TestVibespacesTabCreateFormEsc(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeCreateForm
	tab.createName = "test"

	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeList {
		t.Fatalf("expected list mode after esc, got %d", tab.mode)
	}
}

func TestVibespacesTabDeleteConfirmEsc(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeDeleteConfirm

	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeList {
		t.Fatalf("expected list mode after esc, got %d", tab.mode)
	}
}

func TestExcludedTools(t *testing.T) {
	// Get supported tools for claude-code
	supported := agentSupportedTools(agent.TypeClaudeCode)
	if len(supported) == 0 {
		t.Skip("no supported tools for claude-code")
	}

	// Allow only first tool
	allowed := []string{supported[0]}
	excluded := excludedTools(agent.TypeClaudeCode, allowed)

	if len(excluded) != len(supported)-1 {
		t.Fatalf("expected %d excluded tools, got %d", len(supported)-1, len(excluded))
	}

	// Allow all → none excluded
	excluded = excludedTools(agent.TypeClaudeCode, supported)
	if len(excluded) != 0 {
		t.Fatalf("expected 0 excluded when all allowed, got %d", len(excluded))
	}
}

func TestParseHistoryJSONL(t *testing.T) {
	// Normal entries
	data := []byte(`{"display":"hello","timestamp":1700000000000,"project":"/test","sessionId":"s1"}
{"display":"world","timestamp":1700000001000,"project":"/test","sessionId":"s1"}
{"display":"other","timestamp":1700000002000,"project":"/other","sessionId":"s2"}
{"display":"session2","timestamp":1700000003000,"project":"/test","sessionId":"s3"}
`)
	sessions := parseHistoryJSONL(data, "/test")
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Most recent first (s3 has ts 1700000003000)
	if sessions[0].ID != "s3" {
		t.Fatalf("expected s3 first, got %q", sessions[0].ID)
	}
	if sessions[1].Prompts != 2 {
		t.Fatalf("expected s1 to have 2 prompts, got %d", sessions[1].Prompts)
	}

	// Empty data
	sessions = parseHistoryJSONL(nil, "/test")
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions for nil data, got %d", len(sessions))
	}

	// Malformed lines
	data = []byte("{bad json\n{\"display\":\"ok\",\"timestamp\":1000,\"project\":\"/p\",\"sessionId\":\"s1\"}\n")
	sessions = parseHistoryJSONL(data, "/p")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after skipping malformed, got %d", len(sessions))
	}

	// Empty session ID should be skipped
	data = []byte("{\"display\":\"x\",\"timestamp\":1000,\"project\":\"/p\",\"sessionId\":\"\"}\n")
	sessions = parseHistoryJSONL(data, "/p")
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions for empty sessionId, got %d", len(sessions))
	}
}

func TestParseCodexHistory(t *testing.T) {
	data := []byte(`{"session_id":"cs1","ts":1700000000,"text":"hello"}
{"session_id":"cs1","ts":1700000001,"text":"world"}
{"session_id":"cs2","ts":1700000002,"text":"another"}
`)
	sessions := parseCodexHistory(data)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// cs2 is most recent
	if sessions[0].ID != "cs2" {
		t.Fatalf("expected cs2 first, got %q", sessions[0].ID)
	}

	// Empty data
	sessions = parseCodexHistory(nil)
	if len(sessions) != 0 {
		t.Fatalf("expected 0, got %d", len(sessions))
	}

	// Empty session_id
	data = []byte("{\"session_id\":\"\",\"ts\":1000,\"text\":\"x\"}\n")
	sessions = parseCodexHistory(data)
	if len(sessions) != 0 {
		t.Fatalf("expected 0 for empty session_id, got %d", len(sessions))
	}
}

func TestFormatSessionAge(t *testing.T) {
	tests := []struct {
		input time.Time
		want  string
	}{
		{time.Time{}, "-"},
		{time.Now().Add(-5 * time.Minute), "5m ago"},
		{time.Now().Add(-3 * time.Hour), "3h ago"},
		{time.Now().Add(-48 * time.Hour), "2d ago"},
		{time.Now().Add(-14 * 24 * time.Hour), "2w ago"},
	}
	for _, tt := range tests {
		got := formatSessionAge(tt.input)
		if got != tt.want {
			t.Errorf("formatSessionAge(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHandleCreateFormKeyTextInput(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeCreateForm
	tab.createField = createFieldName
	tab.createName = ""

	// Type a character
	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if tab.createName != "a" {
		t.Fatalf("expected createName='a', got %q", tab.createName)
	}

	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if tab.createName != "ab" {
		t.Fatalf("expected createName='ab', got %q", tab.createName)
	}

	// Backspace
	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.createName != "a" {
		t.Fatalf("expected createName='a' after backspace, got %q", tab.createName)
	}

	// Backspace on other fields
	tab.createField = createFieldCPU
	tab.createCPU = "250m"
	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.createCPU != "250" {
		t.Fatalf("expected createCPU='250', got %q", tab.createCPU)
	}

	tab.createField = createFieldMemory
	tab.createMemory = "512Mi"
	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.createMemory != "512M" {
		t.Fatalf("expected createMemory='512M', got %q", tab.createMemory)
	}

	tab.createField = createFieldStorage
	tab.createStorage = "10Gi"
	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.createStorage != "10G" {
		t.Fatalf("expected createStorage='10G', got %q", tab.createStorage)
	}
}

func TestHandleCreateFormKeyAgentTypeToggle(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeCreateForm
	tab.createField = createFieldAgentType
	tab.createAgentType = agent.TypeClaudeCode

	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.createAgentType != agent.TypeCodex {
		t.Fatalf("expected codex after j, got %q", tab.createAgentType)
	}

	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.createAgentType != agent.TypeClaudeCode {
		t.Fatalf("expected claude-code after k, got %q", tab.createAgentType)
	}
}

func TestHandleCreateFormKeyEmptyNameSubmit(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeCreateForm
	tab.createField = createFieldName
	tab.createName = ""

	// ctrl+s with empty name should not submit (returns t, nil)
	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if tab.mode != vibespacesModeCreateForm {
		t.Fatal("expected to stay in create form after ctrl+s with empty name")
	}

	// enter with empty name on name field should not advance
	tab.handleCreateFormKey(tea.KeyMsg{Type: tea.KeyEnter})
	if tab.createField != createFieldName {
		t.Fatalf("expected to stay on name field, got %d", tab.createField)
	}
}

func TestHandleDeleteConfirmKeyTextInput(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeDeleteConfirm
	tab.deleteName = "test-vs"
	tab.deleteInput = ""

	// Type characters
	tab.handleDeleteConfirmKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	tab.handleDeleteConfirmKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if tab.deleteInput != "te" {
		t.Fatalf("expected 'te', got %q", tab.deleteInput)
	}

	// Backspace
	tab.handleDeleteConfirmKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.deleteInput != "t" {
		t.Fatalf("expected 't' after backspace, got %q", tab.deleteInput)
	}

	// Enter with non-matching input should not submit
	tab.handleDeleteConfirmKey(tea.KeyMsg{Type: tea.KeyEnter})
	if tab.mode != vibespacesModeDeleteConfirm {
		t.Fatal("expected to stay in delete confirm mode with non-matching input")
	}
}

func TestHandleAddAgentKeyTypeToggle(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAddAgent
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.addAgentField = addAgentFieldType
	tab.addAgentType = agent.TypeClaudeCode
	tab.addAgentToolsList = agentSupportedTools(agent.TypeClaudeCode)
	tab.addAgentAllowedSet = make(map[string]bool)
	tab.addAgentDisallowedSet = make(map[string]bool)

	// j toggles type
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.addAgentType != agent.TypeCodex {
		t.Fatalf("expected codex, got %q", tab.addAgentType)
	}

	// Toggle share creds
	tab.addAgentField = addAgentFieldShareCreds
	tab.addAgentShareCreds = false
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if !tab.addAgentShareCreds {
		t.Fatal("expected shareCreds=true after toggle")
	}

	// Toggle skip perms
	tab.addAgentField = addAgentFieldSkipPerms
	tab.addAgentSkipPerms = false
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if !tab.addAgentSkipPerms {
		t.Fatal("expected skipPerms=true after toggle")
	}
}

func TestHandleAddAgentKeyTextFields(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAddAgent
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.addAgentAllowedSet = make(map[string]bool)
	tab.addAgentDisallowedSet = make(map[string]bool)

	// Name field text input
	tab.addAgentField = addAgentFieldName
	tab.addAgentName = ""
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if tab.addAgentName != "x" {
		t.Fatalf("expected 'x', got %q", tab.addAgentName)
	}

	// Model field text input
	tab.addAgentField = addAgentFieldModel
	tab.addAgentModel = ""
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if tab.addAgentModel != "s" {
		t.Fatalf("expected 's', got %q", tab.addAgentModel)
	}

	// MaxTurns field text input
	tab.addAgentField = addAgentFieldMaxTurns
	tab.addAgentMaxTurns = ""
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	if tab.addAgentMaxTurns != "5" {
		t.Fatalf("expected '5', got %q", tab.addAgentMaxTurns)
	}

	// Backspace on name
	tab.addAgentField = addAgentFieldName
	tab.addAgentName = "xyz"
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.addAgentName != "xy" {
		t.Fatalf("expected 'xy', got %q", tab.addAgentName)
	}

	// Backspace on model
	tab.addAgentField = addAgentFieldModel
	tab.addAgentModel = "abc"
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.addAgentModel != "ab" {
		t.Fatalf("expected 'ab', got %q", tab.addAgentModel)
	}

	// Backspace on maxTurns
	tab.addAgentField = addAgentFieldMaxTurns
	tab.addAgentMaxTurns = "10"
	tab.handleAddAgentKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.addAgentMaxTurns != "1" {
		t.Fatalf("expected '1', got %q", tab.addAgentMaxTurns)
	}
}

func TestHandleToolsMultiSelectWith(t *testing.T) {
	tab := newTestVibespacesTab()
	toolsList := []string{"Bash", "Read", "Write", "Edit"}
	cursor := 0
	set := make(map[string]bool)
	opposite := make(map[string]bool)

	// j moves down
	tab.handleToolsMultiSelectWith("j", toolsList, &cursor, set, opposite)
	if cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", cursor)
	}

	// k moves up
	tab.handleToolsMultiSelectWith("k", toolsList, &cursor, set, opposite)
	if cursor != 0 {
		t.Fatalf("expected cursor=0, got %d", cursor)
	}

	// space toggles
	tab.handleToolsMultiSelectWith(" ", toolsList, &cursor, set, opposite)
	if !set["Bash"] {
		t.Fatal("expected Bash toggled on")
	}
	tab.handleToolsMultiSelectWith(" ", toolsList, &cursor, set, opposite)
	if set["Bash"] {
		t.Fatal("expected Bash toggled off")
	}

	// toggling on removes from opposite set
	opposite["Read"] = true
	cursor = 1
	tab.handleToolsMultiSelectWith(" ", toolsList, &cursor, set, opposite)
	if !set["Read"] {
		t.Fatal("expected Read toggled on in set")
	}
	if opposite["Read"] {
		t.Fatal("expected Read removed from opposite set")
	}

	// Clamps at bounds
	cursor = 0
	tab.handleToolsMultiSelectWith("k", toolsList, &cursor, set, opposite)
	if cursor != 0 {
		t.Fatalf("expected cursor=0, got %d", cursor)
	}

	cursor = 3
	tab.handleToolsMultiSelectWith("j", toolsList, &cursor, set, opposite)
	if cursor != 3 {
		t.Fatalf("expected cursor=3 (clamped), got %d", cursor)
	}

	// Empty list returns nil
	cmd := tab.handleToolsMultiSelectWith("j", nil, &cursor, set, opposite)
	if cmd != nil {
		t.Fatal("expected nil cmd for empty list")
	}
}

func TestHandleEditConfigKey(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeEditConfig
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.editConfigAgentName = "claude-1"
	tab.editConfigField = editConfigFieldModel
	tab.editConfigModel = ""
	tab.editConfigMaxTurns = ""
	tab.editConfigSkipPerms = false
	tab.editConfigToolsList = []string{"Bash", "Read", "Write"}
	tab.editConfigAllowedSet = make(map[string]bool)
	tab.editConfigDisallowedSet = make(map[string]bool)

	// Type in model field
	tab.handleEditConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if tab.editConfigModel != "s" {
		t.Fatalf("expected 's', got %q", tab.editConfigModel)
	}

	// Backspace
	tab.handleEditConfigKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.editConfigModel != "" {
		t.Fatalf("expected empty, got %q", tab.editConfigModel)
	}

	// Tab advances to next field
	tab.handleEditConfigKey(tea.KeyMsg{Type: tea.KeyTab})
	if tab.editConfigField != editConfigFieldMaxTurns {
		t.Fatalf("expected maxTurns field, got %d", tab.editConfigField)
	}

	// MaxTurns text input
	tab.handleEditConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if tab.editConfigMaxTurns != "3" {
		t.Fatalf("expected '3', got %q", tab.editConfigMaxTurns)
	}

	// Backspace on maxTurns
	tab.handleEditConfigKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.editConfigMaxTurns != "" {
		t.Fatalf("expected empty, got %q", tab.editConfigMaxTurns)
	}

	// Esc returns to agent view
	tab.handleEditConfigKey(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeAgentView {
		t.Fatalf("expected agentView, got %d", tab.mode)
	}
}

func TestHandleEditConfigSkipPermsToggle(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeEditConfig
	tab.editConfigField = editConfigFieldSkipPerms
	tab.editConfigSkipPerms = false

	tab.handleEditConfigKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if !tab.editConfigSkipPerms {
		t.Fatal("expected skipPerms=true after j")
	}
}

func TestHandleForwardManagerKey(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeForwardManager
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1"},
	}
	tab.agentCursor = 0
	tab.fwdManagerCursor = 0

	// "a" enters add mode
	tab.handleForwardManagerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !tab.fwdManagerAdding {
		t.Fatal("expected fwdManagerAdding=true after 'a'")
	}
	if tab.fwdManagerAddField != fwdManagerAddFieldRemote {
		t.Fatalf("expected remote field, got %d", tab.fwdManagerAddField)
	}

	// Esc from add mode cancels
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.fwdManagerAdding {
		t.Fatal("expected fwdManagerAdding=false after esc")
	}

	// k at top clamps
	tab.handleForwardManagerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.fwdManagerCursor != 0 {
		t.Fatalf("expected 0, got %d", tab.fwdManagerCursor)
	}

	// esc returns to agent view
	tab.handleForwardManagerKey(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != vibespacesModeAgentView {
		t.Fatalf("expected agentView, got %d", tab.mode)
	}
}

func TestHandleFwdManagerAddKeyTextInput(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.fwdManagerAdding = true
	tab.fwdManagerAddField = fwdManagerAddFieldRemote
	tab.fwdManagerAddRemote = ""

	// Type in remote field
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")})
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")})
	if tab.fwdManagerAddRemote != "80" {
		t.Fatalf("expected '80', got %q", tab.fwdManagerAddRemote)
	}

	// Backspace
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.fwdManagerAddRemote != "8" {
		t.Fatalf("expected '8', got %q", tab.fwdManagerAddRemote)
	}

	// Tab advances to local
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyTab})
	if tab.fwdManagerAddField != fwdManagerAddFieldLocal {
		t.Fatalf("expected local field, got %d", tab.fwdManagerAddField)
	}

	// Type in local field
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")})
	if tab.fwdManagerAddLocal != "0" {
		t.Fatalf("expected '0', got %q", tab.fwdManagerAddLocal)
	}

	// Backspace on local
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.fwdManagerAddLocal != "" {
		t.Fatalf("expected empty, got %q", tab.fwdManagerAddLocal)
	}

	// Tab to DNS field
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyTab})
	if tab.fwdManagerAddField != fwdManagerAddFieldDNS {
		t.Fatalf("expected DNS field, got %d", tab.fwdManagerAddField)
	}

	// Space toggles DNS
	tab.fwdManagerAddDNS = false
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !tab.fwdManagerAddDNS {
		t.Fatal("expected DNS=true after space")
	}

	// Tab advances to DNS name when DNS is enabled
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyTab})
	if tab.fwdManagerAddField != fwdManagerAddFieldDNSName {
		t.Fatalf("expected DNS name field, got %d", tab.fwdManagerAddField)
	}

	// Type in DNS name
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if tab.fwdManagerAddDNSName != "m" {
		t.Fatalf("expected 'm', got %q", tab.fwdManagerAddDNSName)
	}

	// Backspace on DNS name
	tab.handleFwdManagerAddKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if tab.fwdManagerAddDNSName != "" {
		t.Fatalf("expected empty, got %q", tab.fwdManagerAddDNSName)
	}
}

func TestVibespacesTabDetailView(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{
		{
			ID:         "12345678-abcd-ef01-2345-abcdef012345",
			Name:       "my-project",
			Status:     "running",
			Persistent: true,
			Image:      "ghcr.io/test/image:latest",
			CreatedAt:  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			Resources: model.Resources{
				CPU:         "250m",
				Memory:      "512Mi",
				Storage:     "10Gi",
				CPULimit:    "1000m",
				MemoryLimit: "1Gi",
			},
		},
	}
	tab.selected = 0
	tab.agentNames = map[string][]string{
		"12345678-abcd-ef01-2345-abcdef012345": {"claude-1", "claude-2"},
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "12345678") {
		t.Error("detail should contain truncated ID")
	}
	if !strings.Contains(view, "10Gi") {
		t.Error("detail should contain storage")
	}
	if !strings.Contains(view, "claude-1") {
		t.Error("detail should contain agent names")
	}
	if !strings.Contains(view, "pvc") || !strings.Contains(view, "PVC") {
		t.Error("detail should contain PVC info for persistent vibespace")
	}
}

func TestVibespacesTabAgentDetailView(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{
		ID:     "1",
		Name:   "test",
		Status: "running",
		Resources: model.Resources{
			CPU:         "250m",
			Memory:      "512Mi",
			CPULimit:    "1000m",
			MemoryLimit: "1Gi",
			Storage:     "10Gi",
		},
		Persistent: true,
	}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode, Status: "running"},
	}
	tab.agentConfigs = map[string]*agent.Config{
		"claude-1": {
			Model:            "sonnet",
			MaxTurns:         10,
			SkipPermissions:  false,
			ShareCredentials: true,
			AllowedTools:     []string{"Bash", "Read"},
			DisallowedTools:  []string{"Write"},
		},
	}
	tab.agentCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "claude-1") {
		t.Error("agent detail should contain agent name")
	}
	if !strings.Contains(view, "sonnet") {
		t.Error("agent detail should contain model name")
	}
	if !strings.Contains(view, "10") {
		t.Error("agent detail should contain max_turns")
	}
	if !strings.Contains(view, "Bash") {
		t.Error("agent detail should contain allowed tools")
	}
	if !strings.Contains(view, "Write") {
		t.Error("agent detail should contain disallowed tools")
	}
}

func TestVibespacesTabTitle(t *testing.T) {
	tab := newTestVibespacesTab()
	if tab.Title() != "Vibespaces" {
		t.Fatalf("expected 'Vibespaces', got %q", tab.Title())
	}
}

func TestVibespacesTabShortHelp(t *testing.T) {
	tab := newTestVibespacesTab()

	// List mode
	tab.mode = vibespacesModeList
	bindings := tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings for list mode")
	}

	// Create form mode
	tab.mode = vibespacesModeCreateForm
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings for create form")
	}

	// Delete confirm mode
	tab.mode = vibespacesModeDeleteConfirm
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings for delete confirm")
	}

	// Agent view mode
	tab.mode = vibespacesModeAgentView
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings for agent view")
	}

	// Add agent mode
	tab.mode = vibespacesModeAddAgent
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings for add agent")
	}

	// Session list mode
	tab.mode = vibespacesModeSessionList
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty keybindings for session list")
	}
}

func TestVibespacesTabListGAndShiftG(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{
		{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"},
	}
	tab.selected = 2

	// G goes to end
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if tab.selected != 3 {
		t.Fatalf("expected selected=3, got %d", tab.selected)
	}

	// g goes to beginning
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if tab.selected != 0 {
		t.Fatalf("expected selected=0, got %d", tab.selected)
	}
}

func TestVibespacesTabSessionListGAndShiftG(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeSessionList
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.sessions = []vsSessionInfo{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	tab.sessionCursor = 1

	// G goes to end
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if tab.sessionCursor != 2 {
		t.Fatalf("expected 2, got %d", tab.sessionCursor)
	}

	// g goes to beginning
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if tab.sessionCursor != 0 {
		t.Fatalf("expected 0, got %d", tab.sessionCursor)
	}
}

func TestVibespacesTabAgentViewGAndShiftG(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{{AgentName: "a1"}, {AgentName: "a2"}, {AgentName: "a3"}}
	tab.agentCursor = 1

	// G goes to end
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if tab.agentCursor != 2 {
		t.Fatalf("expected 2, got %d", tab.agentCursor)
	}

	// g goes to beginning
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if tab.agentCursor != 0 {
		t.Fatalf("expected 0, got %d", tab.agentCursor)
	}
}

func TestVibespacesTabViewCreateForm(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{{ID: "1", Name: "existing"}}
	tab.mode = vibespacesModeCreateForm
	tab.createField = createFieldName
	tab.createName = "my-vs"
	tab.createAgentType = agent.TypeClaudeCode
	tab.createCPU = "250m"
	tab.createMemory = "512Mi"
	tab.createStorage = "10Gi"

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Create") {
		t.Error("create form should contain 'Create'")
	}
	if !strings.Contains(view, "my-vs") {
		t.Error("create form should contain the typed name")
	}
	if !strings.Contains(view, "250m") {
		t.Error("create form should contain CPU value")
	}
}

func TestVibespacesTabViewDeleteConfirm(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{{ID: "1", Name: "to-del"}}
	tab.mode = vibespacesModeDeleteConfirm
	tab.deleteName = "to-del"
	tab.deleteInput = "to-"

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "to-del") {
		t.Error("delete confirm should contain vibespace name")
	}
	if !strings.Contains(view, "to-") {
		t.Error("delete confirm should contain current input")
	}
}

func TestVibespacesTabViewAddAgentForm(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAddAgent
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{{AgentName: "existing"}}
	tab.addAgentField = addAgentFieldName
	tab.addAgentType = agent.TypeClaudeCode
	tab.addAgentName = "new-agent"
	tab.addAgentToolsList = []string{"Bash", "Read", "Write"}
	tab.addAgentAllowedSet = make(map[string]bool)
	tab.addAgentDisallowedSet = make(map[string]bool)

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Add agent") || !strings.Contains(view, "Agent") {
		t.Error("add agent form should contain 'Add agent'")
	}
	if !strings.Contains(view, "new-agent") {
		t.Error("add agent form should contain the typed name")
	}
}

func TestVibespacesTabViewEditConfigForm(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeEditConfig
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode},
	}
	tab.editConfigAgentName = "claude-1"
	tab.editConfigField = editConfigFieldModel
	tab.editConfigModel = "opus"
	tab.editConfigToolsList = []string{"Bash", "Read"}
	tab.editConfigAllowedSet = make(map[string]bool)
	tab.editConfigDisallowedSet = make(map[string]bool)

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Edit config") {
		t.Error("edit config form should contain 'Edit config'")
	}
	if !strings.Contains(view, "claude-1") {
		t.Error("edit config form should contain agent name")
	}
}

func TestVibespacesTabViewForwardManager(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeForwardManager
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode},
	}
	tab.agentCursor = 0
	tab.fwdManagerAdding = true
	tab.fwdManagerAddField = fwdManagerAddFieldRemote
	tab.fwdManagerAddRemote = "8080"

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Forward") {
		t.Error("forward manager should contain 'Forward'")
	}
}

func TestVibespacesTabAddAgentFormFieldNav(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAddAgent
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.addAgentField = addAgentFieldType
	tab.addAgentToolsList = []string{"Bash", "Read", "Write"}

	// Tab advances field
	tab.Update(tea.KeyMsg{Type: tea.KeyTab})
	if tab.addAgentField != addAgentFieldName {
		t.Fatalf("expected name field, got %d", tab.addAgentField)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyTab})
	if tab.addAgentField != addAgentFieldModel {
		t.Fatalf("expected model field, got %d", tab.addAgentField)
	}
}

func TestVibespacesTabForwardManagerWithForwards(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeForwardManager
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode},
	}
	tab.agentCursor = 0
	tab.forwards = []daemon.AgentStatus{
		{
			Name: "claude-1",
			Forwards: []daemon.ForwardInfo{
				{RemotePort: 8080, LocalPort: 18080, Type: "ssh", Status: "active", DNSName: "my-app"},
				{RemotePort: 3000, LocalPort: 13000, Type: "ssh", Status: "active"},
			},
		},
	}
	tab.fwdManagerCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "8080") {
		t.Error("forward manager should contain remote port 8080")
	}
	if !strings.Contains(view, "18080") {
		t.Error("forward manager should contain local port 18080")
	}
	if !strings.Contains(view, "active") {
		t.Error("forward manager should contain status")
	}
	if !strings.Contains(view, "my-app") {
		t.Error("forward manager should contain DNS name")
	}

	// j navigation with forwards
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.fwdManagerCursor != 1 {
		t.Fatalf("expected cursor=1, got %d", tab.fwdManagerCursor)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.fwdManagerCursor != 1 {
		t.Fatalf("expected cursor=1 (clamped), got %d", tab.fwdManagerCursor)
	}
}

func TestVibespacesTabAgentDetailCodex(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test", Status: "running"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "codex-1", AgentType: agent.TypeCodex, Status: "running"},
	}
	tab.agentConfigs = map[string]*agent.Config{
		"codex-1": {
			SkipPermissions:  true,
			ShareCredentials: false,
		},
	}
	tab.agentCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "codex-1") {
		t.Error("should contain codex agent name")
	}
	if !strings.Contains(view, "always") {
		t.Error("should show 'always' for codex skip_permissions")
	}
}

func TestVibespacesTabAgentDetailWithForwards(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test", Status: "running"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode, Status: "running"},
	}
	tab.agentConfigs = map[string]*agent.Config{
		"claude-1": {},
	}
	tab.forwards = []daemon.AgentStatus{
		{
			Name: "claude-1",
			Forwards: []daemon.ForwardInfo{
				{RemotePort: 8080, LocalPort: 18080, Type: "ssh", Status: "active"},
			},
		},
	}
	tab.agentCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Forwards") {
		t.Error("should contain Forwards section")
	}
	if !strings.Contains(view, "8080") {
		t.Error("should contain forward port")
	}
}

func TestVibespacesTabAgentDetailNoConfig(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test", Status: "running"}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode, Status: "running"},
	}
	tab.agentConfigs = map[string]*agent.Config{} // no config for claude-1
	tab.agentCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Loading config") {
		t.Error("should show 'Loading config...' when no config available")
	}
}

func TestVibespacesTabViewDetailWithMounts(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.vibespaces = []*model.Vibespace{
		{
			ID:     "1",
			Name:   "mounted-vs",
			Status: "running",
			Mounts: []model.Mount{
				{HostPath: "/host/path", ContainerPath: "/container/path", ReadOnly: true},
			},
		},
	}
	tab.selected = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "/host/path") {
		t.Error("should contain mount host path")
	}
	if !strings.Contains(view, "/container/path") {
		t.Error("should contain mount container path")
	}
	if !strings.Contains(view, "ro") {
		t.Error("should indicate read-only mount")
	}
}

func TestVibespacesTabSessionListEmpty(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeSessionList
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.sessionAgent = "claude-1"
	tab.sessions = []vsSessionInfo{} // empty

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "No sessions") {
		t.Error("should show 'No sessions found' for empty sessions")
	}
}

func TestVibespacesTabSessionListLoading(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeSessionList
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.sessionAgent = "claude-1"
	tab.sessions = nil // nil = loading

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Loading") {
		t.Error("should show 'Loading sessions...' for nil sessions")
	}
}

func TestVibespacesTabSessionListError(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeSessionList
	tab.selectedVS = &model.Vibespace{ID: "1", Name: "test"}
	tab.sessionAgent = "claude-1"
	tab.sessions = nil
	tab.err = "connection refused"

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "connection refused") {
		t.Error("should show error message")
	}
}

func TestVibespacesTabAgentDetailWithMounts(t *testing.T) {
	tab := newTestVibespacesTab()
	tab.mode = vibespacesModeAgentView
	tab.selectedVS = &model.Vibespace{
		ID:     "1",
		Name:   "test",
		Status: "running",
		Mounts: []model.Mount{
			{HostPath: "/src", ContainerPath: "/workspace", ReadOnly: false},
		},
	}
	tab.viewAgents = []vibespace.AgentInfo{
		{AgentName: "claude-1", AgentType: agent.TypeClaudeCode, Status: "running"},
	}
	tab.agentConfigs = map[string]*agent.Config{
		"claude-1": {},
	}
	tab.agentCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "/src") {
		t.Error("should contain mount host path")
	}
	if !strings.Contains(view, "rw") {
		t.Error("should indicate read-write mount")
	}
}

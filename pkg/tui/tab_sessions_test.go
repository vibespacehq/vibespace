package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

func newTestSessionsTab() *SessionsTab {
	shared := &SharedState{}
	tab := NewSessionsTab(shared)
	tab.SetSize(120, 40)
	return tab
}

// setupTreeTab creates a SessionsTab pre-populated with vibespaces and multi-sessions.
func setupTreeTab(vibespaces []*model.Vibespace, multiSessions []session.Session) *SessionsTab {
	tab := newTestSessionsTab()
	tab.vibespaces = vibespaces
	tab.multiSessions = multiSessions
	tab.rebuildTree()
	return tab
}

// --- Helper function tests (unchanged) ---

func TestTimeAgo(t *testing.T) {
	tests := []struct {
		input time.Time
		want  string
	}{
		{time.Time{}, "-"},
		{time.Now().Add(-30 * time.Second), "just now"},
		{time.Now().Add(-5 * time.Minute), "5m ago"},
		{time.Now().Add(-3 * time.Hour), "3h ago"},
		{time.Now().Add(-48 * time.Hour), "2d ago"},
	}
	for _, tt := range tests {
		got := timeAgo(tt.input)
		if got != tt.want {
			t.Errorf("timeAgo(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSessVibespaceNames(t *testing.T) {
	s := session.Session{}
	if got := sessVibespaceNames(s); got != "-" {
		t.Fatalf("expected '-', got %q", got)
	}

	s.Vibespaces = []session.VibespaceEntry{
		{Name: "project-a"},
		{Name: "project-b"},
	}
	got := sessVibespaceNames(s)
	if !strings.Contains(got, "project-a") || !strings.Contains(got, "project-b") {
		t.Fatalf("expected both names, got %q", got)
	}
}

func TestSessAgentCount(t *testing.T) {
	s := session.Session{}
	if got := sessAgentCount(s); got != "-" {
		t.Fatalf("expected '-', got %q", got)
	}

	s.Vibespaces = []session.VibespaceEntry{{Name: "test"}}
	if got := sessAgentCount(s); got != "all" {
		t.Fatalf("expected 'all', got %q", got)
	}

	s.Vibespaces = []session.VibespaceEntry{
		{Name: "test", Agents: []string{"agent-1", "agent-2"}},
	}
	if got := sessAgentCount(s); got != "2" {
		t.Fatalf("expected '2', got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"hello world long text", 10, "hello wor…"},
		{"first\nsecond", 20, "first"},
		{"", 10, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

// --- Tree building tests ---

func TestRebuildTreeEmpty(t *testing.T) {
	tab := newTestSessionsTab()
	tab.rebuildTree()
	if len(tab.flatTree) != 0 {
		t.Fatalf("expected empty tree, got %d items", len(tab.flatTree))
	}
}

func TestRebuildTreeGroupsOnly(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "project-a", Status: "running"},
			{Name: "project-b", Status: "stopped"},
		},
		nil,
	)
	if len(tab.flatTree) != 2 {
		t.Fatalf("expected 2 groups, got %d items", len(tab.flatTree))
	}
	if tab.flatTree[0].Type != sessionItemGroup || tab.flatTree[0].VSName != "project-a" {
		t.Error("first item should be group 'project-a'")
	}
	if tab.flatTree[1].Type != sessionItemGroup || tab.flatTree[1].VSName != "project-b" {
		t.Error("second item should be group 'project-b'")
	}
}

func TestRebuildTreeWithMultiSessions(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "project-a", Status: "running"},
		},
		[]session.Session{
			{Name: "my-session", Vibespaces: []session.VibespaceEntry{{Name: "project-a"}}},
		},
	)
	// Group collapsed: just the group header with multi count badge
	if len(tab.flatTree) != 1 {
		t.Fatalf("expected 1 item (collapsed group), got %d", len(tab.flatTree))
	}
	if tab.flatTree[0].MultiCount != 1 {
		t.Errorf("expected MultiCount=1, got %d", tab.flatTree[0].MultiCount)
	}

	// Expand group
	tab.expandedVS["project-a"] = true
	tab.rebuildTree()
	if len(tab.flatTree) != 2 {
		t.Fatalf("expected 2 items (group + multi session), got %d", len(tab.flatTree))
	}
	if tab.flatTree[1].Type != sessionItemMulti {
		t.Error("second item should be multi session")
	}
	if tab.flatTree[1].MultiSession.Name != "my-session" {
		t.Errorf("expected 'my-session', got %q", tab.flatTree[1].MultiSession.Name)
	}
}

func TestRebuildTreeCrossVSMultiSession(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "project-a", Status: "running"},
			{Name: "project-b", Status: "running"},
		},
		[]session.Session{
			{
				Name: "cross-session",
				Vibespaces: []session.VibespaceEntry{
					{Name: "project-a"},
					{Name: "project-b"},
				},
			},
		},
	)
	// Expand both groups
	tab.expandedVS["project-a"] = true
	tab.expandedVS["project-b"] = true
	tab.rebuildTree()

	// Should appear under both groups
	multiCount := 0
	for _, item := range tab.flatTree {
		if item.Type == sessionItemMulti {
			multiCount++
			if item.CrossVSCount != 1 {
				t.Errorf("expected CrossVSCount=1, got %d", item.CrossVSCount)
			}
		}
	}
	if multiCount != 2 {
		t.Fatalf("expected cross-vs session to appear under 2 groups, got %d", multiCount)
	}
}

func TestRebuildTreeOrphanMultiSessions(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "project-a", Status: "running"},
		},
		[]session.Session{
			{Name: "orphan-session"}, // no vibespaces
		},
	)
	// Should add an Ungrouped section
	found := false
	for _, item := range tab.flatTree {
		if item.Type == sessionItemGroup && item.VSName == "Ungrouped" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Ungrouped' group for orphan sessions")
	}
}

func TestRebuildTreeWithSingleAgentSessions(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "project-a", Status: "running"},
		},
		nil,
	)
	// Simulate loaded group state with single-agent sessions
	tab.expandedVS["project-a"] = true
	tab.groupStates["project-a"] = &groupLoadState{
		AgentsLoaded: true,
		Agents: []vibespace.AgentInfo{
			{AgentName: "claude-1", AgentType: agent.TypeClaudeCode},
		},
		AgentSessions: map[string][]vsSessionInfo{
			"claude-1": {
				{ID: "sess-1", Title: "fix bug", LastTime: time.Now(), Prompts: 5},
				{ID: "sess-2", Title: "add feature", LastTime: time.Now(), Prompts: 3},
			},
		},
		AgentConfigs: make(map[string]*agent.Config),
	}
	tab.rebuildTree()

	if len(tab.flatTree) != 3 { // 1 group + 2 sessions
		t.Fatalf("expected 3 items, got %d", len(tab.flatTree))
	}
	if tab.flatTree[1].Type != sessionItemSingle || tab.flatTree[1].AgentName != "claude-1" {
		t.Error("second item should be single-agent session for claude-1")
	}
	if tab.flatTree[2].isLastChild != true {
		t.Error("last session should be marked as isLastChild")
	}
}

func TestRebuildTreeLastChildMarking(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "project-a", Status: "running"},
		},
		[]session.Session{
			{Name: "s1", Vibespaces: []session.VibespaceEntry{{Name: "project-a"}}},
			{Name: "s2", Vibespaces: []session.VibespaceEntry{{Name: "project-a"}}},
		},
	)
	tab.expandedVS["project-a"] = true
	tab.rebuildTree()

	// group + s1 + s2
	if len(tab.flatTree) != 3 {
		t.Fatalf("expected 3, got %d", len(tab.flatTree))
	}
	if tab.flatTree[1].isLastChild {
		t.Error("first child should not be last")
	}
	if !tab.flatTree[2].isLastChild {
		t.Error("second child should be last")
	}
}

// --- Navigation tests ---

func TestSessionsTabTreeNavigation(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "project-a", Status: "running"},
			{Name: "project-b", Status: "running"},
		},
		[]session.Session{
			{Name: "s1", Vibespaces: []session.VibespaceEntry{{Name: "project-a"}}},
		},
	)
	// Expand first group
	tab.expandedVS["project-a"] = true
	tab.rebuildTree()
	tab.cursor = 0

	// j moves down
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", tab.cursor)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.cursor != 2 {
		t.Fatalf("expected cursor=2, got %d", tab.cursor)
	}

	// j at end clamps
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.cursor != 2 {
		t.Fatalf("expected cursor=2 (clamped), got %d", tab.cursor)
	}

	// k moves up
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", tab.cursor)
	}
}

func TestSessionsTabNavigationGAndShiftG(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "a", Status: "running"},
			{Name: "b", Status: "running"},
			{Name: "c", Status: "running"},
			{Name: "d", Status: "running"},
		},
		nil,
	)
	tab.cursor = 1

	// G goes to end
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if tab.cursor != 3 {
		t.Fatalf("expected 3, got %d", tab.cursor)
	}

	// g goes to beginning
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if tab.cursor != 0 {
		t.Fatalf("expected 0, got %d", tab.cursor)
	}
}

// --- View tests ---

func TestSessionsTabEmptyView(t *testing.T) {
	tab := newTestSessionsTab()
	view := stripAnsi(tab.View())
	if !strings.Contains(view, "No sessions yet") {
		t.Errorf("empty view should contain 'No sessions yet', got: %s", view)
	}
}

func TestSessionsTabTreeView(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "my-project", Status: "running"},
		},
		[]session.Session{
			{
				Name:     "code-review",
				LastUsed: time.Now(),
				Vibespaces: []session.VibespaceEntry{
					{Name: "my-project", Agents: []string{"claude-1"}},
				},
			},
		},
	)
	tab.expandedVS["my-project"] = true
	tab.rebuildTree()

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "my-project") {
		t.Error("tree view should contain vibespace name")
	}
	if !strings.Contains(view, "code-review") {
		t.Error("tree view should contain session name")
	}
}

func TestSessionsTabPreviewMultiSession(t *testing.T) {
	ms := session.Session{
		Name:      "preview-session",
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Vibespaces: []session.VibespaceEntry{
			{Name: "project-a", Agents: []string{"claude-1"}},
		},
	}
	tab := setupTreeTab(
		[]*model.Vibespace{{Name: "project-a", Status: "running"}},
		[]session.Session{ms},
	)
	tab.expandedVS["project-a"] = true
	tab.rebuildTree()

	// Move cursor to the multi-session item
	for i, item := range tab.flatTree {
		if item.Type == sessionItemMulti {
			tab.cursor = i
			break
		}
	}

	// Set preview data
	tab.previewSessionName = "preview-session"
	tab.previewTotal = 15
	tab.previewMsgs = []*Message{
		NewUserMessage("hello world", "all"),
		NewAssistantMessage("claude-1", "I can help"),
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "preview-session") {
		t.Error("should contain session name")
	}
	if !strings.Contains(view, "15") {
		t.Error("should contain message count")
	}
	if !strings.Contains(view, "You") {
		t.Error("should contain 'You' label for user message")
	}
}

// --- Delete mode tests ---

func TestSessionsTabDeleteModeOnMultiSession(t *testing.T) {
	ms := session.Session{
		Name:       "to-delete",
		Vibespaces: []session.VibespaceEntry{{Name: "project-a"}},
	}
	tab := setupTreeTab(
		[]*model.Vibespace{{Name: "project-a", Status: "running"}},
		[]session.Session{ms},
	)
	tab.expandedVS["project-a"] = true
	tab.rebuildTree()

	// Move to multi-session
	for i, item := range tab.flatTree {
		if item.Type == sessionItemMulti {
			tab.cursor = i
			break
		}
	}

	// d enters delete mode
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if tab.mode != sessionsModeDelete {
		t.Fatalf("expected delete mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Delete") || !strings.Contains(view, "to-delete") {
		t.Errorf("delete view should contain 'Delete' and session name, got: %s", view)
	}
}

func TestSessionsTabDeleteModeNotOnGroup(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{{Name: "project-a", Status: "running"}},
		nil,
	)
	tab.cursor = 0 // on group header

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode (can't delete group), got %d", tab.mode)
	}
}

func TestSessionsTabDeleteModeEscAndQ(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeDelete
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from n in delete, got %d", tab.mode)
	}

	tab.mode = sessionsModeDelete
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from q in delete, got %d", tab.mode)
	}
}

func TestSessionsTabDeleteNoItems(t *testing.T) {
	tab := newTestSessionsTab()
	// d should be no-op when tree is empty
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode, got %d", tab.mode)
	}
}

// --- Wizard tests ---

func TestSessionsTabNewNameMode(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{{Name: "project-a", Status: "running"}},
		nil,
	)

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if tab.mode != sessionsModeNewName {
		t.Fatalf("expected newName mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Step 1") {
		t.Error("new name view should contain 'Step 1'")
	}
}

func TestSessionsTabNewVSPicker(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewVibespaces
	tab.newVibespaces = []vsPickerItem{
		{Name: "vs1", Status: "running"},
		{Name: "vs2", Status: "stopped"},
	}
	tab.newVSSelected = []bool{false, false}
	tab.newVSCursor = 0

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !tab.newVSSelected[0] {
		t.Fatal("expected vs1 selected after space")
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if tab.newVSSelected[0] {
		t.Fatal("expected vs1 deselected after second space")
	}
}

func TestSessionsTabNewAgentPicker(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewAgents
	tab.newAgents = []agentPickerItem{
		{Name: "agent-1", AgentType: "claude-code"},
		{Name: "agent-2", AgentType: "codex"},
	}
	tab.newAgentSelected = []bool{false, false}
	tab.newAgentCursor = 0
	tab.newSelectedVS = []vsPickerItem{{Name: "vs1"}}
	tab.newAgentVSIndex = 0

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.newAgentCursor != 1 {
		t.Fatalf("expected cursor=1, got %d", tab.newAgentCursor)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !tab.newAgentSelected[1] {
		t.Fatal("expected agent-2 selected")
	}
}

func TestSessionsTabEscReturnsToList(t *testing.T) {
	tab := newTestSessionsTab()

	tab.mode = sessionsModeDelete
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from delete, got %d", tab.mode)
	}

	tab.mode = sessionsModeNewName
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from newName, got %d", tab.mode)
	}

	tab.mode = sessionsModeNewVibespaces
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from newVibespaces, got %d", tab.mode)
	}

	tab.mode = sessionsModeNewAgents
	tab.newSelectedVS = []vsPickerItem{{Name: "vs1"}}
	tab.newAgentVSIndex = 0
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from newAgents, got %d", tab.mode)
	}
}

func TestSessionsTabViewPromptNewVibespaces(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewVibespaces
	tab.newSessionName = "test-session"
	tab.newVibespaces = []vsPickerItem{
		{Name: "vs1", Status: "running"},
	}
	tab.newVSSelected = []bool{false}
	tab.newVSCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Step 2") {
		t.Error("vibespace picker should contain 'Step 2'")
	}
	if !strings.Contains(view, "vs1") {
		t.Error("vibespace picker should contain vibespace names")
	}
}

func TestSessionsTabViewPromptNewAgents(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewAgents
	tab.newSelectedVS = []vsPickerItem{{Name: "vs1"}}
	tab.newAgentVSIndex = 0
	tab.newAgents = []agentPickerItem{
		{Name: "agent-1", AgentType: "claude-code", Status: "running"},
	}
	tab.newAgentSelected = []bool{false}
	tab.newAgentCursor = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Step 3") {
		t.Error("agent picker should contain 'Step 3'")
	}
	if !strings.Contains(view, "agent-1") {
		t.Error("agent picker should contain agent names")
	}
}

func TestSessionsTabVSPickerNavigation(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewVibespaces
	tab.newVibespaces = []vsPickerItem{
		{Name: "vs1"}, {Name: "vs2"}, {Name: "vs3"},
	}
	tab.newVSSelected = []bool{false, false, false}
	tab.newVSCursor = 0

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.newVSCursor != 1 {
		t.Fatalf("expected 1, got %d", tab.newVSCursor)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.newVSCursor != 0 {
		t.Fatalf("expected 0, got %d", tab.newVSCursor)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !tab.newVSSelected[0] {
		t.Fatal("expected vs1 selected after x")
	}
}

// --- Update message tests ---

func TestSessionsTabUpdateMultiSessionsLoaded(t *testing.T) {
	tab := newTestSessionsTab()
	tab.vibespaces = []*model.Vibespace{{Name: "vs1", Status: "running"}}

	tab.Update(multiSessionsLoadedMsg{
		sessions: []session.Session{
			{Name: "s1", CreatedAt: time.Now(), Vibespaces: []session.VibespaceEntry{{Name: "vs1"}}},
			{Name: "s2", CreatedAt: time.Now(), Vibespaces: []session.VibespaceEntry{{Name: "vs1"}}},
		},
	})
	if len(tab.multiSessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(tab.multiSessions))
	}
	if tab.err != "" {
		t.Fatalf("expected no error, got %q", tab.err)
	}

	tab.Update(multiSessionsLoadedMsg{err: fmt.Errorf("load error")})
	if tab.err != "load error" {
		t.Fatalf("expected 'load error', got %q", tab.err)
	}
}

func TestSessionsTabUpdateSessionDeleted(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeDelete

	tab.Update(sessionDeletedMsg{})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode, got %d", tab.mode)
	}

	tab.Update(sessionDeletedMsg{err: fmt.Errorf("delete failed")})
	if tab.err != "delete failed" {
		t.Fatalf("expected error, got %q", tab.err)
	}
}

func TestSessionsTabUpdateSessionHistory(t *testing.T) {
	tab := newTestSessionsTab()

	tab.Update(sessionHistoryMsg{
		sessionName: "s1",
		messages:    []*Message{NewUserMessage("hello", "all")},
		totalCount:  5,
	})
	if tab.previewSessionName != "s1" {
		t.Fatalf("expected previewSessionName='s1', got %q", tab.previewSessionName)
	}
	if tab.previewTotal != 5 {
		t.Fatalf("expected previewTotal=5, got %d", tab.previewTotal)
	}
}

func TestSessionsTabUpdateSessionCreated(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewName

	tab.Update(sessionCreatedMsg{err: fmt.Errorf("create failed")})
	if tab.err != "create failed" {
		t.Fatalf("expected error, got %q", tab.err)
	}
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode, got %d", tab.mode)
	}

	sess := &session.Session{Name: "new-session"}
	tab.Update(sessionCreatedMsg{session: sess})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode, got %d", tab.mode)
	}
}

func TestSessionsTabUpdateVibespacesForPicker(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewVibespaces

	tab.Update(vibespacesForPickerMsg{
		vibespaces: []*model.Vibespace{
			{Name: "vs1", Status: "running"},
			{Name: "vs2", Status: "stopped"},
		},
	})
	if len(tab.newVibespaces) != 2 {
		t.Fatalf("expected 2 vibespaces, got %d", len(tab.newVibespaces))
	}
	if len(tab.newVSSelected) != 2 {
		t.Fatalf("expected 2 selected bools, got %d", len(tab.newVSSelected))
	}

	tab.Update(vibespacesForPickerMsg{err: fmt.Errorf("load error")})
	if tab.err != "load error" {
		t.Fatalf("expected error, got %q", tab.err)
	}
}

// --- Misc tests ---

func TestSessionsTabTitle(t *testing.T) {
	tab := newTestSessionsTab()
	if tab.Title() != "Sessions" {
		t.Fatalf("expected 'Sessions', got %q", tab.Title())
	}
}

func TestSessionsTabShortHelp(t *testing.T) {
	tab := newTestSessionsTab()

	tab.mode = sessionsModeList
	if len(tab.ShortHelp()) == 0 {
		t.Fatal("expected non-empty bindings for list mode")
	}

	tab.mode = sessionsModeDelete
	if len(tab.ShortHelp()) == 0 {
		t.Fatal("expected non-empty bindings for delete mode")
	}

	tab.mode = sessionsModeNewName
	if len(tab.ShortHelp()) == 0 {
		t.Fatal("expected non-empty bindings for newName mode")
	}
}

func TestSessionsTabResetNewSession(t *testing.T) {
	tab := newTestSessionsTab()
	tab.newSessionName = "test"
	tab.newVibespaces = []vsPickerItem{{Name: "vs1"}}
	tab.newVSCursor = 2
	tab.err = "something"

	tab.resetNewSession()
	if tab.newSessionName != "" {
		t.Fatal("expected empty session name")
	}
	if tab.newVibespaces != nil {
		t.Fatal("expected nil vibespaces")
	}
	if tab.newVSCursor != 0 {
		t.Fatal("expected cursor 0")
	}
	if tab.err != "" {
		t.Fatal("expected empty error")
	}
}

func TestSessionsTabCursorClampOnRebuild(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{
			{Name: "a", Status: "running"},
			{Name: "b", Status: "running"},
		},
		nil,
	)
	tab.cursor = 5

	tab.rebuildTree()
	if tab.cursor != 1 { // 2 groups, max index is 1
		t.Fatalf("expected cursor=1, got %d", tab.cursor)
	}

	tab.vibespaces = nil
	tab.rebuildTree()
	if tab.cursor != 0 {
		t.Fatalf("expected cursor=0 for empty, got %d", tab.cursor)
	}
}

func TestSessionsTabGroupLoadState(t *testing.T) {
	tab := setupTreeTab(
		[]*model.Vibespace{{Name: "project-a", Status: "running"}},
		nil,
	)

	// Simulate groupAgentsLoadedMsg
	tab.groupStates["project-a"] = &groupLoadState{
		AgentsLoaded: true,
		Agents: []vibespace.AgentInfo{
			{AgentName: "claude-1", AgentType: agent.TypeClaudeCode, Status: "running"},
			{AgentName: "codex-1", AgentType: agent.TypeCodex, Status: "running"},
		},
		AgentSessions: make(map[string][]vsSessionInfo),
		AgentConfigs:  make(map[string]*agent.Config),
	}
	tab.expandedVS["project-a"] = true
	tab.rebuildTree()

	// Only group header (no sessions loaded yet)
	if len(tab.flatTree) != 1 {
		t.Fatalf("expected 1 (group only, no sessions yet), got %d", len(tab.flatTree))
	}

	// Add sessions for one agent
	tab.groupStates["project-a"].AgentSessions["claude-1"] = []vsSessionInfo{
		{ID: "s1", Title: "fix bug"},
	}
	tab.rebuildTree()

	if len(tab.flatTree) != 2 { // group + 1 session
		t.Fatalf("expected 2, got %d", len(tab.flatTree))
	}
}

func TestSessionsTabErrorView(t *testing.T) {
	tab := newTestSessionsTab()
	tab.err = "something went wrong"

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "something went wrong") {
		t.Error("error view should contain error message")
	}
}

func TestSessionsTabVibespacesForTreeMsg(t *testing.T) {
	tab := newTestSessionsTab()

	tab.Update(vibespacesForTreeMsg{
		vibespaces: []*model.Vibespace{
			{Name: "project-a", Status: "running"},
			{Name: "project-b", Status: "stopped"},
		},
	})

	if len(tab.vibespaces) != 2 {
		t.Fatalf("expected 2 vibespaces, got %d", len(tab.vibespaces))
	}
	if len(tab.flatTree) != 2 {
		t.Fatalf("expected 2 tree items, got %d", len(tab.flatTree))
	}

	// Error case
	tab.Update(vibespacesForTreeMsg{err: fmt.Errorf("vs error")})
	if tab.err != "vs error" {
		t.Fatalf("expected error, got %q", tab.err)
	}
}

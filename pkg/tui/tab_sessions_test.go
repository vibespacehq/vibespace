package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/session"
)

func newTestSessionsTab() *SessionsTab {
	shared := &SharedState{}
	tab := NewSessionsTab(shared)
	tab.SetSize(120, 40)
	return tab
}

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
	// No vibespaces
	s := session.Session{}
	if got := sessVibespaceNames(s); got != "-" {
		t.Fatalf("expected '-', got %q", got)
	}

	// With vibespaces
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
	// No vibespaces
	s := session.Session{}
	if got := sessAgentCount(s); got != "-" {
		t.Fatalf("expected '-', got %q", got)
	}

	// Vibespace with no agents means "all"
	s.Vibespaces = []session.VibespaceEntry{{Name: "test"}}
	if got := sessAgentCount(s); got != "all" {
		t.Fatalf("expected 'all', got %q", got)
	}

	// Specific agents
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

func TestSessionsTabEmptyView(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = nil

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "No sessions yet") {
		t.Errorf("empty view should contain 'No sessions yet', got: %s", view)
	}
}

func TestSessionsTabListView(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{Name: "session-alpha", CreatedAt: time.Now(), LastUsed: time.Now()},
		{Name: "session-beta", CreatedAt: time.Now(), LastUsed: time.Now()},
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "session-alpha") {
		t.Error("list view should contain session names")
	}
	if !strings.Contains(view, "session-beta") {
		t.Error("list view should contain session names")
	}
}

func TestSessionsTabNewNameMode(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{Name: "existing", CreatedAt: time.Now()},
	}

	// Key "n" enters newName mode
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if tab.mode != sessionsModeNewName {
		t.Fatalf("expected newName mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Step 1") {
		t.Error("new name view should contain 'Step 1'")
	}
}

func TestSessionsTabDeleteMode(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{Name: "to-delete", CreatedAt: time.Now()},
	}
	tab.selected = 0

	// Key "d" enters delete mode
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if tab.mode != sessionsModeDelete {
		t.Fatalf("expected delete mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Delete") || !strings.Contains(view, "to-delete") {
		t.Errorf("delete view should contain 'Delete' and session name, got: %s", view)
	}
}

func TestSessionsTabNavigation(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{Name: "s1"},
		{Name: "s2"},
		{Name: "s3"},
	}
	tab.selected = 0

	// j moves down
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.selected != 1 {
		t.Fatalf("expected selected=1, got %d", tab.selected)
	}

	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.selected != 2 {
		t.Fatalf("expected selected=2, got %d", tab.selected)
	}

	// j at end clamps
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.selected != 2 {
		t.Fatalf("expected selected=2 (clamped), got %d", tab.selected)
	}

	// k moves up
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.selected != 1 {
		t.Fatalf("expected selected=1, got %d", tab.selected)
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

	// Space toggles selection
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if !tab.newVSSelected[0] {
		t.Fatal("expected vs1 selected after space")
	}

	// Toggle off
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

	// Navigate and toggle
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

	// From delete mode
	tab.mode = sessionsModeDelete
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from delete, got %d", tab.mode)
	}

	// From newName mode
	tab.mode = sessionsModeNewName
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from newName, got %d", tab.mode)
	}

	// From newVibespaces mode
	tab.mode = sessionsModeNewVibespaces
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from newVibespaces, got %d", tab.mode)
	}

	// From newAgents mode
	tab.mode = sessionsModeNewAgents
	tab.newSelectedVS = []vsPickerItem{{Name: "vs1"}}
	tab.newAgentVSIndex = 0
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode from newAgents, got %d", tab.mode)
	}
}

func TestSessionsTabDeleteNoSessions(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = nil

	// "d" should be no-op when no sessions
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode (no sessions to delete), got %d", tab.mode)
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

func TestSessionsTabTitle(t *testing.T) {
	tab := newTestSessionsTab()
	if tab.Title() != "Sessions" {
		t.Fatalf("expected 'Sessions', got %q", tab.Title())
	}
}

func TestSessionsTabShortHelp(t *testing.T) {
	tab := newTestSessionsTab()

	// List mode
	tab.mode = sessionsModeList
	bindings := tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty bindings for list mode")
	}

	// Delete mode
	tab.mode = sessionsModeDelete
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty bindings for delete mode")
	}

	// New name mode
	tab.mode = sessionsModeNewName
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty bindings for newName mode")
	}
}

func TestSessionsTabNavigationGAndShiftG(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{Name: "s1"}, {Name: "s2"}, {Name: "s3"}, {Name: "s4"},
	}
	tab.selected = 1

	// G goes to end
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if tab.selected != 3 {
		t.Fatalf("expected 3, got %d", tab.selected)
	}

	// g goes to beginning
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if tab.selected != 0 {
		t.Fatalf("expected 0, got %d", tab.selected)
	}
}

func TestSessionsTabDeleteModeEscAndQ(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{{Name: "test"}}
	tab.selected = 0

	// "n" and "q" should also exit delete mode
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

func TestSessionsTabDetailView(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{
			Name:      "my-session",
			CreatedAt: time.Now().Add(-2 * time.Hour),
			LastUsed:  time.Now().Add(-30 * time.Minute),
			Vibespaces: []session.VibespaceEntry{
				{Name: "project-a", Agents: []string{"claude-1"}},
			},
		},
	}
	tab.selected = 0

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "my-session") {
		t.Error("view should contain session name")
	}
	if !strings.Contains(view, "project-a") {
		t.Error("view should contain vibespace name")
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

func TestSessionsTabClampSelected(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{{Name: "a"}, {Name: "b"}}
	tab.selected = 5

	tab.clampSelected()
	if tab.selected != 1 {
		t.Fatalf("expected 1, got %d", tab.selected)
	}

	tab.sessions = nil
	tab.selected = 5
	tab.clampSelected()
	if tab.selected != 0 {
		t.Fatalf("expected 0 for empty, got %d", tab.selected)
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

func TestSessionsTabDetailViewWithPreview(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{
			Name:      "preview-session",
			CreatedAt: time.Now(),
			LastUsed:  time.Now(),
		},
	}
	tab.selected = 0
	tab.previewName = "preview-session"
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

func TestSessionsTabUpdateSessionsLoaded(t *testing.T) {
	tab := newTestSessionsTab()

	// Send sessionsLoadedMsg with sessions
	tab.Update(sessionsLoadedMsg{
		sessions: []session.Session{
			{Name: "s1", CreatedAt: time.Now()},
			{Name: "s2", CreatedAt: time.Now()},
		},
	})
	if len(tab.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(tab.sessions))
	}
	if tab.err != "" {
		t.Fatalf("expected no error, got %q", tab.err)
	}

	// Error case
	tab.Update(sessionsLoadedMsg{err: fmt.Errorf("load error")})
	if tab.err != "load error" {
		t.Fatalf("expected 'load error', got %q", tab.err)
	}
}

func TestSessionsTabUpdateSessionDeleted(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeDelete

	// Successful delete
	tab.Update(sessionDeletedMsg{})
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode, got %d", tab.mode)
	}

	// Error case
	tab.Update(sessionDeletedMsg{err: fmt.Errorf("delete failed")})
	if tab.err != "delete failed" {
		t.Fatalf("expected error, got %q", tab.err)
	}
}

func TestSessionsTabUpdateSessionHistory(t *testing.T) {
	tab := newTestSessionsTab()
	tab.sessions = []session.Session{
		{Name: "s1", CreatedAt: time.Now()},
	}
	tab.selected = 0

	tab.Update(sessionHistoryMsg{
		sessionName: "s1",
		messages:    []*Message{NewUserMessage("hello", "all")},
		totalCount:  5,
	})
	if tab.previewName != "s1" {
		t.Fatalf("expected previewName='s1', got %q", tab.previewName)
	}
	if tab.previewTotal != 5 {
		t.Fatalf("expected previewTotal=5, got %d", tab.previewTotal)
	}
}

func TestSessionsTabUpdateSessionCreated(t *testing.T) {
	tab := newTestSessionsTab()
	tab.mode = sessionsModeNewName

	// Error case
	tab.Update(sessionCreatedMsg{err: fmt.Errorf("create failed")})
	if tab.err != "create failed" {
		t.Fatalf("expected error, got %q", tab.err)
	}
	if tab.mode != sessionsModeList {
		t.Fatalf("expected list mode, got %d", tab.mode)
	}

	// Success case returns SwitchToChatMsg
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

	// Error case
	tab.Update(vibespacesForPickerMsg{err: fmt.Errorf("load error")})
	if tab.err != "load error" {
		t.Fatalf("expected error, got %q", tab.err)
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

	// j moves down
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if tab.newVSCursor != 1 {
		t.Fatalf("expected 1, got %d", tab.newVSCursor)
	}

	// k moves up
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if tab.newVSCursor != 0 {
		t.Fatalf("expected 0, got %d", tab.newVSCursor)
	}

	// x also toggles
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !tab.newVSSelected[0] {
		t.Fatal("expected vs1 selected after x")
	}
}

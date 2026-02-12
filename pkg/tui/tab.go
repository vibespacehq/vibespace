package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/session"
)

// Tab is implemented by each top-level tab in the App.
type Tab interface {
	tea.Model
	Title() string
	ShortHelp() []key.Binding
	SetSize(width, height int)
}

// Tab indices.
const (
	TabVibespaces = iota
	TabChat
	TabMonitor
	TabSessions
	TabRemote
	tabCount
)

// TabNames maps index to display name.
var TabNames = [tabCount]string{
	"Vibespaces",
	"Chat",
	"Monitor",
	"Sessions",
	"Remote",
}

// --- Inter-tab messages ---

// TabActivateMsg is sent to a tab when it becomes active.
type TabActivateMsg struct{}

// TabDeactivateMsg is sent to a tab when it loses focus.
type TabDeactivateMsg struct{}

// SwitchTabMsg requests the App to switch to a specific tab.
type SwitchTabMsg struct{ Tab int }

// SwitchToChatMsg requests the App to open the Chat tab with a session.
type SwitchToChatMsg struct {
	Session *session.Session
	Resume  bool
}

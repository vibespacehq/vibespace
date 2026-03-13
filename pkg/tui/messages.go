package tui

import (
	"github.com/vibespacehq/vibespace/pkg/permission"
	"github.com/vibespacehq/vibespace/pkg/session"
)

// RichMessageMsg is sent when an agent produces a rich message
type RichMessageMsg struct {
	AgentKey string
	Message  *Message
}

// AgentConnectedMsg is sent when an agent connection is established
type AgentConnectedMsg struct {
	Address session.AgentAddress
}

// AgentDisconnectedMsg is sent when an agent disconnects
type AgentDisconnectedMsg struct {
	Address session.AgentAddress
	Error   error
}

// AgentErrorMsg is sent when an agent connection error occurs
type AgentErrorMsg struct {
	Address session.AgentAddress
	Error   error
}

// DaemonStatusMsg is sent when daemon status changes
type DaemonStatusMsg struct {
	Vibespace string
	Running   bool
	Error     error
}

// InitCompleteMsg is sent when all initial connections are established
type InitCompleteMsg struct {
	Errors []error
}

// QuitMsg signals the TUI should quit
type QuitMsg struct{}

// SaveSessionMsg signals the ad-hoc session should be saved
type SaveSessionMsg struct {
	Name string
}

// AddAgentMsg signals an agent should be added to the session
type AddAgentMsg struct {
	Address session.AgentAddress
}

// RemoveAgentMsg signals an agent should be removed from the session
type RemoveAgentMsg struct {
	Address session.AgentAddress
}

// FocusAgentMsg signals a focus on a specific agent
type FocusAgentMsg struct {
	Address session.AgentAddress
}

// SplitViewMsg signals a return to split view
type SplitViewMsg struct{}

// TickMsg is sent periodically for updates
type TickMsg struct{}

// FocusReturnMsg is sent when returning from interactive focus mode
type FocusReturnMsg struct {
	Address session.AgentAddress
}

// ThinkingStartMsg signals an agent has started thinking
type ThinkingStartMsg struct {
	AgentKey string
}

// ThinkingEndMsg signals an agent has finished thinking
type ThinkingEndMsg struct {
	AgentKey string
}

// ScrollMsg signals a scroll action
type ScrollMsg struct {
	Direction ScrollDirection
	Amount    int // Number of lines
}

// ScrollDirection represents the direction to scroll
type ScrollDirection int

const (
	ScrollUp ScrollDirection = iota
	ScrollDown
	ScrollTop
	ScrollBottom
)

// HistoryLoadedMsg signals that history has been loaded from persistence
type HistoryLoadedMsg struct {
	Messages []*Message
	Error    error
}

// HistoryClearedMsg signals that history has been cleared
type HistoryClearedMsg struct{}

// historyPollMsg delivers new messages found by polling the history file
type historyPollMsg struct {
	newMessages []*Message
}

// PermissionRequestMsg is sent when the permission server receives a request.
type PermissionRequestMsg struct {
	Request *permission.Request
}

// PermissionDecisionMsg is sent when the user makes a permission decision.
type PermissionDecisionMsg struct {
	ID       string
	Decision permission.Decision
}

// AgentReconnectMsg is sent to trigger a reconnection attempt for an agent
type AgentReconnectMsg struct {
	Address  session.AgentAddress
	Attempt  int // Current attempt number
	MaxRetry int // Maximum retry attempts
}

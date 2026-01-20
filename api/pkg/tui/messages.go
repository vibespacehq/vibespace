package tui

import (
	"vibespace/pkg/session"
)

// AgentOutputMsg is sent when an agent produces output
type AgentOutputMsg struct {
	Address session.AgentAddress
	Output  string
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

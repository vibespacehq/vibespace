package portforward

import (
	"fmt"
	"time"

	"github.com/vibespacehq/vibespace/pkg/config"
)

// ForwardStatus represents the status of a port-forward
type ForwardStatus string

const (
	// StatusPending indicates the forward is waiting to be established
	StatusPending ForwardStatus = "pending"
	// StatusActive indicates the forward is active and working
	StatusActive ForwardStatus = "active"
	// StatusStopped indicates the forward was stopped intentionally
	StatusStopped ForwardStatus = "stopped"
	// StatusError indicates the forward failed
	StatusError ForwardStatus = "error"
	// StatusReconnecting indicates the forward is attempting to reconnect
	StatusReconnecting ForwardStatus = "reconnecting"
)

// ForwardType represents the type/purpose of a port-forward
type ForwardType string

const (
	// TypeSSH is the SSH terminal forward (primary, fast)
	TypeSSH ForwardType = "ssh"
	// TypeTTYD is the ttyd terminal forward (browser fallback)
	TypeTTYD ForwardType = "ttyd"
	// TypeManual is a manually added forward
	TypeManual ForwardType = "manual"
	// TypePermission is the permission server forward for TUI permission prompts
	TypePermission ForwardType = "permission"
)

// ForwardSpec describes a port-forward configuration
type ForwardSpec struct {
	LocalPort  int         `json:"local_port"`
	RemotePort int         `json:"remote_port"`
	Type       ForwardType `json:"type"`
}

// ForwardState represents the runtime state of a single port-forward
type ForwardState struct {
	Spec          ForwardSpec   `json:"spec"`
	Status        ForwardStatus `json:"status"`
	Error         string        `json:"error,omitempty"`
	LastConnected time.Time     `json:"last_connected,omitempty"`
	Reconnects    int           `json:"reconnects"`
}

// AgentSpec describes an agent configuration
type AgentSpec struct {
	Name    string `json:"name"`     // e.g., "claude-1"
	PodName string `json:"pod_name"` // e.g., "vibespace-abc123-xxx"
}

// AgentState represents the runtime state of an agent's port-forwards
type AgentState struct {
	AgentSpec
	Forwards []ForwardState `json:"forwards"`
}

// DefaultSSHPort returns the default port for SSH.
func DefaultSSHPort() int { return config.Global().Ports.SSH }

// DefaultTTYDPort returns the default port for ttyd (browser fallback).
func DefaultTTYDPort() int { return config.Global().Ports.TTYD }

// DefaultPermissionPort returns the default port for the permission server.
func DefaultPermissionPort() int { return config.Global().Ports.Permission }

// CalculateLocalPort calculates the local port based on agent number and remote port.
// Formula: (agentNum - 1) * multiplier + remotePort
func CalculateLocalPort(agentNum int, remotePort int) int {
	return (agentNum-1)*config.Global().Ports.LocalPortMultiplier + remotePort
}

// ParseAgentNumber extracts the agent number from agent name (e.g., "claude-1" -> 1)
func ParseAgentNumber(agentName string) int {
	// Default to 1 if parsing fails
	var num int
	n, _ := fmt.Sscanf(agentName, "claude-%d", &num)
	if n != 1 || num < 1 {
		return 1
	}
	return num
}

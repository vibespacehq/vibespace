// Package jsonapi defines the JSON wire types shared between the CLI and tests.
package jsonapi

import (
	"encoding/json"
	"time"
)

// JSONMeta contains metadata about the CLI response.
type JSONMeta struct {
	SchemaVersion string `json:"schema_version"`
	CLIVersion    string `json:"cli_version"`
	Timestamp     string `json:"timestamp"`
}

// JSONOutput is the standard wrapper for all JSON output.
type JSONOutput struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *JSONError  `json:"error,omitempty"`
	Meta    JSONMeta    `json:"meta"`
}

// JSONError represents an error in JSON output.
type JSONError struct {
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// NewJSONOutput creates a new JSONOutput with metadata populated.
func NewJSONOutput(version string, success bool, data interface{}, err *JSONError) JSONOutput {
	return JSONOutput{
		Success: success,
		Data:    data,
		Error:   err,
		Meta: JSONMeta{
			SchemaVersion: "1",
			CLIVersion:    version,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// VersionOutput is the JSON output for the version command.
type VersionOutput struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// ListOutput is the JSON output for the list command.
type ListOutput struct {
	Vibespaces []VibespaceListItem `json:"vibespaces"`
	Count      int                 `json:"count"`
}

// VibespaceListItem represents a vibespace in list output.
type VibespaceListItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Agents    int    `json:"agents"`
	CPU       string `json:"cpu"`
	Memory    string `json:"memory"`
	Storage   string `json:"storage"`
	CreatedAt string `json:"created_at"`
}

// StatusOutput is the JSON output for the status command.
type StatusOutput struct {
	Cluster    ClusterStatus       `json:"cluster"`
	Components []ComponentStatus   `json:"components,omitempty"`
	Daemon     *DaemonStatus       `json:"daemon,omitempty"`
	Remote     *RemoteStatusOutput `json:"remote,omitempty"`
}

// ClusterStatus represents the cluster status.
type ClusterStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Platform  string `json:"platform,omitempty"`
}

// ComponentStatus represents a component status.
type ComponentStatus struct {
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}

// DaemonStatus represents the daemon status for JSON output.
type DaemonStatus struct {
	Running    bool                       `json:"running"`
	Pid        int                        `json:"pid,omitempty"`
	Uptime     string                     `json:"uptime,omitempty"`
	Vibespaces map[string]DaemonVibespace `json:"vibespaces,omitempty"`
}

// DaemonVibespace represents a vibespace managed by the daemon.
type DaemonVibespace struct {
	AgentCount int `json:"agent_count"`
}

// AgentsOutput is the JSON output for the agents command.
type AgentsOutput struct {
	Vibespace string          `json:"vibespace"`
	Agents    []AgentListItem `json:"agents"`
	Count     int             `json:"count"`
}

// AgentListItem represents an agent in list output.
type AgentListItem struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Vibespace string `json:"vibespace"`
	Status    string `json:"status"`
}

// ForwardsOutput is the JSON output for the forward list command.
type ForwardsOutput struct {
	Vibespace string             `json:"vibespace"`
	Agents    []AgentForwardInfo `json:"agents"`
}

// AgentForwardInfo represents an agent with its forwards.
type AgentForwardInfo struct {
	Name     string        `json:"name"`
	PodName  string        `json:"pod_name,omitempty"`
	Forwards []ForwardInfo `json:"forwards"`
}

// ForwardInfo represents a port forward.
type ForwardInfo struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Reconnects int    `json:"reconnects,omitempty"`
	DNSName    string `json:"dns_name,omitempty"`
}

// SessionListOutput is the JSON output for session list command.
type SessionListOutput struct {
	Sessions []SessionListItem `json:"sessions"`
	Count    int               `json:"count"`
}

// SessionListItem represents a session in list output.
type SessionListItem struct {
	Name       string    `json:"name"`
	Vibespaces int       `json:"vibespaces"`
	LastUsed   time.Time `json:"last_used"`
}

// SessionShowOutput is the JSON output for session show command.
type SessionShowOutput struct {
	Name       string             `json:"name"`
	CreatedAt  time.Time          `json:"created_at"`
	LastUsed   time.Time          `json:"last_used"`
	Layout     string             `json:"layout"`
	Vibespaces []SessionVibespace `json:"vibespaces"`
}

// SessionVibespace represents a vibespace in a session.
type SessionVibespace struct {
	Name   string   `json:"name"`
	Agents []string `json:"agents,omitempty"`
}

// DeleteOutput is the JSON output for delete command.
type DeleteOutput struct {
	Name      string   `json:"name"`
	KeepData  bool     `json:"keep_data"`
	DryRun    bool     `json:"dry_run,omitempty"`
	Resources []string `json:"resources,omitempty"`
}

// CreateOutput is the JSON output for create command.
type CreateOutput struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// InfoOutput is the JSON output for the info command.
type InfoOutput struct {
	Name      string             `json:"name"`
	ID        string             `json:"id"`
	Status    string             `json:"status"`
	PVC       string             `json:"pvc"`
	CPU       string             `json:"cpu"`
	Memory    string             `json:"memory"`
	Storage   string             `json:"storage"`
	Mounts    []MountInfo        `json:"mounts,omitempty"`
	Agents    []AgentInfoOutput  `json:"agents"`
	Forwards  []AgentForwardInfo `json:"forwards,omitempty"`
	CreatedAt string             `json:"created_at"`
}

// MountInfo represents a mount in info output.
type MountInfo struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// AgentInfoOutput represents an agent with config in JSON output.
type AgentInfoOutput struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Status string            `json:"status"`
	Config AgentConfigOutput `json:"config"`
}

// ConfigShowOutput is the JSON output for config show command (single agent).
type ConfigShowOutput struct {
	Vibespace string            `json:"vibespace"`
	Agent     string            `json:"agent"`
	Type      string            `json:"type"`
	Config    AgentConfigOutput `json:"config"`
}

// ConfigShowAllOutput is the JSON output for config show command (all agents).
type ConfigShowAllOutput struct {
	Vibespace string            `json:"vibespace"`
	Agents    []AgentConfigItem `json:"agents"`
}

// AgentConfigItem represents an agent with its config in list output.
type AgentConfigItem struct {
	Agent  string            `json:"agent"`
	Type   string            `json:"type"`
	Config AgentConfigOutput `json:"config"`
}

// AgentConfigOutput represents agent config with all fields (no omitempty).
type AgentConfigOutput struct {
	SkipPermissions  bool     `json:"skip_permissions"`
	ShareCredentials bool     `json:"share_credentials"`
	AllowedTools     []string `json:"allowed_tools"`
	DisallowedTools  []string `json:"disallowed_tools"`
	Model            string   `json:"model"`
	MaxTurns         int      `json:"max_turns"`
	SystemPrompt     string   `json:"system_prompt"`
	ReasoningEffort  string   `json:"reasoning_effort,omitempty"`
}

// InitOutput is the JSON output for the init command.
type InitOutput struct {
	Platform string `json:"platform"`
	CPU      int    `json:"cpu"`
	Memory   int    `json:"memory"`
	Disk     int    `json:"disk"`
}

// StopOutput is the JSON output for the stop command (cluster or vibespace).
type StopOutput struct {
	Stopped bool   `json:"stopped"`
	Target  string `json:"target,omitempty"`
}

// AgentCreateOutput is the JSON output for agent create command.
type AgentCreateOutput struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
	Type      string `json:"type"`
}

// AgentDeleteOutput is the JSON output for agent delete command.
type AgentDeleteOutput struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
}

// StartOutput is the JSON output for start command.
type StartOutput struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent,omitempty"`
}

// ConfigSetOutput is the JSON output for config set command.
type ConfigSetOutput struct {
	Vibespace string            `json:"vibespace"`
	Agent     string            `json:"agent"`
	Config    AgentConfigOutput `json:"config"`
}

// ForwardAddOutput is the JSON output for forward add command.
type ForwardAddOutput struct {
	Vibespace  string `json:"vibespace"`
	Agent      string `json:"agent"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	DNSName    string `json:"dns_name,omitempty"`
}

// ForwardRemoveOutput is the JSON output for forward remove command.
type ForwardRemoveOutput struct {
	Vibespace  string `json:"vibespace"`
	Agent      string `json:"agent"`
	RemotePort int    `json:"remote_port"`
}

// SessionDeleteOutput is the JSON output for session delete command.
type SessionDeleteOutput struct {
	Name string `json:"name"`
}

// MultiListSessionsOutput is the JSON output for multi --list-sessions.
type MultiListSessionsOutput struct {
	Sessions []MultiSessionItem `json:"sessions"`
	Count    int                `json:"count"`
}

// MultiSessionItem represents a session in multi list output.
type MultiSessionItem struct {
	Name       string   `json:"name"`
	Vibespaces []string `json:"vibespaces"`
	CreatedAt  string   `json:"created_at"`
	LastUsed   string   `json:"last_used"`
}

// MultiListAgentsOutput is the JSON output for multi --list-agents.
type MultiListAgentsOutput struct {
	Session string   `json:"session"`
	Agents  []string `json:"agents"`
	Count   int      `json:"count"`
}

// MultiMessageOutput is the JSON output for multi message responses.
type MultiMessageOutput struct {
	Session   string               `json:"session"`
	Request   MultiRequestInfo     `json:"request"`
	Responses []MultiAgentResponse `json:"responses"`
}

// MultiRequestInfo contains request details.
type MultiRequestInfo struct {
	Target  string `json:"target"`
	Message string `json:"message"`
}

// MultiAgentResponse represents a response from a single agent.
type MultiAgentResponse struct {
	Agent     string         `json:"agent"`
	Timestamp string         `json:"timestamp"`
	Content   string         `json:"content"`
	ToolUses  []MultiToolUse `json:"tool_uses,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// MultiToolUse represents a tool use in an agent response.
type MultiToolUse struct {
	Tool  string `json:"tool"`
	Input string `json:"input,omitempty"`
}

// ExecOutput is the JSON output for exec command.
type ExecOutput struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
	Command   string `json:"command"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
}

// ServeOutput is the JSON output for serve command.
type ServeOutput struct {
	Running    bool   `json:"running"`
	ListenPort int    `json:"listen_port"`
	ServerIP   string `json:"server_ip"`
}

// ServeTokenOutput is the JSON output for serve --generate-token.
type ServeTokenOutput struct {
	Token     string `json:"token"`
	ExpiresIn string `json:"expires_in"`
}

// RemoteConnectOutput is the JSON output for remote connect command.
type RemoteConnectOutput struct {
	Connected  bool   `json:"connected"`
	ServerHost string `json:"server_host"`
	LocalIP    string `json:"local_ip"`
	ServerIP   string `json:"server_ip"`
}

// RemoteDisconnectOutput is the JSON output for remote disconnect command.
type RemoteDisconnectOutput struct {
	Disconnected bool `json:"disconnected"`
}

// RemoteStatusOutput is the JSON output for remote status command.
type RemoteStatusOutput struct {
	Connected   bool               `json:"connected"`
	ServerHost  string             `json:"server_host,omitempty"`
	LocalIP     string             `json:"local_ip,omitempty"`
	ServerIP    string             `json:"server_ip,omitempty"`
	ConnectedAt string             `json:"connected_at,omitempty"`
	TunnelUp    bool               `json:"tunnel_up"`
	Diagnostics []DiagnosticOutput `json:"diagnostics,omitempty"`
}

// DiagnosticOutput is the JSON output for a single diagnostic check.
type DiagnosticOutput struct {
	Check   string `json:"check"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// ClientListOutput is the JSON output for serve --list-clients.
type ClientListOutput struct {
	Clients []ClientOutput `json:"clients"`
	Count   int            `json:"count"`
}

// ClientOutput represents a registered client in JSON output.
type ClientOutput struct {
	Name         string `json:"name"`
	PublicKey    string `json:"public_key"`
	AssignedIP   string `json:"assigned_ip"`
	Hostname     string `json:"hostname,omitempty"`
	RegisteredAt string `json:"registered_at"`
}

// ParseJSONOutput unmarshals raw JSON bytes into a two-step parsing envelope.
// The Data field is kept as json.RawMessage so callers can unmarshal into
// specific types with ParseData.
type RawJSONOutput struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *JSONError      `json:"error,omitempty"`
	Meta    JSONMeta        `json:"meta"`
}

// ParseData unmarshals the Data field of a RawJSONOutput into the given type.
func ParseData[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, err
	}
	return v, nil
}

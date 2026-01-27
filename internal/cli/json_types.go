package cli

import "time"

// JSONMeta contains metadata about the CLI response
type JSONMeta struct {
	SchemaVersion string `json:"schema_version"`
	CLIVersion    string `json:"cli_version"`
	Timestamp     string `json:"timestamp"`
}

// JSONOutput is the standard wrapper for all JSON output
type JSONOutput struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *JSONError  `json:"error,omitempty"`
	Meta    JSONMeta    `json:"meta"`
}

// JSONError represents an error in JSON output
type JSONError struct {
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// NewJSONOutput creates a new JSONOutput with metadata populated.
func NewJSONOutput(success bool, data interface{}, err *JSONError) JSONOutput {
	return JSONOutput{
		Success: success,
		Data:    data,
		Error:   err,
		Meta: JSONMeta{
			SchemaVersion: "1",
			CLIVersion:    Version,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// VersionOutput is the JSON output for the version command
type VersionOutput struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// ListOutput is the JSON output for the list command
type ListOutput struct {
	Vibespaces []VibespaceListItem `json:"vibespaces"`
	Count      int                 `json:"count"`
}

// VibespaceListItem represents a vibespace in list output
type VibespaceListItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Agents    int    `json:"agents"`
	CPU       string `json:"cpu"`
	Memory    string `json:"memory"`
	Storage   string `json:"storage"`
	CreatedAt string `json:"created_at"`
}

// StatusOutput is the JSON output for the status command
type StatusOutput struct {
	Cluster    ClusterStatus     `json:"cluster"`
	Components []ComponentStatus `json:"components,omitempty"`
	Daemon     *DaemonStatus     `json:"daemon,omitempty"`
}

// ClusterStatus represents the cluster status
type ClusterStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Platform  string `json:"platform,omitempty"`
}

// ComponentStatus represents a component status
type ComponentStatus struct {
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}

// DaemonStatus represents the daemon status for JSON output
type DaemonStatus struct {
	Running     bool                         `json:"running"`
	Pid         int                          `json:"pid,omitempty"`
	Uptime      string                       `json:"uptime,omitempty"`
	Vibespaces  map[string]DaemonVibespace   `json:"vibespaces,omitempty"`
}

// DaemonVibespace represents a vibespace managed by the daemon
type DaemonVibespace struct {
	AgentCount int `json:"agent_count"`
}

// AgentsOutput is the JSON output for the agents command
type AgentsOutput struct {
	Vibespace string           `json:"vibespace"`
	Agents    []AgentListItem  `json:"agents"`
	Count     int              `json:"count"`
}

// AgentListItem represents an agent in list output
type AgentListItem struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Vibespace string `json:"vibespace"`
	Status    string `json:"status"`
}

// ForwardsOutput is the JSON output for the forward list command
type ForwardsOutput struct {
	Vibespace string              `json:"vibespace"`
	Agents    []AgentForwardInfo  `json:"agents"`
}

// AgentForwardInfo represents an agent with its forwards
type AgentForwardInfo struct {
	Name     string        `json:"name"`
	PodName  string        `json:"pod_name,omitempty"`
	Forwards []ForwardInfo `json:"forwards"`
}

// ForwardInfo represents a port forward
type ForwardInfo struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Reconnects int    `json:"reconnects,omitempty"`
}

// SessionListOutput is the JSON output for session list command
type SessionListOutput struct {
	Sessions []SessionListItem `json:"sessions"`
	Count    int               `json:"count"`
}

// SessionListItem represents a session in list output
type SessionListItem struct {
	Name        string    `json:"name"`
	Vibespaces  int       `json:"vibespaces"`
	LastUsed    time.Time `json:"last_used"`
}

// SessionShowOutput is the JSON output for session show command
type SessionShowOutput struct {
	Name        string               `json:"name"`
	CreatedAt   time.Time            `json:"created_at"`
	LastUsed    time.Time            `json:"last_used"`
	Layout      string               `json:"layout"`
	Vibespaces  []SessionVibespace   `json:"vibespaces"`
}

// SessionVibespace represents a vibespace in a session
type SessionVibespace struct {
	Name   string   `json:"name"`
	Agents []string `json:"agents,omitempty"`
}

// DeleteOutput is the JSON output for delete command
type DeleteOutput struct {
	Name      string `json:"name"`
	KeepData  bool   `json:"keep_data"`
	DryRun    bool   `json:"dry_run,omitempty"`
	Resources []string `json:"resources,omitempty"` // Resources that would be/were deleted
}

// CreateOutput is the JSON output for create command
type CreateOutput struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// ConfigShowOutput is the JSON output for config show command (single agent)
type ConfigShowOutput struct {
	Vibespace string            `json:"vibespace"`
	Agent     string            `json:"agent"`
	Type      string            `json:"type"`
	Config    AgentConfigOutput `json:"config"`
}

// ConfigShowAllOutput is the JSON output for config show command (all agents)
type ConfigShowAllOutput struct {
	Vibespace string              `json:"vibespace"`
	Agents    []AgentConfigItem   `json:"agents"`
}

// AgentConfigItem represents an agent with its config in list output
type AgentConfigItem struct {
	Agent  string            `json:"agent"`
	Type   string            `json:"type"`
	Config AgentConfigOutput `json:"config"`
}

// AgentConfigOutput represents agent config with all fields (no omitempty)
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

// InitOutput is the JSON output for the init command
type InitOutput struct {
	Platform string `json:"platform"`
	CPU      int    `json:"cpu"`
	Memory   int    `json:"memory"`
	Disk     int    `json:"disk"`
}

// StopOutput is the JSON output for the stop command (cluster or vibespace)
type StopOutput struct {
	Stopped bool   `json:"stopped"`
	Target  string `json:"target,omitempty"` // "cluster" or vibespace name
}

// AgentCreateOutput is the JSON output for agent create command
type AgentCreateOutput struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
	Type      string `json:"type"`
}

// AgentDeleteOutput is the JSON output for agent delete command
type AgentDeleteOutput struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent"`
}

// StartOutput is the JSON output for start command
type StartOutput struct {
	Vibespace string `json:"vibespace"`
	Agent     string `json:"agent,omitempty"` // Empty if starting all agents
}

// ConfigSetOutput is the JSON output for config set command
type ConfigSetOutput struct {
	Vibespace string            `json:"vibespace"`
	Agent     string            `json:"agent"`
	Config    AgentConfigOutput `json:"config"`
}

// ForwardAddOutput is the JSON output for forward add command
type ForwardAddOutput struct {
	Vibespace  string `json:"vibespace"`
	Agent      string `json:"agent"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
}

// ForwardRemoveOutput is the JSON output for forward remove command
type ForwardRemoveOutput struct {
	Vibespace  string `json:"vibespace"`
	Agent      string `json:"agent"`
	RemotePort int    `json:"remote_port"`
}

// PortsOutput is the JSON output for ports command
type PortsOutput struct {
	Vibespace string         `json:"vibespace"`
	Ports     []DetectedPort `json:"ports"`
	Count     int            `json:"count"`
}

// SessionDeleteOutput is the JSON output for session delete command
type SessionDeleteOutput struct {
	Name string `json:"name"`
}

// MultiListSessionsOutput is the JSON output for multi --list-sessions
type MultiListSessionsOutput struct {
	Sessions []MultiSessionItem `json:"sessions"`
	Count    int                `json:"count"`
}

// MultiSessionItem represents a session in multi list output
type MultiSessionItem struct {
	Name       string   `json:"name"`
	Vibespaces []string `json:"vibespaces"`
	CreatedAt  string   `json:"created_at"`
	LastUsed   string   `json:"last_used"`
}

// MultiListAgentsOutput is the JSON output for multi --list-agents
type MultiListAgentsOutput struct {
	Session string   `json:"session"`
	Agents  []string `json:"agents"`
	Count   int      `json:"count"`
}

// MultiMessageOutput is the JSON output for multi message responses
type MultiMessageOutput struct {
	Session   string               `json:"session"`
	Request   MultiRequestInfo     `json:"request"`
	Responses []MultiAgentResponse `json:"responses"`
}

// MultiRequestInfo contains request details
type MultiRequestInfo struct {
	Target  string `json:"target"`
	Message string `json:"message"`
}

// MultiAgentResponse represents a response from a single agent
type MultiAgentResponse struct {
	Agent     string             `json:"agent"`
	Timestamp string             `json:"timestamp"`
	Content   string             `json:"content"`
	ToolUses  []MultiToolUse     `json:"tool_uses,omitempty"`
	Error     string             `json:"error,omitempty"`
}

// MultiToolUse represents a tool use in an agent response
type MultiToolUse struct {
	Tool   string `json:"tool"`
	Input  string `json:"input,omitempty"`
}


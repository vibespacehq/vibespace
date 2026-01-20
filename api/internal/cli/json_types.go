package cli

import "time"

// JSONOutput is the standard wrapper for all JSON output
type JSONOutput struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *JSONError  `json:"error,omitempty"`
}

// JSONError represents an error in JSON output
type JSONError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
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
	CreatedAt string `json:"created_at"`
}

// StatusOutput is the JSON output for the status command
type StatusOutput struct {
	Cluster    ClusterStatus    `json:"cluster"`
	Components []ComponentStatus `json:"components,omitempty"`
}

// ClusterStatus represents the cluster status
type ClusterStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Platform  string `json:"platform,omitempty"`
}

// ComponentStatus represents a component status
type ComponentStatus struct {
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
}

// AgentsOutput is the JSON output for the agents command
type AgentsOutput struct {
	Vibespace string           `json:"vibespace"`
	Agents    []AgentListItem  `json:"agents"`
	Count     int              `json:"count"`
}

// AgentListItem represents an agent in list output
type AgentListItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
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

// DaemonStatusOutput is the JSON output for daemon status
type DaemonStatusOutput struct {
	Vibespace   string              `json:"vibespace"`
	Running     bool                `json:"running"`
	Uptime      string              `json:"uptime,omitempty"`
	ActivePorts int                 `json:"active_ports"`
	TotalPorts  int                 `json:"total_ports"`
	Agents      []AgentForwardInfo  `json:"agents,omitempty"`
}

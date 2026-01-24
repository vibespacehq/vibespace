package daemon

import (
	"encoding/json"
)

// RequestType identifies the type of IPC request
type RequestType string

const (
	// RequestListForwards requests the list of all forwards
	RequestListForwards RequestType = "list_forwards"
	// RequestAddForward adds a new forward
	RequestAddForward RequestType = "add_forward"
	// RequestRemoveForward removes a forward
	RequestRemoveForward RequestType = "remove_forward"
	// RequestStatus gets daemon status
	RequestStatus RequestType = "status"
	// RequestShutdown requests daemon shutdown
	RequestShutdown RequestType = "shutdown"
	// RequestPing checks if daemon is alive
	RequestPing RequestType = "ping"
	// RequestRefresh triggers reconciliation
	RequestRefresh RequestType = "refresh"
)

// Request is an IPC request from client to daemon
type Request struct {
	Type      RequestType `json:"type"`
	Vibespace string      `json:"vibespace,omitempty"` // Vibespace name (required for most operations)
	Agent     string      `json:"agent,omitempty"`     // Agent name (e.g., "claude-1")
	Port      int         `json:"port,omitempty"`      // Remote port
	Local     int         `json:"local,omitempty"`     // Local port override (0 = auto-allocate)
}

// Response is an IPC response from daemon to client
type Response struct {
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// NewSuccessResponse creates a successful response with data
func NewSuccessResponse(data interface{}) Response {
	var rawData json.RawMessage
	if data != nil {
		rawData, _ = json.Marshal(data)
	}
	return Response{
		Success: true,
		Data:    rawData,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(err error) Response {
	return Response{
		Success: false,
		Error:   err.Error(),
	}
}

// StatusResponse is the data for a status request
type StatusResponse struct {
	Vibespace   string         `json:"vibespace"`
	Running     bool           `json:"running"`
	StartedAt   string         `json:"started_at"`
	Uptime      string         `json:"uptime"`
	Agents      []AgentStatus  `json:"agents"`
	TotalPorts  int            `json:"total_ports"`
	ActivePorts int            `json:"active_ports"`
}

// AgentStatus contains status info for an agent
type AgentStatus struct {
	Name     string         `json:"name"`
	PodName  string         `json:"pod_name"`
	Forwards []ForwardInfo  `json:"forwards"`
}

// ForwardInfo contains info about a single forward
type ForwardInfo struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Reconnects int    `json:"reconnects"`
}

// ListForwardsResponse is the data for a list_forwards request
type ListForwardsResponse struct {
	Agents []AgentStatus `json:"agents"`
}

// AddForwardResponse is the data for an add_forward request
type AddForwardResponse struct {
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	Status     string `json:"status"`
}

// DaemonStatusResponse is the data for a daemon status request
type DaemonStatusResponse struct {
	Running    bool                       `json:"running"`
	StartedAt  string                     `json:"started_at"`
	Uptime     string                     `json:"uptime"`
	Pid        int                        `json:"pid"`
	Vibespaces map[string]*StatusResponse `json:"vibespaces"` // vibespace name -> status
}

// PingResponse is the data for a ping request
type PingResponse struct {
	Pid int `json:"pid"`
}

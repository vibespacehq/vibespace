package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vibespace/pkg/portforward"
)

// DaemonState represents the persistent state of a daemon
type DaemonState struct {
	Vibespace string                  `json:"vibespace"`
	StartedAt time.Time               `json:"started_at"`
	Agents    map[string]*AgentState  `json:"agents"` // key: agent name
	mu        sync.RWMutex
	filePath  string
}

// AgentState represents the state of an agent's port-forwards
type AgentState struct {
	PodName  string          `json:"pod_name"`
	Forwards []*ForwardState `json:"forwards"`
}

// ForwardState represents the state of a single port-forward
type ForwardState struct {
	LocalPort  int                      `json:"local_port"`
	RemotePort int                      `json:"remote_port"`
	Type       portforward.ForwardType  `json:"type"`
	Status     portforward.ForwardStatus `json:"status"`
	Error      string                   `json:"error,omitempty"`
	Reconnects int                      `json:"reconnects"`
}

// DaemonPaths contains paths for daemon state files
type DaemonPaths struct {
	Dir      string // ~/.vibespace/daemons/
	PidFile  string // ~/.vibespace/daemons/<name>.pid
	SockFile string // ~/.vibespace/daemons/<name>.sock
	JsonFile string // ~/.vibespace/daemons/<name>.json
}

// GetDaemonPaths returns the paths for a daemon's state files
func GetDaemonPaths(vibespace string) (DaemonPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return DaemonPaths{}, fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".vibespace", "daemons")
	return DaemonPaths{
		Dir:      dir,
		PidFile:  filepath.Join(dir, vibespace+".pid"),
		SockFile: filepath.Join(dir, vibespace+".sock"),
		JsonFile: filepath.Join(dir, vibespace+".json"),
	}, nil
}

// EnsureDaemonDir ensures the daemon directory exists
func EnsureDaemonDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".vibespace", "daemons")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create daemon directory: %w", err)
	}

	return nil
}

// NewDaemonState creates a new daemon state
func NewDaemonState(vibespace string) (*DaemonState, error) {
	paths, err := GetDaemonPaths(vibespace)
	if err != nil {
		return nil, err
	}

	return &DaemonState{
		Vibespace: vibespace,
		StartedAt: time.Now(),
		Agents:    make(map[string]*AgentState),
		filePath:  paths.JsonFile,
	}, nil
}

// LoadDaemonState loads daemon state from file
func LoadDaemonState(vibespace string) (*DaemonState, error) {
	paths, err := GetDaemonPaths(vibespace)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(paths.JsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	state.filePath = paths.JsonFile
	if state.Agents == nil {
		state.Agents = make(map[string]*AgentState)
	}

	return &state, nil
}

// Save persists the daemon state to file
func (s *DaemonState) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// GetAgent gets an agent's state, creating it if it doesn't exist
func (s *DaemonState) GetAgent(name string) *AgentState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if agent, exists := s.Agents[name]; exists {
		return agent
	}

	agent := &AgentState{
		Forwards: make([]*ForwardState, 0),
	}
	s.Agents[name] = agent
	return agent
}

// SetAgentPod sets the pod name for an agent
func (s *DaemonState) SetAgentPod(agentName, podName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if agent, exists := s.Agents[agentName]; exists {
		agent.PodName = podName
	} else {
		s.Agents[agentName] = &AgentState{
			PodName:  podName,
			Forwards: make([]*ForwardState, 0),
		}
	}
}

// AddForward adds a forward to an agent's state
func (s *DaemonState) AddForward(agentName string, fwd *ForwardState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent := s.Agents[agentName]
	if agent == nil {
		agent = &AgentState{Forwards: make([]*ForwardState, 0)}
		s.Agents[agentName] = agent
	}

	// Check if forward already exists
	for _, existing := range agent.Forwards {
		if existing.RemotePort == fwd.RemotePort {
			// Update existing
			*existing = *fwd
			return
		}
	}

	agent.Forwards = append(agent.Forwards, fwd)
}

// RemoveForward removes a forward from an agent's state
func (s *DaemonState) RemoveForward(agentName string, remotePort int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent := s.Agents[agentName]
	if agent == nil {
		return
	}

	newForwards := make([]*ForwardState, 0, len(agent.Forwards))
	for _, fwd := range agent.Forwards {
		if fwd.RemotePort != remotePort {
			newForwards = append(newForwards, fwd)
		}
	}
	agent.Forwards = newForwards
}

// GetForward gets a forward from an agent's state
func (s *DaemonState) GetForward(agentName string, remotePort int) *ForwardState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent := s.Agents[agentName]
	if agent == nil {
		return nil
	}

	for _, fwd := range agent.Forwards {
		if fwd.RemotePort == remotePort {
			return fwd
		}
	}
	return nil
}

// UpdateForwardStatus updates the status of a forward
func (s *DaemonState) UpdateForwardStatus(agentName string, remotePort int, status portforward.ForwardStatus, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent := s.Agents[agentName]
	if agent == nil {
		return
	}

	for _, fwd := range agent.Forwards {
		if fwd.RemotePort == remotePort {
			fwd.Status = status
			fwd.Error = errMsg
			return
		}
	}
}

// IncrementReconnects increments the reconnect count for a forward
func (s *DaemonState) IncrementReconnects(agentName string, remotePort int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent := s.Agents[agentName]
	if agent == nil {
		return 0
	}

	for _, fwd := range agent.Forwards {
		if fwd.RemotePort == remotePort {
			fwd.Reconnects++
			return fwd.Reconnects
		}
	}
	return 0
}

// GetAllAgents returns a copy of all agent names
func (s *DaemonState) GetAllAgents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]string, 0, len(s.Agents))
	for name := range s.Agents {
		agents = append(agents, name)
	}
	return agents
}

// RemoveAgent removes an agent from state
func (s *DaemonState) RemoveAgent(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Agents, name)
}

// Cleanup removes all state files for a daemon
func CleanupDaemonFiles(vibespace string) error {
	paths, err := GetDaemonPaths(vibespace)
	if err != nil {
		return err
	}

	// Remove all files, ignoring errors for files that don't exist
	os.Remove(paths.PidFile)
	os.Remove(paths.SockFile)
	os.Remove(paths.JsonFile)

	return nil
}

// WritePidFile writes the PID file for a daemon
func WritePidFile(vibespace string, pid int) error {
	paths, err := GetDaemonPaths(vibespace)
	if err != nil {
		return err
	}

	return os.WriteFile(paths.PidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// ReadPidFile reads the PID from a daemon's PID file
func ReadPidFile(vibespace string) (int, error) {
	paths, err := GetDaemonPaths(vibespace)
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(paths.PidFile)
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, fmt.Errorf("invalid PID file: %w", err)
	}

	return pid, nil
}

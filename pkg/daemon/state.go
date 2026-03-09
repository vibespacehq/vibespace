package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vibespacehq/vibespace/pkg/portforward"
)

// DaemonState represents the runtime state of the daemon
type DaemonState struct {
	StartedAt  time.Time                  `json:"started_at"`
	Vibespaces map[string]*VibespaceState `json:"vibespaces"` // vibespace name -> state
	mu         sync.RWMutex
}

// VibespaceState represents the state of a single vibespace
type VibespaceState struct {
	ID     string                 `json:"id"`
	Agents map[string]*AgentState `json:"agents"` // agent name -> state
	mu     sync.RWMutex
}

// NewDaemonState creates a new daemon state
func NewDaemonState() *DaemonState {
	return &DaemonState{
		StartedAt:  time.Now(),
		Vibespaces: make(map[string]*VibespaceState),
	}
}

func (s *DaemonState) GetOrCreateVibespace(name string) *VibespaceState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if vs, exists := s.Vibespaces[name]; exists {
		return vs
	}

	vs := &VibespaceState{
		Agents: make(map[string]*AgentState),
	}
	s.Vibespaces[name] = vs
	return vs
}

func (s *DaemonState) GetVibespace(name string) *VibespaceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Vibespaces[name]
}

// RemoveVibespace removes a vibespace from state
func (s *DaemonState) RemoveVibespace(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Vibespaces, name)
}

// GetAllVibespaces returns all vibespace names
func (s *DaemonState) GetAllVibespaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.Vibespaces))
	for name := range s.Vibespaces {
		names = append(names, name)
	}
	return names
}

// SetAgentPod sets the pod name for an agent in a vibespace
func (vs *VibespaceState) SetAgentPod(agentName, podName string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if agent, exists := vs.Agents[agentName]; exists {
		agent.PodName = podName
	} else {
		vs.Agents[agentName] = &AgentState{
			PodName:  podName,
			Forwards: make([]*ForwardState, 0),
		}
	}
}

func (vs *VibespaceState) GetAgentPod(agentName string) (string, bool) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if agent, exists := vs.Agents[agentName]; exists {
		return agent.PodName, true
	}
	return "", false
}

// GetAgentState returns the full agent state (nil if not found)
func (vs *VibespaceState) GetAgentState(agentName string) *AgentState {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if agent, exists := vs.Agents[agentName]; exists {
		return agent
	}
	return nil
}

// RemoveAgent removes an agent from vibespace state
func (vs *VibespaceState) RemoveAgent(agentName string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	delete(vs.Agents, agentName)
}

// AddForward adds a forward to an agent's state
func (vs *VibespaceState) AddForward(agentName string, fwd *ForwardState) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	agent := vs.Agents[agentName]
	if agent == nil {
		agent = &AgentState{Forwards: make([]*ForwardState, 0)}
		vs.Agents[agentName] = agent
	}

	// Check if forward already exists
	for _, existing := range agent.Forwards {
		if existing.RemotePort == fwd.RemotePort {
			*existing = *fwd
			return
		}
	}

	agent.Forwards = append(agent.Forwards, fwd)
}

// GetAllAgentNames returns all agent names in the vibespace
func (vs *VibespaceState) GetAllAgentNames() []string {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	names := make([]string, 0, len(vs.Agents))
	for name := range vs.Agents {
		names = append(names, name)
	}
	return names
}

// AgentState represents the state of an agent's port-forwards
type AgentState struct {
	PodName  string          `json:"pod_name"`
	Forwards []*ForwardState `json:"forwards"`
}

// ForwardState represents the state of a single port-forward
type ForwardState struct {
	LocalPort  int                       `json:"local_port"`
	RemotePort int                       `json:"remote_port"`
	Type       portforward.ForwardType   `json:"type"`
	Status     portforward.ForwardStatus `json:"status"`
	Error      string                    `json:"error,omitempty"`
	Reconnects int                       `json:"reconnects"`
	DNSName    string                    `json:"dns_name,omitempty"`
}

// EnsureDaemonDir ensures the daemon directory exists
func EnsureDaemonDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create base directory and forwards subdirectory
	baseDir := filepath.Join(home, ".vibespace")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create daemon directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "forwards"), 0755); err != nil {
		return fmt.Errorf("failed to create forwards directory: %w", err)
	}

	return nil
}

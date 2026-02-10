package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DesiredForward represents a desired port-forward configuration
type DesiredForward struct {
	ContainerPort int `json:"container"`
	LocalPort     int `json:"local"`
}

// DesiredState represents the desired port-forward state for a vibespace
type DesiredState struct {
	Agents map[string][]DesiredForward `json:"agents"` // agent name -> forwards
	mu     sync.RWMutex
}

// DesiredStateManager manages desired state files for all vibespaces
type DesiredStateManager struct {
	dir    string                   // ~/.vibespace/forwards/
	states map[string]*DesiredState // vibespace -> state
	mu     sync.RWMutex
}

// NewDesiredStateManager creates a new desired state manager
func NewDesiredStateManager() (*DesiredStateManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".vibespace", "forwards")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create forwards directory: %w", err)
	}

	mgr := &DesiredStateManager{
		dir:    dir,
		states: make(map[string]*DesiredState),
	}

	// Load existing state files
	if err := mgr.loadAll(); err != nil {
		// Log but don't fail - we can start fresh
		fmt.Printf("Warning: failed to load existing desired states: %v\n", err)
	}

	return mgr, nil
}

// loadAll loads all existing desired state files
func (m *DesiredStateManager) loadAll() error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			vibespace := entry.Name()[:len(entry.Name())-5] // Remove .json
			state, err := m.load(vibespace)
			if err != nil {
				continue
			}
			m.states[vibespace] = state
		}
	}

	return nil
}

// load loads a single vibespace's desired state
func (m *DesiredStateManager) load(vibespace string) (*DesiredState, error) {
	path := filepath.Join(m.dir, vibespace+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state DesiredState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.Agents == nil {
		state.Agents = make(map[string][]DesiredForward)
	}

	return &state, nil
}

// GetOrCreate returns the desired state for a vibespace, creating if needed
func (m *DesiredStateManager) GetOrCreate(vibespace string) *DesiredState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.states[vibespace]; exists {
		return state
	}

	state := &DesiredState{
		Agents: make(map[string][]DesiredForward),
	}
	m.states[vibespace] = state
	return state
}

// Get returns the desired state for a vibespace, or nil if not exists
func (m *DesiredStateManager) Get(vibespace string) *DesiredState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[vibespace]
}

// Save persists the desired state for a vibespace
func (m *DesiredStateManager) Save(vibespace string) error {
	m.mu.RLock()
	state, exists := m.states[vibespace]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal desired state: %w", err)
	}

	path := filepath.Join(m.dir, vibespace+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write desired state: %w", err)
	}

	return nil
}

// Remove deletes a vibespace's desired state
func (m *DesiredStateManager) Remove(vibespace string) error {
	m.mu.Lock()
	delete(m.states, vibespace)
	m.mu.Unlock()

	path := filepath.Join(m.dir, vibespace+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// AddForward adds a forward to a vibespace's agent
func (s *DesiredState) AddForward(agentName string, forward DesiredForward) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	for _, f := range s.Agents[agentName] {
		if f.ContainerPort == forward.ContainerPort {
			return // Already exists
		}
	}

	s.Agents[agentName] = append(s.Agents[agentName], forward)
}

// RemoveForward removes a forward from a vibespace's agent
func (s *DesiredState) RemoveForward(agentName string, containerPort int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	forwards := s.Agents[agentName]
	newForwards := make([]DesiredForward, 0, len(forwards))
	for _, f := range forwards {
		if f.ContainerPort != containerPort {
			newForwards = append(newForwards, f)
		}
	}
	s.Agents[agentName] = newForwards
}

// GetAgentForwards returns the forwards for an agent
func (s *DesiredState) GetAgentForwards(agentName string) []DesiredForward {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	forwards := s.Agents[agentName]
	result := make([]DesiredForward, len(forwards))
	copy(result, forwards)
	return result
}

// GetAllAgents returns all agent names with forwards
func (s *DesiredState) GetAllAgents() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]string, 0, len(s.Agents))
	for name := range s.Agents {
		agents = append(agents, name)
	}
	return agents
}

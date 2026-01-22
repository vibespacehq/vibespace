package portforward

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
	"k8s.io/client-go/rest"
)

// StateCallback is called when forward state changes
type StateCallback func(agentName string, remotePort int, status ForwardStatus, errMsg string)

// Manager manages port-forwards for a vibespace
type Manager struct {
	vibespace  string
	config     *rest.Config
	allocator  *PortAllocator
	forwarders map[string]*Forwarder // key: "agent:remotePort"
	agents     map[string]string     // agent name -> pod name
	mu         sync.RWMutex

	// Callbacks
	onStateChange StateCallback

	// Reconnection settings
	reconnectEnabled bool
	maxReconnects    int
	reconnectDelay   time.Duration
}

// ManagerConfig contains configuration for creating a Manager
type ManagerConfig struct {
	Vibespace        string
	Config           *rest.Config
	OnStateChange    StateCallback
	ReconnectEnabled bool
	MaxReconnects    int
	ReconnectDelay   time.Duration
}

// NewManager creates a new port-forward manager
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxReconnects == 0 {
		cfg.MaxReconnects = 10
	}
	if cfg.ReconnectDelay == 0 {
		cfg.ReconnectDelay = 5 * time.Second
	}

	return &Manager{
		vibespace:        cfg.Vibespace,
		config:           cfg.Config,
		allocator:        NewPortAllocator(cfg.Vibespace),
		forwarders:       make(map[string]*Forwarder),
		agents:           make(map[string]string),
		onStateChange:    cfg.OnStateChange,
		reconnectEnabled: cfg.ReconnectEnabled,
		maxReconnects:    cfg.MaxReconnects,
		reconnectDelay:   cfg.ReconnectDelay,
	}
}

// SetAgentPod sets the pod name for an agent and updates all its forwarders
func (m *Manager) SetAgentPod(agentName, podName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[agentName] = podName

	// Update pod name in all forwarders for this agent
	prefix := agentName + ":"
	for key, fwd := range m.forwarders {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			fwd.SetPodName(podName)
		}
	}
}

// GetAgentPod returns the pod name for an agent
func (m *Manager) GetAgentPod(agentName string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	podName, exists := m.agents[agentName]
	return podName, exists
}

// AddForward adds and starts a new port-forward
func (m *Manager) AddForward(agentName string, remotePort int, forwardType ForwardType, localPortOverride int) (int, error) {
	m.mu.Lock()

	podName, exists := m.agents[agentName]
	if !exists {
		m.mu.Unlock()
		return 0, fmt.Errorf("%s: %w", agentName, vserrors.ErrUnknownAgent)
	}

	key := fmt.Sprintf("%s:%d", agentName, remotePort)

	// Check if already exists
	if fwd, exists := m.forwarders[key]; exists {
		m.mu.Unlock()
		if fwd.Status() == StatusActive {
			return fwd.LocalPort(), nil
		}
		// Exists but not active, stop it first
		fwd.Stop()
	}

	// Allocate local port
	var localPort int
	var err error
	if localPortOverride > 0 {
		localPort = localPortOverride
	} else {
		localPort, err = m.allocator.AllocatePort(agentName, remotePort)
		if err != nil {
			m.mu.Unlock()
			return 0, fmt.Errorf("failed to allocate port: %w", err)
		}
	}

	// Create forwarder
	fwd := NewForwarder(ForwarderConfig{
		Config:     m.config,
		Namespace:  "vibespace",
		PodName:    podName,
		LocalPort:  localPort,
		RemotePort: remotePort,
		OnStopped:  m.handleForwarderStopped,
	})

	m.forwarders[key] = fwd
	m.mu.Unlock()

	// Start the forwarder
	if err := fwd.Start(); err != nil {
		m.mu.Lock()
		delete(m.forwarders, key)
		m.allocator.ReleasePort(agentName, remotePort)
		m.mu.Unlock()

		if m.onStateChange != nil {
			m.onStateChange(agentName, remotePort, StatusError, err.Error())
		}
		return 0, err
	}

	if m.onStateChange != nil {
		m.onStateChange(agentName, remotePort, StatusActive, "")
	}

	slog.Info("forward added",
		"agent", agentName,
		"pod", podName,
		"local_port", localPort,
		"remote_port", remotePort,
		"type", forwardType)

	return localPort, nil
}

// RemoveForward stops and removes a port-forward
func (m *Manager) RemoveForward(agentName string, remotePort int) error {
	m.mu.Lock()
	key := fmt.Sprintf("%s:%d", agentName, remotePort)

	fwd, exists := m.forwarders[key]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("%s port %d: %w", agentName, remotePort, vserrors.ErrForwardNotFound)
	}

	delete(m.forwarders, key)
	m.allocator.ReleasePort(agentName, remotePort)
	m.mu.Unlock()

	fwd.Stop()

	if m.onStateChange != nil {
		m.onStateChange(agentName, remotePort, StatusStopped, "")
	}

	slog.Info("forward removed",
		"agent", agentName,
		"remote_port", remotePort)

	return nil
}

// StopForward stops a forward without removing it (can be restarted)
func (m *Manager) StopForward(agentName string, remotePort int) error {
	m.mu.RLock()
	key := fmt.Sprintf("%s:%d", agentName, remotePort)
	fwd, exists := m.forwarders[key]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%s port %d: %w", agentName, remotePort, vserrors.ErrForwardNotFound)
	}

	fwd.Stop()

	if m.onStateChange != nil {
		m.onStateChange(agentName, remotePort, StatusStopped, "")
	}

	return nil
}

// StartForward starts a stopped forward
func (m *Manager) StartForward(agentName string, remotePort int) error {
	m.mu.RLock()
	key := fmt.Sprintf("%s:%d", agentName, remotePort)
	fwd, exists := m.forwarders[key]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%s port %d: %w", agentName, remotePort, vserrors.ErrForwardNotFound)
	}

	if fwd.Status() == StatusActive {
		return nil // Already running
	}

	fwd.Reset()
	if err := fwd.Start(); err != nil {
		if m.onStateChange != nil {
			m.onStateChange(agentName, remotePort, StatusError, err.Error())
		}
		return err
	}

	if m.onStateChange != nil {
		m.onStateChange(agentName, remotePort, StatusActive, "")
	}

	return nil
}

// RestartForward restarts a forward
func (m *Manager) RestartForward(agentName string, remotePort int) error {
	if err := m.StopForward(agentName, remotePort); err != nil {
		return err
	}

	// Brief delay for cleanup
	time.Sleep(100 * time.Millisecond)

	return m.StartForward(agentName, remotePort)
}

// RestartAll restarts all forwards
func (m *Manager) RestartAll() error {
	m.mu.RLock()
	keys := make([]string, 0, len(m.forwarders))
	for key := range m.forwarders {
		keys = append(keys, key)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, key := range keys {
		// Key format is "agentName:remotePort"
		lastColon := -1
		for i := len(key) - 1; i >= 0; i-- {
			if key[i] == ':' {
				lastColon = i
				break
			}
		}
		if lastColon == -1 {
			continue
		}
		agentName := key[:lastColon]
		var remotePort int
		fmt.Sscanf(key[lastColon+1:], "%d", &remotePort)

		if err := m.RestartForward(agentName, remotePort); err != nil {
			lastErr = err
			slog.Error("failed to restart forward",
				"agent", agentName,
				"remote_port", remotePort,
				"error", err)
		}
	}

	return lastErr
}

// ForwardInfo contains information about a forward
type ForwardInfo struct {
	LocalPort  int
	RemotePort int
	Status     ForwardStatus
	Reconnects int
	Error      error
}

// ListAllForwards returns all forwards for all agents
func (m *Manager) ListAllForwards() map[string][]*ForwardInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*ForwardInfo)

	for key, fwd := range m.forwarders {
		// Key format is "agentName:remotePort"
		// Find the last colon to split (agent names can contain colons)
		lastColon := -1
		for i := len(key) - 1; i >= 0; i-- {
			if key[i] == ':' {
				lastColon = i
				break
			}
		}
		if lastColon == -1 {
			continue // Invalid key format
		}
		agentName := key[:lastColon]

		result[agentName] = append(result[agentName], &ForwardInfo{
			LocalPort:  fwd.LocalPort(),
			RemotePort: fwd.RemotePort(),
			Status:     fwd.Status(),
			Reconnects: fwd.Reconnects(),
			Error:      fwd.LastError(),
		})
	}

	return result
}

// StopAll stops all forwards
func (m *Manager) StopAll() {
	m.mu.Lock()
	forwarders := make([]*Forwarder, 0, len(m.forwarders))
	for _, fwd := range m.forwarders {
		forwarders = append(forwarders, fwd)
	}
	m.forwarders = make(map[string]*Forwarder)
	m.mu.Unlock()

	for _, fwd := range forwarders {
		fwd.Stop()
	}

	slog.Info("all forwards stopped")
}

// handleForwarderStopped is called when a forwarder stops unexpectedly
func (m *Manager) handleForwarderStopped(fwd *Forwarder) {
	if !m.reconnectEnabled {
		return
	}

	// Check if this was an intentional stop
	if fwd.Status() == StatusStopped {
		return
	}

	go m.reconnectForwarder(fwd)
}

// reconnectForwarder attempts to reconnect a forwarder
func (m *Manager) reconnectForwarder(fwd *Forwarder) {
	reconnects := fwd.IncrementReconnects()
	if reconnects > m.maxReconnects {
		slog.Error("max reconnects exceeded",
			"pod", fwd.PodName(),
			"local_port", fwd.LocalPort(),
			"remote_port", fwd.RemotePort(),
			"reconnects", reconnects)
		fwd.SetStatus(StatusError)
		return
	}

	slog.Info("attempting reconnect",
		"pod", fwd.PodName(),
		"local_port", fwd.LocalPort(),
		"remote_port", fwd.RemotePort(),
		"attempt", reconnects)

	fwd.SetStatus(StatusReconnecting)

	// Wait before reconnecting
	time.Sleep(m.reconnectDelay)

	// Reset and restart
	fwd.Reset()
	if err := fwd.Start(); err != nil {
		slog.Error("reconnect failed",
			"pod", fwd.PodName(),
			"local_port", fwd.LocalPort(),
			"error", err)
		// The forwarder will call handleForwarderStopped again, triggering another reconnect
	}
}

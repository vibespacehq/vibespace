package portforward

import (
	"fmt"
	"net"
	"sync"
)

// PortAllocator manages port allocation for agents
type PortAllocator struct {
	vibespace     string
	allocatedPorts map[string]int // key: "agent:remotePort" -> localPort
	mu             sync.RWMutex
}

// NewPortAllocator creates a new port allocator for a vibespace
func NewPortAllocator(vibespace string) *PortAllocator {
	return &PortAllocator{
		vibespace:      vibespace,
		allocatedPorts: make(map[string]int),
	}
}

// AllocatePort allocates a local port for an agent's remote port
// Uses the formula: (agentNum - 1) * 10000 + remotePort
func (a *PortAllocator) AllocatePort(agentName string, remotePort int) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := fmt.Sprintf("%s:%d", agentName, remotePort)

	// Check if already allocated
	if localPort, exists := a.allocatedPorts[key]; exists {
		return localPort, nil
	}

	// Calculate the preferred port based on agent number
	agentNum := ParseAgentNumber(agentName)
	preferredPort := CalculateLocalPort(agentNum, remotePort)

	// Try to use the preferred port
	localPort := preferredPort
	if !isPortAvailable(localPort) {
		// Port is in use, find an alternative
		var err error
		localPort, err = findAvailablePort(preferredPort)
		if err != nil {
			return 0, fmt.Errorf("failed to allocate port for %s (remote %d): %w", agentName, remotePort, err)
		}
	}

	a.allocatedPorts[key] = localPort
	return localPort, nil
}

// ReleasePort releases an allocated port
func (a *PortAllocator) ReleasePort(agentName string, remotePort int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := fmt.Sprintf("%s:%d", agentName, remotePort)
	delete(a.allocatedPorts, key)
}

// ReleaseAllForAgent releases all allocated ports for an agent
func (a *PortAllocator) ReleaseAllForAgent(agentName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	prefix := agentName + ":"
	for key := range a.allocatedPorts {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(a.allocatedPorts, key)
		}
	}
}

// isPortAvailable checks if a port is available for binding
func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// findAvailablePort finds an available port starting from the preferred port
func findAvailablePort(preferred int) (int, error) {
	// Try the next 100 ports
	for offset := 1; offset <= 100; offset++ {
		port := preferred + offset
		if isPortAvailable(port) {
			return port, nil
		}
	}

	// Fall back to letting the OS assign a port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("no available ports: %w", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

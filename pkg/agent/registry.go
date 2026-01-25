package agent

import (
	"fmt"
	"sync"
)

// registry holds the registered agent factories.
var (
	registryMu sync.RWMutex
	registry   = make(map[Type]func() CodingAgent)
)

// Register registers an agent factory for a given type.
// This is typically called from agent implementation init() functions.
func Register(agentType Type, factory func() CodingAgent) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[agentType] = factory
}

// Get returns a new instance of the agent for the given type.
// Returns an error if the agent type is not registered.
func Get(agentType Type) (CodingAgent, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	factory, ok := registry[agentType]
	if !ok {
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}
	return factory(), nil
}

// MustGet returns a new instance of the agent for the given type.
// Panics if the agent type is not registered.
func MustGet(agentType Type) CodingAgent {
	agent, err := Get(agentType)
	if err != nil {
		panic(err)
	}
	return agent
}

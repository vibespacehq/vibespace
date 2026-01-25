package deployment

import (
	"github.com/yagizdagabak/vibespace/pkg/agent"
)

// AgentInfo contains information about an agent
type AgentInfo struct {
	// ID is the unique identifier (UUID) for this agent
	ID string

	// AgentType is the type of agent (claude-code, codex, etc.)
	AgentType agent.Type

	// AgentNum is the sequential number within the agent type (1, 2, 3...)
	AgentNum int

	// AgentName is the display name (e.g., "claude-1", "codex-2")
	AgentName string

	// DeploymentName is the Kubernetes Deployment name
	DeploymentName string

	// Status is the current status ("running", "stopped", "creating")
	Status string

	// Config holds agent configuration
	Config *agent.Config
}

// CreateDeploymentRequest contains parameters for creating a Deployment
type CreateDeploymentRequest struct {
	VibespaceID string
	Name        string

	// Agent identification
	AgentID   string     // UUID for this agent
	AgentType agent.Type // Type of agent (claude-code, codex, etc.)
	AgentNum  int        // Sequential number (1, 2, 3...)
	AgentName string     // Display name (claude-1, codex-2)

	// Container
	Image     string
	Resources Resources

	// Configuration
	Env              map[string]string
	Persistent       bool
	PVCName          string
	ShareCredentials bool          // Share credentials with other agents
	Config           *agent.Config // Agent configuration
}

// CreateAgentRequest contains parameters for creating an agent deployment
type CreateAgentRequest struct {
	VibespaceID string
	Name        string // Vibespace name

	// Agent identification
	AgentID   string     // UUID for this agent
	AgentType agent.Type // Type of agent (claude-code, codex, etc.)
	AgentNum  int        // Sequential number (1, 2, 3...)
	AgentName string     // Display name (e.g., "claude-2", "codex-1")

	// Container
	Image     string
	Resources Resources

	// Configuration
	Env              map[string]string
	PVCName          string        // Shared PVC name for all agents
	ShareCredentials bool          // Share credentials with other agents
	Config           *agent.Config // Agent configuration
}

// Resources defines compute resources for a deployment
type Resources struct {
	CPU     string
	Memory  string
	Storage string
}

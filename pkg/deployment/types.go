package deployment

import "github.com/yagizdagabak/vibespace/pkg/model"

// AgentInfo contains information about an agent
type AgentInfo struct {
	ClaudeID       string              // "1", "2", etc.
	AgentName      string              // "claude-1", "claude-2", etc.
	DeploymentName string              // Deployment name
	Status         string              // "running", "stopped", "creating"
	ClaudeConfig   *model.ClaudeConfig // Claude configuration for this agent
}

// CreateDeploymentRequest contains parameters for creating a Deployment
type CreateDeploymentRequest struct {
	VibespaceID      string
	Name             string
	ClaudeID         string // Claude instance ID (1, 2, 3, etc.)
	Image            string
	Resources        Resources
	Env              map[string]string
	Persistent       bool
	PVCName          string
	ShareCredentials bool                // Share credentials with other agents via /vibespace/.vibespace
	ClaudeConfig     *model.ClaudeConfig // Claude agent configuration
}

// CreateAgentRequest contains parameters for creating an agent deployment
type CreateAgentRequest struct {
	VibespaceID      string
	Name             string // Vibespace name
	AgentName        string // Agent name (custom or auto-generated like "claude-2")
	ClaudeID         string // Claude instance ID (2, 3, 4, etc.)
	Image            string
	Resources        Resources
	Env              map[string]string
	PVCName          string              // Shared PVC name for all agents
	ShareCredentials bool                // Share credentials with other agents via /vibespace/.vibespace
	ClaudeConfig     *model.ClaudeConfig // Claude agent configuration
}

// Resources defines compute resources for a deployment
type Resources struct {
	CPU     string
	Memory  string
	Storage string
}

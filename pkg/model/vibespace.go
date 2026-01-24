package model

// Vibespace represents an isolated development environment with Claude Code.
//
// Each vibespace runs as a Kubernetes Deployment with:
// - A minimal Linux container with Claude Code CLI pre-installed
// - Persistent storage for project files
// - Dynamic port detection and exposure
type Vibespace struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Status     string           `json:"status"` // creating, running, stopped, error
	Resources  Resources        `json:"resources"`
	Services   []ExposedService `json:"services,omitempty"` // Dynamically detected services
	Persistent bool             `json:"persistent"`
	CreatedAt  string           `json:"created_at"`
	UpdatedAt  string           `json:"updated_at,omitempty"`
	DeletedAt  string           `json:"deleted_at,omitempty"`
}

// ExposedService represents a dynamically detected service running in the vibespace.
// Claude detects running processes (dev servers, APIs, etc.) and exposes them automatically.
type ExposedService struct {
	Name string `json:"name"`           // e.g., "next-dev", "api-server"
	Port int    `json:"port"`           // Internal port (e.g., 3000)
	URL  string `json:"url,omitempty"`  // External access URL
}

// Resources represents resource allocations for a vibespace
type Resources struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

// CreateVibespaceRequest represents the request to create a new vibespace.
type CreateVibespaceRequest struct {
	// Name is the vibespace identifier (must be unique, lowercase, alphanumeric + hyphens)
	Name string `json:"name" binding:"required"`

	// Persistent enables data persistence across vibespace restarts
	Persistent bool `json:"persistent"`

	// ShareCredentials enables sharing Claude credentials across all agents
	ShareCredentials bool `json:"share_credentials"`

	// Resources defines CPU and memory limits
	Resources *Resources `json:"resources,omitempty"`

	// Env provides vibespace-specific environment variables
	Env map[string]string `json:"env,omitempty"`

	// GithubRepo is an optional GitHub repo URL to clone on startup
	GithubRepo string `json:"github_repo,omitempty"`

	// ClaudeConfig configures the Claude Code agent (nil = defaults)
	ClaudeConfig *ClaudeConfig `json:"claude_config,omitempty"`
}

// UpdateVibespaceRequest represents the request to update a vibespace
type UpdateVibespaceRequest struct {
	Name      string            `json:"name,omitempty"`
	Resources *Resources        `json:"resources,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

package model

// Template represents a pre-configured development environment with specific
// tools, ports, and AI agent support.
//
// Templates define the base configuration for workspaces. Each template is
// built as a container image during cluster setup and stored in the local registry.
// The actual image name follows the pattern: workspace-{template}-{agent}:latest
//
// Available templates:
//   - nextjs: Next.js 15.5 with React 19, TypeScript, Tailwind CSS
//   - vue: Vue 3.5 with Vite, TypeScript, Composition API
//   - jupyter: Python 3.14 with Jupyter Lab and data science libraries
//
// Each template supports multiple AI agents (claude, codex, gemini) as different
// image variants.
type Template struct {
	// ID is the unique template identifier (nextjs, vue, jupyter)
	ID string `json:"id"`

	// Name is the display name shown in UI
	Name string `json:"name"`

	// Description provides details about the stack and tools
	Description string `json:"description"`

	// Image is the base registry path (without agent suffix)
	// Example: localhost:5000/workspace-nextjs
	// Full image: localhost:5000/workspace-nextjs-claude:latest
	Image string `json:"image"`

	// Category groups templates by type (web, datascience, etc.)
	Category string `json:"category"`

	// Tools lists pre-installed software and versions
	Tools []string `json:"tools"`

	// Ports maps service names to port numbers
	// Common ports: code-server (8080), dev server (3000/5173), jupyter (8888)
	Ports map[string]int `json:"ports"`

	// Agents lists supported AI agents for this template
	Agents []string `json:"agents"`

	// Env provides template-specific environment variables
	Env map[string]string `json:"env"`

	// CreatedAt is the template creation timestamp (ISO 8601)
	CreatedAt string `json:"created_at"`
}

// CreateWorkspaceRequest represents the request to create a new workspace.
//
// Workspaces are isolated development environments based on a template.
// Each workspace runs as a Knative Service in Kubernetes with persistent storage.
type CreateWorkspaceRequest struct {
	// Name is the workspace identifier (must be unique, lowercase, alphanumeric + hyphens)
	Name string `json:"name" binding:"required"`

	// Template specifies which template to use (nextjs, vue, jupyter)
	Template string `json:"template" binding:"required"`

	// Persistent enables data persistence across workspace restarts
	Persistent bool `json:"persistent"`

	// Resources defines CPU and memory limits
	Resources *Resources `json:"resources,omitempty"`

	// Env provides workspace-specific environment variables
	Env map[string]string `json:"env,omitempty"`

	// GithubRepo is an optional GitHub repo URL to clone on startup
	// Example: https://github.com/user/repo.git
	GithubRepo string `json:"github_repo,omitempty"`

	// Agent specifies which AI agent to use (claude, codex, gemini)
	// This determines which template image variant is deployed
	Agent string `json:"agent,omitempty"`
}

// UpdateWorkspaceRequest represents the request to update a workspace
type UpdateWorkspaceRequest struct {
	Name      string            `json:"name,omitempty"`
	Resources *Resources        `json:"resources,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

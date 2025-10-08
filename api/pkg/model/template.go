package model

// Template represents a workspace template
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	Category    string            `json:"category"`
	Tools       []string          `json:"tools"`
	Ports       map[string]int    `json:"ports"`
	Env         map[string]string `json:"env"`
	CreatedAt   string            `json:"created_at"`
}

// CreateWorkspaceRequest represents the request to create a workspace
type CreateWorkspaceRequest struct {
	Name       string            `json:"name" binding:"required"`
	Template   string            `json:"template" binding:"required"`
	Persistent bool              `json:"persistent"`
	Resources  *Resources        `json:"resources,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

// UpdateWorkspaceRequest represents the request to update a workspace
type UpdateWorkspaceRequest struct {
	Name      string            `json:"name,omitempty"`
	Resources *Resources        `json:"resources,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

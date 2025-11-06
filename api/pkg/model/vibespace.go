package model

// Vibespace represents a vibespace configuration and state
type Vibespace struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Template   string            `json:"template"`
	Status     string            `json:"status"`
	Resources  Resources         `json:"resources"`
	URLs       map[string]string `json:"urls"`
	Persistent bool              `json:"persistent"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at,omitempty"`
	DeletedAt  string            `json:"deleted_at,omitempty"`
}

// Resources represents resource allocations for a vibespace
type Resources struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

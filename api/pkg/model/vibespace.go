package model

// Vibespace represents a vibespace configuration and state
type Vibespace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ProjectName string            `json:"project_name"` // DNS-friendly name for subdomain (e.g., "myproject")
	Template    string            `json:"template"`
	Status      string            `json:"status"`
	Resources   Resources         `json:"resources"`
	Ports       Ports             `json:"ports"`        // Port allocations for code/preview/prod services
	URLs        map[string]string `json:"urls"`         // Will contain code/preview/prod URLs
	Persistent  bool              `json:"persistent"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
	DeletedAt   string            `json:"deleted_at,omitempty"`
}

// Ports represents port allocations for vibespace services
// Each vibespace exposes 3 services via Traefik:
// - Code: code-server UI (VS Code in browser)
// - Preview: Development server (npm run dev, python manage.py runserver, etc.)
// - Prod: Production build server (next start, static file server, etc.)
type Ports struct {
	Code    int `json:"code"`    // Port for code-server (exposed as code.{project}.vibe.space)
	Preview int `json:"preview"` // Port for dev server (exposed as preview.{project}.vibe.space)
	Prod    int `json:"prod"`    // Port for prod server (exposed as prod.{project}.vibe.space)
}

// Resources represents resource allocations for a vibespace
type Resources struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

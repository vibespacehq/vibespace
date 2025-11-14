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

// Ports represents external-facing port allocations for vibespace services.
//
// Single-Port Architecture (Knative + Caddy):
// In Knative mode, all external traffic arrives on port 8080 where Caddy reverse proxy listens.
// Caddy inspects the Host header and routes internally to:
// - Code: 8081 (code-server)
// - Preview: 3000 (development server: npm run dev, python manage.py runserver, etc.)
// - Prod: 3001 (production server: next start, static file server, etc.)
//
// External Access (via Traefik):
// - code.{project}.vibe.space → Knative Service:8080 (Caddy) → localhost:8081 (code-server)
// - preview.{project}.vibe.space → Knative Service:8080 (Caddy) → localhost:3000 (preview)
// - prod.{project}.vibe.space → Knative Service:8080 (Caddy) → localhost:3001 (production)
//
// This struct returns external-facing ports (all 8080 in Knative mode).
// Frontend should use vibespace.urls (DNS URLs) instead of constructing URLs from ports.
// See ADR 0009 for architectural rationale.
type Ports struct {
	Code    int `json:"code"`    // External port (8080 in Knative mode)
	Preview int `json:"preview"` // External port (8080 in Knative mode)
	Prod    int `json:"prod"`    // External port (8080 in Knative mode)
}

// Resources represents resource allocations for a vibespace
type Resources struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

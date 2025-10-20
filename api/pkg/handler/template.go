package handler

import (
	"net/http"

	"workspace/pkg/model"

	"github.com/gin-gonic/gin"
)

// TemplateHandler handles template HTTP requests
type TemplateHandler struct {
	// In a real implementation, this would use a template service
	// For now, we'll return hardcoded templates
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler() *TemplateHandler {
	return &TemplateHandler{}
}

// List handles GET /api/v1/templates
func (h *TemplateHandler) List(c *gin.Context) {
	// Template metadata reflecting October 2025 stable versions
	// Images are built during cluster setup and stored in local registry
	// Each template supports multiple AI agents (claude, codex, gemini)
	// Actual image name: localhost:5000/workspace-{template}-{agent}:latest
	supportedAgents := []string{"claude", "codex", "gemini"}

	templates := []model.Template{
		{
			ID:          "nextjs",
			Name:        "Next.js 15.5",
			Description: "Next.js 15.5.5 with React 19, TypeScript 5.9.3, Tailwind CSS 4.1, and Turbopack",
			Image:       "localhost:5000/workspace-nextjs", // -agent suffix added at runtime
			Category:    "web",
			Tools:       []string{"Node.js 24.x", "npm 11.6.2", "pnpm 10.18.3", "TypeScript 5.9.3", "code-server 4.104.3"},
			Ports: map[string]int{
				"code-server": 8080,
				"dev":         3000,
			},
			Agents:    supportedAgents,
			CreatedAt: "2025-10-16T00:00:00Z",
		},
		{
			ID:          "vue",
			Name:        "Vue 3.5",
			Description: "Vue 3.5.22 with Vite 7.1.10, TypeScript 5.9.3, and Composition API",
			Image:       "localhost:5000/workspace-vue", // -agent suffix added at runtime
			Category:    "web",
			Tools:       []string{"Node.js 24.x", "npm 11.6.2", "pnpm 10.18.3", "Vite 7.1.10", "TypeScript 5.9.3", "code-server 4.104.3"},
			Ports: map[string]int{
				"code-server": 8080,
				"dev":         5173,
			},
			Agents:    supportedAgents,
			CreatedAt: "2025-10-16T00:00:00Z",
		},
		{
			ID:          "jupyter",
			Name:        "Jupyter Lab 4.4",
			Description: "Python 3.14.0 with Jupyter Lab 4.4.9, NumPy, Pandas, Matplotlib, and data science libraries",
			Image:       "localhost:5000/workspace-jupyter", // -agent suffix added at runtime
			Category:    "datascience",
			Tools:       []string{"Python 3.14.0", "Jupyter Lab 4.4.9", "NumPy", "Pandas", "Matplotlib", "Scikit-learn", "code-server 4.104.3"},
			Ports: map[string]int{
				"code-server": 8080,
				"jupyter":     8888,
			},
			Agents:    supportedAgents,
			CreatedAt: "2025-10-16T00:00:00Z",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
	})
}

// Get handles GET /api/v1/templates/:id
func (h *TemplateHandler) Get(c *gin.Context) {
	id := c.Param("id")

	// Template metadata reflecting October 2025 stable versions
	// Images are built during cluster setup and stored in local registry
	// Each template supports multiple AI agents (claude, codex, gemini)
	supportedAgents := []string{"claude", "codex", "gemini"}

	templates := map[string]model.Template{
		"nextjs": {
			ID:          "nextjs",
			Name:        "Next.js 15.5",
			Description: "Next.js 15.5.5 with React 19, TypeScript 5.9.3, Tailwind CSS 4.1, and Turbopack",
			Image:       "localhost:5000/workspace-nextjs",
			Category:    "web",
			Tools:       []string{"Node.js 24.x", "npm 11.6.2", "pnpm 10.18.3", "TypeScript 5.9.3", "code-server 4.104.3"},
			Ports: map[string]int{
				"code-server": 8080,
				"dev":         3000,
			},
			Agents:    supportedAgents,
			CreatedAt: "2025-10-16T00:00:00Z",
		},
		"vue": {
			ID:          "vue",
			Name:        "Vue 3.5",
			Description: "Vue 3.5.22 with Vite 7.1.10, TypeScript 5.9.3, and Composition API",
			Image:       "localhost:5000/workspace-vue",
			Category:    "web",
			Tools:       []string{"Node.js 24.x", "npm 11.6.2", "pnpm 10.18.3", "Vite 7.1.10", "TypeScript 5.9.3", "code-server 4.104.3"},
			Ports: map[string]int{
				"code-server": 8080,
				"dev":         5173,
			},
			Agents:    supportedAgents,
			CreatedAt: "2025-10-16T00:00:00Z",
		},
		"jupyter": {
			ID:          "jupyter",
			Name:        "Jupyter Lab 4.4",
			Description: "Python 3.14.0 with Jupyter Lab 4.4.9, NumPy, Pandas, Matplotlib, and data science libraries",
			Image:       "localhost:5000/workspace-jupyter",
			Category:    "datascience",
			Tools:       []string{"Python 3.14.0", "Jupyter Lab 4.4.9", "NumPy", "Pandas", "Matplotlib", "Scikit-learn", "code-server 4.104.3"},
			Ports: map[string]int{
				"code-server": 8080,
				"jupyter":     8888,
			},
			Agents:    supportedAgents,
			CreatedAt: "2025-10-16T00:00:00Z",
		},
	}

	template, ok := templates[id]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Template not found",
		})
		return
	}

	c.JSON(http.StatusOK, template)
}

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
	templates := []model.Template{
		{
			ID:          "nextjs",
			Name:        "Next.js",
			Description: "React framework with TypeScript",
			Image:       "workspace-nextjs:latest",
			Category:    "web",
			Tools:       []string{"Node.js", "pnpm", "TypeScript"},
			Ports: map[string]int{
				"dev": 3000,
			},
			CreatedAt: "2025-01-01T00:00:00Z",
		},
		{
			ID:          "vue",
			Name:        "Vue 3",
			Description: "Progressive JavaScript framework",
			Image:       "workspace-vue:latest",
			Category:    "web",
			Tools:       []string{"Node.js", "Vite", "TypeScript"},
			Ports: map[string]int{
				"dev": 5173,
			},
			CreatedAt: "2025-01-01T00:00:00Z",
		},
		{
			ID:          "python",
			Name:        "Python",
			Description: "Python development environment",
			Image:       "workspace-python:latest",
			Category:    "backend",
			Tools:       []string{"Python", "pip", "poetry"},
			Ports: map[string]int{
				"app": 8000,
			},
			CreatedAt: "2025-01-01T00:00:00Z",
		},
		{
			ID:          "go",
			Name:        "Go",
			Description: "Go development environment",
			Image:       "workspace-go:latest",
			Category:    "backend",
			Tools:       []string{"Go", "Air"},
			Ports: map[string]int{
				"app": 8080,
			},
			CreatedAt: "2025-01-01T00:00:00Z",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
	})
}

// Get handles GET /api/v1/templates/:id
func (h *TemplateHandler) Get(c *gin.Context) {
	id := c.Param("id")

	// Hardcoded for now - would fetch from database/registry
	templates := map[string]model.Template{
		"nextjs": {
			ID:          "nextjs",
			Name:        "Next.js",
			Description: "React framework with TypeScript",
			Image:       "workspace-nextjs:latest",
			Category:    "web",
			Tools:       []string{"Node.js", "pnpm", "TypeScript"},
			Ports: map[string]int{
				"dev": 3000,
			},
			CreatedAt: "2025-01-01T00:00:00Z",
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

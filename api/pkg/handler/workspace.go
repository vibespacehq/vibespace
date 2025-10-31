package handler

import (
	"context"
	"net/http"
	"time"

	"workspace/pkg/model"
	"workspace/pkg/workspace"

	"github.com/gin-gonic/gin"
)

// WorkspaceHandler handles workspace HTTP requests
type WorkspaceHandler struct {
	service *workspace.Service
}

// NewWorkspaceHandler creates a new workspace handler
func NewWorkspaceHandler(service *workspace.Service) *WorkspaceHandler {
	return &WorkspaceHandler{
		service: service,
	}
}

// List handles GET /api/v1/workspaces
func (h *WorkspaceHandler) List(c *gin.Context) {
	workspaces, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list workspaces",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workspaces": workspaces,
	})
}

// Get handles GET /api/v1/workspaces/:id
func (h *WorkspaceHandler) Get(c *gin.Context) {
	id := c.Param("id")

	workspace, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Workspace not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

// Create handles POST /api/v1/workspaces
func (h *WorkspaceHandler) Create(c *gin.Context) {
	var req model.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	workspace, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create workspace",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, workspace)
}

// Delete handles DELETE /api/v1/workspaces/:id
func (h *WorkspaceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete workspace",
			"details": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

// Start handles POST /api/v1/workspaces/:id/start
func (h *WorkspaceHandler) Start(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Start(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to start workspace",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workspace starting",
		"id":      id,
	})
}

// Stop handles POST /api/v1/workspaces/:id/stop
func (h *WorkspaceHandler) Stop(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.Stop(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to stop workspace",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workspace stopping",
		"id":      id,
	})
}

// Access handles GET /api/v1/workspaces/:id/access
func (h *WorkspaceHandler) Access(c *gin.Context) {
	id := c.Param("id")

	// Create a context with timeout for starting the port-forward
	// The port-forward itself runs detached, but setup should have a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	urls, err := h.service.Access(ctx, id)
	if err != nil {
		// Log full error internally for debugging
		c.Error(err)
		// Return sanitized error to client
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to access workspace. Please ensure it is running.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"urls": urls,
	})
}

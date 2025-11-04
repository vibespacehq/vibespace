package handler

import (
	"context"
	"log/slog"
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
		slog.Error("failed to list workspaces",
			"error", err,
			"remote_addr", c.ClientIP())
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

	slog.Info("workspace get request received",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	workspace, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		slog.Warn("workspace not found",
			"workspace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Workspace not found",
			"details": err.Error(),
		})
		return
	}

	slog.Info("workspace get request completed",
		"workspace_id", id,
		"status", workspace.Status,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, workspace)
}

// Create handles POST /api/v1/workspaces
func (h *WorkspaceHandler) Create(c *gin.Context) {
	var req model.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid workspace create request",
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	slog.Info("workspace create request received",
		"template", req.Template,
		"name", req.Name,
		"agent", req.Agent,
		"github_repo", req.GithubRepo,
		"persistent", req.Persistent,
		"remote_addr", c.ClientIP())

	startTime := time.Now()
	workspace, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to create workspace",
			"error", err,
			"template", req.Template,
			"name", req.Name,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create workspace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("workspace created successfully",
		"workspace_id", workspace.ID,
		"name", workspace.Name,
		"template", workspace.Template,
		"duration_ms", time.Since(startTime).Milliseconds(),
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusCreated, workspace)
}

// Delete handles DELETE /api/v1/workspaces/:id
func (h *WorkspaceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	slog.Info("workspace delete request received",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete workspace",
			"workspace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete workspace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("workspace deleted successfully",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	c.Status(http.StatusNoContent)
}

// Start handles POST /api/v1/workspaces/:id/start
func (h *WorkspaceHandler) Start(c *gin.Context) {
	id := c.Param("id")

	slog.Info("workspace start request received",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	if err := h.service.Start(c.Request.Context(), id); err != nil {
		slog.Error("failed to start workspace",
			"workspace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to start workspace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("workspace start request completed",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"message": "Workspace starting",
		"id":      id,
	})
}

// Stop handles POST /api/v1/workspaces/:id/stop
func (h *WorkspaceHandler) Stop(c *gin.Context) {
	id := c.Param("id")

	slog.Info("workspace stop request received",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	if err := h.service.Stop(c.Request.Context(), id); err != nil {
		slog.Error("failed to stop workspace",
			"workspace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to stop workspace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("workspace stop request completed",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"message": "Workspace stopping",
		"id":      id,
	})
}

// Access handles GET /api/v1/workspaces/:id/access
func (h *WorkspaceHandler) Access(c *gin.Context) {
	id := c.Param("id")

	slog.Info("workspace access request received",
		"workspace_id", id,
		"remote_addr", c.ClientIP())

	// Create a context with timeout for starting the port-forward
	// The port-forward itself runs detached, but setup should have a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	urls, err := h.service.Access(ctx, id)
	if err != nil {
		// Log full error internally for debugging
		c.Error(err)
		slog.Error("failed to access workspace",
			"workspace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		// Return sanitized error to client
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to access workspace. Please ensure it is running.",
		})
		return
	}

	slog.Info("workspace access request completed",
		"workspace_id", id,
		"urls_count", len(urls),
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"urls": urls,
	})
}

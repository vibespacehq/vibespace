package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"vibespace/pkg/model"
	"vibespace/pkg/vibespace"

	"github.com/gin-gonic/gin"
)

// VibespaceHandler handles vibespace HTTP requests
type VibespaceHandler struct {
	service *vibespace.Service
}

// NewVibespaceHandler creates a new vibespace handler
func NewVibespaceHandler(service *vibespace.Service) *VibespaceHandler {
	return &VibespaceHandler{
		service: service,
	}
}

// List handles GET /api/v1/vibespaces
func (h *VibespaceHandler) List(c *gin.Context) {
	vibespaces, err := h.service.List(c.Request.Context())
	if err != nil {
		slog.Error("failed to list vibespaces",
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list vibespaces",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"vibespaces": vibespaces,
	})
}

// Get handles GET /api/v1/vibespaces/:id
func (h *VibespaceHandler) Get(c *gin.Context) {
	id := c.Param("id")

	slog.Info("vibespace get request received",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	vibespace, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		slog.Warn("vibespace not found",
			"vibespace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Vibespace not found",
			"details": err.Error(),
		})
		return
	}

	slog.Info("vibespace get request completed",
		"vibespace_id", id,
		"status", vibespace.Status,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, vibespace)
}

// Create handles POST /api/v1/vibespaces
func (h *VibespaceHandler) Create(c *gin.Context) {
	var req model.CreateVibespaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid vibespace create request",
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	slog.Info("vibespace create request received",
		"template", req.Template,
		"name", req.Name,
		"agent", req.Agent,
		"github_repo", req.GithubRepo,
		"persistent", req.Persistent,
		"remote_addr", c.ClientIP())

	startTime := time.Now()
	vibespace, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to create vibespace",
			"error", err,
			"template", req.Template,
			"name", req.Name,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create vibespace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("vibespace created successfully",
		"vibespace_id", vibespace.ID,
		"name", vibespace.Name,
		"template", vibespace.Template,
		"duration_ms", time.Since(startTime).Milliseconds(),
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusCreated, vibespace)
}

// Delete handles DELETE /api/v1/vibespaces/:id
func (h *VibespaceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	slog.Info("vibespace delete request received",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete vibespace",
			"vibespace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete vibespace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("vibespace deleted successfully",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	c.Status(http.StatusNoContent)
}

// Start handles POST /api/v1/vibespaces/:id/start
func (h *VibespaceHandler) Start(c *gin.Context) {
	id := c.Param("id")

	slog.Info("vibespace start request received",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	if err := h.service.Start(c.Request.Context(), id); err != nil {
		slog.Error("failed to start vibespace",
			"vibespace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to start vibespace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("vibespace start request completed",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"message": "Vibespace starting",
		"id":      id,
	})
}

// Stop handles POST /api/v1/vibespaces/:id/stop
func (h *VibespaceHandler) Stop(c *gin.Context) {
	id := c.Param("id")

	slog.Info("vibespace stop request received",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	if err := h.service.Stop(c.Request.Context(), id); err != nil {
		slog.Error("failed to stop vibespace",
			"vibespace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to stop vibespace",
			"details": err.Error(),
		})
		return
	}

	slog.Info("vibespace stop request completed",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"message": "Vibespace stopping",
		"id":      id,
	})
}

// Access handles GET /api/v1/vibespaces/:id/access
func (h *VibespaceHandler) Access(c *gin.Context) {
	id := c.Param("id")

	slog.Info("vibespace access request received",
		"vibespace_id", id,
		"remote_addr", c.ClientIP())

	// Create a context with timeout for starting the port-forward
	// The port-forward itself runs detached, but setup should have a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	urls, err := h.service.Access(ctx, id)
	if err != nil {
		// Log full error internally for debugging
		c.Error(err)
		slog.Error("failed to access vibespace",
			"vibespace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		// Return sanitized error to client
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to access vibespace. Please ensure it is running.",
		})
		return
	}

	slog.Info("vibespace access request completed",
		"vibespace_id", id,
		"urls_count", len(urls),
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"urls": urls,
	})
}

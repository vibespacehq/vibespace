package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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
		"name", req.Name,
		"github_repo", req.GithubRepo,
		"persistent", req.Persistent,
		"remote_addr", c.ClientIP())

	startTime := time.Now()
	vibespace, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to create vibespace",
			"error", err,
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	urls, err := h.service.Access(ctx, id)
	if err != nil {
		c.Error(err)
		slog.Error("failed to access vibespace",
			"vibespace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
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

// RegisterService handles POST /api/v1/vibespaces/:id/services
// Called by the port detector daemon in the container when it detects a new listening port.
func (h *VibespaceHandler) RegisterService(c *gin.Context) {
	id := c.Param("id")

	var req model.ExposedService
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid service registration request",
			"vibespace_id", id,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"details": err.Error(),
		})
		return
	}

	slog.Info("service registration request received",
		"vibespace_id", id,
		"service_name", req.Name,
		"port", req.Port,
		"remote_addr", c.ClientIP())

	url, err := h.service.RegisterService(c.Request.Context(), id, &req)
	if err != nil {
		slog.Error("failed to register service",
			"vibespace_id", id,
			"service_name", req.Name,
			"port", req.Port,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to register service",
			"details": err.Error(),
		})
		return
	}

	slog.Info("service registered successfully",
		"vibespace_id", id,
		"service_name", req.Name,
		"port", req.Port,
		"url", url,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"url":  url,
		"port": req.Port,
		"name": req.Name,
	})
}

// UnregisterService handles DELETE /api/v1/vibespaces/:id/services/:port
// Called by the port detector daemon when a service stops listening.
func (h *VibespaceHandler) UnregisterService(c *gin.Context) {
	id := c.Param("id")
	portStr := c.Param("port")

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid port parameter",
		})
		return
	}

	slog.Info("service unregistration request received",
		"vibespace_id", id,
		"port", port,
		"remote_addr", c.ClientIP())

	if err := h.service.UnregisterService(c.Request.Context(), id, port); err != nil {
		slog.Error("failed to unregister service",
			"vibespace_id", id,
			"port", port,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to unregister service",
			"details": err.Error(),
		})
		return
	}

	slog.Info("service unregistered successfully",
		"vibespace_id", id,
		"port", port,
		"remote_addr", c.ClientIP())

	c.Status(http.StatusNoContent)
}

// GetServiceURL handles GET /api/v1/vibespaces/:id/services/:port
// Returns the URL for a specific port on a vibespace.
func (h *VibespaceHandler) GetServiceURL(c *gin.Context) {
	id := c.Param("id")
	portStr := c.Param("port")

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid port parameter",
			"details": fmt.Sprintf("port must be a number between 1 and 65535, got: %s", portStr),
		})
		return
	}

	slog.Info("service URL request received",
		"vibespace_id", id,
		"port", port,
		"remote_addr", c.ClientIP())

	url, err := h.service.GetServiceURL(c.Request.Context(), id, port)
	if err != nil {
		slog.Error("failed to get service URL",
			"vibespace_id", id,
			"port", port,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get service URL",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":  url,
		"port": port,
	})
}

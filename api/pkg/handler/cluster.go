package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"vibespace/pkg/k8s"

	"github.com/gin-gonic/gin"
)

// ClusterHandler handles cluster-related HTTP requests
type ClusterHandler struct {
	k8sClient *k8s.Client
}

// NewClusterHandler creates a new ClusterHandler
func NewClusterHandler(k8sClient *k8s.Client) *ClusterHandler {
	return &ClusterHandler{
		k8sClient: k8sClient,
	}
}

// ClusterStatusResponse represents the cluster status response
type ClusterStatusResponse struct {
	Healthy    bool                   `json:"healthy"`
	Version    string                 `json:"version,omitempty"`
	Components *k8s.ClusterComponents `json:"components"`
	Config     *k8s.ClusterConfig     `json:"config,omitempty"`
	Message    string                 `json:"message,omitempty"`
}

// GetStatus returns the cluster status including all components
// GET /api/v1/cluster/status
func (h *ClusterHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()

	slog.Info("cluster status check requested",
		"remote_addr", c.ClientIP())

	// Check components
	components, err := h.k8sClient.CheckComponents(ctx)
	if err != nil {
		slog.Error("failed to check cluster components",
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check cluster components",
			"details": err.Error(),
		})
		return
	}

	// Get cluster version
	version, err := h.k8sClient.Clientset().Discovery().ServerVersion()
	var versionStr string
	if err == nil {
		versionStr = version.GitVersion
	}

	// Get Knative domain configuration
	domain, _ := h.k8sClient.GetKnativeDomain(ctx)
	config := &k8s.ClusterConfig{
		KnativeDomain: domain,
	}

	response := ClusterStatusResponse{
		Healthy:    components.AllComponentsReady(),
		Version:    versionStr,
		Components: components,
		Config:     config,
		Message:    components.GetStatusSummary(),
	}

	slog.Info("cluster status check completed",
		"healthy", components.AllComponentsReady(),
		"version", versionStr,
		"knative_healthy", components.Knative.Healthy,
		"traefik_healthy", components.Traefik.Healthy,
		"registry_healthy", components.Registry.Healthy,
		"buildkit_healthy", components.BuildKit.Healthy,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, response)
}

// SetupCluster installs all required components and streams progress via SSE
// POST /api/v1/cluster/setup
func (h *ClusterHandler) SetupCluster(c *gin.Context) {
	ctx := c.Request.Context()

	slog.Info("cluster setup request received",
		"remote_addr", c.ClientIP())

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create a channel for progress updates
	progressChan := make(chan k8s.SetupProgress, 10)
	done := make(chan error, 1)

	// Start installation in background
	go func() {
		defer close(progressChan)
		defer close(done)

		// Create progress callback
		progressFn := func(progress k8s.SetupProgress) {
			progressChan <- progress
		}

		// Install components
		err := h.k8sClient.EnsureClusterComponents(ctx, progressFn)
		if err != nil {
			slog.Error("cluster setup failed during component installation",
				"error", err)
			done <- err
			return
		}

		// Apply configuration
		config := k8s.DefaultClusterConfig()
		err = h.k8sClient.ApplyConfiguration(ctx, config)
		if err != nil {
			slog.Error("cluster setup failed during configuration",
				"error", err)
			done <- fmt.Errorf("failed to apply configuration: %w", err)
			return
		}

		slog.Info("cluster setup completed successfully")
		done <- nil
	}()

	// Stream progress to client
	c.Stream(func(w io.Writer) bool {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				return false
			}

			// Send progress event
			data, _ := json.Marshal(progress)
			c.SSEvent("progress", string(data))
			c.Writer.Flush()
			return true

		case err := <-done:
			if err != nil {
				// Send error event
				errorData, _ := json.Marshal(map[string]interface{}{
					"status": "error",
					"error":  err.Error(),
				})
				c.SSEvent("error", string(errorData))
			} else {
				// Send completion event
				completeData, _ := json.Marshal(map[string]interface{}{
					"status":  "done",
					"message": "Cluster setup complete",
				})
				c.SSEvent("complete", string(completeData))
			}
			c.Writer.Flush()
			return false

		case <-time.After(30 * time.Second):
			// Timeout - send keepalive
			c.SSEvent("keepalive", "")
			c.Writer.Flush()
			return true
		}
	})
}

// EnsureComponents ensures components are ready before vibespace operations
// This is called internally by vibespace creation to ensure setup is complete
func (h *ClusterHandler) EnsureComponents(ctx context.Context) error {
	// Check if components are ready
	components, err := h.k8sClient.CheckComponents(ctx)
	if err != nil {
		return fmt.Errorf("failed to check components: %w", err)
	}

	if components.AllComponentsReady() {
		return nil
	}

	// Install missing components without progress callback
	err = h.k8sClient.EnsureClusterComponents(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to ensure components: %w", err)
	}

	// Apply default configuration
	config := k8s.DefaultClusterConfig()
	err = h.k8sClient.ApplyConfiguration(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}

	return nil
}

// ListContexts returns all available Kubernetes contexts
// GET /api/v1/cluster/contexts
func (h *ClusterHandler) ListContexts(c *gin.Context) {
	slog.Info("list contexts request received",
		"remote_addr", c.ClientIP())

	contexts, err := k8s.ListContexts()
	if err != nil {
		slog.Error("failed to list contexts",
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list contexts",
			"details": err.Error(),
		})
		return
	}

	slog.Info("list contexts request completed",
		"count", len(contexts),
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"contexts": contexts,
	})
}

// SwitchContextRequest represents the request body for context switching
type SwitchContextRequest struct {
	Confirmed bool `json:"confirmed"`
}

// SwitchContext switches to a different Kubernetes context
// POST /api/v1/cluster/contexts/:name/switch
func (h *ClusterHandler) SwitchContext(c *gin.Context) {
	contextName := c.Param("name")
	if contextName == "" {
		slog.Warn("switch context request missing context name",
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Context name is required",
		})
		return
	}

	// Check if context is remote
	isRemote := k8s.IsContextRemote(contextName)

	slog.Info("switch context request received",
		"context", contextName,
		"is_remote", isRemote,
		"remote_addr", c.ClientIP())

	// For remote clusters, require confirmation
	if isRemote {
		var req SwitchContextRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.Warn("invalid switch context request body",
				"context", contextName,
				"error", err,
				"remote_addr", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		if !req.Confirmed {
			slog.Warn("switch context to remote cluster requires confirmation",
				"context", contextName,
				"remote_addr", c.ClientIP())
			c.JSON(http.StatusBadRequest, gin.H{
				"error":              "Confirmation required for remote cluster",
				"requires_confirmation": true,
				"is_remote":            true,
				"context":              contextName,
			})
			return
		}
	}

	err := k8s.SwitchContext(contextName)
	if err != nil {
		slog.Error("failed to switch context",
			"context", contextName,
			"is_remote", isRemote,
			"error", err,
			"remote_addr", c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to switch context",
			"details": err.Error(),
		})
		return
	}

	slog.Info("context switched successfully",
		"context", contextName,
		"is_remote", isRemote,
		"remote_addr", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Switched to context: %s", contextName),
		"context":   contextName,
		"is_remote": isRemote,
	})
}

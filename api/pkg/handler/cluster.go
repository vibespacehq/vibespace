package handler

import (
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

// NOTE: ListContexts and SwitchContext handlers removed with ADR 0006
//
// With bundled Kubernetes (ADR 0006), there is only one cluster managed by
// vibespace. Context switching is no longer needed.
//
// Previously supported endpoints:
// - GET /api/v1/cluster/contexts - List available contexts
// - POST /api/v1/cluster/contexts/:name/switch - Switch active context
//
// These endpoints have been removed. See:
// - ADR 0006: docs/adr/0006-bundled-kubernetes-runtime.md
// - api/pkg/k8s/context.go for more details

package template

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/buildkit/client"
	"golang.org/x/sync/errgroup"
)

//go:embed images/config/vscode-settings.json
var vscodeSettingsData []byte

const (
	// Port numbers for workspace services
	BuildKitPort   = 1234 // BuildKit daemon port
	RegistryPort   = 5000 // Local Docker registry port
	CodeServerPort = 8080 // code-server (VS Code) port
	JupyterLabPort = 8888 // Jupyter Lab port

	// Default timeout values (can be overridden via environment variables)
	DefaultBuildKitReadyTimeout = 30 * time.Second
	DefaultTempDirCleanupAge    = time.Hour
)

var (
	// BuildKitReadyTimeout is the max time to wait for BuildKit to become ready
	// Can be configured via BUILDKIT_READY_TIMEOUT environment variable (e.g., "60s", "1m")
	BuildKitReadyTimeout = getTimeoutFromEnv("BUILDKIT_READY_TIMEOUT", DefaultBuildKitReadyTimeout)

	// TempDirCleanupAge is the age threshold for cleaning up old temp directories
	// Can be configured via TEMP_DIR_CLEANUP_AGE environment variable (e.g., "30m", "2h")
	TempDirCleanupAge = getTimeoutFromEnv("TEMP_DIR_CLEANUP_AGE", DefaultTempDirCleanupAge)
)

// getTimeoutFromEnv reads a duration from environment variable with fallback
func getTimeoutFromEnv(envVar string, defaultValue time.Duration) time.Duration {
	envVal := os.Getenv(envVar)
	if envVal == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(envVal)
	if err != nil {
		slog.Warn("Invalid duration in environment variable, using default",
			"env_var", envVar,
			"invalid_value", envVal,
			"default", defaultValue,
			"error", err)
		return defaultValue
	}

	slog.Info("Using timeout from environment variable",
		"env_var", envVar,
		"value", duration)
	return duration
}

// K8sClient interface for starting port-forwards
type K8sClient interface {
	StartPortForward(ctx context.Context, namespace, service string, localPort, remotePort int) error
	StopPortForward(namespace, service string) error
}

// Builder handles image building via BuildKit
type Builder struct {
	registryURL  string
	buildkitAddr string
	k8sClient    K8sClient
}

// NewBuilder creates a new Builder with port-forward support
// registryURL: registry to push images to (e.g., "127.0.0.1:5000")
// k8sClient: Kubernetes client for managing port-forwards
func NewBuilder(registryURL string, k8sClient K8sClient) *Builder {
	b := &Builder{
		registryURL:  registryURL,
		buildkitAddr: fmt.Sprintf("tcp://127.0.0.1:%d", BuildKitPort), // Use IPv4 explicitly
		k8sClient:    k8sClient,
	}

	// Clean up old temp directories on startup
	b.cleanupOldTempDirs()

	return b
}

// cleanupOldTempDirs removes old workspace build temp directories
// This prevents accumulation of sensitive build context files if process crashed
func (b *Builder) cleanupOldTempDirs() {
	tmpDir := os.TempDir()
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		slog.Warn("Failed to read temp dir for cleanup",
			"error", err,
			"path", tmpDir)
		return
	}

	cleaned := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Match pattern: workspace-build-*
		if !strings.HasPrefix(entry.Name(), "workspace-build-") {
			continue
		}

		// Check if directory is older than 1 hour
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > TempDirCleanupAge {
			oldPath := filepath.Join(tmpDir, entry.Name())
			if err := os.RemoveAll(oldPath); err == nil {
				cleaned++
				slog.Debug("Cleaned up old temp directory",
					"path", oldPath,
					"age", time.Since(info.ModTime()).String())
			}
		}
	}

	if cleaned > 0 {
		slog.Info("Cleaned up old workspace build directories",
			"count", cleaned,
			"threshold", TempDirCleanupAge.String())
	}
}

// waitForBuildKit polls BuildKit endpoint until it's ready
func (b *Builder) waitForBuildKit(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Try to connect to BuildKit (use 127.0.0.1 for IPv4)
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", BuildKitPort), time.Second)
			if err == nil {
				conn.Close()
				slog.Info("BuildKit port-forward ready",
					"address", fmt.Sprintf("127.0.0.1:%d", BuildKitPort))
				return nil
			}

			if time.Now().After(deadline) {
				slog.Error("Timeout waiting for BuildKit",
					"timeout", timeout,
					"last_error", err)
				return fmt.Errorf("timeout waiting for BuildKit to be ready: %w", err)
			}
		}
	}
}

// EnsurePortForwards ensures BuildKit is port-forwarded
// Note: Registry doesn't need port-forward since BuildKit accesses it via cluster DNS
func (b *Builder) EnsurePortForwards(ctx context.Context) error {
	slog.Info("Setting up port-forward to BuildKit",
		"namespace", "default",
		"service", "buildkitd",
		"port", BuildKitPort)

	// Start port-forward for BuildKit
	// This allows API server (on host) to connect to BuildKit (in cluster)
	if err := b.k8sClient.StartPortForward(ctx, "default", "buildkitd", BuildKitPort, BuildKitPort); err != nil {
		slog.Error("Failed to port-forward BuildKit",
			"error", err,
			"port", BuildKitPort)
		return fmt.Errorf("failed to port-forward BuildKit: %w", err)
	}

	slog.Info("Port-forward started, waiting for BuildKit to be ready",
		"timeout", BuildKitReadyTimeout.String())

	// Wait for BuildKit to be ready
	if err := b.waitForBuildKit(ctx, BuildKitReadyTimeout); err != nil {
		return fmt.Errorf("BuildKit not ready: %w", err)
	}

	slog.Info("BuildKit port-forward established and verified")
	return nil
}

// StopPortForwards stops the BuildKit port-forward
func (b *Builder) StopPortForwards() error {
	_ = b.k8sClient.StopPortForward("default", "buildkitd")
	return nil
}

// BuildProgress represents build progress
type BuildProgress struct {
	Template string `json:"template"`
	Status   string `json:"status"` // building, pushing, done, error
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

// BuildProgressFunc is a callback for build progress
type BuildProgressFunc func(progress BuildProgress)

// BuildImage builds a template image using BuildKit for a specific agent
func (b *Builder) BuildImage(ctx context.Context, templateID, agent string, progressFn BuildProgressFunc) error {
	// Image naming:
	// - Base images: workspace-base-{agent}:latest (e.g., workspace-base-claude:latest)
	// - Template images: workspace-{template}-{agent}:latest (e.g., workspace-nextjs-claude:latest)
	var imageName string
	if templateID == "base" {
		imageName = fmt.Sprintf("%s/workspace-base-%s:latest", b.registryURL, agent)
	} else {
		imageName = fmt.Sprintf("%s/workspace-%s-%s:latest", b.registryURL, templateID, agent)
	}

	displayName := fmt.Sprintf("%s-%s", templateID, agent)
	if progressFn != nil {
		progressFn(BuildProgress{
			Template: displayName,
			Status:   "building",
			Message:  fmt.Sprintf("Building %s...", imageName),
		})
	}

	// Get Dockerfile and context files
	dockerfile, err := GetDockerfile(templateID, agent)
	if err != nil {
		if progressFn != nil {
			progressFn(BuildProgress{
				Template: displayName,
				Status:   "error",
				Error:    err.Error(),
			})
		}
		return err
	}

	agentMD, err := GetAgentMD(templateID, agent)
	if err != nil {
		if progressFn != nil {
			progressFn(BuildProgress{
				Template: displayName,
				Status:   "error",
				Error:    err.Error(),
			})
		}
		return err
	}

	// Create temporary build context
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("workspace-build-%s-%s-*", templateID, agent))
	if err != nil {
		if progressFn != nil {
			progressFn(BuildProgress{
				Template: displayName,
				Status:   "error",
				Error:    fmt.Sprintf("Failed to create temp dir: %v", err),
			})
		}
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Log temp directory for debugging (helps track down leaks)
	slog.Info("Building image",
		"image", imageName,
		"temp_dir", tempDir,
		"template", templateID,
		"agent", agent)

	// Write Dockerfile
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, dockerfile, 0644); err != nil {
		if progressFn != nil {
			progressFn(BuildProgress{
				Template: displayName,
				Status:   "error",
				Error:    fmt.Sprintf("Failed to write Dockerfile: %v", err),
			})
		}
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Write agent instruction file
	var agentFileName string
	if templateID == "base" {
		if agent == "claude" {
			agentFileName = "CLAUDE.md"
		} else {
			agentFileName = "AGENT.md"
		}
	} else {
		agentFileName = "CLAUDE.md"
	}
	agentMDPath := filepath.Join(tempDir, agentFileName)
	if err := os.WriteFile(agentMDPath, agentMD, 0644); err != nil {
		if progressFn != nil {
			progressFn(BuildProgress{
				Template: displayName,
				Status:   "error",
				Error:    fmt.Sprintf("Failed to write agent instructions: %v", err),
			})
		}
		return fmt.Errorf("failed to write agent instructions: %w", err)
	}

	// Copy vscode-settings.json for base images
	if templateID == "base" {
		// Create config directory in temp build context
		configDir := filepath.Join(tempDir, "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			if progressFn != nil {
				progressFn(BuildProgress{
					Template: displayName,
					Status:   "error",
					Error:    fmt.Sprintf("Failed to create config directory %s: %v", configDir, err),
				})
			}
			return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
		}

		// Use embedded vscode-settings.json data (no file path dependency)
		// This ensures the settings work regardless of working directory or deployment environment
		if len(vscodeSettingsData) == 0 {
			return fmt.Errorf("embedded vscode-settings.json is empty (check go:embed directive)")
		}

		// Write embedded data to temp config directory
		settingsDestPath := filepath.Join(configDir, "vscode-settings.json")
		if err := os.WriteFile(settingsDestPath, vscodeSettingsData, 0644); err != nil {
			if progressFn != nil {
				progressFn(BuildProgress{
					Template: displayName,
					Status:   "error",
					Error:    fmt.Sprintf("Failed to write vscode-settings.json to %s: %v", settingsDestPath, err),
				})
			}
			return fmt.Errorf("failed to write vscode-settings.json to %s: %w", settingsDestPath, err)
		}

		slog.Debug("copied vscode-settings.json to build context",
			"template", templateID,
			"agent", agent,
			"dest", settingsDestPath,
			"size_bytes", len(vscodeSettingsData))
	}

	// Connect to BuildKit daemon
	c, err := client.New(ctx, b.buildkitAddr)
	if err != nil {
		if progressFn != nil {
			progressFn(BuildProgress{
				Template: displayName,
				Status:   "error",
				Error:    fmt.Sprintf("Failed to connect to BuildKit: %v", err),
			})
		}
		return fmt.Errorf("failed to connect to BuildKit: %w", err)
	}
	defer c.Close()

	// Create build options
	buildOpts := client.SolveOpt{
		LocalDirs: map[string]string{
			"context":    tempDir,
			"dockerfile": tempDir,
		},
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"filename": "Dockerfile",
		},
		Exports: []client.ExportEntry{
			{
				Type: "image",
				Attrs: map[string]string{
					"name": imageName,
					"push": "true",
				},
			},
		},
	}

	// Add build args for template images
	if templateID != "base" {
		buildOpts.FrontendAttrs["build-arg:AGENT"] = agent
	}

	// Build with progress tracking
	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	// Goroutine to run the build
	eg.Go(func() error {
		_, err := c.Solve(ctx, nil, buildOpts, ch)
		return err
	})

	// Goroutine to handle progress
	eg.Go(func() error {
		// Simple progress tracking - consume status updates
		for range ch {
			// Status updates arrive here, we could parse them
			// For now, just drain the channel
		}
		return nil
	})

	// Wait for build to complete
	if err := eg.Wait(); err != nil {
		if progressFn != nil {
			progressFn(BuildProgress{
				Template: displayName,
				Status:   "error",
				Error:    fmt.Sprintf("Build failed: %v", err),
			})
		}
		return fmt.Errorf("build failed: %w", err)
	}

	// Build completed successfully (image was pushed during build via export)
	if progressFn != nil {
		progressFn(BuildProgress{
			Template: displayName,
			Status:   "done",
			Message:  fmt.Sprintf("%s built and pushed successfully", imageName),
		})
	}

	return nil
}

// BuildBaseImages builds all base images (one per agent)
func (b *Builder) BuildBaseImages(ctx context.Context, progressFn BuildProgressFunc) error {
	agents := GetAllAgents()
	for _, agent := range agents {
		if err := b.BuildImage(ctx, "base", agent, progressFn); err != nil {
			return fmt.Errorf("failed to build base-%s: %w", agent, err)
		}
	}
	return nil
}

// BuildAllTemplates builds all template images for all agents
// First builds all base images (3: claude, codex, gemini)
// Then builds all template×agent combinations (9: nextjs×3, vue×3, jupyter×3)
func (b *Builder) BuildAllTemplates(ctx context.Context, progressFn BuildProgressFunc) error {
	// Build all base images first (one per agent)
	if err := b.BuildBaseImages(ctx, progressFn); err != nil {
		return fmt.Errorf("failed to build base images: %w", err)
	}

	// Build all template×agent combinations
	templates := GetAllTemplateIDs()
	agents := GetAllAgents()

	for _, agent := range agents {
		for _, templateID := range templates {
			if err := b.BuildImage(ctx, templateID, agent, progressFn); err != nil {
				displayName := fmt.Sprintf("%s-%s", templateID, agent)
				if progressFn != nil {
					progressFn(BuildProgress{
						Template: displayName,
						Status:   "error",
						Error:    err.Error(),
					})
				}
				return fmt.Errorf("failed to build %s: %w", displayName, err)
			}
		}
	}

	return nil
}

// TODO: Image caching optimization (future enhancement)
// Currently, images are rebuilt every time. To optimize:
// 1. Query registry API to check if image:tag exists
// 2. Use image digests for cache validation
// 3. Skip build if image already exists
// Example implementation:
//   func (b *Builder) ImageExists(ctx context.Context, imageName string) (bool, error) {
//       resp, err := http.Get(fmt.Sprintf("http://%s/v2/%s/manifests/latest", b.registryURL, imageName))
//       if err != nil || resp.StatusCode == http.StatusNotFound {
//           return false, nil
//       }
//       return resp.StatusCode == http.StatusOK, nil
//   }

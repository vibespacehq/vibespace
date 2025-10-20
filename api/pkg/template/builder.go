package template

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/moby/buildkit/client"
	"golang.org/x/sync/errgroup"
)

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
	return &Builder{
		registryURL:  registryURL,
		buildkitAddr: "tcp://127.0.0.1:1234", // Use IPv4 explicitly
		k8sClient:    k8sClient,
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
			conn, err := net.DialTimeout("tcp", "127.0.0.1:1234", time.Second)
			if err == nil {
				conn.Close()
				fmt.Println("✓ BuildKit port-forward ready at 127.0.0.1:1234")
				return nil
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for BuildKit to be ready: %w", err)
			}
		}
	}
}

// EnsurePortForwards ensures BuildKit is port-forwarded
// Note: Registry doesn't need port-forward since BuildKit accesses it via cluster DNS
func (b *Builder) EnsurePortForwards(ctx context.Context) error {
	fmt.Println("Setting up port-forward to BuildKit...")

	// Start port-forward for BuildKit (1234:1234)
	// This allows API server (on host) to connect to BuildKit (in cluster)
	if err := b.k8sClient.StartPortForward(ctx, "default", "buildkitd", 1234, 1234); err != nil {
		return fmt.Errorf("failed to port-forward BuildKit: %w", err)
	}

	fmt.Println("Port-forward started, waiting for BuildKit to be ready...")

	// Wait for BuildKit to be ready
	if err := b.waitForBuildKit(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("BuildKit not ready: %w", err)
	}

	fmt.Println("✓ BuildKit port-forward established and verified")
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

// ImageExists checks if an image exists in the registry
// TODO: Implement registry check
func (b *Builder) ImageExists(ctx context.Context, templateID string) (bool, error) {
	// TODO: Query registry API to check if image exists
	// For now, always return false to trigger build
	return false, nil
}

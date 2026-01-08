package k8s

import (
	"bufio"
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"vibespace/pkg/template"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	BuildConfigMapName = "vibespace-dockerfiles"
	BuildJobName       = "vibespace-build-templates"
	BuildJobTimeout    = 30 * time.Minute
)

// createBuildConfigMap creates a ConfigMap containing all Dockerfiles and support files
// needed by the build Job. The Job mounts this ConfigMap and uses the files as build context.
func (c *Client) createBuildConfigMap(ctx context.Context) error {
	slog.Info("creating build ConfigMap with Dockerfiles and support files")

	// Collect all files for the ConfigMap
	data := make(map[string]string)

	// Add all Dockerfiles
	dockerfiles := template.GetAllDockerfiles()
	for name, content := range dockerfiles {
		data[name] = string(content)
		slog.Debug("added Dockerfile to ConfigMap", "name", name, "size", len(content))
	}

	// Add all agent instruction files (CLAUDE.md, AGENT.md)
	agentMDs := template.GetAllAgentMDs()
	for name, content := range agentMDs {
		data[name] = string(content)
		slog.Debug("added agent MD to ConfigMap", "name", name, "size", len(content))
	}

	// Add all support files (Caddyfile, scripts, etc.)
	supportFiles := template.GetAllSupportFiles()
	for name, content := range supportFiles {
		data[name] = string(content)
		slog.Debug("added support file to ConfigMap", "name", name, "size", len(content))
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BuildConfigMapName,
			Namespace: "default",
			Labels: map[string]string{
				"app":       "vibespace",
				"component": "build",
			},
		},
		Data: data,
	}

	// Delete existing ConfigMap if it exists (idempotent)
	_ = c.clientset.CoreV1().ConfigMaps("default").Delete(ctx, BuildConfigMapName, metav1.DeleteOptions{})

	_, err := c.clientset.CoreV1().ConfigMaps("default").Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create build ConfigMap: %w", err)
	}

	slog.Info("build ConfigMap created",
		"name", BuildConfigMapName,
		"dockerfiles", len(dockerfiles),
		"agent_mds", len(agentMDs),
		"support_files", len(supportFiles))

	return nil
}

// deleteBuildConfigMap deletes the build ConfigMap
func (c *Client) deleteBuildConfigMap(ctx context.Context) error {
	err := c.clientset.CoreV1().ConfigMaps("default").Delete(ctx, BuildConfigMapName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete build ConfigMap: %w", err)
	}
	slog.Info("build ConfigMap deleted", "name", BuildConfigMapName)
	return nil
}

// createBuildJob creates a Kubernetes Job that runs buildctl to build all template images
// The Job runs in two phases:
// Phase 1: Build all 3 base images in parallel
// Phase 2: Build all 9 template images in parallel (after base images complete)
func (c *Client) createBuildJob(ctx context.Context) error {
	slog.Info("creating build Job")

	// Delete existing Job if it exists (cleanup from previous run)
	_ = c.clientset.BatchV1().Jobs("default").Delete(ctx, BuildJobName, metav1.DeleteOptions{
		PropagationPolicy: func() *metav1.DeletionPropagation {
			p := metav1.DeletePropagationForeground
			return &p
		}(),
	})

	// Wait a moment for cleanup
	time.Sleep(2 * time.Second)

	backoffLimit := int32(2)
	ttlSeconds := int32(300) // 5 minutes after completion

	// Build script that runs builds sequentially
	// The script creates proper directory structure for each build because:
	// - Dockerfiles expect files at paths like "CLAUDE.md", "config/vscode-settings.json"
	// - ConfigMap has flat file names like "base-claude-CLAUDE.md", "vscode-settings.json"
	//
	// NOTE: This Job is used for custom template builds. Pre-built templates are
	// mirrored from GHCR using MirrorGHCRImages() instead.
	buildScript := `#!/bin/sh
set -e

BUILDKIT_ADDR="tcp://buildkitd:1234"
REGISTRY="registry.default.svc.cluster.local:5000"
SRC="/dockerfiles"

# Helper function to setup base image build context
setup_base_context() {
    local agent=$1
    local ctx="/tmp/build-base-${agent}"

    rm -rf "$ctx"
    mkdir -p "$ctx/config"

    # Copy Dockerfile (remove prefix for standard naming)
    cp "$SRC/base-${agent}-Dockerfile" "$ctx/Dockerfile"

    # Copy Caddyfile (shared across all bases)
    cp "$SRC/Caddyfile" "$ctx/Caddyfile"

    # Copy vscode settings into config/ subdirectory
    cp "$SRC/vscode-settings.json" "$ctx/config/vscode-settings.json"

    # Copy agent-specific instruction file
    # Claude uses CLAUDE.md, others use AGENT.md
    if [ "$agent" = "claude" ]; then
        cp "$SRC/base-claude-CLAUDE.md" "$ctx/CLAUDE.md"
    else
        cp "$SRC/base-${agent}-AGENT.md" "$ctx/AGENT.md"
    fi

    echo "$ctx"
}

# Helper function to setup template image build context
setup_template_context() {
    local tmpl=$1
    local agent=$2
    local ctx="/tmp/build-${tmpl}-${agent}"

    rm -rf "$ctx"
    mkdir -p "$ctx"

    # Copy template Dockerfile
    cp "$SRC/${tmpl}-Dockerfile" "$ctx/Dockerfile"

    # Copy template-specific CLAUDE.md (used by all agents for template instructions)
    cp "$SRC/${tmpl}-CLAUDE.md" "$ctx/CLAUDE.md"

    # Copy preview and prod scripts
    cp "$SRC/${tmpl}-preview.sh" "$ctx/preview.sh"
    cp "$SRC/${tmpl}-prod.sh" "$ctx/prod.sh"

    # Copy supervisord config and entrypoint
    cp "$SRC/supervisord.conf" "$ctx/supervisord.conf"
    cp "$SRC/entrypoint.sh" "$ctx/entrypoint.sh"

    echo "$ctx"
}

echo "=== Phase 1: Building base images sequentially ==="
for agent in claude codex gemini; do
    CTX=$(setup_base_context "$agent")
    echo "Building base-${agent} from $CTX..."
    if buildctl --addr "$BUILDKIT_ADDR" build \
        --frontend dockerfile.v0 \
        --local context="$CTX" \
        --local dockerfile="$CTX" \
        --output type=image,name="${REGISTRY}/vibespace-base-${agent}:latest",push=true,registry.insecure=true; then
        echo "✓ base-${agent} done"
    else
        echo "✗ base-${agent} failed"
        exit 1
    fi
done
echo "All base images built successfully"

echo "=== Phase 2: Building template images sequentially ==="
for tmpl in nextjs vue jupyter; do
    for agent in claude codex gemini; do
        CTX=$(setup_template_context "$tmpl" "$agent")
        echo "Building ${tmpl}-${agent} from $CTX..."
        if buildctl --addr "$BUILDKIT_ADDR" build \
            --frontend dockerfile.v0 \
            --local context="$CTX" \
            --local dockerfile="$CTX" \
            --opt build-arg:AGENT="${agent}" \
            --output type=image,name="${REGISTRY}/vibespace-${tmpl}-${agent}:latest",push=true,registry.insecure=true; then
            echo "✓ ${tmpl}-${agent} done"
        else
            echo "✗ ${tmpl}-${agent} failed"
            exit 1
        fi
    done
done

echo "=== All 12 images built successfully ==="
`

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BuildJobName,
			Namespace: "default",
			Labels: map[string]string{
				"app":       "vibespace",
				"component": "build",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       "vibespace",
						"component": "build",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "builder",
							Image:   "moby/buildkit:v0.17.3",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{buildScript},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dockerfiles",
									MountPath: "/dockerfiles",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "dockerfiles",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: BuildConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := c.clientset.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create build Job: %w", err)
	}

	slog.Info("build Job created", "name", BuildJobName)
	return nil
}

// WatchBuildJob watches the build Job until completion, streaming logs for progress
func (c *Client) WatchBuildJob(ctx context.Context, progressFn SetupProgressFunc) error {
	slog.Info("watching build Job", "name", BuildJobName, "timeout", BuildJobTimeout)

	if progressFn != nil {
		progressFn(SetupProgress{
			Component: "templates",
			Status:    "building",
			Message:   "Building template images (this may take several minutes)...",
		})
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, BuildJobTimeout)
	defer cancel()

	// Watch Job status
	watcher, err := c.clientset.BatchV1().Jobs("default").Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", BuildJobName),
	})
	if err != nil {
		return fmt.Errorf("failed to watch build Job: %w", err)
	}
	defer watcher.Stop()

	// Start log streaming in background
	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()
	go c.streamBuildLogs(logCtx, progressFn)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for build Job: %w", ctx.Err())

		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("build Job watch channel closed")
			}

			if event.Type == watch.Error {
				return fmt.Errorf("error watching build Job")
			}

			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			// Check for completion
			for _, condition := range job.Status.Conditions {
				if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
					slog.Info("build Job completed successfully")
					if progressFn != nil {
						progressFn(SetupProgress{
							Component: "templates",
							Status:    "done",
							Message:   "All template images built successfully",
						})
					}
					return nil
				}

				if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
					errMsg := fmt.Sprintf("build Job failed: %s", condition.Message)
					slog.Error("build Job failed", "reason", condition.Reason, "message", condition.Message)
					if progressFn != nil {
						progressFn(SetupProgress{
							Component: "templates",
							Status:    "error",
							Error:     errMsg,
						})
					}
					return stderrors.New(errMsg)
				}
			}
		}
	}
}

// streamBuildLogs streams logs from the build Job pod to provide progress updates
func (c *Client) streamBuildLogs(ctx context.Context, progressFn SetupProgressFunc) {
	// Wait for pod to be created
	time.Sleep(5 * time.Second)

	// Find the pod created by the Job
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pods, err := c.clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", BuildJobName),
		})
		if err != nil || len(pods.Items) == 0 {
			time.Sleep(2 * time.Second)
			continue
		}

		pod := &pods.Items[0]
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			time.Sleep(2 * time.Second)
			continue
		}

		// Stream logs
		req := c.clientset.CoreV1().Pods("default").GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow: true,
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			slog.Debug("failed to stream build logs", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			slog.Debug("build log", "line", line)

			// Parse progress from log lines
			if progressFn != nil {
				// Look for completion markers
				if strings.Contains(line, "✓") {
					progressFn(SetupProgress{
						Component: "templates",
						Status:    "building",
						Message:   line,
					})
				} else if strings.Contains(line, "Phase 1") || strings.Contains(line, "Phase 2") || strings.Contains(line, "===") {
					progressFn(SetupProgress{
						Component: "templates",
						Status:    "building",
						Message:   line,
					})
				}
			}
		}

		stream.Close()
		return
	}
}

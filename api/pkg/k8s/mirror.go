package k8s

import (
	"bufio"
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	MirrorJobName    = "vibespace-mirror-images"
	MirrorJobTimeout = 15 * time.Minute

	// GHCR source registry
	GHCRRegistry = "ghcr.io/yagizdagabak/vibespace"

	// Local registry (ClusterIP service)
	LocalRegistry = "registry.default.svc.cluster.local:5000"
)

// TemplateImages defines all images to mirror from GHCR to local registry
var TemplateImages = []string{
	// Base images
	"vibespace-base-claude",
	"vibespace-base-codex",
	"vibespace-base-gemini",
	// Next.js template images
	"vibespace-nextjs-claude",
	"vibespace-nextjs-codex",
	"vibespace-nextjs-gemini",
	// Vue template images
	"vibespace-vue-claude",
	"vibespace-vue-codex",
	"vibespace-vue-gemini",
	// Jupyter template images
	"vibespace-jupyter-claude",
	"vibespace-jupyter-codex",
	"vibespace-jupyter-gemini",
}

// MirrorGHCRImages mirrors all pre-built images from GHCR to the local registry.
// This runs a Kubernetes Job that uses crane to copy images.
// Must be called after InstallRegistry() and the registry is ready.
func (c *Client) MirrorGHCRImages(ctx context.Context, progressFn SetupProgressFunc) error {
	slog.Info("starting GHCR image mirroring",
		"source", GHCRRegistry,
		"destination", LocalRegistry,
		"image_count", len(TemplateImages))

	if progressFn != nil {
		progressFn(SetupProgress{
			Component: "images",
			Status:    "installing",
			Message:   "Mirroring template images from GHCR...",
		})
	}

	// Create and run the mirror Job
	if err := c.createMirrorJob(ctx); err != nil {
		if progressFn != nil {
			progressFn(SetupProgress{
				Component: "images",
				Status:    "error",
				Error:     fmt.Sprintf("Failed to create mirror Job: %v", err),
			})
		}
		return fmt.Errorf("failed to create mirror Job: %w", err)
	}

	// Watch Job until completion
	if err := c.watchMirrorJob(ctx, progressFn); err != nil {
		return fmt.Errorf("mirror Job failed: %w", err)
	}

	slog.Info("all images mirrored successfully")
	return nil
}

// createMirrorJob creates a Kubernetes Job that uses crane to copy images
func (c *Client) createMirrorJob(ctx context.Context) error {
	slog.Info("creating mirror Job")

	// Delete existing Job if it exists
	_ = c.clientset.BatchV1().Jobs("default").Delete(ctx, MirrorJobName, metav1.DeleteOptions{
		PropagationPolicy: func() *metav1.DeletionPropagation {
			p := metav1.DeletePropagationForeground
			return &p
		}(),
	})

	// Wait for cleanup
	time.Sleep(2 * time.Second)

	backoffLimit := int32(2)
	ttlSeconds := int32(300) // 5 minutes after completion

	// Build the image list for the script
	var imageList strings.Builder
	for _, img := range TemplateImages {
		imageList.WriteString(fmt.Sprintf("%s\n", img))
	}

	// Mirror script using crane
	// crane is a simple tool for copying container images between registries
	mirrorScript := fmt.Sprintf(`#!/bin/sh
set -e

SRC_REGISTRY="%s"
DST_REGISTRY="%s"

echo "=== Mirroring images from GHCR to local registry ==="
echo "Source: $SRC_REGISTRY"
echo "Destination: $DST_REGISTRY"
echo ""

# List of images to mirror
IMAGES="
%s"

# Counter for progress
total=12
count=0

for img in $IMAGES; do
    # Skip empty lines
    [ -z "$img" ] && continue

    count=$((count + 1))
    echo "[$count/$total] Mirroring $img..."

    SRC="${SRC_REGISTRY}/${img}:latest"
    DST="${DST_REGISTRY}/${img}:latest"

    if crane copy "$SRC" "$DST" --insecure; then
        echo "✓ $img copied successfully"
    else
        echo "✗ $img failed"
        exit 1
    fi
done

echo ""
echo "=== All $total images mirrored successfully ==="
`, GHCRRegistry, LocalRegistry, imageList.String())

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MirrorJobName,
			Namespace: "default",
			Labels: map[string]string{
				"app":       "vibespace",
				"component": "mirror",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       "vibespace",
						"component": "mirror",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "mirror",
							Image:   "gcr.io/go-containerregistry/crane:latest",
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{mirrorScript},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("256Mi"),
									corev1.ResourceCPU:    resource.MustParse("500m"),
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
		return fmt.Errorf("failed to create mirror Job: %w", err)
	}

	slog.Info("mirror Job created", "name", MirrorJobName)
	return nil
}

// watchMirrorJob watches the mirror Job until completion, streaming logs for progress
func (c *Client) watchMirrorJob(ctx context.Context, progressFn SetupProgressFunc) error {
	slog.Info("watching mirror Job", "name", MirrorJobName, "timeout", MirrorJobTimeout)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, MirrorJobTimeout)
	defer cancel()

	// Watch Job status
	watcher, err := c.clientset.BatchV1().Jobs("default").Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", MirrorJobName),
	})
	if err != nil {
		return fmt.Errorf("failed to watch mirror Job: %w", err)
	}
	defer watcher.Stop()

	// Start log streaming in background
	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()
	go c.streamMirrorLogs(logCtx, progressFn)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for mirror Job: %w", ctx.Err())

		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("mirror Job watch channel closed")
			}

			if event.Type == watch.Error {
				return fmt.Errorf("error watching mirror Job")
			}

			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			// Check for completion
			for _, condition := range job.Status.Conditions {
				if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
					slog.Info("mirror Job completed successfully")
					if progressFn != nil {
						progressFn(SetupProgress{
							Component: "images",
							Status:    "done",
							Message:   "All template images mirrored successfully",
						})
					}
					return nil
				}

				if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
					errMsg := fmt.Sprintf("mirror Job failed: %s", condition.Message)
					slog.Error("mirror Job failed", "reason", condition.Reason, "message", condition.Message)
					if progressFn != nil {
						progressFn(SetupProgress{
							Component: "images",
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

// streamMirrorLogs streams logs from the mirror Job pod to provide progress updates
func (c *Client) streamMirrorLogs(ctx context.Context, progressFn SetupProgressFunc) {
	// Wait for pod to be created
	time.Sleep(3 * time.Second)

	// Find the pod created by the Job
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pods, err := c.clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("job-name=%s", MirrorJobName),
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
			slog.Debug("failed to stream mirror logs", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			line := scanner.Text()
			slog.Debug("mirror log", "line", line)

			// Parse progress from log lines
			if progressFn != nil {
				// Look for completion markers
				if strings.Contains(line, "✓") || strings.Contains(line, "Mirroring") {
					progressFn(SetupProgress{
						Component: "images",
						Status:    "installing",
						Message:   line,
					})
				}
			}
		}

		stream.Close()
		return
	}
}


package workspace

import (
	"context"
	"fmt"
	"time"

	"workspace/pkg/k8s"
	"workspace/pkg/model"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Service handles workspace operations
type Service struct {
	k8sClient *k8s.Client
}

// NewService creates a new workspace service
func NewService(k8sClient *k8s.Client) *Service {
	return &Service{
		k8sClient: k8sClient,
	}
}

// List returns all workspaces
func (s *Service) List(ctx context.Context) ([]*model.Workspace, error) {
	pods, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.WorkspaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=workspace",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	workspaces := make([]*model.Workspace, 0, len(pods.Items))
	for _, pod := range pods.Items {
		workspace := podToWorkspace(&pod)
		workspaces = append(workspaces, workspace)
	}

	return workspaces, nil
}

// Get returns a workspace by ID
func (s *Service) Get(ctx context.Context, id string) (*model.Workspace, error) {
	pod, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.WorkspaceNamespace).Get(ctx, fmt.Sprintf("workspace-%s", id), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	return podToWorkspace(pod), nil
}

// Create creates a new workspace
func (s *Service) Create(ctx context.Context, req *model.CreateWorkspaceRequest) (*model.Workspace, error) {
	// Ensure namespace exists
	if err := s.k8sClient.EnsureNamespace(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	id := uuid.New().String()[:8] // Short ID
	podName := fmt.Sprintf("workspace-%s", id)
	pvcName := fmt.Sprintf("workspace-%s-pvc", id)

	// Set default resources if not provided
	resources := req.Resources
	if resources == nil {
		resources = &model.Resources{
			CPU:     "1",
			Memory:  "2Gi",
			Storage: "10Gi",
		}
	}

	// Create PVC for persistent storage if requested
	if req.Persistent {
		if err := s.createPVC(ctx, pvcName, resources.Storage); err != nil {
			return nil, fmt.Errorf("failed to create PVC: %w", err)
		}
	}

	// Build init containers (for git clone if needed)
	initContainers := []corev1.Container{}
	if req.GithubRepo != "" {
		initContainers = append(initContainers, corev1.Container{
			Name:  "git-clone",
			Image: "alpine/git:latest",
			Command: []string{"sh", "-c"},
			Args: []string{
				fmt.Sprintf("git clone %s /workspace/repo || echo 'Failed to clone repository'", req.GithubRepo),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace-data",
					MountPath: "/workspace",
				},
			},
		})
	}

	// Build volumes
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	if req.Persistent {
		volumes = append(volumes, corev1.Volume{
			Name: "workspace-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "workspace-data",
			MountPath: "/workspace",
		})
	}

	// Build environment variables (including agent config)
	env := envMapToEnvVars(req.Env)
	if req.Agent != "" {
		env = append(env, corev1.EnvVar{
			Name:  "WORKSPACE_AGENT",
			Value: req.Agent,
		})
	}

	// Determine image to use from local registry
	// Image naming: workspace-{template}-{agent}:latest (e.g., workspace-nextjs-claude:latest)
	// Default to "claude" if no agent specified
	agent := req.Agent
	if agent == "" {
		agent = "claude"
	}
	workspaceImage := fmt.Sprintf("localhost:5000/workspace-%s-%s:latest", req.Template, agent)

	// Configure ports based on template
	// All templates expose code-server on port 8080
	// Jupyter also exposes Jupyter Lab on port 8888
	ports := []corev1.ContainerPort{
		{
			Name:          "code-server",
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Add Jupyter Lab port if this is a Jupyter workspace
	if req.Template == "jupyter" {
		ports = append(ports, corev1.ContainerPort{
			Name:          "jupyter",
			ContainerPort: 8888,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Create pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: k8s.WorkspaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       req.Name,
				"app.kubernetes.io/managed-by": "workspace",
				"workspace.dev/id":             id,
				"workspace.dev/template":       req.Template,
			},
			Annotations: map[string]string{
				"workspace.dev/github-repo": req.GithubRepo,
				"workspace.dev/agent":       req.Agent,
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: initContainers,
			Containers: []corev1.Container{
				{
					Name:         "code-server",
					Image:        workspaceImage,
					Ports:        ports,
					Env:          env,
					VolumeMounts: volumeMounts,
					// Resources will be added later
				},
			},
			Volumes:       volumes,
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}

	created, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.WorkspaceNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace pod: %w", err)
	}

	return podToWorkspace(created), nil
}

// Delete deletes a workspace
// Phase 1 (MVP): Deletes the pod only
// Phase 2 (Knative): Will delete Knative Service which handles all resources
func (s *Service) Delete(ctx context.Context, id string) error {
	// MVP Implementation: Delete pod
	// Note: PVCs are left for manual cleanup to prevent accidental data loss
	//
	// Future Knative Implementation:
	// - Delete Knative Service instead of Pod
	// - Knative will handle cleanup of all related resources
	// - PVC deletion policy will be configurable per workspace
	// - kubectl delete ksvc workspace-{id} -n workspace

	podName := fmt.Sprintf("workspace-%s", id)

	err := s.k8sClient.Clientset().CoreV1().Pods(k8s.WorkspaceNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete workspace pod: %w", err)
	}

	return nil
}

// Start starts a stopped workspace (recreates the pod)
// Phase 1 (MVP): Limited implementation - pod recreation not yet supported
// Phase 2 (Knative): Will use Knative scale-from-zero (minScale=0 -> minScale=1)
func (s *Service) Start(ctx context.Context, id string) error {
	// MVP Implementation: Check if pod exists
	// If pod exists, it's already running (no-op)
	// If pod doesn't exist, we can't recreate it yet (needs metadata storage)
	//
	// Future Knative Implementation:
	// - Knative Services automatically scale from zero on first request
	// - Start operation will patch the Knative Service to set minScale=1
	// - No need to manually recreate pods

	podName := fmt.Sprintf("workspace-%s", id)
	_, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.WorkspaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		// Pod already exists, nothing to do
		return nil
	}

	// TODO: Implement pod recreation from PVC metadata (Phase 1.5)
	// For now, just return nil (pod doesn't exist = can't start without metadata)
	return nil
}

// Stop stops a running workspace
// Phase 1 (MVP): No-op - not implemented yet
// Phase 2 (Knative): Will use Knative scale-to-zero (minScale=1 -> minScale=0)
func (s *Service) Stop(ctx context.Context, id string) error {
	// MVP Implementation: Not implemented
	// Stopping requires either:
	// 1. Deleting pod and recreating from stored metadata (complex)
	// 2. Using Knative scale-to-zero (Phase 2 approach)
	//
	// Future Knative Implementation:
	// - Patch Knative Service to set minScale=0 annotation
	// - Knative will gracefully scale down to zero replicas after idle timeout
	// - On next request, Knative auto-scales back up with the same PVC attached
	// - Provides true scale-to-zero without manual pod deletion
	// - kubectl patch ksvc workspace-{id} -n workspace -p '{"spec":{"template":{"metadata":{"annotations":{"autoscaling.knative.dev/min-scale":"0"}}}}}'

	// For now, return success without doing anything
	// The frontend will show "stopping" status briefly, then pod will remain running
	return nil
}

// Helper functions

// createPVC creates a PersistentVolumeClaim for workspace storage
func (s *Service) createPVC(ctx context.Context, name string, storage string) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k8s.WorkspaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "workspace",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(storage),
				},
			},
			StorageClassName: stringPtr("local-path"), // k3s default storage class
		},
	}

	_, err := s.k8sClient.Clientset().CoreV1().PersistentVolumeClaims(k8s.WorkspaceNamespace).Create(ctx, pvc, metav1.CreateOptions{})
	return err
}

func podToWorkspace(pod *corev1.Pod) *model.Workspace {
	status := "unknown"
	switch pod.Status.Phase {
	case corev1.PodPending:
		status = "creating"
	case corev1.PodRunning:
		status = "running"
	case corev1.PodSucceeded, corev1.PodFailed:
		status = "stopped"
	}

	id := pod.Labels["workspace.dev/id"]
	template := pod.Labels["workspace.dev/template"]

	// Build URLs based on template
	// All templates expose code-server on port 8080
	urls := map[string]string{
		"code-server": fmt.Sprintf("http://workspace-%s.local", id),
	}

	// Jupyter template also exposes Jupyter Lab on port 8888
	if template == "jupyter" {
		urls["jupyter"] = fmt.Sprintf("http://workspace-%s-8888.local", id)
	}

	return &model.Workspace{
		ID:       id,
		Name:     pod.Labels["app.kubernetes.io/name"],
		Template: template,
		Status:   status,
		Resources: model.Resources{
			CPU:     "1",
			Memory:  "2Gi",
			Storage: "10Gi",
		},
		URLs:       urls,
		Persistent: true,
		CreatedAt:  pod.CreationTimestamp.Format(time.RFC3339),
	}
}

func envMapToEnvVars(envMap map[string]string) []corev1.EnvVar {
	if envMap == nil {
		return nil
	}

	envVars := make([]corev1.EnvVar, 0, len(envMap))
	for key, value := range envMap {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	return envVars
}

// parseQuantity converts string storage size to Kubernetes resource.Quantity
func parseQuantity(storage string) resource.Quantity {
	// Parse storage string (e.g., "10Gi", "5G", "1Ti")
	qty, err := resource.ParseQuantity(storage)
	if err != nil {
		// If parsing fails, default to 10Gi
		qty = resource.MustParse("10Gi")
	}
	return qty
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

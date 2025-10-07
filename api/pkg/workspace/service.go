package workspace

import (
	"context"
	"fmt"
	"time"

	"workspace/pkg/k8s"
	"workspace/pkg/model"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
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

	// Set default resources if not provided
	resources := req.Resources
	if resources == nil {
		resources = &model.Resources{
			CPU:     "1",
			Memory:  "2Gi",
			Storage: "10Gi",
		}
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
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "code-server",
					Image: fmt.Sprintf("workspace-%s:latest", req.Template),
					Ports: []corev1.ContainerPort{
						{
							Name:          "code-server",
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Env: envMapToEnvVars(req.Env),
					// Resources will be added later
				},
			},
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
func (s *Service) Delete(ctx context.Context, id string) error {
	podName := fmt.Sprintf("workspace-%s", id)
	err := s.k8sClient.Clientset().CoreV1().Pods(k8s.WorkspaceNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	return nil
}

// Start starts a stopped workspace
func (s *Service) Start(ctx context.Context, id string) error {
	// For now, workspaces are always running
	// This will be implemented with Knative scale-from-zero later
	return nil
}

// Stop stops a running workspace
func (s *Service) Stop(ctx context.Context, id string) error {
	// For now, workspaces are always running
	// This will be implemented with Knative scale-from-zero later
	return nil
}

// Helper functions

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
		URLs: map[string]string{
			"code-server": fmt.Sprintf("http://workspace-%s.local", id),
		},
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

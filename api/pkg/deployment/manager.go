package deployment

import (
	"context"
	"fmt"

	"vibespace/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeploymentManager manages Kubernetes Deployments and Services for vibespaces
type DeploymentManager struct {
	k8sClient *k8s.Client
}

// NewDeploymentManager creates a new Deployment manager
func NewDeploymentManager(k8sClient *k8s.Client) *DeploymentManager {
	return &DeploymentManager{
		k8sClient: k8sClient,
	}
}

// CreateDeployment creates a Deployment and Service for a vibespace
func (m *DeploymentManager) CreateDeployment(ctx context.Context, req *CreateDeploymentRequest) error {
	deploymentName := fmt.Sprintf("vibespace-%s", req.VibespaceID)

	// Build labels (claude-1 is always the first agent)
	agentName := fmt.Sprintf("claude-%s", req.ClaudeID)
	labels := map[string]string{
		"app.kubernetes.io/name":       req.Name,
		"app.kubernetes.io/managed-by": "vibespace",
		"vibespace.dev/id":             req.VibespaceID,
		"vibespace.dev/claude-id":      req.ClaudeID,
		"vibespace.dev/agent-name":     agentName,
	}

	// Build environment variables
	env := m.buildEnvironment(req.VibespaceID, req.Name, agentName, req.ClaudeID, req.Env, req.ShareCredentials)

	// Build volumes and volume mounts
	volumes, volumeMounts := m.buildVolumesAndMounts(req.Persistent, req.PVCName)

	// Build init containers for persistent storage
	initContainers := m.buildInitContainers(req.Persistent)

	// Create the Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: k8s.VibespaceNamespace,
			Labels:    labels,
			Annotations: map[string]string{
				"vibespace.dev/created-at": metav1.Now().Format("2006-01-02T15:04:05Z"),
				"vibespace.dev/storage":    req.Resources.Storage,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"vibespace.dev/id":        req.VibespaceID,
					"vibespace.dev/claude-id": req.ClaudeID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:  "vibespace",
							Image: req.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "ttyd",
									ContainerPort: 7681,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "ssh",
									ContainerPort: 22,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: env,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(req.Resources.CPU),
									corev1.ResourceMemory: resource.MustParse(req.Resources.Memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(req.Resources.CPU),
									corev1.ResourceMemory: resource.MustParse(req.Resources.Memory),
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	_, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}

	// Create the Service
	if err := m.createService(ctx, deploymentName, req.VibespaceID, req.ClaudeID, labels); err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}

	return nil
}

// createService creates a ClusterIP Service for a deployment
func (m *DeploymentManager) createService(ctx context.Context, name, vibespaceID, claudeID string, labels map[string]string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k8s.VibespaceNamespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"vibespace.dev/id":        vibespaceID,
				"vibespace.dev/claude-id": claudeID,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "ttyd",
					Port:       7681,
					TargetPort: intstr.FromInt(7681),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "ssh",
					Port:       22,
					TargetPort: intstr.FromInt(22),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	_, err := m.k8sClient.Clientset().CoreV1().Services(k8s.VibespaceNamespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// CreateAgentDeployment creates a Deployment for an additional agent in a vibespace
func (m *DeploymentManager) CreateAgentDeployment(ctx context.Context, req *CreateAgentRequest) error {
	deploymentName := fmt.Sprintf("vibespace-%s-claude-%s", req.VibespaceID, req.ClaudeID)

	// Build labels
	labels := map[string]string{
		"app.kubernetes.io/name":       req.Name,
		"app.kubernetes.io/managed-by": "vibespace",
		"vibespace.dev/id":             req.VibespaceID,
		"vibespace.dev/claude-id":      req.ClaudeID,
		"vibespace.dev/agent-name":     req.AgentName,
		"vibespace.dev/is-agent":       "true",
	}

	// Build environment variables
	env := m.buildEnvironment(req.VibespaceID, req.Name, req.AgentName, req.ClaudeID, req.Env, req.ShareCredentials)

	// Build volumes and volume mounts (share the same PVC)
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	if req.PVCName != "" {
		volumes = []corev1.Volume{
			{
				Name: "vibespace-data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: req.PVCName,
					},
				},
			},
		}
		volumeMounts = []corev1.VolumeMount{
			{
				Name:      "vibespace-data",
				MountPath: "/vibespace",
			},
		}
	}

	// Create the Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: k8s.VibespaceNamespace,
			Labels:    labels,
			Annotations: map[string]string{
				"vibespace.dev/created-at": metav1.Now().Format("2006-01-02T15:04:05Z"),
				"vibespace.dev/storage":    req.Resources.Storage,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"vibespace.dev/id":        req.VibespaceID,
					"vibespace.dev/claude-id": req.ClaudeID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vibespace",
							Image: req.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "ttyd",
									ContainerPort: 7681,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "ssh",
									ContainerPort: 22,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: env,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(req.Resources.CPU),
									corev1.ResourceMemory: resource.MustParse(req.Resources.Memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(req.Resources.CPU),
									corev1.ResourceMemory: resource.MustParse(req.Resources.Memory),
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	_, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create agent Deployment: %w", err)
	}

	// Create the Service
	if err := m.createService(ctx, deploymentName, req.VibespaceID, req.ClaudeID, labels); err != nil {
		return fmt.Errorf("failed to create agent Service: %w", err)
	}

	return nil
}

// GetDeployment retrieves a Deployment by vibespace ID
func (m *DeploymentManager) GetDeployment(ctx context.Context, vibespaceID string) (*appsv1.Deployment, error) {
	deploymentName := fmt.Sprintf("vibespace-%s", vibespaceID)
	deployment, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Deployment: %w", err)
	}
	return deployment, nil
}

// GetDeploymentByName retrieves a Deployment by user-friendly name (using label selector)
func (m *DeploymentManager) GetDeploymentByName(ctx context.Context, name string) (*appsv1.Deployment, error) {
	deployments, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Deployments: %w", err)
	}

	if len(deployments.Items) == 0 {
		return nil, fmt.Errorf("vibespace '%s' not found", name)
	}

	return &deployments.Items[0], nil
}

// ListDeployments lists all Deployments for vibespaces
func (m *DeploymentManager) ListDeployments(ctx context.Context) ([]appsv1.Deployment, error) {
	deployments, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=vibespace",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Deployments: %w", err)
	}
	return deployments.Items, nil
}

// DeleteDeployment deletes a Deployment and its Service
func (m *DeploymentManager) DeleteDeployment(ctx context.Context, vibespaceID string) error {
	deploymentName := fmt.Sprintf("vibespace-%s", vibespaceID)

	// Delete the Deployment
	err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Deployment: %w", err)
	}

	// Delete the Service
	err = m.k8sClient.Clientset().CoreV1().Services(k8s.VibespaceNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Service: %w", err)
	}

	return nil
}

// DeleteAgentDeployment deletes an agent's Deployment and Service
func (m *DeploymentManager) DeleteAgentDeployment(ctx context.Context, vibespaceID string, claudeID string) error {
	deploymentName := fmt.Sprintf("vibespace-%s-claude-%s", vibespaceID, claudeID)

	// Delete the Deployment
	err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete agent Deployment: %w", err)
	}

	// Delete the Service
	err = m.k8sClient.Clientset().CoreV1().Services(k8s.VibespaceNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete agent Service: %w", err)
	}

	return nil
}

// ScaleAllDeploymentsForVibespace scales all deployments for a vibespace
func (m *DeploymentManager) ScaleAllDeploymentsForVibespace(ctx context.Context, vibespaceID string, replicas int32) error {
	deployments, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
	})
	if err != nil {
		return fmt.Errorf("failed to list Deployments: %w", err)
	}

	for _, deployment := range deployments.Items {
		deployment.Spec.Replicas = &replicas
		_, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Update(ctx, &deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to scale Deployment %s: %w", deployment.Name, err)
		}
	}

	return nil
}

// ListAgentsForVibespace lists all agents (Deployments) for a vibespace
func (m *DeploymentManager) ListAgentsForVibespace(ctx context.Context, vibespaceID string) ([]AgentInfo, error) {
	deployments, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Deployments: %w", err)
	}

	agents := make([]AgentInfo, 0, len(deployments.Items))
	for _, deploy := range deployments.Items {
		claudeID := "1" // Default for main deployment
		if cid, ok := deploy.Labels["vibespace.dev/claude-id"]; ok {
			claudeID = cid
		}

		// Get agent name from label, fallback to claude-{id} for backward compatibility
		agentName := fmt.Sprintf("claude-%s", claudeID)
		if name, ok := deploy.Labels["vibespace.dev/agent-name"]; ok && name != "" {
			agentName = name
		}

		status := deploymentStatusToString(&deploy)

		agents = append(agents, AgentInfo{
			ClaudeID:       claudeID,
			AgentName:      agentName,
			DeploymentName: deploy.Name,
			Status:         status,
		})
	}

	return agents, nil
}

// GetNextAgentID returns the next available claude ID for a vibespace
func (m *DeploymentManager) GetNextAgentID(ctx context.Context, vibespaceID string) (string, error) {
	agents, err := m.ListAgentsForVibespace(ctx, vibespaceID)
	if err != nil {
		return "", err
	}

	// Find the highest existing ID
	maxID := 0
	for _, agent := range agents {
		var id int
		if _, err := fmt.Sscanf(agent.ClaudeID, "%d", &id); err == nil {
			if id > maxID {
				maxID = id
			}
		}
	}

	return fmt.Sprintf("%d", maxID+1), nil
}

// Helper functions

// buildEnvironment creates environment variables for the vibespace
func (m *DeploymentManager) buildEnvironment(vibespaceID, vibspaceName, agentName, claudeID string, userEnv map[string]string, shareCredentials bool) []corev1.EnvVar {
	env := []corev1.EnvVar{}

	// Add user-provided environment variables
	for key, value := range userEnv {
		env = append(env, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	// Vibespace metadata
	env = append(env, corev1.EnvVar{
		Name:  "VIBESPACE_ID",
		Value: vibespaceID,
	})
	env = append(env, corev1.EnvVar{
		Name:  "VIBESPACE_NAME",
		Value: vibspaceName,
	})
	env = append(env, corev1.EnvVar{
		Name:  "VIBESPACE_AGENT",
		Value: agentName,
	})
	env = append(env, corev1.EnvVar{
		Name:  "VIBESPACE_CLAUDE_ID",
		Value: claudeID,
	})

	// SSH authorized keys from secret (optional - allows SSH access)
	env = append(env, corev1.EnvVar{
		Name: "AUTHORIZED_KEYS",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("vibespace-%s-ssh-keys", vibespaceID),
				},
				Key:      "authorized_keys",
				Optional: boolPtr(true),
			},
		},
	})

	// Credential sharing
	if shareCredentials {
		env = append(env, corev1.EnvVar{
			Name:  "VIBESPACE_SHARE_CREDENTIALS",
			Value: "true",
		})
	}

	return env
}

// buildVolumesAndMounts creates volumes and volume mounts for the vibespace
func (m *DeploymentManager) buildVolumesAndMounts(persistent bool, pvcName string) ([]corev1.Volume, []corev1.VolumeMount) {
	if !persistent {
		return nil, nil
	}

	volumes := []corev1.Volume{
		{
			Name: "vibespace-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "vibespace-data",
			MountPath: "/vibespace",
		},
	}

	return volumes, volumeMounts
}

// buildInitContainers creates init containers for the vibespace
func (m *DeploymentManager) buildInitContainers(persistent bool) []corev1.Container {
	if !persistent {
		return nil
	}

	return []corev1.Container{
		{
			Name:    "fix-permissions",
			Image:   "busybox:latest",
			Command: []string{"sh", "-c"},
			Args:    []string{"chown -R 1000:1000 /vibespace && chmod -R 755 /vibespace"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vibespace-data",
					MountPath: "/vibespace",
				},
			},
		},
	}
}

// deploymentStatusToString converts Deployment status to a string
func deploymentStatusToString(deploy *appsv1.Deployment) string {
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0 {
		return "stopped"
	}

	if deploy.Status.ReadyReplicas > 0 {
		return "running"
	}

	if deploy.Status.Replicas > 0 {
		return "creating"
	}

	return "stopped"
}

func boolPtr(b bool) *bool {
	return &b
}

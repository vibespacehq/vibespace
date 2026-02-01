package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/agent"
	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
	"github.com/yagizdagabak/vibespace/pkg/k8s"
	"github.com/yagizdagabak/vibespace/pkg/model"

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

	// Determine agent type and name
	agentType := req.AgentType
	if agentType == "" {
		agentType = agent.TypeClaudeCode // Default
	}

	agentNum := req.AgentNum
	if agentNum == 0 {
		agentNum = 1 // First agent
	}

	// Get agent implementation for naming
	agentImpl, err := agent.Get(agentType)
	if err != nil {
		return fmt.Errorf("unknown agent type %s: %w", agentType, err)
	}

	agentName := req.AgentName
	if agentName == "" {
		agentName = fmt.Sprintf("%s-%d", agentImpl.DefaultAgentPrefix(), agentNum)
	}

	// Build labels
	labels := map[string]string{
		"app.kubernetes.io/name":       req.Name,
		"app.kubernetes.io/managed-by": "vibespace",
		"vibespace.dev/id":             req.VibespaceID,
		"vibespace.dev/agent-id":       req.AgentID,
		"vibespace.dev/agent-type":     string(agentType),
		"vibespace.dev/agent-num":      fmt.Sprintf("%d", agentNum),
		"vibespace.dev/agent-name":     agentName,
		"vibespace.dev/primary":        fmt.Sprintf("%t", req.Primary),
	}

	// Build environment variables
	env := m.buildEnvironment(req.VibespaceID, req.Name, agentName, agentType, req.Env, req.ShareCredentials, req.Config)

	// Build volumes and volume mounts
	volumes, volumeMounts := m.buildVolumesAndMounts(req.Persistent, req.PVCName, req.Mounts)

	// Build init containers for persistent storage
	initContainers := m.buildInitContainers(req.Persistent)

	// Build annotations
	annotations := map[string]string{
		"vibespace.dev/created-at": metav1.Now().Format("2006-01-02T15:04:05Z"),
		"vibespace.dev/storage":    req.Resources.Storage,
	}

	// Store mounts as JSON annotation for retrieval
	if len(req.Mounts) > 0 {
		mountsJSON, err := json.Marshal(req.Mounts)
		if err == nil {
			annotations["vibespace.dev/mounts"] = string(mountsJSON)
		}
	}

	// Create the Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        deploymentName,
			Namespace:   k8s.VibespaceNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"vibespace.dev/id":         req.VibespaceID,
					"vibespace.dev/agent-name": agentName,
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

	_, err = m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}

	// Create the Service
	if err := m.createService(ctx, deploymentName, req.VibespaceID, agentName, labels); err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}

	return nil
}

// createService creates a ClusterIP Service for a deployment
func (m *DeploymentManager) createService(ctx context.Context, name, vibespaceID, agentName string, labels map[string]string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k8s.VibespaceNamespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"vibespace.dev/id":         vibespaceID,
				"vibespace.dev/agent-name": agentName,
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
	// Determine agent type
	agentType := req.AgentType
	if agentType == "" {
		agentType = agent.TypeClaudeCode // Default
	}

	agentNum := req.AgentNum
	if agentNum == 0 {
		agentNum = 1
	}

	// Get agent implementation for naming
	agentImpl, err := agent.Get(agentType)
	if err != nil {
		return fmt.Errorf("unknown agent type %s: %w", agentType, err)
	}

	agentName := req.AgentName
	if agentName == "" {
		agentName = fmt.Sprintf("%s-%d", agentImpl.DefaultAgentPrefix(), agentNum)
	}

	// Deployment naming: vibespace-{id}-{agent-name}
	deploymentName := fmt.Sprintf("vibespace-%s-%s", req.VibespaceID, agentName)

	// Build labels
	labels := map[string]string{
		"app.kubernetes.io/name":       req.Name,
		"app.kubernetes.io/managed-by": "vibespace",
		"vibespace.dev/id":             req.VibespaceID,
		"vibespace.dev/agent-id":       req.AgentID,
		"vibespace.dev/agent-type":     string(agentType),
		"vibespace.dev/agent-num":      fmt.Sprintf("%d", agentNum),
		"vibespace.dev/agent-name":     agentName,
		"vibespace.dev/primary":        fmt.Sprintf("%t", req.Primary),
		"vibespace.dev/is-agent":       "true",
	}

	// Build environment variables
	env := m.buildEnvironment(req.VibespaceID, req.Name, agentName, agentType, req.Env, req.ShareCredentials, req.Config)

	// Build volumes and volume mounts (share the same PVC and mounts)
	persistent := req.PVCName != ""
	volumes, volumeMounts := m.buildVolumesAndMounts(persistent, req.PVCName, req.Mounts)

	// Build annotations
	agentAnnotations := map[string]string{
		"vibespace.dev/created-at": metav1.Now().Format("2006-01-02T15:04:05Z"),
		"vibespace.dev/storage":    req.Resources.Storage,
	}

	// Store mounts as JSON annotation for retrieval
	if len(req.Mounts) > 0 {
		mountsJSON, err := json.Marshal(req.Mounts)
		if err == nil {
			agentAnnotations["vibespace.dev/mounts"] = string(mountsJSON)
		}
	}

	// Create the Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        deploymentName,
			Namespace:   k8s.VibespaceNamespace,
			Labels:      labels,
			Annotations: agentAnnotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"vibespace.dev/id":         req.VibespaceID,
					"vibespace.dev/agent-name": agentName,
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

	_, err = m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create agent Deployment: %w", err)
	}

	// Create the Service
	if err := m.createService(ctx, deploymentName, req.VibespaceID, agentName, labels); err != nil {
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
		return nil, fmt.Errorf("vibespace '%s' not found: %w", name, vserrors.ErrVibespaceNotFound)
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
func (m *DeploymentManager) DeleteAgentDeployment(ctx context.Context, vibespaceID string, agentName string) error {
	deploymentName := fmt.Sprintf("vibespace-%s-%s", vibespaceID, agentName)

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

// ScaleAgentDeployment scales a specific agent's deployment
func (m *DeploymentManager) ScaleAgentDeployment(ctx context.Context, vibespaceID, agentName string, replicas int32) error {
	labelSelector := fmt.Sprintf("vibespace.dev/id=%s,vibespace.dev/agent-name=%s", vibespaceID, agentName)
	deployments, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list Deployments: %w", err)
	}

	if len(deployments.Items) == 0 {
		return fmt.Errorf("agent '%s' not found in vibespace: %w", agentName, vserrors.ErrAgentNotFound)
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
		// Get agent type from labels (default: claude-code for backward compat)
		agentType := agent.TypeClaudeCode
		if t, ok := deploy.Labels["vibespace.dev/agent-type"]; ok && t != "" {
			agentType = agent.Type(t)
		}

		// Get agent number from labels
		agentNum := 1
		if n, ok := deploy.Labels["vibespace.dev/agent-num"]; ok {
			fmt.Sscanf(n, "%d", &agentNum)
		}

		// Get agent name from label
		agentName := deploy.Labels["vibespace.dev/agent-name"]
		if agentName == "" {
			// Fallback: derive from agent type and number
			agentImpl, _ := agent.Get(agentType)
			if agentImpl != nil {
				agentName = fmt.Sprintf("%s-%d", agentImpl.DefaultAgentPrefix(), agentNum)
			} else {
				agentName = fmt.Sprintf("agent-%d", agentNum)
			}
		}

		// Get agent ID (UUID)
		agentID := deploy.Labels["vibespace.dev/agent-id"]

		// Get primary flag from labels
		isPrimary := deploy.Labels["vibespace.dev/primary"] == "true"

		status := deploymentStatusToString(&deploy)

		agents = append(agents, AgentInfo{
			ID:             agentID,
			AgentType:      agentType,
			AgentNum:       agentNum,
			AgentName:      agentName,
			IsPrimary:      isPrimary,
			DeploymentName: deploy.Name,
			Status:         status,
		})
	}

	return agents, nil
}

// GetAgentConfig extracts agent config from deployment env vars
func (m *DeploymentManager) GetAgentConfig(ctx context.Context, vibespaceID, agentName string) (*agent.Config, error) {
	// Find the deployment for this agent
	deployments, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	var deploy *appsv1.Deployment
	for i := range deployments.Items {
		d := &deployments.Items[i]
		if name, ok := d.Labels["vibespace.dev/agent-name"]; ok && name == agentName {
			deploy = d
			break
		}
	}

	if deploy == nil {
		return nil, fmt.Errorf("agent '%s' not found: %w", agentName, vserrors.ErrAgentNotFound)
	}

	return m.extractConfigFromDeployment(deploy), nil
}

// extractConfigFromDeployment extracts agent config from deployment env vars
func (m *DeploymentManager) extractConfigFromDeployment(deploy *appsv1.Deployment) *agent.Config {
	config := &agent.Config{}

	if len(deploy.Spec.Template.Spec.Containers) == 0 {
		return config
	}

	for _, env := range deploy.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "VIBESPACE_SKIP_PERMISSIONS":
			config.SkipPermissions = env.Value == "true"
		case "VIBESPACE_ALLOWED_TOOLS":
			if env.Value != "" {
				config.AllowedTools = strings.Split(env.Value, ",")
			}
		case "VIBESPACE_DISALLOWED_TOOLS":
			if env.Value != "" {
				config.DisallowedTools = strings.Split(env.Value, ",")
			}
		case "VIBESPACE_MODEL":
			config.Model = env.Value
		case "VIBESPACE_MAX_TURNS":
			if v, err := strconv.Atoi(env.Value); err == nil {
				config.MaxTurns = v
			}
		case "VIBESPACE_SYSTEM_PROMPT":
			config.SystemPrompt = env.Value
		case "VIBESPACE_SHARE_CREDENTIALS":
			config.ShareCredentials = env.Value == "true"
		case "VIBESPACE_REASONING_EFFORT":
			config.ReasoningEffort = env.Value
		}
	}

	return config
}

// UpdateAgentConfig updates the agent config (triggers pod restart)
func (m *DeploymentManager) UpdateAgentConfig(ctx context.Context, vibespaceID, agentName string, config *agent.Config) error {
	// Find the deployment for this agent
	deployments, err := m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
	})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	var deploy *appsv1.Deployment
	for i := range deployments.Items {
		d := &deployments.Items[i]
		if name, ok := d.Labels["vibespace.dev/agent-name"]; ok && name == agentName {
			deploy = d
			break
		}
	}

	if deploy == nil {
		return fmt.Errorf("agent '%s' not found: %w", agentName, vserrors.ErrAgentNotFound)
	}

	// Update environment variables
	if len(deploy.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("deployment has no containers")
	}

	container := &deploy.Spec.Template.Spec.Containers[0]
	newEnv := m.updateEnvVars(container.Env, config)
	container.Env = newEnv

	// Update the deployment (triggers rolling update)
	_, err = m.k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return nil
}

// updateEnvVars updates or adds agent config env vars
func (m *DeploymentManager) updateEnvVars(env []corev1.EnvVar, config *agent.Config) []corev1.EnvVar {
	// Map of env vars to update
	configVars := map[string]string{
		"VIBESPACE_SKIP_PERMISSIONS": "",
		"VIBESPACE_ALLOWED_TOOLS":    "",
		"VIBESPACE_DISALLOWED_TOOLS": "",
		"VIBESPACE_MODEL":            "",
		"VIBESPACE_MAX_TURNS":        "",
		"VIBESPACE_SYSTEM_PROMPT":    "",
		"VIBESPACE_REASONING_EFFORT": "",
	}

	if config != nil {
		if config.SkipPermissions {
			configVars["VIBESPACE_SKIP_PERMISSIONS"] = "true"
		}
		if len(config.AllowedTools) > 0 {
			configVars["VIBESPACE_ALLOWED_TOOLS"] = config.AllowedToolsString()
		}
		if len(config.DisallowedTools) > 0 {
			configVars["VIBESPACE_DISALLOWED_TOOLS"] = config.DisallowedToolsString()
		}
		if config.Model != "" {
			configVars["VIBESPACE_MODEL"] = config.Model
		}
		if config.MaxTurns > 0 {
			configVars["VIBESPACE_MAX_TURNS"] = strconv.Itoa(config.MaxTurns)
		}
		if config.SystemPrompt != "" {
			configVars["VIBESPACE_SYSTEM_PROMPT"] = config.SystemPrompt
		}
		if config.ReasoningEffort != "" {
			configVars["VIBESPACE_REASONING_EFFORT"] = config.ReasoningEffort
		}
	}

	// Remove existing config vars and non-config vars to result
	result := make([]corev1.EnvVar, 0, len(env))
	for _, e := range env {
		if _, isConfigVar := configVars[e.Name]; !isConfigVar {
			result = append(result, e)
		}
	}

	// Add config vars with non-empty values
	for name, value := range configVars {
		if value != "" {
			result = append(result, corev1.EnvVar{Name: name, Value: value})
		}
	}

	return result
}

// Helper functions

// buildEnvironment creates environment variables for the vibespace
func (m *DeploymentManager) buildEnvironment(vibespaceID, vibspaceName, agentName string, agentType agent.Type, userEnv map[string]string, shareCredentials bool, config *agent.Config) []corev1.EnvVar {
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
		Name:  "VIBESPACE_AGENT_TYPE",
		Value: string(agentType),
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

	// Agent configuration environment variables
	if config != nil {
		if config.SkipPermissions {
			env = append(env, corev1.EnvVar{
				Name:  "VIBESPACE_SKIP_PERMISSIONS",
				Value: "true",
			})
		}
		if len(config.AllowedTools) > 0 {
			env = append(env, corev1.EnvVar{
				Name:  "VIBESPACE_ALLOWED_TOOLS",
				Value: config.AllowedToolsString(),
			})
		}
		if len(config.DisallowedTools) > 0 {
			env = append(env, corev1.EnvVar{
				Name:  "VIBESPACE_DISALLOWED_TOOLS",
				Value: config.DisallowedToolsString(),
			})
		}
		if config.Model != "" {
			env = append(env, corev1.EnvVar{
				Name:  "VIBESPACE_MODEL",
				Value: config.Model,
			})
		}
		if config.MaxTurns > 0 {
			env = append(env, corev1.EnvVar{
				Name:  "VIBESPACE_MAX_TURNS",
				Value: strconv.Itoa(config.MaxTurns),
			})
		}
		if config.SystemPrompt != "" {
			env = append(env, corev1.EnvVar{
				Name:  "VIBESPACE_SYSTEM_PROMPT",
				Value: config.SystemPrompt,
			})
		}
		if config.ReasoningEffort != "" {
			env = append(env, corev1.EnvVar{
				Name:  "VIBESPACE_REASONING_EFFORT",
				Value: config.ReasoningEffort,
			})
		}
	}

	return env
}

// buildVolumesAndMounts creates volumes and volume mounts for the vibespace
func (m *DeploymentManager) buildVolumesAndMounts(persistent bool, pvcName string, mounts []model.Mount) ([]corev1.Volume, []corev1.VolumeMount) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// Add PVC volume for persistent storage
	if persistent && pvcName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "vibespace-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "vibespace-data",
			MountPath: "/vibespace",
		})
	}

	// Add host path volumes for mounts
	for i, mount := range mounts {
		volumeName := fmt.Sprintf("host-mount-%d", i)
		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: mount.HostPath,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mount.ContainerPath,
			ReadOnly:  mount.ReadOnly,
		})
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

package knative

import (
	"context"
	"fmt"

	"vibespace/pkg/k8s"
	"vibespace/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// KnativeGVR is the GroupVersionResource for Knative Services
var KnativeGVR = schema.GroupVersionResource{
	Group:    "serving.knative.dev",
	Version:  "v1",
	Resource: "services",
}

// ServiceManager manages Knative Services for vibespaces
type ServiceManager struct {
	k8sClient     *k8s.Client
	dynamicClient dynamic.Interface
}

// NewServiceManager creates a new Knative Service manager
func NewServiceManager(k8sClient *k8s.Client) (*ServiceManager, error) {
	dynamicClient, err := dynamic.NewForConfig(k8sClient.Config())
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &ServiceManager{
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
	}, nil
}

// CreateServiceRequest contains parameters for creating a Knative Service
type CreateServiceRequest struct {
	VibespaceID string
	Name        string
	ProjectName string
	ClaudeID    string // Claude instance ID (1, 2, 3, etc.)
	Image       string
	Resources   model.Resources
	Env         map[string]string
	Persistent  bool
	PVCName     string
}

// CreateService creates a Knative Service for a vibespace
func (m *ServiceManager) CreateService(ctx context.Context, req *CreateServiceRequest) error {
	// Build init containers
	initContainers := m.buildInitContainers(req)

	// Build volumes and volume mounts
	volumes, volumeMounts := m.buildVolumesAndMounts(req)

	// Build environment variables
	env := m.buildEnvironment(req)

	// Single port for ttyd web terminal
	ports := []map[string]interface{}{
		{
			"containerPort": 7681,
			"name":          "http1", // Knative requires 'http1' or 'h2c'
			"protocol":      "TCP",
		},
	}

	// Build Knative Service manifest
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.knative.dev/v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("vibespace-%s", req.VibespaceID),
				"namespace": k8s.VibespaceNamespace,
				"labels": map[string]string{
					"app.kubernetes.io/name":       req.Name,
					"app.kubernetes.io/managed-by": "vibespace",
					"vibespace.dev/id":             req.VibespaceID,
					"vibespace.dev/project-name":   req.ProjectName,
				},
				"annotations": map[string]string{
					"vibespace.dev/created-at": metav1.Now().Format("2006-01-02T15:04:05Z"),
				},
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"app.kubernetes.io/name":     req.Name,
							"vibespace.dev/id":           req.VibespaceID,
							"vibespace.dev/project-name": req.ProjectName,
						},
						"annotations": map[string]string{
							// Scale-to-zero configuration
							"autoscaling.knative.dev/minScale": "0",
							"autoscaling.knative.dev/maxScale": "1",
							// Scale down after 10 minutes of inactivity
							"autoscaling.knative.dev/scaleDownDelay": "10m",
							// Target concurrency (1 = single user)
							"autoscaling.knative.dev/target": "1",
						},
					},
					"spec": map[string]interface{}{
						"containerConcurrency": 1,   // Single user per pod
						"timeoutSeconds":       600, // 10 minutes (Knative maximum)
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "vibespace",
								"image": req.Image,
								"ports": ports,
								"env":   env,
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"cpu":    req.Resources.CPU,
										"memory": req.Resources.Memory,
									},
									"limits": map[string]interface{}{
										"cpu":    req.Resources.CPU,
										"memory": req.Resources.Memory,
									},
								},
								"volumeMounts": volumeMounts,
							},
						},
						"initContainers": initContainers,
						"volumes":        volumes,
					},
				},
			},
		},
	}

	// Create the Knative Service
	_, err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).Create(
		ctx,
		service,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create Knative Service: %w", err)
	}

	return nil
}

// buildInitContainers creates init containers for the vibespace
func (m *ServiceManager) buildInitContainers(req *CreateServiceRequest) []interface{} {
	initContainers := []interface{}{}

	// Fix permissions for persistent storage
	if req.Persistent {
		initContainers = append(initContainers, map[string]interface{}{
			"name":    "fix-permissions",
			"image":   "busybox:latest",
			"command": []string{"sh", "-c"},
			"args": []string{
				"chown -R 1000:1000 /vibespace && chmod -R 755 /vibespace",
			},
			"volumeMounts": []interface{}{
				map[string]interface{}{
					"name":      "vibespace-data",
					"mountPath": "/vibespace",
				},
			},
		})
	}

	return initContainers
}

// buildVolumesAndMounts creates volumes and volume mounts for the vibespace
func (m *ServiceManager) buildVolumesAndMounts(req *CreateServiceRequest) ([]interface{}, []interface{}) {
	volumes := []interface{}{}
	volumeMounts := []interface{}{}

	if req.Persistent {
		volumes = append(volumes, map[string]interface{}{
			"name": "vibespace-data",
			"persistentVolumeClaim": map[string]interface{}{
				"claimName": req.PVCName,
			},
		})
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      "vibespace-data",
			"mountPath": "/vibespace",
		})
	}

	return volumes, volumeMounts
}

// buildEnvironment creates environment variables for the vibespace
func (m *ServiceManager) buildEnvironment(req *CreateServiceRequest) []interface{} {
	env := []interface{}{}

	// Add user-provided environment variables
	for key, value := range req.Env {
		env = append(env, map[string]interface{}{
			"name":  key,
			"value": value,
		})
	}

	// Vibespace metadata
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_ID",
		"value": req.VibespaceID,
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_PROJECT",
		"value": req.ProjectName,
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_CLAUDE_ID",
		"value": req.ClaudeID,
	})

	// SSH authorized keys from secret (optional - allows SSH access)
	env = append(env, map[string]interface{}{
		"name": "AUTHORIZED_KEYS",
		"valueFrom": map[string]interface{}{
			"secretKeyRef": map[string]interface{}{
				"name":     fmt.Sprintf("vibespace-%s-ssh-keys", req.VibespaceID),
				"key":      "authorized_keys",
				"optional": true,
			},
		},
	})

	return env
}

// GetService retrieves a Knative Service by ID
func (m *ServiceManager) GetService(ctx context.Context, vibespaceID string) (*unstructured.Unstructured, error) {
	serviceName := fmt.Sprintf("vibespace-%s", vibespaceID)
	service, err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).Get(
		ctx,
		serviceName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Knative Service: %w", err)
	}

	return service, nil
}

// GetServiceByName retrieves a Knative Service by user-friendly name (using label selector)
func (m *ServiceManager) GetServiceByName(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	services, err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list Knative Services: %w", err)
	}

	if len(services.Items) == 0 {
		return nil, fmt.Errorf("vibespace '%s' not found", name)
	}

	return &services.Items[0], nil
}

// ListServices lists all Knative Services for vibespaces
func (m *ServiceManager) ListServices(ctx context.Context) (*unstructured.UnstructuredList, error) {
	services, err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=vibespace",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list Knative Services: %w", err)
	}

	return services, nil
}

// DeleteService deletes a Knative Service
func (m *ServiceManager) DeleteService(ctx context.Context, vibespaceID string) error {
	serviceName := fmt.Sprintf("vibespace-%s", vibespaceID)
	err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).Delete(
		ctx,
		serviceName,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete Knative Service: %w", err)
	}

	return nil
}

// GetServiceStatus retrieves the status of a Knative Service
// Returns: ready (bool), reason (string), url (string)
func (m *ServiceManager) GetServiceStatus(ctx context.Context, vibespaceID string) (bool, string, string, error) {
	service, err := m.GetService(ctx, vibespaceID)
	if err != nil {
		return false, "", "", err
	}

	// Extract status from the unstructured object
	status, found, err := unstructured.NestedMap(service.Object, "status")
	if err != nil || !found {
		return false, "Unknown", "", nil
	}

	// Check conditions
	conditions, found, err := unstructured.NestedSlice(status, "conditions")
	if err != nil || !found {
		return false, "Unknown", "", nil
	}

	// Find Ready condition
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condition, "type")
		if condType == "Ready" {
			condStatus, _, _ := unstructured.NestedString(condition, "status")
			condReason, _, _ := unstructured.NestedString(condition, "reason")

			ready := condStatus == "True"
			return ready, condReason, "", nil
		}
	}

	return false, "Unknown", "", nil
}

// PatchService patches a Knative Service with new annotations
// Used primarily for scaling operations (minScale/maxScale)
func (m *ServiceManager) PatchService(ctx context.Context, vibespaceID string, annotations map[string]string) error {
	serviceName := fmt.Sprintf("vibespace-%s", vibespaceID)

	// Get current service
	service, err := m.GetService(ctx, vibespaceID)
	if err != nil {
		return fmt.Errorf("failed to get service for patching: %w", err)
	}

	// Get current annotations
	currentAnnotations, _, _ := unstructured.NestedStringMap(service.Object, "spec", "template", "metadata", "annotations")
	if currentAnnotations == nil {
		currentAnnotations = make(map[string]string)
	}

	// Merge new annotations
	for key, value := range annotations {
		currentAnnotations[key] = value
	}

	// Update annotations in the service
	if err := unstructured.SetNestedStringMap(service.Object, currentAnnotations, "spec", "template", "metadata", "annotations"); err != nil {
		return fmt.Errorf("failed to set annotations: %w", err)
	}

	// Update the service
	_, err = m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).Update(
		ctx,
		service,
		metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch Knative Service %s: %w", serviceName, err)
	}

	return nil
}

// CreateAgentServiceRequest contains parameters for creating an agent service
type CreateAgentServiceRequest struct {
	VibespaceID string
	Name        string
	ProjectName string
	ClaudeID    string // Claude instance ID (2, 3, 4, etc.)
	Image       string
	Resources   model.Resources
	Env         map[string]string
	PVCName     string // Shared PVC name for all agents
}

// CreateAgentService creates a Knative Service for an additional agent in a vibespace
// Service naming: vibespace-<id>-claude-<N>
func (m *ServiceManager) CreateAgentService(ctx context.Context, req *CreateAgentServiceRequest) error {
	serviceName := fmt.Sprintf("vibespace-%s-claude-%s", req.VibespaceID, req.ClaudeID)

	// Build volumes and volume mounts (share the same PVC)
	var volumes, volumeMounts []interface{}
	if req.PVCName != "" {
		volumes = []interface{}{
			map[string]interface{}{
				"name": "vibespace-data",
				"persistentVolumeClaim": map[string]interface{}{
					"claimName": req.PVCName,
				},
			},
		}
		volumeMounts = []interface{}{
			map[string]interface{}{
				"name":      "vibespace-data",
				"mountPath": "/vibespace",
			},
		}
	}

	// Build environment variables
	env := []interface{}{}
	for key, value := range req.Env {
		env = append(env, map[string]interface{}{
			"name":  key,
			"value": value,
		})
	}
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_ID",
		"value": req.VibespaceID,
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_PROJECT",
		"value": req.ProjectName,
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_CLAUDE_ID",
		"value": req.ClaudeID,
	})

	// SSH authorized keys from secret (optional - allows SSH access)
	env = append(env, map[string]interface{}{
		"name": "AUTHORIZED_KEYS",
		"valueFrom": map[string]interface{}{
			"secretKeyRef": map[string]interface{}{
				"name":     fmt.Sprintf("vibespace-%s-ssh-keys", req.VibespaceID),
				"key":      "authorized_keys",
				"optional": true,
			},
		},
	})

	// Single port for ttyd web terminal
	ports := []map[string]interface{}{
		{
			"containerPort": 7681,
			"name":          "http1",
			"protocol":      "TCP",
		},
	}

	// Build Knative Service manifest
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.knative.dev/v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      serviceName,
				"namespace": k8s.VibespaceNamespace,
				"labels": map[string]string{
					"app.kubernetes.io/name":       req.Name,
					"app.kubernetes.io/managed-by": "vibespace",
					"vibespace.dev/id":             req.VibespaceID,
					"vibespace.dev/project-name":   req.ProjectName,
					"vibespace.dev/claude-id":      req.ClaudeID,
					"vibespace.dev/is-agent":       "true",
				},
				"annotations": map[string]string{
					"vibespace.dev/created-at": metav1.Now().Format("2006-01-02T15:04:05Z"),
				},
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"app.kubernetes.io/name":     req.Name,
							"vibespace.dev/id":           req.VibespaceID,
							"vibespace.dev/project-name": req.ProjectName,
							"vibespace.dev/claude-id":    req.ClaudeID,
						},
						"annotations": map[string]string{
							"autoscaling.knative.dev/minScale":       "1", // Agents start immediately
							"autoscaling.knative.dev/maxScale":       "1",
							"autoscaling.knative.dev/scaleDownDelay": "10m",
							"autoscaling.knative.dev/target":         "1",
						},
					},
					"spec": map[string]interface{}{
						"containerConcurrency": 1,
						"timeoutSeconds":       600,
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "vibespace",
								"image": req.Image,
								"ports": ports,
								"env":   env,
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"cpu":    req.Resources.CPU,
										"memory": req.Resources.Memory,
									},
									"limits": map[string]interface{}{
										"cpu":    req.Resources.CPU,
										"memory": req.Resources.Memory,
									},
								},
								"volumeMounts": volumeMounts,
							},
						},
						"volumes": volumes,
					},
				},
			},
		},
	}

	// Create the Knative Service
	_, err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).Create(
		ctx,
		service,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create agent service: %w", err)
	}

	return nil
}

// AgentInfo contains information about an agent
type AgentInfo struct {
	ClaudeID    string // "1", "2", etc.
	AgentName   string // "claude-1", "claude-2", etc.
	ServiceName string // Knative service name
	Status      string // running, stopped, creating, etc.
}

// ListAgentsForVibespace lists all agents (Knative services) for a vibespace
func (m *ServiceManager) ListAgentsForVibespace(ctx context.Context, vibespaceID string) ([]AgentInfo, error) {
	services, err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	agents := make([]AgentInfo, 0, len(services.Items))
	for _, svc := range services.Items {
		metadata, _ := svc.Object["metadata"].(map[string]interface{})
		labels, _ := metadata["labels"].(map[string]interface{})
		name, _ := metadata["name"].(string)

		claudeID := "1" // Default for main service
		if cid, ok := labels["vibespace.dev/claude-id"].(string); ok {
			claudeID = cid
		}

		status := knativeStatusToString(&svc)

		agents = append(agents, AgentInfo{
			ClaudeID:    claudeID,
			AgentName:   fmt.Sprintf("claude-%s", claudeID),
			ServiceName: name,
			Status:      status,
		})
	}

	return agents, nil
}

// knativeStatusToString converts Knative status to a string
func knativeStatusToString(svc *unstructured.Unstructured) string {
	status, found, _ := unstructured.NestedMap(svc.Object, "status")
	if !found {
		return "creating"
	}

	conditions, found, _ := unstructured.NestedSlice(status, "conditions")
	if !found {
		return "creating"
	}

	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _, _ := unstructured.NestedString(condition, "type")
		if condType == "Ready" {
			condStatus, _, _ := unstructured.NestedString(condition, "status")
			if condStatus == "True" {
				return "running"
			}
			reason, _, _ := unstructured.NestedString(condition, "reason")
			if reason == "NoTraffic" {
				return "stopped"
			}
			if reason == "IngressNotConfigured" {
				// Check ConfigurationsReady
				for _, c2 := range conditions {
					cond2, ok := c2.(map[string]interface{})
					if !ok {
						continue
					}
					if t, _, _ := unstructured.NestedString(cond2, "type"); t == "ConfigurationsReady" {
						if s, _, _ := unstructured.NestedString(cond2, "status"); s == "True" {
							return "running"
						}
					}
				}
			}
			return "creating"
		}
	}

	return "creating"
}

// DeleteAgentService deletes an agent's Knative Service
func (m *ServiceManager) DeleteAgentService(ctx context.Context, vibespaceID string, claudeID string) error {
	serviceName := fmt.Sprintf("vibespace-%s-claude-%s", vibespaceID, claudeID)
	err := m.dynamicClient.Resource(KnativeGVR).Namespace(k8s.VibespaceNamespace).Delete(
		ctx,
		serviceName,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete agent service: %w", err)
	}

	return nil
}

// GetNextAgentID returns the next available claude ID for a vibespace
func (m *ServiceManager) GetNextAgentID(ctx context.Context, vibespaceID string) (string, error) {
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

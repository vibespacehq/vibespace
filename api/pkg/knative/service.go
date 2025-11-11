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
	Template    string
	Agent       string
	Image       string
	Ports       model.Ports
	Resources   model.Resources
	Env         map[string]string
	Persistent  bool
	PVCName     string
	GithubRepo  string
}

// CreateService creates a Knative Service for a vibespace
// The service runs a single pod with multiple ports:
// - Port 8080: code-server (VS Code in browser)
// - Port 3000: preview server (npm run dev, etc.)
// - Port 3001: production server (next start, static files, etc.)
func (m *ServiceManager) CreateService(ctx context.Context, req *CreateServiceRequest) error {
	// Build init containers
	initContainers := m.buildInitContainers(req)

	// Build volumes and volume mounts
	volumes, volumeMounts := m.buildVolumesAndMounts(req)

	// Build environment variables
	env := m.buildEnvironment(req)

	// Build container ports
	// We expose all 3 ports from the container, Traefik will route to them
	ports := []map[string]interface{}{
		{
			"containerPort": req.Ports.Code,
			"name":          "code",
			"protocol":      "TCP",
		},
		{
			"containerPort": req.Ports.Preview,
			"name":          "preview",
			"protocol":      "TCP",
		},
		{
			"containerPort": req.Ports.Prod,
			"name":          "prod",
			"protocol":      "TCP",
		},
	}

	// Build Knative Service manifest
	// Knative uses a special structure with spec.template.spec (pod template)
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
					"vibespace.dev/template":       req.Template,
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
							"app.kubernetes.io/name":       req.Name,
							"vibespace.dev/id":             req.VibespaceID,
							"vibespace.dev/template":       req.Template,
							"vibespace.dev/project-name":   req.ProjectName,
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
						"containerConcurrency": 1, // Single user per pod
						"timeoutSeconds":       3600, // 1 hour timeout (code sessions can be long)
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
								"securityContext": map[string]interface{}{
									"runAsUser":  1001,
									"runAsGroup": 1001,
									"fsGroup":    1001,
								},
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
				"chown -R 1001:1001 /vibespace && chmod -R 755 /vibespace",
			},
			"volumeMounts": []interface{}{
				map[string]interface{}{
					"name":      "vibespace-data",
					"mountPath": "/vibespace",
				},
			},
		})
	}

	// Git clone init container
	if req.GithubRepo != "" {
		initContainers = append(initContainers, map[string]interface{}{
			"name":    "git-clone",
			"image":   "alpine/git:latest",
			"command": []string{"sh", "-c"},
			"args": []string{
				fmt.Sprintf("git clone %s /vibespace/repo || echo 'Failed to clone repository'", req.GithubRepo),
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

	// Add vibespace metadata
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_ID",
		"value": req.VibespaceID,
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_NAME",
		"value": req.Name,
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_PROJECT_NAME",
		"value": req.ProjectName,
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_TEMPLATE",
		"value": req.Template,
	})

	// Add agent configuration
	if req.Agent != "" {
		env = append(env, map[string]interface{}{
			"name":  "VIBESPACE_AGENT",
			"value": req.Agent,
		})
	}

	// Add port configuration for entrypoint script
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_CODE_PORT",
		"value": fmt.Sprintf("%d", req.Ports.Code),
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_PREVIEW_PORT",
		"value": fmt.Sprintf("%d", req.Ports.Preview),
	})
	env = append(env, map[string]interface{}{
		"name":  "VIBESPACE_PROD_PORT",
		"value": fmt.Sprintf("%d", req.Ports.Prod),
	})

	return env
}

// GetService retrieves a Knative Service
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

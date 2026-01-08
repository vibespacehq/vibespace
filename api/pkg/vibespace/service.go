package vibespace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"vibespace/pkg/k8s"
	"vibespace/pkg/knative"
	"vibespace/pkg/model"
	"vibespace/pkg/network"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultImage is the single container image used for all vibespaces
// Contains a minimal Linux distro with Claude Code CLI pre-installed
const DefaultImage = "ghcr.io/yagizdagabak/vibespace/claude:latest"

// Service handles vibespace operations
type Service struct {
	k8sClient            *k8s.Client
	knativeManager       *knative.ServiceManager
	ingressRouteManager  *network.IngressRouteManager
}

// NewService creates a new vibespace service
func NewService(k8sClient *k8s.Client) *Service {
	var knativeMgr *knative.ServiceManager
	var ingressMgr *network.IngressRouteManager

	if k8sClient != nil {
		var err error
		knativeMgr, err = knative.NewServiceManager(k8sClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager",
				"error", err)
		}

		// Initialize IngressRoute manager for dynamic port exposure
		baseDomain := getBaseDomain()
		ingressMgr = network.NewIngressRouteManager(k8sClient.DynamicClient(), baseDomain)
	}

	return &Service{
		k8sClient:           k8sClient,
		knativeManager:      knativeMgr,
		ingressRouteManager: ingressMgr,
	}
}

// List returns all vibespaces
func (s *Service) List(ctx context.Context) ([]*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		slog.Warn("kubernetes client not available, returning empty list", "error", err)
		return []*model.Vibespace{}, nil
	}

	if s.knativeManager == nil {
		slog.Warn("knative manager is nil, returning empty list")
		return []*model.Vibespace{}, nil
	}

	services, err := s.knativeManager.ListServices(ctx)
	if err != nil {
		slog.Error("failed to list Knative Services", "error", err)
		return nil, fmt.Errorf("failed to list Knative Services: %w", err)
	}

	vibespaces := make([]*model.Vibespace, 0, len(services.Items))
	for i := range services.Items {
		vibespace := knativeServiceToVibespace(&services.Items[i])
		vibespaces = append(vibespaces, vibespace)
	}

	return vibespaces, nil
}

// Get returns a vibespace by ID
func (s *Service) Get(ctx context.Context, id string) (*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return nil, fmt.Errorf("knative manager is not initialized")
	}

	slog.Info("getting vibespace", "vibespace_id", id)

	service, err := s.knativeManager.GetService(ctx, id)
	if err != nil {
		slog.Warn("vibespace not found", "vibespace_id", id, "error", err)
		return nil, fmt.Errorf("vibespace not found: %w", err)
	}

	vibespace := knativeServiceToVibespace(service)
	slog.Info("got vibespace successfully", "vibespace_id", id, "status", vibespace.Status)

	return vibespace, nil
}

// Create creates a new vibespace
func (s *Service) Create(ctx context.Context, req *model.CreateVibespaceRequest) (*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return nil, fmt.Errorf("knative manager is not initialized")
	}

	slog.Info("creating vibespace",
		"name", req.Name,
		"github_repo", req.GithubRepo,
		"persistent", req.Persistent)

	// Ensure namespace exists
	if err := s.k8sClient.EnsureNamespace(ctx); err != nil {
		slog.Error("failed to ensure namespace", "error", err, "namespace", k8s.VibespaceNamespace)
		return nil, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	id := uuid.New().String()[:8]
	pvcName := fmt.Sprintf("vibespace-%s-pvc", id)

	// Generate unique project name for DNS routing
	existingVibespaces, _ := s.List(ctx)
	existingNames := make([]string, 0, len(existingVibespaces))
	for _, vs := range existingVibespaces {
		if vs.ProjectName != "" {
			existingNames = append(existingNames, vs.ProjectName)
		}
	}
	projectName := generateUniqueProjectName(existingNames)

	slog.Debug("generated vibespace id and project name",
		"vibespace_id", id,
		"project_name", projectName,
		"pvc_name", pvcName)

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
		slog.Info("creating persistent volume claim",
			"vibespace_id", id,
			"pvc_name", pvcName,
			"storage", resources.Storage)
		if err := s.createPVC(ctx, pvcName, resources.Storage); err != nil {
			slog.Error("failed to create pvc",
				"vibespace_id", id,
				"pvc_name", pvcName,
				"error", err)
			return nil, fmt.Errorf("failed to create PVC: %w", err)
		}
		slog.Info("pvc created successfully", "vibespace_id", id, "pvc_name", pvcName)
	}

	// Validate GitHub repo URL if provided
	if req.GithubRepo != "" && !isValidGitURL(req.GithubRepo) {
		return nil, fmt.Errorf("invalid GitHub repository URL: must be a valid HTTPS or SSH Git URL")
	}

	// Get image from environment or use default
	image := getVibespaceImage()

	slog.Info("creating vibespace with Knative Service",
		"vibespace_id", id,
		"project_name", projectName,
		"image", image)

	// Create Knative Service
	err := s.knativeManager.CreateService(ctx, &knative.CreateServiceRequest{
		VibespaceID: id,
		Name:        req.Name,
		ProjectName: projectName,
		ClaudeID:    "1", // First Claude instance
		Image:       image,
		Resources:   *resources,
		Env:         req.Env,
		Persistent:  req.Persistent,
		PVCName:     pvcName,
	})
	if err != nil {
		slog.Error("failed to create Knative Service", "vibespace_id", id, "error", err)
		return nil, fmt.Errorf("failed to create Knative Service: %w", err)
	}

	// Create IngressRoutes for dynamic port exposure
	// This creates both main route ({project}.vibe.space) and wildcard route (*.{project}.vibe.space)
	if s.ingressRouteManager != nil {
		serviceName := fmt.Sprintf("vibespace-%s", id)
		if err := s.ingressRouteManager.CreateVibespaceRoutes(ctx, projectName, serviceName, k8s.VibespaceNamespace); err != nil {
			slog.Warn("failed to create IngressRoutes (vibespace still functional)",
				"vibespace_id", id,
				"project_name", projectName,
				"error", err)
			// Don't fail the creation - vibespace is still usable via port-forward
		}
	}

	vibespace := &model.Vibespace{
		ID:          id,
		Name:        req.Name,
		ProjectName: projectName,
		Status:      "creating",
		Resources:   *resources,
		Persistent:  req.Persistent,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	slog.Info("vibespace created successfully",
		"vibespace_id", id,
		"project_name", projectName)

	return vibespace, nil
}

// Delete deletes a vibespace
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return fmt.Errorf("knative manager is not initialized")
	}

	slog.Info("deleting vibespace", "vibespace_id", id)

	// Get vibespace to retrieve project name for IngressRoute cleanup
	vibespace, err := s.Get(ctx, id)
	if err != nil {
		slog.Warn("could not get vibespace details for cleanup", "vibespace_id", id, "error", err)
	}

	// Delete IngressRoutes first
	if s.ingressRouteManager != nil && vibespace != nil && vibespace.ProjectName != "" {
		if err := s.ingressRouteManager.DeleteVibespaceRoutes(ctx, vibespace.ProjectName, k8s.VibespaceNamespace); err != nil {
			slog.Warn("failed to delete IngressRoutes", "vibespace_id", id, "error", err)
			// Continue with Knative Service deletion
		}
	}

	if err := s.knativeManager.DeleteService(ctx, id); err != nil {
		return fmt.Errorf("failed to delete Knative Service: %w", err)
	}

	slog.Info("vibespace deleted successfully", "vibespace_id", id)

	// Note: PVCs are left for manual cleanup to prevent accidental data loss
	return nil
}

// Start starts a stopped vibespace
func (s *Service) Start(ctx context.Context, id string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return fmt.Errorf("knative manager is not initialized")
	}

	slog.Info("starting vibespace", "vibespace_id", id)

	// Set minScale=1 to ensure at least 1 replica is running
	annotations := map[string]string{
		"autoscaling.knative.dev/minScale": "1",
	}

	err := s.knativeManager.PatchService(ctx, id, annotations)
	if err != nil {
		slog.Error("failed to start vibespace", "vibespace_id", id, "error", err)
		return fmt.Errorf("failed to start vibespace: %w", err)
	}

	slog.Info("vibespace started successfully", "vibespace_id", id, "minScale", "1")
	return nil
}

// Stop stops a running vibespace
func (s *Service) Stop(ctx context.Context, id string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return fmt.Errorf("knative manager is not initialized")
	}

	slog.Info("stopping vibespace", "vibespace_id", id)

	// Set minScale=0 to allow scaling to zero replicas
	annotations := map[string]string{
		"autoscaling.knative.dev/minScale": "0",
	}

	err := s.knativeManager.PatchService(ctx, id, annotations)
	if err != nil {
		slog.Error("failed to stop vibespace", "vibespace_id", id, "error", err)
		return fmt.Errorf("failed to stop vibespace: %w", err)
	}

	slog.Info("vibespace stopped successfully", "vibespace_id", id, "minScale", "0")
	return nil
}

// Access returns the URL where the vibespace can be accessed
func (s *Service) Access(ctx context.Context, id string) (map[string]string, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	slog.Info("getting vibespace access URL", "vibespace_id", id)

	vibespace, err := s.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get vibespace: %w", err)
	}

	baseDomain := getBaseDomain()
	urls := map[string]string{
		"main": fmt.Sprintf("https://%s.%s", vibespace.ProjectName, baseDomain),
	}

	slog.Info("vibespace access URL generated",
		"vibespace_id", id,
		"project_name", vibespace.ProjectName,
		"url", urls["main"])

	return urls, nil
}

// RegisterService registers a dynamically detected service for a vibespace.
// Called by the port detector daemon in the container when it detects a new listening port.
// Returns the URL where the service can be accessed.
func (s *Service) RegisterService(ctx context.Context, id string, service *model.ExposedService) (string, error) {
	if err := s.ensureClients(); err != nil {
		return "", fmt.Errorf("kubernetes is not available: %w", err)
	}

	slog.Info("registering service for vibespace",
		"vibespace_id", id,
		"service_name", service.Name,
		"port", service.Port)

	// Get vibespace to retrieve project name
	vibespace, err := s.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get vibespace: %w", err)
	}

	// Generate URL for the service
	baseDomain := getBaseDomain()
	url := fmt.Sprintf("https://%d.%s.%s", service.Port, vibespace.ProjectName, baseDomain)

	slog.Info("service registered successfully",
		"vibespace_id", id,
		"project_name", vibespace.ProjectName,
		"port", service.Port,
		"url", url)

	return url, nil
}

// UnregisterService unregisters a service from a vibespace.
// Called by the port detector daemon when a service stops listening.
func (s *Service) UnregisterService(ctx context.Context, id string, port int) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available: %w", err)
	}

	slog.Info("unregistering service from vibespace",
		"vibespace_id", id,
		"port", port)

	// Currently just logging - could be extended to update state or notify clients
	return nil
}

// GetServiceURL returns the URL for a specific port on a vibespace
func (s *Service) GetServiceURL(ctx context.Context, id string, port int) (string, error) {
	vibespace, err := s.Get(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get vibespace: %w", err)
	}

	baseDomain := getBaseDomain()
	return fmt.Sprintf("https://%d.%s.%s", port, vibespace.ProjectName, baseDomain), nil
}

// Helper functions

// ensureClients attempts to initialize k8s client if not already done
func (s *Service) ensureClients() error {
	if s.k8sClient == nil {
		slog.Info("k8s client is nil, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			return err
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully")

		// Also initialize Knative manager
		knativeMgr, err := knative.NewServiceManager(newClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager", "error", err)
		} else {
			s.knativeManager = knativeMgr
			slog.Info("knative manager initialized successfully")
		}

		// Initialize IngressRoute manager for dynamic port exposure
		baseDomain := getBaseDomain()
		s.ingressRouteManager = network.NewIngressRouteManager(newClient.DynamicClient(), baseDomain)
		slog.Info("ingress route manager initialized successfully")
	}
	return nil
}

// createPVC creates a PersistentVolumeClaim for vibespace storage
func (s *Service) createPVC(ctx context.Context, name string, storage string) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k8s.VibespaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "vibespace",
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

	_, err := s.k8sClient.Clientset().CoreV1().PersistentVolumeClaims(k8s.VibespaceNamespace).Create(ctx, pvc, metav1.CreateOptions{})
	return err
}

// knativeServiceToVibespace converts a Knative Service to Vibespace model
func knativeServiceToVibespace(svc *unstructured.Unstructured) *model.Vibespace {
	metadata, _ := svc.Object["metadata"].(map[string]interface{})
	labels, _ := metadata["labels"].(map[string]interface{})
	annotations, _ := metadata["annotations"].(map[string]interface{})

	id, _ := labels["vibespace.dev/id"].(string)
	name, _ := labels["app.kubernetes.io/name"].(string)
	projectName, _ := labels["vibespace.dev/project-name"].(string)
	createdAt, _ := annotations["vibespace.dev/created-at"].(string)

	status := knativeStatusToVibespaceStatus(svc)

	return &model.Vibespace{
		ID:          id,
		Name:        name,
		ProjectName: projectName,
		Status:      status,
		Resources: model.Resources{
			CPU:     "1",
			Memory:  "2Gi",
			Storage: "10Gi",
		},
		Persistent: true,
		CreatedAt:  createdAt,
	}
}

// knativeStatusToVibespaceStatus maps Knative Ready condition to vibespace status
func knativeStatusToVibespaceStatus(svc *unstructured.Unstructured) string {
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
			return "creating"
		}
	}

	return "creating"
}

// parseQuantity converts string storage size to Kubernetes resource.Quantity
func parseQuantity(storage string) resource.Quantity {
	qty, err := resource.ParseQuantity(storage)
	if err != nil {
		qty = resource.MustParse("10Gi")
	}
	return qty
}

func stringPtr(s string) *string {
	return &s
}

// getVibespaceImage returns the container image to use for vibespaces
func getVibespaceImage() string {
	image := os.Getenv("VIBESPACE_IMAGE")
	if image == "" {
		return DefaultImage
	}
	return image
}

// getBaseDomain returns the base domain for vibespace URLs
func getBaseDomain() string {
	domain := os.Getenv("DNS_BASE_DOMAIN")
	if domain == "" {
		return "vibe.space"
	}
	return domain
}

// generateUniqueProjectName generates a unique project name not in existingNames
func generateUniqueProjectName(existingNames []string) string {
	adjectives := []string{"swift", "bright", "calm", "bold", "keen", "wise", "pure", "fair", "true", "warm"}
	nouns := []string{"fox", "owl", "bear", "wolf", "hawk", "deer", "seal", "crow", "dove", "lynx"}

	existingSet := make(map[string]bool)
	for _, name := range existingNames {
		existingSet[name] = true
	}

	// Try random combinations
	for attempts := 0; attempts < 100; attempts++ {
		adj := adjectives[time.Now().UnixNano()%int64(len(adjectives))]
		noun := nouns[time.Now().UnixNano()%int64(len(nouns))]
		name := fmt.Sprintf("%s-%s", adj, noun)
		if !existingSet[name] {
			return name
		}
		time.Sleep(time.Nanosecond)
	}

	// Fallback: use timestamp
	return fmt.Sprintf("space-%d", time.Now().Unix())
}

// isValidGitURL validates a Git repository URL
func isValidGitURL(url string) bool {
	if len(url) < 10 {
		return false
	}

	dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r", "$(", "&&", "||"}
	for _, char := range dangerousChars {
		if strings.Contains(url, char) {
			return false
		}
	}

	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "git@")
}

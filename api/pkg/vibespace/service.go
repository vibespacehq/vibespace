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

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultImage is the single container image used for all vibespaces
// Contains a minimal Linux distro with Claude Code CLI pre-installed
const DefaultImage = "ghcr.io/yagizdagabak/vibespace/claude:latest"

// Service handles vibespace operations
type Service struct {
	k8sClient      *k8s.Client
	knativeManager *knative.ServiceManager
}

// NewService creates a new vibespace service
func NewService(k8sClient *k8s.Client) *Service {
	var knativeMgr *knative.ServiceManager

	if k8sClient != nil {
		var err error
		knativeMgr, err = knative.NewServiceManager(k8sClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager",
				"error", err)
		}
	}

	return &Service{
		k8sClient:      k8sClient,
		knativeManager: knativeMgr,
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

// Get returns a vibespace by name or ID
func (s *Service) Get(ctx context.Context, nameOrID string) (*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return nil, fmt.Errorf("knative manager is not initialized")
	}

	slog.Info("getting vibespace", "vibespace_id", nameOrID)

	// First try to find by name (label selector)
	service, err := s.knativeManager.GetServiceByName(ctx, nameOrID)
	if err != nil {
		// Fall back to lookup by ID
		service, err = s.knativeManager.GetService(ctx, nameOrID)
		if err != nil {
			slog.Warn("vibespace not found", "vibespace_id", nameOrID, "error", err)
			return nil, fmt.Errorf("vibespace not found: %w", err)
		}
	}

	vibespace := knativeServiceToVibespace(service)
	slog.Info("got vibespace successfully", "vibespace_id", nameOrID, "status", vibespace.Status)

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

	// Resources must be provided by the CLI
	if req.Resources == nil {
		return nil, fmt.Errorf("resources are required")
	}
	resources := req.Resources

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

	// Create SSH key secret for terminal access
	// Uses dedicated vibespace keypair (generates if needed)
	pubKey, err := EnsureSSHKey()
	if err != nil {
		slog.Warn("failed to ensure SSH key - SSH access will not work",
			"error", err)
	} else {
		if err := s.createSSHKeySecret(ctx, id, pubKey); err != nil {
			slog.Warn("failed to create SSH key secret", "error", err)
		} else {
			slog.Info("SSH key secret created", "vibespace_id", id)
		}
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
	err = s.knativeManager.CreateService(ctx, &knative.CreateServiceRequest{
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

// Delete deletes a vibespace by name or ID
func (s *Service) Delete(ctx context.Context, nameOrID string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return fmt.Errorf("knative manager is not initialized")
	}

	slog.Info("deleting vibespace", "vibespace_id", nameOrID)

	// Get vibespace to retrieve the internal ID and project name
	vibespace, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	// Use the internal ID for deletion
	if err := s.knativeManager.DeleteService(ctx, vibespace.ID); err != nil {
		return fmt.Errorf("failed to delete Knative Service: %w", err)
	}

	slog.Info("vibespace deleted successfully", "vibespace_id", vibespace.ID, "name", vibespace.Name)

	// Note: PVCs are left for manual cleanup to prevent accidental data loss
	return nil
}

// Start starts a stopped vibespace
func (s *Service) Start(ctx context.Context, nameOrID string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return fmt.Errorf("knative manager is not initialized")
	}

	// Resolve name to internal ID
	vibespace, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("starting vibespace", "vibespace_id", vibespace.ID, "name", vibespace.Name)

	// Set minScale=1 to ensure at least 1 replica is running
	annotations := map[string]string{
		"autoscaling.knative.dev/minScale": "1",
	}

	err = s.knativeManager.PatchService(ctx, vibespace.ID, annotations)
	if err != nil {
		slog.Error("failed to start vibespace", "vibespace_id", vibespace.ID, "error", err)
		return fmt.Errorf("failed to start vibespace: %w", err)
	}

	slog.Info("vibespace started successfully", "vibespace_id", vibespace.ID, "minScale", "1")
	return nil
}

// Stop stops a running vibespace
func (s *Service) Stop(ctx context.Context, nameOrID string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return fmt.Errorf("knative manager is not initialized")
	}

	// Resolve name to internal ID
	vibespace, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("stopping vibespace", "vibespace_id", vibespace.ID, "name", vibespace.Name)

	// Set minScale=0 to allow scaling to zero replicas
	annotations := map[string]string{
		"autoscaling.knative.dev/minScale": "0",
	}

	err = s.knativeManager.PatchService(ctx, vibespace.ID, annotations)
	if err != nil {
		slog.Error("failed to stop vibespace", "vibespace_id", vibespace.ID, "error", err)
		return fmt.Errorf("failed to stop vibespace: %w", err)
	}

	slog.Info("vibespace stopped successfully", "vibespace_id", vibespace.ID, "minScale", "0")
	return nil
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

	// Extract resources from the Knative service spec
	resources := extractResourcesFromKnativeService(svc)

	return &model.Vibespace{
		ID:          id,
		Name:        name,
		ProjectName: projectName,
		Status:      status,
		Resources:   resources,
		Persistent:  true,
		CreatedAt:   createdAt,
	}
}

// extractResourcesFromKnativeService extracts CPU and memory from Knative service spec
func extractResourcesFromKnativeService(svc *unstructured.Unstructured) model.Resources {
	// Navigate: spec.template.spec.containers[0].resources.requests
	containers, found, _ := unstructured.NestedSlice(svc.Object, "spec", "template", "spec", "containers")
	if !found || len(containers) == 0 {
		return model.Resources{}
	}

	container, ok := containers[0].(map[string]interface{})
	if !ok {
		return model.Resources{}
	}

	resources, ok := container["resources"].(map[string]interface{})
	if !ok {
		return model.Resources{}
	}

	requests, ok := resources["requests"].(map[string]interface{})
	if !ok {
		return model.Resources{}
	}

	cpu, _ := requests["cpu"].(string)
	memory, _ := requests["memory"].(string)

	return model.Resources{
		CPU:    cpu,
		Memory: memory,
		// Storage is not stored in Knative spec - it's in the PVC
		// We don't need it after creation anyway
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
			// We use port-forwarding, so IngressNotConfigured is expected.
			// Check if the workload itself is ready via ConfigurationsReady.
			if reason == "IngressNotConfigured" {
				// Check ConfigurationsReady condition instead
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

// AgentInfo contains information about an agent
type AgentInfo struct {
	ClaudeID  string // "1", "2", etc.
	AgentName string // "claude-1", "claude-2", etc.
	Status    string // running, stopped, creating
}

// SpawnAgent creates a new Claude agent in a vibespace
// Returns the agent name (e.g., "claude-2")
func (s *Service) SpawnAgent(ctx context.Context, nameOrID string) (string, error) {
	if err := s.ensureClients(); err != nil {
		return "", fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return "", fmt.Errorf("knative manager is not initialized")
	}

	// Get the vibespace
	vs, err := s.Get(ctx, nameOrID)
	if err != nil {
		return "", fmt.Errorf("vibespace not found: %w", err)
	}

	// Get next available claude ID
	nextID, err := s.knativeManager.GetNextAgentID(ctx, vs.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get next agent ID: %w", err)
	}

	slog.Info("spawning agent", "vibespace_id", vs.ID, "name", vs.Name, "claude_id", nextID)

	// Get the image
	image := getVibespaceImage()

	// Create the agent service
	pvcName := ""
	if vs.Persistent {
		pvcName = fmt.Sprintf("vibespace-%s-pvc", vs.ID)
	}

	err = s.knativeManager.CreateAgentService(ctx, &knative.CreateAgentServiceRequest{
		VibespaceID: vs.ID,
		Name:        vs.Name,
		ProjectName: vs.ProjectName,
		ClaudeID:    nextID,
		Image:       image,
		Resources:   vs.Resources,
		Env:         nil,
		PVCName:     pvcName,
	})
	if err != nil {
		slog.Error("failed to spawn agent", "vibespace_id", vs.ID, "claude_id", nextID, "error", err)
		return "", fmt.Errorf("failed to spawn agent: %w", err)
	}

	agentName := fmt.Sprintf("claude-%s", nextID)
	slog.Info("agent spawned successfully", "vibespace_id", vs.ID, "agent", agentName)

	return agentName, nil
}

// KillAgent removes a Claude agent from a vibespace
// Cannot kill claude-1 (the main agent)
func (s *Service) KillAgent(ctx context.Context, nameOrID string, agentID string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return fmt.Errorf("knative manager is not initialized")
	}

	// Parse agent name to get claude ID
	claudeID := agentID
	if strings.HasPrefix(agentID, "claude-") {
		claudeID = strings.TrimPrefix(agentID, "claude-")
	}

	// Cannot kill claude-1
	if claudeID == "1" {
		return fmt.Errorf("cannot kill claude-1: it is the main agent. Delete the vibespace to remove it")
	}

	// Get the vibespace
	vs, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("killing agent", "vibespace_id", vs.ID, "name", vs.Name, "claude_id", claudeID)

	// Delete the agent service
	err = s.knativeManager.DeleteAgentService(ctx, vs.ID, claudeID)
	if err != nil {
		slog.Error("failed to kill agent", "vibespace_id", vs.ID, "claude_id", claudeID, "error", err)
		return fmt.Errorf("failed to kill agent: %w", err)
	}

	slog.Info("agent killed successfully", "vibespace_id", vs.ID, "claude_id", claudeID)
	return nil
}

// ListAgents returns all agents in a vibespace
func (s *Service) ListAgents(ctx context.Context, nameOrID string) ([]AgentInfo, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.knativeManager == nil {
		return nil, fmt.Errorf("knative manager is not initialized")
	}

	// Get the vibespace
	vs, err := s.Get(ctx, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("vibespace not found: %w", err)
	}

	// List agents from Knative
	knativeAgents, err := s.knativeManager.ListAgentsForVibespace(ctx, vs.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Convert to our AgentInfo type
	agents := make([]AgentInfo, len(knativeAgents))
	for i, ka := range knativeAgents {
		agents[i] = AgentInfo{
			ClaudeID:  ka.ClaudeID,
			AgentName: ka.AgentName,
			Status:    ka.Status,
		}
	}

	return agents, nil
}

// createSSHKeySecret creates a Kubernetes Secret containing the SSH public key
func (s *Service) createSSHKeySecret(ctx context.Context, vibespaceID string, pubKey string) error {
	secretName := fmt.Sprintf("vibespace-%s-ssh-keys", vibespaceID)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: k8s.VibespaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "vibespace",
				"vibespace.dev/id":             vibespaceID,
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"authorized_keys": pubKey,
		},
	}

	_, err := s.k8sClient.Clientset().CoreV1().Secrets(k8s.VibespaceNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing secret
			_, err = s.k8sClient.Clientset().CoreV1().Secrets(k8s.VibespaceNamespace).Update(ctx, secret, metav1.UpdateOptions{})
		}
	}
	return err
}

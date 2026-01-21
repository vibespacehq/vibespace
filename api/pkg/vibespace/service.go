package vibespace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"vibespace/pkg/deployment"
	"vibespace/pkg/k8s"
	"vibespace/pkg/model"

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultImage is the single container image used for all vibespaces
// Contains a minimal Linux distro with Claude Code CLI pre-installed
const DefaultImage = "ghcr.io/yagizdagabak/vibespace/claude:latest"

// Service handles vibespace operations
type Service struct {
	k8sClient         *k8s.Client
	deploymentManager *deployment.DeploymentManager
}

// NewService creates a new vibespace service
func NewService(k8sClient *k8s.Client) *Service {
	var deployMgr *deployment.DeploymentManager

	if k8sClient != nil {
		deployMgr = deployment.NewDeploymentManager(k8sClient)
	}

	return &Service{
		k8sClient:         k8sClient,
		deploymentManager: deployMgr,
	}
}

// List returns all vibespaces
func (s *Service) List(ctx context.Context) ([]*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		slog.Warn("kubernetes client not available, returning empty list", "error", err)
		return []*model.Vibespace{}, nil
	}

	if s.deploymentManager == nil {
		slog.Warn("deployment manager is nil, returning empty list")
		return []*model.Vibespace{}, nil
	}

	deployments, err := s.deploymentManager.ListDeployments(ctx)
	if err != nil {
		slog.Error("failed to list Deployments", "error", err)
		return nil, fmt.Errorf("failed to list Deployments: %w", err)
	}

	// Filter to only include main deployments (not agent deployments)
	// Main deployments don't have the "is-agent" label
	vibespaces := make([]*model.Vibespace, 0)
	for i := range deployments {
		// Skip agent deployments
		if deployments[i].Labels["vibespace.dev/is-agent"] == "true" {
			continue
		}
		vibespace := deploymentToVibespace(&deployments[i])
		vibespaces = append(vibespaces, vibespace)
	}

	return vibespaces, nil
}

// Get returns a vibespace by name or ID
func (s *Service) Get(ctx context.Context, nameOrID string) (*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.deploymentManager == nil {
		return nil, fmt.Errorf("deployment manager is not initialized")
	}

	slog.Info("getting vibespace", "vibespace_id", nameOrID)

	// First try to find by name (label selector)
	deploy, err := s.deploymentManager.GetDeploymentByName(ctx, nameOrID)
	if err != nil {
		// Fall back to lookup by ID
		deploy, err = s.deploymentManager.GetDeployment(ctx, nameOrID)
		if err != nil {
			slog.Warn("vibespace not found", "vibespace_id", nameOrID, "error", err)
			return nil, fmt.Errorf("vibespace not found: %w", err)
		}
	}

	vibespace := deploymentToVibespace(deploy)
	slog.Info("got vibespace successfully", "vibespace_id", nameOrID, "status", vibespace.Status)

	return vibespace, nil
}

// Create creates a new vibespace
func (s *Service) Create(ctx context.Context, req *model.CreateVibespaceRequest) (*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.deploymentManager == nil {
		return nil, fmt.Errorf("deployment manager is not initialized")
	}

	// Validate vibespace name format
	if err := ValidateName(req.Name); err != nil {
		return nil, fmt.Errorf("invalid vibespace name: %w", err)
	}

	slog.Info("creating vibespace",
		"name", req.Name,
		"github_repo", req.GithubRepo,
		"persistent", req.Persistent)

	// Check if a vibespace with this name already exists
	existingVibespaces, _ := s.List(ctx)
	for _, vs := range existingVibespaces {
		if vs.Name == req.Name {
			return nil, fmt.Errorf("vibespace '%s' already exists", req.Name)
		}
	}

	// Ensure namespace exists
	if err := s.k8sClient.EnsureNamespace(ctx); err != nil {
		slog.Error("failed to ensure namespace", "error", err, "namespace", k8s.VibespaceNamespace)
		return nil, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	id := uuid.New().String()[:8]
	pvcName := fmt.Sprintf("vibespace-%s-pvc", id)

	slog.Debug("generated vibespace id",
		"vibespace_id", id,
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

	slog.Info("creating vibespace with Deployment",
		"vibespace_id", id,
		"name", req.Name,
		"image", image)

	// Create Deployment
	err = s.deploymentManager.CreateDeployment(ctx, &deployment.CreateDeploymentRequest{
		VibespaceID:      id,
		Name:             req.Name,
		ClaudeID:         "1", // First Claude instance
		Image:            image,
		Resources: deployment.Resources{
			CPU:     resources.CPU,
			Memory:  resources.Memory,
			Storage: resources.Storage,
		},
		Env:              req.Env,
		Persistent:       req.Persistent,
		PVCName:          pvcName,
		ShareCredentials: req.ShareCredentials,
	})
	if err != nil {
		slog.Error("failed to create Deployment", "vibespace_id", id, "error", err)
		return nil, fmt.Errorf("failed to create Deployment: %w", err)
	}

	vibespace := &model.Vibespace{
		ID:         id,
		Name:       req.Name,
		Status:     "creating",
		Resources:  *resources,
		Persistent: req.Persistent,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	slog.Info("vibespace created successfully",
		"vibespace_id", id,
		"name", req.Name)

	return vibespace, nil
}

// DeleteOptions configures vibespace deletion behavior
type DeleteOptions struct {
	KeepData  bool              // If true, preserve PVC (storage data)
	Vibespace *model.Vibespace  // If provided, skip lookup (avoids redundant API call)
}

// Delete deletes a vibespace by name or ID
func (s *Service) Delete(ctx context.Context, nameOrID string, opts *DeleteOptions) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.deploymentManager == nil {
		return fmt.Errorf("deployment manager is not initialized")
	}

	if opts == nil {
		opts = &DeleteOptions{}
	}

	slog.Info("deleting vibespace", "vibespace_id", nameOrID, "keep_data", opts.KeepData)

	// Use provided vibespace or look it up
	vibespace := opts.Vibespace
	if vibespace == nil {
		var err error
		vibespace, err = s.Get(ctx, nameOrID)
		if err != nil {
			return fmt.Errorf("vibespace not found: %w", err)
		}
	}

	// Delete all agent deployments first
	agents, err := s.deploymentManager.ListAgentsForVibespace(ctx, vibespace.ID)
	if err == nil {
		for _, agent := range agents {
			if agent.ClaudeID != "1" {
				if err := s.deploymentManager.DeleteAgentDeployment(ctx, vibespace.ID, agent.ClaudeID); err != nil {
					slog.Warn("failed to delete agent deployment", "agent", agent.AgentName, "error", err)
				}
			}
		}
	}

	// Use the internal ID for deletion
	if err := s.deploymentManager.DeleteDeployment(ctx, vibespace.ID); err != nil {
		return fmt.Errorf("failed to delete Deployment: %w", err)
	}

	slog.Info("vibespace deleted successfully", "vibespace_id", vibespace.ID, "name", vibespace.Name)

	// Clean up associated resources unless --keep-data is specified
	if !opts.KeepData {
		// Delete PVC
		pvcName := fmt.Sprintf("vibespace-%s-pvc", vibespace.ID)
		if err := s.deletePVC(ctx, pvcName); err != nil {
			slog.Warn("failed to delete PVC", "pvc", pvcName, "error", err)
		} else {
			slog.Info("PVC deleted", "pvc", pvcName)
		}

		// Delete SSH key secret
		secretName := fmt.Sprintf("vibespace-%s-ssh-keys", vibespace.ID)
		if err := s.deleteSecret(ctx, secretName); err != nil {
			slog.Warn("failed to delete SSH key secret", "secret", secretName, "error", err)
		} else {
			slog.Info("SSH key secret deleted", "secret", secretName)
		}
	}

	return nil
}

// deletePVC deletes a PersistentVolumeClaim
func (s *Service) deletePVC(ctx context.Context, name string) error {
	err := s.k8sClient.Clientset().CoreV1().PersistentVolumeClaims(k8s.VibespaceNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

// deleteSecret deletes a Secret
func (s *Service) deleteSecret(ctx context.Context, name string) error {
	err := s.k8sClient.Clientset().CoreV1().Secrets(k8s.VibespaceNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

// Start starts a stopped vibespace
func (s *Service) Start(ctx context.Context, nameOrID string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.deploymentManager == nil {
		return fmt.Errorf("deployment manager is not initialized")
	}

	// Resolve name to internal ID
	vibespace, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("starting vibespace", "vibespace_id", vibespace.ID, "name", vibespace.Name)

	// Scale all deployments for this vibespace to 1 replica
	err = s.deploymentManager.ScaleAllDeploymentsForVibespace(ctx, vibespace.ID, 1)
	if err != nil {
		slog.Error("failed to start vibespace", "vibespace_id", vibespace.ID, "error", err)
		return fmt.Errorf("failed to start vibespace: %w", err)
	}

	slog.Info("vibespace started successfully", "vibespace_id", vibespace.ID, "replicas", "1")
	return nil
}

// Stop stops a running vibespace
func (s *Service) Stop(ctx context.Context, nameOrID string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.deploymentManager == nil {
		return fmt.Errorf("deployment manager is not initialized")
	}

	// Resolve name to internal ID
	vibespace, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("stopping vibespace", "vibespace_id", vibespace.ID, "name", vibespace.Name)

	// Scale all deployments for this vibespace to 0 replicas
	err = s.deploymentManager.ScaleAllDeploymentsForVibespace(ctx, vibespace.ID, 0)
	if err != nil {
		slog.Error("failed to stop vibespace", "vibespace_id", vibespace.ID, "error", err)
		return fmt.Errorf("failed to stop vibespace: %w", err)
	}

	slog.Info("vibespace stopped successfully", "vibespace_id", vibespace.ID, "replicas", "0")
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

		// Also initialize Deployment manager
		s.deploymentManager = deployment.NewDeploymentManager(newClient)
		slog.Info("deployment manager initialized successfully")
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

// deploymentToVibespace converts a Deployment to Vibespace model
func deploymentToVibespace(deploy *appsv1.Deployment) *model.Vibespace {
	labels := deploy.Labels
	annotations := deploy.Annotations

	id := labels["vibespace.dev/id"]
	name := labels["app.kubernetes.io/name"]
	createdAt := ""
	if annotations != nil {
		createdAt = annotations["vibespace.dev/created-at"]
	}

	status := deploymentStatusToVibespaceStatus(deploy)

	// Extract resources from the Deployment spec
	resources := extractResourcesFromDeployment(deploy)

	return &model.Vibespace{
		ID:         id,
		Name:       name,
		Status:     status,
		Resources:  resources,
		Persistent: true,
		CreatedAt:  createdAt,
	}
}

// extractResourcesFromDeployment extracts CPU, memory, and storage from Deployment
func extractResourcesFromDeployment(deploy *appsv1.Deployment) model.Resources {
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return model.Resources{}
	}

	requests := containers[0].Resources.Requests
	cpu := ""
	memory := ""

	if cpuQty, ok := requests[corev1.ResourceCPU]; ok {
		cpu = cpuQty.String()
	}
	if memQty, ok := requests[corev1.ResourceMemory]; ok {
		memory = memQty.String()
	}

	// Storage is stored as annotation since it's actually in the PVC
	storage := ""
	if deploy.Annotations != nil {
		storage = deploy.Annotations["vibespace.dev/storage"]
	}

	return model.Resources{
		CPU:     cpu,
		Memory:  memory,
		Storage: storage,
	}
}

// deploymentStatusToVibespaceStatus maps Deployment status to vibespace status
func deploymentStatusToVibespaceStatus(deploy *appsv1.Deployment) string {
	// If replicas is set to 0, the vibespace is stopped
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0 {
		return "stopped"
	}

	// If we have ready replicas, the vibespace is running
	if deploy.Status.ReadyReplicas > 0 {
		return "running"
	}

	// If there are replicas but none ready yet, it's still creating
	if deploy.Status.Replicas > 0 {
		return "creating"
	}

	return "stopped"
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

// ValidateName checks if a name is valid (DNS-friendly: lowercase, alphanumeric, hyphens)
// Returns an error describing the validation failure, or nil if valid
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("name cannot exceed 63 characters")
	}
	if len(name) < 2 {
		return fmt.Errorf("name must be at least 2 characters")
	}

	// Must start with a letter
	if name[0] < 'a' || name[0] > 'z' {
		return fmt.Errorf("name must start with a lowercase letter")
	}

	// Must end with alphanumeric
	last := name[len(name)-1]
	if !((last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) {
		return fmt.Errorf("name must end with a lowercase letter or number")
	}

	// Check each character
	for i, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		if c == '-' {
			// No consecutive hyphens
			if i > 0 && name[i-1] == '-' {
				return fmt.Errorf("name cannot contain consecutive hyphens")
			}
			continue
		}
		if c >= 'A' && c <= 'Z' {
			return fmt.Errorf("name must be lowercase (found '%c')", c)
		}
		return fmt.Errorf("name can only contain lowercase letters, numbers, and hyphens (found '%c')", c)
	}

	return nil
}

// AgentInfo contains information about an agent
type AgentInfo struct {
	ClaudeID  string // "1", "2", etc.
	AgentName string // "claude-1", "claude-2", etc.
	Status    string // running, stopped, creating
}

// SpawnAgentOptions contains options for spawning an agent
type SpawnAgentOptions struct {
	// Name is an optional custom agent name (auto-generated if empty)
	Name string

	// ShareCredentials enables credential sharing via /vibespace/.vibespace
	// When enabled, Claude config, git config, and SSH keys are shared across agents
	ShareCredentials bool
}

// SpawnAgent creates a new Claude agent in a vibespace
// Returns the agent name (e.g., "claude-2" or custom name)
func (s *Service) SpawnAgent(ctx context.Context, nameOrID string, opts *SpawnAgentOptions) (string, error) {
	if err := s.ensureClients(); err != nil {
		return "", fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.deploymentManager == nil {
		return "", fmt.Errorf("deployment manager is not initialized")
	}

	// Default options
	if opts == nil {
		opts = &SpawnAgentOptions{}
	}

	// Get the vibespace
	vs, err := s.Get(ctx, nameOrID)
	if err != nil {
		return "", fmt.Errorf("vibespace not found: %w", err)
	}

	// Get existing agents for uniqueness check
	existingAgents, err := s.ListAgents(ctx, vs.ID)
	if err != nil {
		return "", fmt.Errorf("failed to list agents: %w", err)
	}

	var agentName string
	var claudeID string

	if opts.Name != "" {
		// Custom name provided - validate format
		if err := ValidateName(opts.Name); err != nil {
			return "", fmt.Errorf("invalid agent name: %w", err)
		}

		// Check uniqueness
		for _, agent := range existingAgents {
			if agent.AgentName == opts.Name {
				return "", fmt.Errorf("agent '%s' already exists in this vibespace", opts.Name)
			}
		}

		agentName = opts.Name
		// Generate a unique claude ID for internal use
		nextID, err := s.deploymentManager.GetNextAgentID(ctx, vs.ID)
		if err != nil {
			return "", fmt.Errorf("failed to get next agent ID: %w", err)
		}
		claudeID = nextID
	} else {
		// Auto-generate name from claude ID
		nextID, err := s.deploymentManager.GetNextAgentID(ctx, vs.ID)
		if err != nil {
			return "", fmt.Errorf("failed to get next agent ID: %w", err)
		}
		claudeID = nextID
		agentName = fmt.Sprintf("claude-%s", nextID)
	}

	slog.Info("spawning agent", "vibespace_id", vs.ID, "vibespace_name", vs.Name, "agent_name", agentName, "claude_id", claudeID, "share_credentials", opts.ShareCredentials)

	// Get the image
	image := getVibespaceImage()

	// Create the agent deployment
	pvcName := ""
	if vs.Persistent {
		pvcName = fmt.Sprintf("vibespace-%s-pvc", vs.ID)
	}

	err = s.deploymentManager.CreateAgentDeployment(ctx, &deployment.CreateAgentRequest{
		VibespaceID: vs.ID,
		Name:        vs.Name,
		AgentName:   agentName,
		ClaudeID:    claudeID,
		Image:       image,
		Resources: deployment.Resources{
			CPU:     vs.Resources.CPU,
			Memory:  vs.Resources.Memory,
			Storage: vs.Resources.Storage,
		},
		Env:              nil,
		PVCName:          pvcName,
		ShareCredentials: opts.ShareCredentials,
	})
	if err != nil {
		slog.Error("failed to spawn agent", "vibespace_id", vs.ID, "agent_name", agentName, "error", err)
		return "", fmt.Errorf("failed to spawn agent: %w", err)
	}

	slog.Info("agent spawned successfully", "vibespace_id", vs.ID, "agent", agentName)

	return agentName, nil
}

// KillAgent removes a Claude agent from a vibespace
// Cannot kill claude-1 (the main agent)
func (s *Service) KillAgent(ctx context.Context, nameOrID string, agentID string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
	}

	if s.deploymentManager == nil {
		return fmt.Errorf("deployment manager is not initialized")
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

	// Delete the agent deployment
	err = s.deploymentManager.DeleteAgentDeployment(ctx, vs.ID, claudeID)
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

	if s.deploymentManager == nil {
		return nil, fmt.Errorf("deployment manager is not initialized")
	}

	// Get the vibespace
	vs, err := s.Get(ctx, nameOrID)
	if err != nil {
		return nil, fmt.Errorf("vibespace not found: %w", err)
	}

	// List agents from Deployments
	deploymentAgents, err := s.deploymentManager.ListAgentsForVibespace(ctx, vs.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Convert to our AgentInfo type
	agents := make([]AgentInfo, len(deploymentAgents))
	for i, da := range deploymentAgents {
		agents[i] = AgentInfo{
			ClaudeID:  da.ClaudeID,
			AgentName: da.AgentName,
			Status:    da.Status,
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

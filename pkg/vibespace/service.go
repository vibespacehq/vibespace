package vibespace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/yagizdagabak/vibespace/pkg/agent"
	// Import agent implementations to trigger init() registration
	_ "github.com/yagizdagabak/vibespace/pkg/agent/claude"
	_ "github.com/yagizdagabak/vibespace/pkg/agent/codex"
	"github.com/yagizdagabak/vibespace/pkg/deployment"
	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
	"github.com/yagizdagabak/vibespace/pkg/k8s"
	"github.com/yagizdagabak/vibespace/pkg/model"

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


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
		return nil, fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return nil, vserrors.ErrDeploymentManagerNotInitialized
	}

	slog.Info("getting vibespace", "vibespace_id", nameOrID)

	// First try to find by name (label selector)
	deploy, err := s.deploymentManager.GetDeploymentByName(ctx, nameOrID)
	if err != nil {
		// Fall back to lookup by ID
		deploy, err = s.deploymentManager.GetDeployment(ctx, nameOrID)
		if err != nil {
			slog.Warn("vibespace not found", "vibespace_id", nameOrID, "error", err)
			return nil, fmt.Errorf("%s: %w", nameOrID, vserrors.ErrVibespaceNotFound)
		}
	}

	vibespace := deploymentToVibespace(deploy)
	slog.Info("got vibespace successfully", "vibespace_id", nameOrID, "status", vibespace.Status)

	return vibespace, nil
}

// Create creates a new vibespace
func (s *Service) Create(ctx context.Context, req *model.CreateVibespaceRequest) (*model.Vibespace, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return nil, vserrors.ErrDeploymentManagerNotInitialized
	}

	// Validate vibespace name format
	if err := ValidateName(req.Name); err != nil {
		return nil, fmt.Errorf("invalid vibespace name: %w", err)
	}

	// Validate agent name format if custom name provided
	if req.AgentName != "" {
		if err := ValidateName(req.AgentName); err != nil {
			return nil, fmt.Errorf("invalid agent name: %w", err)
		}
	}

	slog.Info("creating vibespace",
		"name", req.Name,
		"github_repo", req.GithubRepo,
		"persistent", req.Persistent)

	// Check if a vibespace with this name already exists
	existingVibespaces, _ := s.List(ctx)
	for _, vs := range existingVibespaces {
		if vs.Name == req.Name {
			return nil, fmt.Errorf("'%s': %w", req.Name, vserrors.ErrVibespaceExists)
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

	// Determine agent type and get the appropriate image
	agentType := req.AgentType
	if agentType == "" {
		agentType = agent.TypeClaudeCode // Default
	}

	agentImpl, err := agent.Get(agentType)
	if err != nil {
		return nil, fmt.Errorf("unknown agent type '%s': %w", agentType, err)
	}

	// Get image: environment override, or agent default
	image := getVibespaceImage(agentType)
	if image == "" {
		image = agentImpl.ContainerImage()
	}

	slog.Info("creating vibespace with Deployment",
		"vibespace_id", id,
		"name", req.Name,
		"agent_type", agentType,
		"image", image)

	// Get agent configuration
	config := req.AgentConfig

	// Create Deployment
	err = s.deploymentManager.CreateDeployment(ctx, &deployment.CreateDeploymentRequest{
		VibespaceID: id,
		Name:        req.Name,
		AgentType:   agentType,
		AgentNum:    1, // First agent
		AgentName:   req.AgentName, // Custom name or empty for default
		Primary:     true, // This is the original agent created with the vibespace
		Image:       image,
		Resources: deployment.Resources{
			CPU:     resources.CPU,
			Memory:  resources.Memory,
			Storage: resources.Storage,
		},
		Env:              req.Env,
		Persistent:       req.Persistent,
		PVCName:          pvcName,
		ShareCredentials: req.ShareCredentials,
		Config:           config,
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
		return fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return vserrors.ErrDeploymentManagerNotInitialized
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

	// Delete all agent deployments first (except primary agent which is handled with main deployment)
	agentList, err := s.deploymentManager.ListAgentsForVibespace(ctx, vibespace.ID)
	if err == nil {
		for _, ag := range agentList {
			if !ag.IsPrimary {
				if err := s.deploymentManager.DeleteAgentDeployment(ctx, vibespace.ID, ag.AgentName); err != nil {
					slog.Warn("failed to delete agent deployment", "agent", ag.AgentName, "error", err)
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
		return fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return vserrors.ErrDeploymentManagerNotInitialized
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
		return fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return vserrors.ErrDeploymentManagerNotInitialized
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

// StartAgent starts a specific agent in a vibespace
func (s *Service) StartAgent(ctx context.Context, nameOrID string, agentName string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return vserrors.ErrDeploymentManagerNotInitialized
	}

	// Resolve name to internal ID
	vibespace, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("starting agent", "vibespace_id", vibespace.ID, "agent", agentName)

	// Scale the agent's deployment to 1 replica
	err = s.deploymentManager.ScaleAgentDeployment(ctx, vibespace.ID, agentName, 1)
	if err != nil {
		slog.Error("failed to start agent", "vibespace_id", vibespace.ID, "agent", agentName, "error", err)
		return fmt.Errorf("failed to start agent: %w", err)
	}

	slog.Info("agent started successfully", "vibespace_id", vibespace.ID, "agent", agentName)
	return nil
}

// StopAgent stops a specific agent in a vibespace
func (s *Service) StopAgent(ctx context.Context, nameOrID string, agentName string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return vserrors.ErrDeploymentManagerNotInitialized
	}

	// Resolve name to internal ID
	vibespace, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("stopping agent", "vibespace_id", vibespace.ID, "agent", agentName)

	// Scale the agent's deployment to 0 replicas
	err = s.deploymentManager.ScaleAgentDeployment(ctx, vibespace.ID, agentName, 0)
	if err != nil {
		slog.Error("failed to stop agent", "vibespace_id", vibespace.ID, "agent", agentName, "error", err)
		return fmt.Errorf("failed to stop agent: %w", err)
	}

	slog.Info("agent stopped successfully", "vibespace_id", vibespace.ID, "agent", agentName)
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

// getVibespaceImage returns the container image override for a given agent type.
// Returns empty string if no override is set (caller should use agent's default).
func getVibespaceImage(agentType agent.Type) string {
	// Check for type-specific override first
	typeEnvVar := fmt.Sprintf("VIBESPACE_IMAGE_%s", strings.ToUpper(strings.ReplaceAll(string(agentType), "-", "_")))
	if image := os.Getenv(typeEnvVar); image != "" {
		return image
	}

	// Fall back to generic override
	if image := os.Getenv("VIBESPACE_IMAGE"); image != "" {
		return image
	}

	return "" // No override - use agent's default
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
	// ID is the agent's UUID (may be empty for legacy agents)
	ID string

	// AgentType is the type of agent (claude-code, codex, etc.)
	AgentType agent.Type

	// AgentNum is the sequential number within the agent type (1, 2, 3...)
	AgentNum int

	// AgentName is the display name (e.g., "claude-1", "codex-2")
	AgentName string

	// IsPrimary indicates this is the original agent created with the vibespace
	IsPrimary bool

	// Status is the current status: running, stopped, creating
	Status string
}

// SpawnAgentOptions contains options for spawning an agent
type SpawnAgentOptions struct {
	// Name is an optional custom agent name (auto-generated if empty)
	Name string

	// AgentType specifies the type of agent to spawn (default: same as first agent)
	AgentType agent.Type

	// ShareCredentials enables credential sharing via /vibespace/.vibespace
	// When enabled, Claude config, git config, and SSH keys are shared across agents
	ShareCredentials bool

	// Config is the agent configuration (nil = inherit from vibespace)
	Config *agent.Config
}

// SpawnAgent creates a new agent in a vibespace
// Returns the agent name (e.g., "claude-2" or custom name)
func (s *Service) SpawnAgent(ctx context.Context, nameOrID string, opts *SpawnAgentOptions) (string, error) {
	if err := s.ensureClients(); err != nil {
		return "", fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return "", vserrors.ErrDeploymentManagerNotInitialized
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

	// Get existing agents for uniqueness check and to determine default agent type
	existingAgents, err := s.ListAgents(ctx, vs.ID)
	if err != nil {
		return "", fmt.Errorf("failed to list agents: %w", err)
	}

	// Determine agent type: use specified type, or inherit from primary agent's type
	agentType := opts.AgentType
	if agentType == "" {
		// Find the primary agent and inherit its type
		for _, a := range existingAgents {
			if a.IsPrimary {
				agentType = a.AgentType
				break
			}
		}
		// Fallback to first agent if no primary found (shouldn't happen)
		if agentType == "" && len(existingAgents) > 0 {
			agentType = existingAgents[0].AgentType
		}
		// Final fallback to default
		if agentType == "" {
			agentType = agent.TypeClaudeCode
		}
	}

	agentImpl, err := agent.Get(agentType)
	if err != nil {
		return "", fmt.Errorf("unknown agent type '%s': %w", agentType, err)
	}

	// Calculate next agent number for this type
	nextNum := 1
	for _, a := range existingAgents {
		if a.AgentType == agentType && a.AgentNum >= nextNum {
			nextNum = a.AgentNum + 1
		}
	}

	var agentName string
	if opts.Name != "" {
		// Custom name provided - validate format
		if err := ValidateName(opts.Name); err != nil {
			return "", fmt.Errorf("invalid agent name: %w", err)
		}

		// Check uniqueness
		for _, a := range existingAgents {
			if a.AgentName == opts.Name {
				return "", fmt.Errorf("agent '%s' already exists in this vibespace", opts.Name)
			}
		}
		agentName = opts.Name
	} else {
		// Auto-generate name from agent type and number
		agentName = fmt.Sprintf("%s-%d", agentImpl.DefaultAgentPrefix(), nextNum)
	}

	slog.Info("spawning agent",
		"vibespace_id", vs.ID,
		"vibespace_name", vs.Name,
		"agent_type", agentType,
		"agent_name", agentName,
		"agent_num", nextNum,
		"share_credentials", opts.ShareCredentials)

	// Get the image
	image := getVibespaceImage(agentType)
	if image == "" {
		image = agentImpl.ContainerImage()
	}

	// Create the agent deployment
	pvcName := ""
	if vs.Persistent {
		pvcName = fmt.Sprintf("vibespace-%s-pvc", vs.ID)
	}

	// Get agent configuration
	config := opts.Config

	err = s.deploymentManager.CreateAgentDeployment(ctx, &deployment.CreateAgentRequest{
		VibespaceID: vs.ID,
		Name:        vs.Name,
		AgentType:   agentType,
		AgentNum:    nextNum,
		AgentName:   agentName,
		Image:       image,
		Resources: deployment.Resources{
			CPU:     vs.Resources.CPU,
			Memory:  vs.Resources.Memory,
			Storage: vs.Resources.Storage,
		},
		Env:              nil,
		PVCName:          pvcName,
		ShareCredentials: opts.ShareCredentials,
		Config:           config,
	})
	if err != nil {
		slog.Error("failed to spawn agent", "vibespace_id", vs.ID, "agent_name", agentName, "error", err)
		return "", fmt.Errorf("failed to spawn agent: %w", err)
	}

	slog.Info("agent spawned successfully", "vibespace_id", vs.ID, "agent", agentName)

	return agentName, nil
}

// KillAgent removes an agent from a vibespace
// Cannot kill the primary agent (agent number 1)
func (s *Service) KillAgent(ctx context.Context, nameOrID string, agentName string) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return vserrors.ErrDeploymentManagerNotInitialized
	}

	// Get the vibespace
	vs, err := s.Get(ctx, nameOrID)
	if err != nil {
		return fmt.Errorf("vibespace not found: %w", err)
	}

	// Find the agent to check if it's the primary one
	agents, err := s.ListAgents(ctx, vs.ID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	var targetAgent *AgentInfo
	for i := range agents {
		if agents[i].AgentName == agentName {
			targetAgent = &agents[i]
			break
		}
	}

	if targetAgent == nil {
		return fmt.Errorf("agent '%s' not found", agentName)
	}

	// Cannot kill the primary agent (the original agent created with the vibespace)
	if targetAgent.IsPrimary {
		return fmt.Errorf("cannot kill %s: it is the primary agent. Delete the vibespace to remove it", agentName)
	}

	slog.Info("killing agent", "vibespace_id", vs.ID, "name", vs.Name, "agent", agentName)

	// Delete the agent deployment
	err = s.deploymentManager.DeleteAgentDeployment(ctx, vs.ID, agentName)
	if err != nil {
		slog.Error("failed to kill agent", "vibespace_id", vs.ID, "agent", agentName, "error", err)
		return fmt.Errorf("failed to kill agent: %w", err)
	}

	slog.Info("agent killed successfully", "vibespace_id", vs.ID, "agent", agentName)
	return nil
}

// ListAgents returns all agents in a vibespace
func (s *Service) ListAgents(ctx context.Context, nameOrID string) ([]AgentInfo, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return nil, vserrors.ErrDeploymentManagerNotInitialized
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
			ID:        da.ID,
			AgentType: da.AgentType,
			AgentNum:  da.AgentNum,
			AgentName: da.AgentName,
			IsPrimary: da.IsPrimary,
			Status:    da.Status,
		}
	}

	return agents, nil
}

// GetAgentConfig returns the configuration for an agent
func (s *Service) GetAgentConfig(ctx context.Context, vibespaceNameOrID, agentName string) (*agent.Config, error) {
	if err := s.ensureClients(); err != nil {
		return nil, fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return nil, vserrors.ErrDeploymentManagerNotInitialized
	}

	// Get vibespace to find deployment name
	vs, err := s.Get(ctx, vibespaceNameOrID)
	if err != nil {
		return nil, err
	}

	return s.deploymentManager.GetAgentConfig(ctx, vs.ID, agentName)
}

// UpdateAgentConfig updates the configuration for an agent (triggers pod restart)
func (s *Service) UpdateAgentConfig(ctx context.Context, vibespaceNameOrID, agentName string, config *agent.Config) error {
	if err := s.ensureClients(); err != nil {
		return fmt.Errorf("please install and start Kubernetes first: %w", vserrors.ErrKubernetesNotAvailable)
	}

	if s.deploymentManager == nil {
		return vserrors.ErrDeploymentManagerNotInitialized
	}

	// Get vibespace to find deployment name
	vs, err := s.Get(ctx, vibespaceNameOrID)
	if err != nil {
		return err
	}

	return s.deploymentManager.UpdateAgentConfig(ctx, vs.ID, agentName, config)
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

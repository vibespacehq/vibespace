package vibespace

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"vibespace/pkg/k8s"
	"vibespace/pkg/knative"
	"vibespace/pkg/model"
	"vibespace/pkg/network"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Port range for vibespace port-forwarding (8080-9079)
	PortRangeStart = 8080
	PortRangeEnd   = 9079
	// Maximum number of ports to try before giving up
	MaxPortTries = 10
)

// Service handles vibespace operations
type Service struct {
	k8sClient           *k8s.Client
	knativeManager      *knative.ServiceManager
	ingressRouteManager *network.IngressRouteManager
}

// NewService creates a new vibespace service
func NewService(k8sClient *k8s.Client) *Service {
	// Initialize Knative Service manager (may be nil if k8sClient is nil)
	var knativeMgr *knative.ServiceManager
	var ingressMgr *network.IngressRouteManager

	if k8sClient != nil {
		var err error
		knativeMgr, err = knative.NewServiceManager(k8sClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager",
				"error", err)
		}

		ingressMgr, err = network.NewIngressRouteManager(k8sClient, getBaseDomain())
		if err != nil {
			slog.Warn("failed to initialize IngressRoute manager",
				"error", err)
		}
	}

	return &Service{
		k8sClient:           k8sClient,
		knativeManager:      knativeMgr,
		ingressRouteManager: ingressMgr,
	}
}

// List returns all vibespaces
func (s *Service) List(ctx context.Context) ([]*model.Vibespace, error) {
	// Check if k8s client is available, try to reinitialize
	if s.k8sClient == nil {
		slog.Info("k8s client is nil in vibespace service, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			slog.Warn("kubernetes client not available, returning empty list",
				"error", err)
			return []*model.Vibespace{}, nil
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully in vibespace service")

		// Also reinitialize Knative and IngressRoute managers
		knativeMgr, err := knative.NewServiceManager(newClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager after k8s client init",
				"error", err)
		} else {
			s.knativeManager = knativeMgr
			slog.Info("knative manager initialized successfully")
		}

		ingressMgr, err := network.NewIngressRouteManager(newClient, getBaseDomain())
		if err != nil {
			slog.Warn("failed to initialize IngressRoute manager after k8s client init",
				"error", err)
		} else {
			s.ingressRouteManager = ingressMgr
			slog.Info("ingress route manager initialized successfully")
		}
	}

	// MODE 1: List Knative Services (default)
	if isKnativeRoutingEnabled() {
		// Check if knativeManager is still nil after initialization
		if s.knativeManager == nil {
			slog.Warn("knative manager is nil, returning empty list")
			return []*model.Vibespace{}, nil
		}

		services, err := s.knativeManager.ListServices(ctx)
		if err != nil {
			slog.Error("failed to list Knative Services",
				"error", err)
			return nil, fmt.Errorf("failed to list Knative Services: %w", err)
		}

		vibespaces := make([]*model.Vibespace, 0, len(services.Items))
		for i := range services.Items {
			vibespace := knativeServiceToVibespace(&services.Items[i])
			vibespaces = append(vibespaces, vibespace)
		}

		return vibespaces, nil
	}

	// MODE 2: List Pods (legacy fallback)
	pods, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=vibespace",
	})
	if err != nil {
		slog.Error("failed to list vibespace pods",
			"error", err)
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	vibespaces := make([]*model.Vibespace, 0, len(pods.Items))
	for _, pod := range pods.Items {
		vibespace := podToVibespace(&pod)
		vibespaces = append(vibespaces, vibespace)
	}

	return vibespaces, nil
}

// Get returns a vibespace by ID
func (s *Service) Get(ctx context.Context, id string) (*model.Vibespace, error) {
	// Check if k8s client is available, try to reinitialize
	if s.k8sClient == nil {
		slog.Info("k8s client is nil in vibespace service, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully in vibespace service")

		// Also reinitialize Knative and IngressRoute managers
		knativeMgr, err := knative.NewServiceManager(newClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager after k8s client init", "error", err)
		} else {
			s.knativeManager = knativeMgr
			slog.Info("knative manager initialized successfully")
		}

		ingressMgr, err := network.NewIngressRouteManager(newClient, getBaseDomain())
		if err != nil {
			slog.Warn("failed to initialize IngressRoute manager after k8s client init", "error", err)
		} else {
			s.ingressRouteManager = ingressMgr
			slog.Info("ingress route manager initialized successfully")
		}
	}

	slog.Info("getting vibespace",
		"vibespace_id", id)

	// MODE 1: Get Knative Service (default)
	if isKnativeRoutingEnabled() {
		// Check if knativeManager is still nil after initialization
		if s.knativeManager == nil {
			return nil, fmt.Errorf("knative manager is not initialized")
		}

		service, err := s.knativeManager.GetService(ctx, id)
		if err != nil {
			slog.Warn("vibespace not found (Knative Service)",
				"vibespace_id", id,
				"error", err)
			return nil, fmt.Errorf("vibespace not found: %w", err)
		}

		vibespace := knativeServiceToVibespace(service)

		slog.Info("got vibespace successfully",
			"vibespace_id", id,
			"status", vibespace.Status)

		return vibespace, nil
	}

	// MODE 2: Get Pod (legacy fallback)
	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("getting vibespace pod",
		"vibespace_id", id,
		"pod_name", podName)

	pod, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		slog.Warn("vibespace not found",
			"vibespace_id", id,
			"pod_name", podName,
			"error", err)
		return nil, fmt.Errorf("vibespace not found: %w", err)
	}

	slog.Info("got vibespace successfully",
		"vibespace_id", id,
		"status", pod.Status.Phase)

	return podToVibespace(pod), nil
}

// Create creates a new vibespace
func (s *Service) Create(ctx context.Context, req *model.CreateVibespaceRequest) (*model.Vibespace, error) {
	// Check if k8s client is available, try to reinitialize
	if s.k8sClient == nil {
		slog.Info("k8s client is nil in vibespace service, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully in vibespace service")

		// Also reinitialize Knative and IngressRoute managers
		knativeMgr, err := knative.NewServiceManager(newClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager after k8s client init", "error", err)
		} else {
			s.knativeManager = knativeMgr
			slog.Info("knative manager initialized successfully")
		}

		ingressMgr, err := network.NewIngressRouteManager(newClient, getBaseDomain())
		if err != nil {
			slog.Warn("failed to initialize IngressRoute manager after k8s client init", "error", err)
		} else {
			s.ingressRouteManager = ingressMgr
			slog.Info("ingress route manager initialized successfully")
		}
	}

	slog.Info("creating vibespace",
		"name", req.Name,
		"template", req.Template,
		"agent", req.Agent,
		"github_repo", req.GithubRepo,
		"persistent", req.Persistent)

	// Ensure namespace exists
	if err := s.k8sClient.EnsureNamespace(ctx); err != nil {
		slog.Error("failed to ensure namespace",
			"error", err,
			"namespace", k8s.VibespaceNamespace)
		return nil, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	id := uuid.New().String()[:8] // Short ID
	pvcName := fmt.Sprintf("vibespace-%s-pvc", id)

	// Generate unique project name for DNS routing
	existingVibespaces, _ := s.List(ctx)
	existingNames := make([]string, 0, len(existingVibespaces))
	for _, vs := range existingVibespaces {
		if vs.ProjectName != "" {
			existingNames = append(existingNames, vs.ProjectName)
		}
	}
	projectName := model.GenerateUniqueProjectName(existingNames)

	// Allocate ports for multi-process container
	ports := model.AllocatePorts(8080) // Base port 8080

	slog.Debug("generated vibespace id and project name",
		"vibespace_id", id,
		"project_name", projectName,
		"pvc_name", pvcName,
		"ports", ports)

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
		slog.Info("pvc created successfully",
			"vibespace_id", id,
			"pvc_name", pvcName)
	}

	// Build init containers
	initContainers := []corev1.Container{}

	// Always add init container to fix vibespace directory ownership
	// This ensures the vibespace user (UID 1001) can write to /vibespace
	if req.Persistent {
		initContainers = append(initContainers, corev1.Container{
			Name:  "fix-permissions",
			Image: "busybox:latest",
			Command: []string{"sh", "-c"},
			Args: []string{
				"chown -R 1001:1001 /vibespace && chmod -R 755 /vibespace",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vibespace-data",
					MountPath: "/vibespace",
				},
			},
		})
	}

	// Add git clone init container if needed
	if req.GithubRepo != "" {
		// Validate GitHub repo URL to prevent command injection
		if !isValidGitURL(req.GithubRepo) {
			return nil, fmt.Errorf("invalid GitHub repository URL: must be a valid HTTPS Git URL")
		}

		initContainers = append(initContainers, corev1.Container{
			Name:  "git-clone",
			Image: "alpine/git:latest",
			Command: []string{"sh", "-c"},
			Args: []string{
				fmt.Sprintf("git clone %s /vibespace/repo || echo 'Failed to clone repository'", req.GithubRepo),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vibespace-data",
					MountPath: "/vibespace",
				},
			},
		})
	}

	// Build volumes
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	if req.Persistent {
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

	// Build environment variables (including agent config)
	env := envMapToEnvVars(req.Env)
	if req.Agent != "" {
		env = append(env, corev1.EnvVar{
			Name:  "VIBESPACE_AGENT",
			Value: req.Agent,
		})
	}

	// Determine image to use from local registry
	// Image naming: vibespace-{template}-{agent}:latest (e.g., vibespace-nextjs-claude:latest)
	// Default to "claude" if no agent specified
	agent := req.Agent
	if agent == "" {
		agent = "claude"
	}
	// Use host.docker.internal to access registry from Docker containers in Colima
	// Colima runs Docker in Lima VM - host.docker.internal resolves to the host machine
	// Registry is exposed as NodePort 30500 on host, accessible via host.docker.internal:30500
	// Configured in ~/.colima/default/colima.yaml as insecure-registry
	vibespaceImage := fmt.Sprintf("host.docker.internal:30500/vibespace-%s-%s:latest", req.Template, agent)

	// MODE 1: Create Knative Service + IngressRoutes (default)
	if isKnativeRoutingEnabled() {
		// Check if knativeManager is still nil after initialization
		if s.knativeManager == nil {
			return nil, fmt.Errorf("knative manager is not initialized")
		}

		slog.Info("creating vibespace with Knative Service",
			"vibespace_id", id,
			"project_name", projectName,
			"image", vibespaceImage)

		// Create Knative Service
		err := s.knativeManager.CreateService(ctx, &knative.CreateServiceRequest{
			VibespaceID: id,
			Name:        req.Name,
			ProjectName: projectName,
			Template:    req.Template,
			Agent:       agent,
			Image:       vibespaceImage,
			Ports:       ports,
			Resources:   *resources,
			Env:         req.Env,
			Persistent:  req.Persistent,
			PVCName:     pvcName,
			GithubRepo:  req.GithubRepo,
		})
		if err != nil {
			slog.Error("failed to create Knative Service",
				"vibespace_id", id,
				"error", err)
			return nil, fmt.Errorf("failed to create Knative Service: %w", err)
		}

		slog.Info("Knative Service created, now creating IngressRoutes",
			"vibespace_id", id,
			"project_name", projectName)

		// Create IngressRoutes for DNS routing
		if isDnsEnabled() {
			if s.ingressRouteManager == nil {
				slog.Error("ingress route manager is not initialized, cannot create IngressRoutes",
					"vibespace_id", id)
				s.knativeManager.DeleteService(ctx, id)
				return nil, fmt.Errorf("ingress route manager is not initialized")
			}

			err = s.ingressRouteManager.CreateIngressRoutes(ctx, &network.CreateIngressRoutesRequest{
				VibespaceID: id,
				ProjectName: projectName,
				Namespace:   k8s.VibespaceNamespace,
			})
			if err != nil {
				// Rollback: delete Knative Service
				slog.Error("failed to create IngressRoutes, rolling back Knative Service",
					"vibespace_id", id,
					"error", err)
				s.knativeManager.DeleteService(ctx, id)
				return nil, fmt.Errorf("failed to create IngressRoutes: %w", err)
			}

			slog.Info("IngressRoutes created successfully",
				"vibespace_id", id,
				"project_name", projectName)
		}

		// Build response with URLs
		urls := model.GenerateURLs(projectName)

		vibespace := &model.Vibespace{
			ID:          id,
			Name:        req.Name,
			ProjectName: projectName,
			Template:    req.Template,
			Status:      "creating",
			Resources:   *resources,
			Ports:       ports,
			URLs:        urls,
			Persistent:  req.Persistent,
			CreatedAt:   time.Now().Format(time.RFC3339),
		}

		slog.Info("vibespace created successfully",
			"vibespace_id", id,
			"project_name", projectName,
			"template", req.Template,
			"agent", req.Agent,
			"urls", urls)

		return vibespace, nil
	}

	// MODE 2: Create Pod (legacy fallback mode)
	slog.Info("creating vibespace with Pod (legacy mode)",
		"vibespace_id", id)

	// Configure container ports
	containerPorts := []corev1.ContainerPort{
		{
			Name:          "code-server",
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Add template-specific preview ports
	switch req.Template {
	case "nextjs":
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          "preview",
			ContainerPort: 3000,
			Protocol:      corev1.ProtocolTCP,
		})
	case "vue":
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          "preview",
			ContainerPort: 5173,
			Protocol:      corev1.ProtocolTCP,
		})
	case "jupyter":
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          "jupyter",
			ContainerPort: 8888,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	podName := fmt.Sprintf("vibespace-%s", id)

	// Create pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: k8s.VibespaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       req.Name,
				"app.kubernetes.io/managed-by": "vibespace",
				"vibespace.dev/id":             id,
				"vibespace.dev/template":       req.Template,
			},
			Annotations: map[string]string{
				"vibespace.dev/github-repo": req.GithubRepo,
				"vibespace.dev/agent":       req.Agent,
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: int64Ptr(1001), // Match vibespace user GID
			},
			InitContainers: initContainers,
			Containers: []corev1.Container{
				{
					Name:         "code-server",
					Image:        vibespaceImage,
					Ports:        containerPorts,
					Env:          env,
					VolumeMounts: volumeMounts,
					// Resources will be added later
				},
			},
			Volumes:       volumes,
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}

	slog.Info("creating vibespace pod",
		"vibespace_id", id,
		"pod_name", podName,
		"image", vibespaceImage,
		"init_containers", len(initContainers))

	created, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		slog.Error("failed to create vibespace pod",
			"vibespace_id", id,
			"pod_name", podName,
			"error", err)
		return nil, fmt.Errorf("failed to create vibespace pod: %w", err)
	}

	slog.Info("vibespace created successfully (legacy mode)",
		"vibespace_id", id,
		"pod_name", podName,
		"status", created.Status.Phase,
		"template", req.Template,
		"agent", req.Agent)

	return podToVibespace(created), nil
}

// Delete deletes a vibespace
// Phase 1 (MVP): Deletes the pod only
// Phase 2 (Knative): Will delete Knative Service which handles all resources
func (s *Service) Delete(ctx context.Context, id string) error {
	// Check if k8s client is available, try to reinitialize
	if s.k8sClient == nil {
		slog.Info("k8s client is nil in vibespace service, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully in vibespace service")

		// Also reinitialize Knative and IngressRoute managers
		knativeMgr, err := knative.NewServiceManager(newClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager after k8s client init", "error", err)
		} else {
			s.knativeManager = knativeMgr
			slog.Info("knative manager initialized successfully")
		}

		ingressMgr, err := network.NewIngressRouteManager(newClient, getBaseDomain())
		if err != nil {
			slog.Warn("failed to initialize IngressRoute manager after k8s client init", "error", err)
		} else {
			s.ingressRouteManager = ingressMgr
			slog.Info("ingress route manager initialized successfully")
		}
	}

	slog.Info("deleting vibespace",
		"vibespace_id", id)

	// MODE 1: Delete Knative Service + IngressRoutes (default)
	if isKnativeRoutingEnabled() {
		// Check if knativeManager is still nil after initialization
		if s.knativeManager == nil {
			return fmt.Errorf("knative manager is not initialized")
		}

		// Delete IngressRoutes first (DNS routing)
		if isDnsEnabled() {
			if s.ingressRouteManager != nil {
				if err := s.ingressRouteManager.DeleteIngressRoutes(ctx, id, k8s.VibespaceNamespace); err != nil {
					slog.Warn("failed to delete IngressRoutes",
						"vibespace_id", id,
						"error", err)
					// Continue with Knative Service deletion even if IngressRoute deletion fails
				}
			} else {
				slog.Warn("ingress route manager is not initialized, skipping IngressRoute deletion",
					"vibespace_id", id)
			}
		}

		// Delete Knative Service
		if err := s.knativeManager.DeleteService(ctx, id); err != nil {
			return fmt.Errorf("failed to delete Knative Service: %w", err)
		}

		slog.Info("vibespace deleted successfully",
			"vibespace_id", id)

		// Note: PVCs are left for manual cleanup to prevent accidental data loss
		return nil
	}

	// MODE 2: Delete Pod (legacy fallback)
	podName := fmt.Sprintf("vibespace-%s", id)

	// Stop any active port-forward for this vibespace before deleting
	// This prevents orphaned kubectl processes and memory leaks
	if err := s.k8sClient.StopPortForward(k8s.VibespaceNamespace, podName); err != nil {
		// Log but don't fail deletion - port-forward may not exist
		slog.Warn("failed to stop port-forward during deletion",
			"vibespace_id", id,
			"pod_name", podName,
			"error", err)
	}

	err := s.k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete vibespace pod: %w", err)
	}

	slog.Info("vibespace pod deleted successfully",
		"vibespace_id", id,
		"pod_name", podName)

	// Note: PVCs are left for manual cleanup to prevent accidental data loss
	return nil
}

// Start starts a stopped vibespace (recreates the pod)
// Phase 1 (MVP): Limited implementation - pod recreation not yet supported
// Phase 2 (Knative): Will use Knative scale-from-zero (minScale=0 -> minScale=1)
func (s *Service) Start(ctx context.Context, id string) error {
	// Check if k8s client is available, try to reinitialize
	if s.k8sClient == nil {
		slog.Info("k8s client is nil in vibespace service, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully in vibespace service")

		// Also reinitialize Knative and IngressRoute managers
		knativeMgr, err := knative.NewServiceManager(newClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager after k8s client init", "error", err)
		} else {
			s.knativeManager = knativeMgr
			slog.Info("knative manager initialized successfully")
		}

		ingressMgr, err := network.NewIngressRouteManager(newClient, getBaseDomain())
		if err != nil {
			slog.Warn("failed to initialize IngressRoute manager after k8s client init", "error", err)
		} else {
			s.ingressRouteManager = ingressMgr
			slog.Info("ingress route manager initialized successfully")
		}
	}

	slog.Info("starting vibespace",
		"vibespace_id", id)

	// MODE 1: Patch Knative Service to scale up (default)
	if isKnativeRoutingEnabled() {
		// Check if knativeManager is still nil after initialization
		if s.knativeManager == nil {
			return fmt.Errorf("knative manager is not initialized")
		}

		// Set minScale=1 to ensure at least 1 replica is running
		annotations := map[string]string{
			"autoscaling.knative.dev/minScale": "1",
		}

		err := s.knativeManager.PatchService(ctx, id, annotations)
		if err != nil {
			slog.Error("failed to start vibespace (Knative)",
				"vibespace_id", id,
				"error", err)
			return fmt.Errorf("failed to start vibespace: %w", err)
		}

		slog.Info("vibespace started successfully",
			"vibespace_id", id,
			"minScale", "1")

		return nil
	}

	// MODE 2: Check Pod status (legacy fallback)
	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("checking vibespace pod status",
		"vibespace_id", id,
		"pod_name", podName)

	_, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		// Pod already exists, nothing to do
		slog.Info("vibespace already running",
			"vibespace_id", id,
			"pod_name", podName)
		return nil
	}

	// Limitation: Cannot restart a stopped vibespace in legacy Pod mode
	slog.Warn("cannot restart stopped vibespace in legacy Pod mode",
		"vibespace_id", id,
		"pod_name", podName)
	return nil
}

// Stop stops a running vibespace
// Phase 1 (MVP): Deletes the pod while preserving PVC (data remains intact)
// Phase 2 (Knative): Will use Knative scale-to-zero (minScale=1 -> minScale=0)
func (s *Service) Stop(ctx context.Context, id string) error {
	// Check if k8s client is available, try to reinitialize
	if s.k8sClient == nil {
		slog.Info("k8s client is nil in vibespace service, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			return fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully in vibespace service")

		// Also reinitialize Knative and IngressRoute managers
		knativeMgr, err := knative.NewServiceManager(newClient)
		if err != nil {
			slog.Warn("failed to initialize Knative Service manager after k8s client init", "error", err)
		} else {
			s.knativeManager = knativeMgr
			slog.Info("knative manager initialized successfully")
		}

		ingressMgr, err := network.NewIngressRouteManager(newClient, getBaseDomain())
		if err != nil {
			slog.Warn("failed to initialize IngressRoute manager after k8s client init", "error", err)
		} else {
			s.ingressRouteManager = ingressMgr
			slog.Info("ingress route manager initialized successfully")
		}
	}

	slog.Info("stopping vibespace",
		"vibespace_id", id)

	// MODE 1: Patch Knative Service to scale to zero (default)
	if isKnativeRoutingEnabled() {
		// Check if knativeManager is still nil after initialization
		if s.knativeManager == nil {
			return fmt.Errorf("knative manager is not initialized")
		}

		// Set minScale=0 to allow scaling to zero replicas
		annotations := map[string]string{
			"autoscaling.knative.dev/minScale": "0",
		}

		err := s.knativeManager.PatchService(ctx, id, annotations)
		if err != nil {
			slog.Error("failed to stop vibespace (Knative)",
				"vibespace_id", id,
				"error", err)
			return fmt.Errorf("failed to stop vibespace: %w", err)
		}

		slog.Info("vibespace stopped successfully",
			"vibespace_id", id,
			"minScale", "0")

		// Note: PVC remains attached, data is preserved
		return nil
	}

	// MODE 2: Delete Pod (legacy fallback)
	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("stopping vibespace pod",
		"vibespace_id", id,
		"pod_name", podName)

	// Stop any active port-forward for this vibespace
	if err := s.k8sClient.StopPortForward(k8s.VibespaceNamespace, podName); err != nil {
		slog.Warn("failed to stop port-forward during stop",
			"vibespace_id", id,
			"pod_name", podName,
			"error", err)
	}

	// Delete the pod (PVC remains, preserving vibespace data)
	err := s.k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Pod already deleted/stopped
			slog.Info("vibespace already stopped",
				"vibespace_id", id,
				"pod_name", podName)
			return nil
		}
		slog.Error("failed to stop vibespace",
			"vibespace_id", id,
			"pod_name", podName,
			"error", err)
		return fmt.Errorf("failed to stop vibespace: %w", err)
	}

	slog.Info("vibespace pod stopped successfully",
		"vibespace_id", id,
		"pod_name", podName)

	// Note: PVC remains, preserving vibespace data
	return nil
}

// isKnativeRoutingEnabled checks if Knative routing is enabled via environment variable
// Returns true if ENABLE_KNATIVE_ROUTING=true, false otherwise (default: true for MVP)
func isKnativeRoutingEnabled() bool {
	enabled := os.Getenv("ENABLE_KNATIVE_ROUTING")
	// Default to true if not set (Knative routing is the default in MVP)
	if enabled == "" {
		return true
	}
	return enabled == "true" || enabled == "1"
}

// isDnsEnabled checks if custom DNS resolution is enabled via environment variable
// Returns true if ENABLE_DNS=true, false otherwise (default: true for MVP)
func isDnsEnabled() bool {
	enabled := os.Getenv("ENABLE_DNS")
	// Default to true if not set (DNS is the default in MVP)
	if enabled == "" {
		return true
	}
	return enabled == "true" || enabled == "1"
}

// getBaseDomain returns the base domain for vibespace subdomains
// Default: vibe.space
func getBaseDomain() string {
	domain := os.Getenv("DNS_BASE_DOMAIN")
	if domain == "" {
		return "vibe.space"
	}
	return domain
}

// Access makes a vibespace accessible
// Returns a map of URLs where the vibespace can be accessed
//
// Routing modes:
// 1. Knative + DNS (default): Returns DNS subdomain URLs (code.{project}.vibe.space)
// 2. Port-forward (fallback): Returns localhost URLs with port-forwarding
func (s *Service) Access(ctx context.Context, id string) (map[string]string, error) {
	// Check if k8s client is available, try to reinitialize
	if s.k8sClient == nil {
		slog.Info("k8s client is nil in vibespace service, attempting to initialize")
		newClient, err := k8s.NewClient()
		if err != nil {
			return nil, fmt.Errorf("kubernetes is not available - please install and start Kubernetes first")
		}
		s.k8sClient = newClient
		slog.Info("k8s client initialized successfully in vibespace service")
	}

	slog.Info("starting vibespace access",
		"vibespace_id", id,
		"knative_routing", isKnativeRoutingEnabled(),
		"dns_enabled", isDnsEnabled())

	// MODE 1: Knative + DNS routing (default)
	if isKnativeRoutingEnabled() && isDnsEnabled() {
		return s.accessViaDNS(ctx, id)
	}

	// MODE 2: Port-forward fallback (backward compatibility)
	return s.accessViaPortForward(ctx, id)
}

// accessViaDNS returns DNS subdomain URLs for a vibespace
// Used when Knative routing + DNS are enabled (default mode)
func (s *Service) accessViaDNS(ctx context.Context, id string) (map[string]string, error) {
	// Get vibespace to extract ProjectName
	vibespace, err := s.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get vibespace: %w", err)
	}

	if vibespace.ProjectName == "" {
		return nil, fmt.Errorf("vibespace has no project name (required for DNS routing)")
	}

	// Create IngressRoute manager
	ingressMgr, err := network.NewIngressRouteManager(s.k8sClient, getBaseDomain())
	if err != nil {
		slog.Warn("failed to create IngressRoute manager, falling back to port-forward",
			"vibespace_id", id,
			"error", err)
		return s.accessViaPortForward(ctx, id)
	}

	// Get DNS URLs
	urls := ingressMgr.GetVibespaceURLs(vibespace.ProjectName)

	slog.Info("vibespace access via DNS completed",
		"vibespace_id", id,
		"project_name", vibespace.ProjectName,
		"urls_count", len(urls),
		"urls", urls)

	return urls, nil
}

// accessViaPortForward returns localhost URLs with port-forwarding
// Used when Knative routing or DNS is disabled (backward compatibility mode)
func (s *Service) accessViaPortForward(ctx context.Context, id string) (map[string]string, error) {
	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("using port-forward access mode",
		"vibespace_id", id,
		"pod_name", podName)

	// Verify pod exists and is running
	pod, err := s.k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("vibespace not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get vibespace: %w", err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("vibespace is not running (status: %s)", pod.Status.Phase)
	}

	// Get template from pod labels
	template := pod.Labels["vibespace.dev/template"]

	slog.Info("pod verified and running",
		"vibespace_id", id,
		"pod_status", pod.Status.Phase,
		"template", template)

	urls := make(map[string]string)

	// Forward code-server port (8080)
	codeServerPort, err := s.findAndForwardPort(ctx, podName, id, 8080, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to forward code-server port: %w", err)
	}
	urls["code-server"] = fmt.Sprintf("http://127.0.0.1:%d", codeServerPort)

	slog.Info("code-server port-forward established",
		"vibespace_id", id,
		"local_port", codeServerPort,
		"remote_port", 8080,
		"url", urls["code-server"])

	// Forward preview port based on template
	var previewPort int
	var previewName string
	switch template {
	case "nextjs":
		previewPort = 3000
		previewName = "preview"
	case "vue":
		previewPort = 5173
		previewName = "preview"
	case "jupyter":
		previewPort = 8888
		previewName = "jupyter"
	}

	if previewPort > 0 {
		localPreviewPort, err := s.findAndForwardPort(ctx, podName, id, previewPort, 1)
		if err != nil {
			// Log warning but don't fail - code-server is already accessible
			slog.Warn("failed to forward preview port",
				"vibespace_id", id,
				"preview_name", previewName,
				"remote_port", previewPort,
				"error", err)
		} else {
			urls[previewName] = fmt.Sprintf("http://127.0.0.1:%d", localPreviewPort)
			slog.Info("preview port-forward established",
				"vibespace_id", id,
				"preview_name", previewName,
				"local_port", localPreviewPort,
				"remote_port", previewPort,
				"url", urls[previewName])
		}
	}

	slog.Info("vibespace access completed via port-forward",
		"vibespace_id", id,
		"urls_count", len(urls),
		"urls", urls)

	return urls, nil
}

// findAndForwardPort finds an available local port and starts a port-forward
// offset is added to the base port calculation to avoid collisions when forwarding multiple ports
func (s *Service) findAndForwardPort(ctx context.Context, podName, vibespaceID string, remotePort, offset int) (int, error) {
	// Find an available local port
	// Start with deterministic port based on vibespace ID + offset
	// Port range: 8080-9079 (1000 ports)
	// With offset multiplier of 100, supports ~10 different ports per vibespace
	basePort := PortRangeStart + hashStringToPort(vibespaceID) + offset*100

	// Ensure basePort is within valid range
	if basePort > PortRangeEnd {
		basePort = PortRangeStart + (basePort % (PortRangeEnd - PortRangeStart + 1))
	}

	localPort := 0

	for i := 0; i < MaxPortTries; i++ {
		candidatePort := basePort + i
		if candidatePort > PortRangeEnd {
			// Wrap around to start of range
			candidatePort = PortRangeStart + (candidatePort - PortRangeEnd - 1)
		}

		if isPortAvailable(candidatePort) {
			localPort = candidatePort
			break
		}
	}

	if localPort == 0 {
		return 0, fmt.Errorf("no available ports found in range %d-%d", PortRangeStart, PortRangeEnd)
	}

	// Start port-forward
	err := s.k8sClient.StartPortForwardToPod(ctx, k8s.VibespaceNamespace, podName, localPort, remotePort)
	if err != nil {
		return 0, fmt.Errorf("failed to start port-forward: %w", err)
	}

	return localPort, nil
}

// Helper functions

// createPVC creates a PersistentVolumeClaim for vibespace storage
func (s *Service) createPVC(ctx context.Context, name string, storage string) error {
	// k8s client check is performed in the calling function (Create)
	// so we don't need to check again here
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

func podToVibespace(pod *corev1.Pod) *model.Vibespace {
	status := "unknown"
	switch pod.Status.Phase {
	case corev1.PodPending:
		status = "creating"
	case corev1.PodRunning:
		status = "running"
	case corev1.PodSucceeded, corev1.PodFailed:
		status = "stopped"
	}

	id := pod.Labels["vibespace.dev/id"]
	template := pod.Labels["vibespace.dev/template"]

	// Build URLs based on template
	// All templates expose code-server on port 8080
	urls := map[string]string{
		"code-server": fmt.Sprintf("http://vibespace-%s.local", id),
	}

	// Add template-specific URLs
	switch template {
	case "nextjs":
		urls["preview"] = fmt.Sprintf("http://vibespace-%s-3000.local", id)
	case "vue":
		urls["preview"] = fmt.Sprintf("http://vibespace-%s-5173.local", id)
	case "jupyter":
		urls["jupyter"] = fmt.Sprintf("http://vibespace-%s-8888.local", id)
	}

	return &model.Vibespace{
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

// knativeServiceToVibespace converts a Knative Service to Vibespace model
func knativeServiceToVibespace(svc *unstructured.Unstructured) *model.Vibespace {
	// Extract metadata
	metadata, _ := svc.Object["metadata"].(map[string]interface{})
	labels, _ := metadata["labels"].(map[string]interface{})
	annotations, _ := metadata["annotations"].(map[string]interface{})

	// Extract labels with type conversion
	id, _ := labels["vibespace.dev/id"].(string)
	name, _ := labels["app.kubernetes.io/name"].(string)
	template, _ := labels["vibespace.dev/template"].(string)
	projectName, _ := labels["vibespace.dev/project-name"].(string)
	createdAt, _ := annotations["vibespace.dev/created-at"].(string)

	// Extract status from Knative Service
	status := knativeStatusToVibespaceStatus(svc)

	// Extract ports from container spec
	ports := extractPortsFromKnativeService(svc)

	// Generate URLs from project name
	urls := model.GenerateURLs(projectName)

	return &model.Vibespace{
		ID:          id,
		Name:        name,
		ProjectName: projectName,
		Template:    template,
		Status:      status,
		Resources: model.Resources{
			CPU:     "1",
			Memory:  "2Gi",
			Storage: "10Gi",
		},
		Ports:      ports,
		URLs:       urls,
		Persistent: true, // TODO: Detect from volumes
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

	// Check Ready condition
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
			// Check if scaled to zero
			reason, _, _ := unstructured.NestedString(condition, "reason")
			if reason == "NoTraffic" {
				return "stopped"
			}
			return "creating"
		}
	}

	return "creating"
}

// extractPortsFromKnativeService returns external-facing port allocations for a Knative Service.
//
// Single-Port Architecture (Knative + Caddy):
// In Knative mode, the container spec only exposes port 8080 (Caddy reverse proxy).
// Caddy handles internal routing to:
// - code-server: localhost:8081
// - preview server: localhost:3000
// - production server: localhost:3001
//
// This function returns external-facing ports (all 8080) for API responses.
// The actual container port extraction is skipped because Knative Service only exposes one port.
// Frontend should use vibespace.urls (DNS URLs) instead of constructing URLs from ports.
// See ADR 0009 for architectural rationale.
func extractPortsFromKnativeService(svc *unstructured.Unstructured) model.Ports {
	// In Caddy mode, Knative Service only exposes port 8080 externally
	// Return external-facing ports (all 8080)
	// Internal routing (8081, 3000, 3001) is handled by Caddy
	return model.Ports{
		Code:    8080, // External port (Caddy listens on 8080)
		Preview: 8080, // External port (Caddy routes internally to 3000)
		Prod:    8080, // External port (Caddy routes internally to 3001)
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

// int64Ptr returns a pointer to an int64
func int64Ptr(i int64) *int64 {
	return &i
}

// hashStringToPort converts a vibespace ID to a consistent port offset (0-999)
// This ensures the same vibespace always gets the same local port
func hashStringToPort(id string) int {
	hash := 0
	for _, c := range id {
		hash = (hash*31 + int(c)) % 1000
	}
	return hash
}

// isPortAvailable checks if a port is available for binding
// Returns true if the port can be bound to, false otherwise
func isPortAvailable(port int) bool {
	// Try to bind to the port - if successful, it's available
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// isValidGitURL validates a Git repository URL to prevent command injection
// Accepts HTTPS URLs for GitHub, GitLab, Bitbucket, and SSH URLs
func isValidGitURL(url string) bool {
	// Check for empty or suspiciously short URLs
	if len(url) < 10 {
		return false
	}

	// Check for shell metacharacters that could be used for injection
	dangerousChars := []string{";", "|", "&", "$", "`", "\n", "\r", "$(", "&&", "||"}
	for _, char := range dangerousChars {
		if strings.Contains(url, char) {
			return false
		}
	}

	// Validate URL format - must be HTTPS or SSH
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "git@")
}

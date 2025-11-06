package vibespace

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"vibespace/pkg/k8s"
	"vibespace/pkg/model"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
	k8sClient *k8s.Client
}

// NewService creates a new vibespace service
func NewService(k8sClient *k8s.Client) *Service {
	return &Service{
		k8sClient: k8sClient,
	}
}

// List returns all vibespaces
func (s *Service) List(ctx context.Context) ([]*model.Vibespace, error) {
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
	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("getting vibespace",
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
	podName := fmt.Sprintf("vibespace-%s", id)
	pvcName := fmt.Sprintf("vibespace-%s-pvc", id)

	slog.Debug("generated vibespace id",
		"vibespace_id", id,
		"pod_name", podName,
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
	// Use NodePort registry (accessible from node's localhost)
	// Registry is exposed on NodePort 30500, accessible at localhost:30500 from within k3d nodes
	vibespaceImage := fmt.Sprintf("localhost:30500/vibespace-%s-%s:latest", req.Template, agent)

	// Configure ports based on template
	// All templates expose code-server on port 8080
	// Templates also expose their development server ports for preview
	ports := []corev1.ContainerPort{
		{
			Name:          "code-server",
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Add template-specific preview ports
	switch req.Template {
	case "nextjs":
		ports = append(ports, corev1.ContainerPort{
			Name:          "preview",
			ContainerPort: 3000,
			Protocol:      corev1.ProtocolTCP,
		})
	case "vue":
		ports = append(ports, corev1.ContainerPort{
			Name:          "preview",
			ContainerPort: 5173,
			Protocol:      corev1.ProtocolTCP,
		})
	case "jupyter":
		ports = append(ports, corev1.ContainerPort{
			Name:          "jupyter",
			ContainerPort: 8888,
			Protocol:      corev1.ProtocolTCP,
		})
	}

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
					Ports:        ports,
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

	slog.Info("vibespace created successfully",
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
	// MVP Implementation: Delete pod
	// Note: PVCs are left for manual cleanup to prevent accidental data loss
	//
	// Future Knative Implementation:
	// - Delete Knative Service instead of Pod
	// - Knative will handle cleanup of all related resources
	// - PVC deletion policy will be configurable per vibespace
	// - kubectl delete ksvc vibespace-{id} -n vibespace

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

	return nil
}

// Start starts a stopped vibespace (recreates the pod)
// Phase 1 (MVP): Limited implementation - pod recreation not yet supported
// Phase 2 (Knative): Will use Knative scale-from-zero (minScale=0 -> minScale=1)
func (s *Service) Start(ctx context.Context, id string) error {
	// MVP Phase 1 Implementation: Check if pod exists
	// If pod exists, it's already running (no-op)
	// If pod doesn't exist, we can't recreate it (no metadata storage in Phase 1)
	//
	// MVP Phase 2 (Knative migration):
	// - Knative Services automatically scale from zero on first request
	// - Start operation will patch the Knative Service to set minScale=1
	// - No need to manually recreate pods or store metadata

	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("starting vibespace",
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

	// Limitation: Cannot restart a stopped vibespace in MVP Phase 1
	// This will be resolved in MVP Phase 2 with Knative migration
	slog.Warn("cannot restart stopped vibespace in MVP Phase 1",
		"vibespace_id", id,
		"pod_name", podName)
	return nil
}

// Stop stops a running vibespace
// Phase 1 (MVP): Deletes the pod while preserving PVC (data remains intact)
// Phase 2 (Knative): Will use Knative scale-to-zero (minScale=1 -> minScale=0)
func (s *Service) Stop(ctx context.Context, id string) error {
	// MVP Implementation: Delete pod to stop it, PVC remains for data persistence
	// Note: Without metadata storage, we can't automatically restart stopped vibespaces
	// This will be addressed in Phase 1.5 or Phase 2 with Knative
	//
	// Future Knative Implementation:
	// - Patch Knative Service to set minScale=0 annotation
	// - Knative will gracefully scale down to zero replicas after idle timeout
	// - On next request, Knative auto-scales back up with the same PVC attached
	// - Provides true scale-to-zero without manual pod deletion
	// - kubectl patch ksvc vibespace-{id} -n vibespace -p '{"spec":{"template":{"metadata":{"annotations":{"autoscaling.knative.dev/min-scale":"0"}}}}}'

	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("stopping vibespace",
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

	slog.Info("vibespace stopped successfully",
		"vibespace_id", id,
		"pod_name", podName)

	return nil
}

// Access makes a vibespace accessible by starting port-forwards
// Returns a map of local URLs where the vibespace can be accessed
func (s *Service) Access(ctx context.Context, id string) (map[string]string, error) {
	podName := fmt.Sprintf("vibespace-%s", id)

	slog.Info("starting vibespace access",
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

	slog.Info("vibespace access completed successfully",
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

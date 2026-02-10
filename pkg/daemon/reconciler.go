package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/yagizdagabak/vibespace/pkg/k8s"
	"github.com/yagizdagabak/vibespace/pkg/portforward"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodInfo contains information about a running pod
type PodInfo struct {
	Name      string
	Agent     string // Agent display name (e.g., "claude-1", "codex-2")
	AgentType string // Agent type (e.g., "claude-code", "codex")
	Vibespace string
	Phase     corev1.PodPhase
}

// Reconciler reconciles desired state with actual pod state
type Reconciler struct {
	desiredMgr *DesiredStateManager
	state      *DaemonState
	manager    *portforward.Manager
	clientset  *kubernetes.Clientset
	mu         sync.Mutex
}

// ReconcilerConfig contains configuration for the reconciler
type ReconcilerConfig struct {
	DesiredMgr *DesiredStateManager
	State      *DaemonState
	Manager    *portforward.Manager
	Clientset  *kubernetes.Clientset
}

// NewReconciler creates a new reconciler
func NewReconciler(cfg ReconcilerConfig) *Reconciler {
	return &Reconciler{
		desiredMgr: cfg.DesiredMgr,
		state:      cfg.State,
		manager:    cfg.Manager,
		clientset:  cfg.Clientset,
	}
}

// Reconcile reconciles the desired state with the actual pod state
func (r *Reconciler) Reconcile(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Debug("starting reconciliation")

	// 1. List all running pods across all vibespaces
	pods, err := r.listAllVibespacesPods(ctx)
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// 2. Group pods by vibespace
	podsByVibespace := r.groupByVibespace(pods)

	// 3. Reconcile each vibespace
	for vibespace, vsPods := range podsByVibespace {
		r.reconcileVibespace(ctx, vibespace, vsPods)
	}

	// 4. Cleanup forwards for vibespaces with no pods
	r.cleanupEmptyVibespaces(podsByVibespace)

	slog.Debug("reconciliation complete")
	return nil
}

// ReconcileVibespace reconciles a single vibespace
func (r *Reconciler) ReconcileVibespace(ctx context.Context, vibespace string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	pods, err := r.listVibespacesPods(ctx, vibespace)
	if err != nil {
		return err
	}

	r.reconcileVibespace(ctx, vibespace, pods)
	return nil
}

// listAllVibespacesPods lists all pods with vibespace labels
func (r *Reconciler) listAllVibespacesPods(ctx context.Context) ([]PodInfo, error) {
	podList, err := r.clientset.CoreV1().Pods(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=vibespace",
	})
	if err != nil {
		return nil, err
	}

	return r.podListToInfo(podList), nil
}

// listVibespacesPods lists pods for a specific vibespace
func (r *Reconciler) listVibespacesPods(ctx context.Context, vibespace string) ([]PodInfo, error) {
	// We need to look up by name, not ID, since that's what we store
	// First get the vibespace state to find the ID
	vsState := r.state.GetVibespace(vibespace)
	if vsState == nil {
		// Try looking up by name in labels
		podList, err := r.clientset.CoreV1().Pods(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app.kubernetes.io/managed-by=vibespace,app.kubernetes.io/name=%s", vibespace),
		})
		if err != nil {
			return nil, err
		}
		return r.podListToInfo(podList), nil
	}

	// Look up by ID
	podList, err := r.clientset.CoreV1().Pods(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vsState.ID),
	})
	if err != nil {
		return nil, err
	}

	return r.podListToInfo(podList), nil
}

// podListToInfo converts a pod list to PodInfo slice
func (r *Reconciler) podListToInfo(podList *corev1.PodList) []PodInfo {
	var pods []PodInfo
	for _, pod := range podList.Items {
		labels := pod.Labels
		if labels == nil {
			continue
		}

		// Get vibespace name
		vibespace := labels["app.kubernetes.io/name"]
		if vibespace == "" {
			vibespace = labels["vibespace.dev/id"]
		}

		// Get agent type (default: claude-code)
		agentType := labels["vibespace.dev/agent-type"]
		if agentType == "" {
			agentType = "claude-code"
		}

		// Get agent name from label
		agentName := labels["vibespace.dev/agent-name"]
		if agentName == "" {
			// Fallback: derive from agent type and number
			agentNum := labels["vibespace.dev/agent-num"]
			if agentNum == "" {
				agentNum = "1"
			}
			if agentType == "claude-code" {
				agentName = fmt.Sprintf("claude-%s", agentNum)
			} else if agentType == "codex" {
				agentName = fmt.Sprintf("codex-%s", agentNum)
			} else {
				agentName = fmt.Sprintf("agent-%s", agentNum)
			}
		}

		pods = append(pods, PodInfo{
			Name:      pod.Name,
			Agent:     agentName,
			AgentType: agentType,
			Vibespace: vibespace,
			Phase:     pod.Status.Phase,
		})
	}
	return pods
}

// groupByVibespace groups pods by their vibespace
func (r *Reconciler) groupByVibespace(pods []PodInfo) map[string][]PodInfo {
	result := make(map[string][]PodInfo)
	for _, pod := range pods {
		result[pod.Vibespace] = append(result[pod.Vibespace], pod)
	}
	return result
}

// agentKey creates a unique key for an agent across vibespaces
func agentKey(vibespace, agentName string) string {
	return vibespace + "/" + agentName
}

// reconcileVibespace reconciles a single vibespace's forwards
func (r *Reconciler) reconcileVibespace(ctx context.Context, vibespace string, pods []PodInfo) {
	slog.Debug("reconciling vibespace", "vibespace", vibespace, "pod_count", len(pods))

	// Get or create vibespace state
	vsState := r.state.GetOrCreateVibespace(vibespace)

	// Build map of current running pods
	runningPods := make(map[string]string) // agent -> pod name
	for _, pod := range pods {
		if pod.Phase == corev1.PodRunning {
			runningPods[pod.Agent] = pod.Name
		}
	}

	// Get desired state for this vibespace
	desired := r.desiredMgr.GetOrCreate(vibespace)

	// For each agent with desired forwards, ensure they exist
	for agentName, forwards := range desired.Agents {
		podName, running := runningPods[agentName]
		if !running {
			// Pod not running, teardown any existing forwards
			r.teardownAgentForwards(vibespace, agentName)
			continue
		}

		// Update pod mapping (use composite key for manager, simple name for state)
		key := agentKey(vibespace, agentName)
		r.manager.SetAgentPod(key, podName)
		vsState.SetAgentPod(agentName, podName)

		// Ensure forwards exist
		for _, fwd := range forwards {
			r.ensureForward(vibespace, agentName, fwd)
		}
	}

	// For all running pods, ensure default forwards (SSH + ttyd) exist
	// This covers both agents with and without desired forwards
	for agentName, podName := range runningPods {
		key := agentKey(vibespace, agentName)
		r.manager.SetAgentPod(key, podName)
		vsState.SetAgentPod(agentName, podName)
		r.ensureDefaultForwards(vibespace, agentName)
	}

	// Remove agents that no longer have running pods
	for agentName := range vsState.Agents {
		if _, running := runningPods[agentName]; !running {
			r.teardownAgentForwards(vibespace, agentName)
			vsState.RemoveAgent(agentName)
		}
	}
}

// cleanupEmptyVibespaces removes forwards for vibespaces with no running pods
func (r *Reconciler) cleanupEmptyVibespaces(currentVibespaces map[string][]PodInfo) {
	for _, vibespace := range r.state.GetAllVibespaces() {
		if _, exists := currentVibespaces[vibespace]; !exists {
			slog.Info("cleaning up empty vibespace", "vibespace", vibespace)
			vsState := r.state.GetVibespace(vibespace)
			if vsState != nil {
				for agentName := range vsState.Agents {
					r.teardownAgentForwards(vibespace, agentName)
				}
			}
			r.state.RemoveVibespace(vibespace)
			// Clean up desired state file
			if err := r.desiredMgr.Remove(vibespace); err != nil {
				slog.Warn("failed to remove desired state", "vibespace", vibespace, "error", err)
			}
		}
	}
}

// ensureForward ensures a forward exists for an agent
func (r *Reconciler) ensureForward(vibespace, agentName string, fwd DesiredForward) {
	key := agentKey(vibespace, agentName)

	// Check if forward already exists
	allForwards := r.manager.ListAllForwards()
	if forwards, ok := allForwards[key]; ok {
		for _, existing := range forwards {
			if existing.RemotePort == fwd.ContainerPort {
				return // Already exists
			}
		}
	}

	// Create forward
	localPort, err := r.manager.AddForward(key, fwd.ContainerPort, portforward.TypeManual, fwd.LocalPort)
	if err != nil {
		slog.Error("failed to create forward",
			"vibespace", vibespace,
			"agent", agentName,
			"port", fwd.ContainerPort,
			"error", err)
		return
	}

	// Update state
	vsState := r.state.GetVibespace(vibespace)
	if vsState != nil {
		vsState.AddForward(agentName, &ForwardState{
			LocalPort:  localPort,
			RemotePort: fwd.ContainerPort,
			Type:       portforward.TypeManual,
			Status:     portforward.StatusActive,
		})
	}

	slog.Info("created forward",
		"vibespace", vibespace,
		"agent", agentName,
		"local", localPort,
		"remote", fwd.ContainerPort)
}

// ensureDefaultForwards ensures SSH and ttyd forwards exist for an agent
func (r *Reconciler) ensureDefaultForwards(vibespace, agentName string) {
	key := agentKey(vibespace, agentName)

	// Check which defaults already exist
	allForwards := r.manager.ListAllForwards()
	existingForwards := allForwards[key]

	hasSSH := false
	hasTTYD := false
	for _, fwd := range existingForwards {
		if fwd.RemotePort == portforward.DefaultSSHPort {
			hasSSH = true
		}
		if fwd.RemotePort == portforward.DefaultTTYDPort {
			hasTTYD = true
		}
	}

	vsState := r.state.GetVibespace(vibespace)
	if vsState == nil {
		return
	}

	// Create SSH forward if missing
	if !hasSSH {
		sshLocalPort, err := r.manager.AddForward(key, portforward.DefaultSSHPort, portforward.TypeSSH, 0)
		if err != nil {
			slog.Error("failed to create SSH forward", "agent", agentName, "error", err)
		} else {
			vsState.AddForward(agentName, &ForwardState{
				LocalPort:  sshLocalPort,
				RemotePort: portforward.DefaultSSHPort,
				Type:       portforward.TypeSSH,
				Status:     portforward.StatusActive,
			})
			slog.Info("created default SSH forward", "vibespace", vibespace, "agent", agentName, "local_port", sshLocalPort)
		}
	}

	// Create ttyd forward if missing
	if !hasTTYD {
		ttydLocalPort, err := r.manager.AddForward(key, portforward.DefaultTTYDPort, portforward.TypeTTYD, 0)
		if err != nil {
			slog.Error("failed to create ttyd forward", "agent", agentName, "error", err)
		} else {
			vsState.AddForward(agentName, &ForwardState{
				LocalPort:  ttydLocalPort,
				RemotePort: portforward.DefaultTTYDPort,
				Type:       portforward.TypeTTYD,
				Status:     portforward.StatusActive,
			})
			slog.Info("created default ttyd forward", "vibespace", vibespace, "agent", agentName, "local_port", ttydLocalPort)
		}
	}
}

// teardownAgentForwards removes all forwards for an agent
func (r *Reconciler) teardownAgentForwards(vibespace, agentName string) {
	slog.Debug("tearing down agent forwards", "vibespace", vibespace, "agent", agentName)
	key := agentKey(vibespace, agentName)
	r.manager.RemoveAgentForwards(key)
}

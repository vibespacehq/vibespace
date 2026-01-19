package portforward

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// PodMonitor watches pods for a vibespace and notifies on changes
type PodMonitor struct {
	clientset   kubernetes.Interface
	namespace   string
	vibespaceID string
	manager     *Manager

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PodMonitorConfig contains configuration for creating a PodMonitor
type PodMonitorConfig struct {
	Clientset   kubernetes.Interface
	Namespace   string
	VibespaceID string
	Manager     *Manager
}

// NewPodMonitor creates a new pod monitor
func NewPodMonitor(cfg PodMonitorConfig) *PodMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &PodMonitor{
		clientset:   cfg.Clientset,
		namespace:   cfg.Namespace,
		vibespaceID: cfg.VibespaceID,
		manager:     cfg.Manager,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts watching pods
func (m *PodMonitor) Start() error {
	slog.Info("starting pod monitor",
		"namespace", m.namespace,
		"vibespace", m.vibespaceID)

	m.wg.Add(1)
	go m.watchLoop()

	return nil
}

// Stop stops the monitor
func (m *PodMonitor) Stop() {
	slog.Info("stopping pod monitor", "vibespace", m.vibespaceID)
	m.cancel()
	m.wg.Wait()
}

// watchLoop continuously watches for pod changes
func (m *PodMonitor) watchLoop() {
	defer m.wg.Done()

	labelSelector := fmt.Sprintf("vibespace.dev/id=%s", m.vibespaceID)

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		// Create watch
		watcher, err := m.clientset.CoreV1().Pods(m.namespace).Watch(m.ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			slog.Error("failed to create pod watch", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		m.handleWatchEvents(watcher)
		watcher.Stop()

		// Brief delay before reconnecting watch
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// handleWatchEvents processes watch events
func (m *PodMonitor) handleWatchEvents(watcher watch.Interface) {
	for {
		select {
		case <-m.ctx.Done():
			return

		case event, ok := <-watcher.ResultChan():
			if !ok {
				// Watch channel closed, need to reconnect
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			switch event.Type {
			case watch.Added:
				m.handlePodAdded(pod)
			case watch.Modified:
				m.handlePodModified(pod)
			case watch.Deleted:
				m.handlePodDeleted(pod)
			}
		}
	}
}

// handlePodAdded handles a new pod being added
func (m *PodMonitor) handlePodAdded(pod *corev1.Pod) {
	agentName := m.podToAgentName(pod)
	slog.Info("pod added",
		"pod", pod.Name,
		"agent", agentName,
		"phase", pod.Status.Phase)

	if pod.Status.Phase == corev1.PodRunning && isPodReady(pod) {
		oldPod, exists := m.manager.GetAgentPod(agentName)
		if !exists || oldPod != pod.Name {
			m.manager.UpdateAgentPod(agentName, pod.Name)
		}
	}
}

// handlePodModified handles a pod being modified
func (m *PodMonitor) handlePodModified(pod *corev1.Pod) {
	agentName := m.podToAgentName(pod)
	slog.Debug("pod modified",
		"pod", pod.Name,
		"agent", agentName,
		"phase", pod.Status.Phase)

	switch pod.Status.Phase {
	case corev1.PodRunning:
		if isPodReady(pod) {
			oldPod, exists := m.manager.GetAgentPod(agentName)
			if !exists || oldPod != pod.Name {
				slog.Info("pod became ready, updating agent",
					"pod", pod.Name,
					"agent", agentName,
					"old_pod", oldPod)
				m.manager.UpdateAgentPod(agentName, pod.Name)
			}
		}

	case corev1.PodFailed, corev1.PodSucceeded:
		slog.Warn("pod terminated",
			"pod", pod.Name,
			"agent", agentName,
			"phase", pod.Status.Phase)
		// The forwarders will detect the connection drop and trigger reconnection
	}
}

// handlePodDeleted handles a pod being deleted
func (m *PodMonitor) handlePodDeleted(pod *corev1.Pod) {
	agentName := m.podToAgentName(pod)
	slog.Info("pod deleted",
		"pod", pod.Name,
		"agent", agentName)

	// The forwarders will detect the connection drop and trigger reconnection
	// when a new pod comes up
}

// podToAgentName maps a pod to an agent name
// Currently assumes single agent "claude-1" per vibespace
// TODO: Support multi-agent by reading agent label from pod
func (m *PodMonitor) podToAgentName(pod *corev1.Pod) string {
	// Check for agent label
	if agentLabel, ok := pod.Labels["vibespace.dev/agent"]; ok {
		return agentLabel
	}
	// Default to claude-1
	return "claude-1"
}

// isPodReady checks if a pod is ready to receive traffic
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// DiscoverPods discovers all pods for a vibespace
func DiscoverPods(ctx context.Context, clientset kubernetes.Interface, namespace, vibespaceID string) (map[string]string, error) {
	labelSelector := fmt.Sprintf("vibespace.dev/id=%s", vibespaceID)

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	agents := make(map[string]string)
	agentIndex := 1

	for _, pod := range pods.Items {
		// Skip non-running pods
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Check for agent label
		agentName := ""
		if label, ok := pod.Labels["vibespace.dev/agent"]; ok {
			agentName = label
		} else {
			agentName = fmt.Sprintf("claude-%d", agentIndex)
			agentIndex++
		}

		agents[agentName] = pod.Name
	}

	if len(agents) == 0 {
		return nil, fmt.Errorf("no running pods found for vibespace %s", vibespaceID)
	}

	return agents, nil
}

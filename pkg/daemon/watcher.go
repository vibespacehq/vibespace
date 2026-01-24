package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yagizdagabak/vibespace/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// PodEvent represents a pod change event
type PodEvent struct {
	Type      watch.EventType
	Vibespace string // vibespace name (resolved from ID)
	Agent     string // agent name from vibespace.dev/agent-name label
	PodName   string
	Phase     corev1.PodPhase
}

// PodWatcher watches all vibespace pods for changes
type PodWatcher struct {
	clientset *kubernetes.Clientset
	eventCh   chan PodEvent
	stopCh    chan struct{}

	// Cache to resolve vibespace IDs to names
	vibespaceNames map[string]string // ID -> name
}

// NewPodWatcher creates a new pod watcher
func NewPodWatcher(clientset *kubernetes.Clientset) *PodWatcher {
	return &PodWatcher{
		clientset:      clientset,
		eventCh:        make(chan PodEvent, 100),
		stopCh:         make(chan struct{}),
		vibespaceNames: make(map[string]string),
	}
}

// Start begins watching pods. Should be called in a goroutine.
func (w *PodWatcher) Start(ctx context.Context) error {
	slog.Info("starting pod watcher")

	// Watch all pods with vibespace label
	labelSelector := "app.kubernetes.io/managed-by=vibespace"

	for {
		select {
		case <-ctx.Done():
			slog.Info("pod watcher context cancelled")
			return ctx.Err()
		case <-w.stopCh:
			slog.Info("pod watcher stopped")
			return nil
		default:
		}

		// Get resource version for watching
		podList, err := w.clientset.CoreV1().Pods(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			slog.Error("failed to list pods", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Update vibespace name cache from existing pods
		for _, pod := range podList.Items {
			if labels := pod.Labels; labels != nil {
				if id := labels["vibespace.dev/id"]; id != "" {
					if name := labels["app.kubernetes.io/name"]; name != "" {
						w.vibespaceNames[id] = name
					}
				}
			}
		}

		resourceVersion := podList.ResourceVersion

		// Start watch
		watcher, err := w.clientset.CoreV1().Pods(k8s.VibespaceNamespace).Watch(ctx, metav1.ListOptions{
			LabelSelector:   labelSelector,
			ResourceVersion: resourceVersion,
		})
		if err != nil {
			slog.Error("failed to start pod watch", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Process events
		if err := w.processEvents(ctx, watcher); err != nil {
			slog.Warn("watch ended", "error", err)
		}
		watcher.Stop()

		// Brief pause before restarting watch
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopCh:
			return nil
		case <-time.After(time.Second):
		}
	}
}

// processEvents handles watch events until an error occurs
func (w *PodWatcher) processEvents(ctx context.Context, watcher watch.Interface) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopCh:
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			if event.Type == watch.Error {
				return fmt.Errorf("watch error")
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			podEvent := w.podToEvent(event.Type, pod)
			if podEvent != nil {
				select {
				case w.eventCh <- *podEvent:
				default:
					slog.Warn("event channel full, dropping event", "pod", pod.Name)
				}
			}
		}
	}
}

// podToEvent converts a pod to a PodEvent
func (w *PodWatcher) podToEvent(eventType watch.EventType, pod *corev1.Pod) *PodEvent {
	labels := pod.Labels
	if labels == nil {
		return nil
	}

	// Get vibespace ID and resolve to name
	vibespaceID := labels["vibespace.dev/id"]
	if vibespaceID == "" {
		return nil
	}

	// Update cache with name if available
	if name := labels["app.kubernetes.io/name"]; name != "" {
		w.vibespaceNames[vibespaceID] = name
	}

	// Resolve ID to name
	vibespace := w.vibespaceNames[vibespaceID]
	if vibespace == "" {
		vibespace = vibespaceID // Fall back to ID if name not known
	}

	// Get agent name (prefer agent-name, fall back to claude-id)
	agentName := ""
	if name, ok := labels["vibespace.dev/agent-name"]; ok && name != "" {
		agentName = name
	} else if cid, ok := labels["vibespace.dev/claude-id"]; ok && cid != "" {
		agentName = fmt.Sprintf("claude-%s", cid)
	}

	if agentName == "" {
		agentName = "claude-1"
	}

	slog.Debug("pod event",
		"type", eventType,
		"vibespace", vibespace,
		"agent", agentName,
		"pod", pod.Name,
		"phase", pod.Status.Phase)

	return &PodEvent{
		Type:      eventType,
		Vibespace: vibespace,
		Agent:     agentName,
		PodName:   pod.Name,
		Phase:     pod.Status.Phase,
	}
}

// Events returns the channel of pod events
func (w *PodWatcher) Events() <-chan PodEvent {
	return w.eventCh
}

// Stop stops the watcher
func (w *PodWatcher) Stop() {
	close(w.stopCh)
}

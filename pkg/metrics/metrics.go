package metrics

import (
	"context"
	"fmt"

	"github.com/vibespacehq/vibespace/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

// PodMetrics holds aggregated resource usage for a single pod.
type PodMetrics struct {
	Name             string
	Namespace        string
	CPUMillis        int64
	MemoryBytes      int64
	VibspaceName     string // from app.kubernetes.io/name label
	AgentName        string // from vibespace.dev/agent-name label
	CPULimitMillis   int64  // from pod spec container limits
	MemoryLimitBytes int64  // from pod spec container limits
}

// NodeMetrics holds resource usage for a single node.
type NodeMetrics struct {
	Name                   string
	CPUMillis              int64
	MemoryBytes            int64
	CPUAllocatableMillis   int64 // from node.Status.Allocatable
	MemoryAllocatableBytes int64 // from node.Status.Allocatable
}

// Fetcher retrieves resource metrics from the Kubernetes metrics API.
type Fetcher struct {
	client *k8s.Client
}

// NewFetcher creates a Fetcher backed by the given k8s client.
func NewFetcher(c *k8s.Client) *Fetcher {
	return &Fetcher{client: c}
}

// metricsClient returns the metrics API clientset from the underlying k8s client.
func (f *Fetcher) metricsClient() (metricsv.Interface, error) {
	return f.client.MetricsClientset()
}

// Available returns true if the metrics API is responding.
func (f *Fetcher) Available(ctx context.Context) bool {
	mc, err := f.metricsClient()
	if err != nil {
		return false
	}
	_, err = mc.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// FetchPodMetrics returns resource usage for vibespace-managed pods in the given namespace.
// Enriches each pod with vibespace name, agent name, and resource limits from the pod spec.
func (f *Fetcher) FetchPodMetrics(ctx context.Context, namespace string) ([]PodMetrics, error) {
	mc, err := f.metricsClient()
	if err != nil {
		return nil, err
	}

	metricsList, err := mc.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pod metrics: %w", err)
	}

	// Fetch vibespace-managed pods to get labels and resource limits
	podList, err := f.client.Clientset().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=vibespace",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Build lookup: pod name → core pod
	podLookup := make(map[string]*corev1.Pod, len(podList.Items))
	for i := range podList.Items {
		podLookup[podList.Items[i].Name] = &podList.Items[i]
	}

	result := make([]PodMetrics, 0, len(metricsList.Items))
	for _, pm := range metricsList.Items {
		// Only include vibespace-managed pods
		pod, ok := podLookup[pm.Name]
		if !ok {
			continue
		}

		var cpuMillis, memBytes int64
		for _, c := range pm.Containers {
			cpuMillis += c.Usage.Cpu().MilliValue()
			memBytes += c.Usage.Memory().Value()
		}

		// Extract limits from pod spec containers
		var cpuLimitMillis, memLimitBytes int64
		for _, c := range pod.Spec.Containers {
			if lim, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
				cpuLimitMillis += lim.MilliValue()
			}
			if lim, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
				memLimitBytes += lim.Value()
			}
		}

		result = append(result, PodMetrics{
			Name:             pm.Name,
			Namespace:        pm.Namespace,
			CPUMillis:        cpuMillis,
			MemoryBytes:      memBytes,
			VibspaceName:     pod.Labels["app.kubernetes.io/name"],
			AgentName:        pod.Labels["vibespace.dev/agent-name"],
			CPULimitMillis:   cpuLimitMillis,
			MemoryLimitBytes: memLimitBytes,
		})
	}
	return result, nil
}

// FetchNodeMetrics returns resource usage for all nodes in the cluster.
// Enriches each node with allocatable capacity from the core API.
func (f *Fetcher) FetchNodeMetrics(ctx context.Context) ([]NodeMetrics, error) {
	mc, err := f.metricsClient()
	if err != nil {
		return nil, err
	}

	metricsList, err := mc.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list node metrics: %w", err)
	}

	// Fetch nodes to get allocatable capacity
	nodeList, err := f.client.Clientset().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Build lookup: node name → allocatable resources
	type allocatable struct {
		cpu    resource.Quantity
		memory resource.Quantity
	}
	nodeLookup := make(map[string]allocatable, len(nodeList.Items))
	for _, n := range nodeList.Items {
		nodeLookup[n.Name] = allocatable{
			cpu:    n.Status.Allocatable[corev1.ResourceCPU],
			memory: n.Status.Allocatable[corev1.ResourceMemory],
		}
	}

	result := make([]NodeMetrics, 0, len(metricsList.Items))
	for _, nm := range metricsList.Items {
		node := NodeMetrics{
			Name:        nm.Name,
			CPUMillis:   nm.Usage.Cpu().MilliValue(),
			MemoryBytes: nm.Usage.Memory().Value(),
		}
		if a, ok := nodeLookup[nm.Name]; ok {
			node.CPUAllocatableMillis = a.cpu.MilliValue()
			node.MemoryAllocatableBytes = a.memory.Value()
		}
		result = append(result, node)
	}
	return result, nil
}

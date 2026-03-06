package cli

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vibespacehq/vibespace/pkg/agent"
	_ "github.com/vibespacehq/vibespace/pkg/agent/claude"
	_ "github.com/vibespacehq/vibespace/pkg/agent/codex"
	"github.com/vibespacehq/vibespace/pkg/k8s"
	"github.com/vibespacehq/vibespace/pkg/vibespace"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newFakeService creates a vibespace.Service backed by a fake k8s clientset.
func newFakeService(t *testing.T, objects ...runtime.Object) (*vibespace.Service, *fake.Clientset) {
	t.Helper()
	cs := fake.NewClientset(objects...)
	client := k8s.NewClientFromClientset(cs)
	return vibespace.NewService(client), cs
}

// int32Ptr returns a pointer to an int32.
func int32Ptr(i int32) *int32 { return &i }

// fakeVibespaceDeployment creates a fake Deployment that looks like a running vibespace
// with a primary claude-code agent named "claude-1".
func fakeVibespaceDeployment(name, id string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vibespace-" + id,
			Namespace: k8s.VibespaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "vibespace",
				"app.kubernetes.io/name":       name,
				"vibespace.dev/id":             id,
				"vibespace.dev/agent-name":     "claude-1",
				"vibespace.dev/agent-type":     "claude-code",
				"vibespace.dev/agent-num":      "1",
				"vibespace.dev/is-primary":     "true",
			},
			Annotations: map[string]string{
				"vibespace.dev/storage":    "10Gi",
				"vibespace.dev/created-at": "2025-01-15T10:00:00Z",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"vibespace.dev/id": id},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"vibespace.dev/id": id},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vibespace",
							Image: "ghcr.io/vibespacehq/vibespace-claude-code:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    mustParseQuantity("250m"),
									corev1.ResourceMemory: mustParseQuantity("512Mi"),
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:      1,
			ReadyReplicas: 1,
		},
	}
}

// fakeStoppedDeployment creates a stopped vibespace deployment (replicas=0).
func fakeStoppedDeployment(name, id string) *appsv1.Deployment {
	d := fakeVibespaceDeployment(name, id)
	d.Spec.Replicas = int32Ptr(0)
	d.Status.Replicas = 0
	d.Status.ReadyReplicas = 0
	return d
}

// fakeAgentDeployment creates a secondary agent deployment.
func fakeAgentDeployment(vsName, vsID, agentName string, agentType agent.Type, agentNum int) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vibespace-" + vsID + "-" + agentName,
			Namespace: k8s.VibespaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "vibespace",
				"app.kubernetes.io/name":       vsName,
				"vibespace.dev/id":             vsID,
				"vibespace.dev/agent-name":     agentName,
				"vibespace.dev/agent-type":     string(agentType),
				"vibespace.dev/agent-num":      intStr(agentNum),
				"vibespace.dev/is-agent":       "true",
				"vibespace.dev/is-primary":     "false",
			},
			Annotations: map[string]string{
				"vibespace.dev/storage":    "10Gi",
				"vibespace.dev/created-at": "2025-01-15T10:00:00Z",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"vibespace.dev/id":         vsID,
					"vibespace.dev/agent-name": agentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"vibespace.dev/id":         vsID,
						"vibespace.dev/agent-name": agentName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vibespace",
							Image: "ghcr.io/vibespacehq/vibespace-claude-code:latest",
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:      1,
			ReadyReplicas: 1,
		},
	}
}

// mustParseQuantity is a test helper for resource quantities.
func mustParseQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}

func intStr(i int) string {
	return fmt.Sprintf("%d", i)
}

// newTestConfigSetCmd creates a fresh cobra.Command with the same flags as configSetCmd,
// avoiding shared state between tests (cobra flags retain Changed state).
func newTestConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "set"}
	cmd.Flags().Bool("skip-permissions", false, "")
	cmd.Flags().Bool("no-skip-permissions", false, "")
	cmd.Flags().String("allowed-tools", "", "")
	cmd.Flags().String("disallowed-tools", "", "")
	cmd.Flags().String("model", "", "")
	cmd.Flags().Int("max-turns", 0, "")
	cmd.Flags().String("system-prompt", "", "")
	cmd.Flags().String("reasoning-effort", "", "")
	return cmd
}

// addReactor adds a reactor to intercept a specific verb/resource with an error.
func addReactor(cs *fake.Clientset, verb, resource string, err error) {
	cs.PrependReactor(verb, resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

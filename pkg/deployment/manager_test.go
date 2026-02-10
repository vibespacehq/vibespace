package deployment

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/yagizdagabak/vibespace/pkg/agent"
	_ "github.com/yagizdagabak/vibespace/pkg/agent/claude"
	"github.com/yagizdagabak/vibespace/pkg/k8s"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestDeploymentManager(t *testing.T) *DeploymentManager {
	t.Helper()
	client, err := k8s.NewClient()
	if err != nil {
		t.Skip("k8s not available:", err)
	}
	ctx := context.Background()
	if err := client.EnsureNamespace(ctx); err != nil {
		t.Fatalf("EnsureNamespace: %v", err)
	}
	return NewDeploymentManager(client)
}

func testDeploymentRequest(id string) *CreateDeploymentRequest {
	return &CreateDeploymentRequest{
		VibespaceID: id,
		Name:        "test-" + id,
		AgentID:     uuid.New().String()[:8],
		AgentType:   agent.TypeClaudeCode,
		AgentNum:    1,
		Primary:     true,
		Image:       "busybox:latest",
		Resources: Resources{
			CPU:     "100m",
			Memory:  "64Mi",
			Storage: "1Gi",
		},
	}
}

func TestCreateDeployment(t *testing.T) {
	mgr := newTestDeploymentManager(t)
	ctx := context.Background()

	id := uuid.New().String()[:8]
	req := testDeploymentRequest(id)

	t.Cleanup(func() {
		_ = mgr.DeleteDeployment(context.Background(), id)
	})

	if err := mgr.CreateDeployment(ctx, req); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	// Verify deployment exists
	deploy, err := mgr.GetDeployment(ctx, id)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}

	// Check labels
	if got := deploy.Labels["app.kubernetes.io/name"]; got != req.Name {
		t.Errorf("label app.kubernetes.io/name = %q, want %q", got, req.Name)
	}
	if got := deploy.Labels["vibespace.dev/id"]; got != id {
		t.Errorf("label vibespace.dev/id = %q, want %q", got, id)
	}
	if got := deploy.Labels["vibespace.dev/agent-type"]; got != string(agent.TypeClaudeCode) {
		t.Errorf("label vibespace.dev/agent-type = %q, want %q", got, string(agent.TypeClaudeCode))
	}
	if got := deploy.Labels["app.kubernetes.io/managed-by"]; got != "vibespace" {
		t.Errorf("label app.kubernetes.io/managed-by = %q, want %q", got, "vibespace")
	}
	if got := deploy.Labels["vibespace.dev/primary"]; got != "true" {
		t.Errorf("label vibespace.dev/primary = %q, want %q", got, "true")
	}
	if got := deploy.Labels["vibespace.dev/agent-name"]; got != "claude-1" {
		t.Errorf("label vibespace.dev/agent-name = %q, want %q", got, "claude-1")
	}

	// Check container
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	c := containers[0]
	if c.Image != "busybox:latest" {
		t.Errorf("image = %q, want %q", c.Image, "busybox:latest")
	}
	if len(c.Ports) < 2 {
		t.Fatalf("expected at least 2 ports, got %d", len(c.Ports))
	}

	// Verify port names
	portNames := map[string]int32{}
	for _, p := range c.Ports {
		portNames[p.Name] = p.ContainerPort
	}
	if portNames["ttyd"] != 7681 {
		t.Errorf("ttyd port = %d, want 7681", portNames["ttyd"])
	}
	if portNames["ssh"] != 22 {
		t.Errorf("ssh port = %d, want 22", portNames["ssh"])
	}

	// Verify resource requests
	cpuReq := c.Resources.Requests["cpu"]
	if cpuReq.String() != "100m" {
		t.Errorf("cpu request = %q, want %q", cpuReq.String(), "100m")
	}
	memReq := c.Resources.Requests["memory"]
	if memReq.String() != "64Mi" {
		t.Errorf("memory request = %q, want %q", memReq.String(), "64Mi")
	}

	// Verify Service exists with correct ports
	svcName := fmt.Sprintf("vibespace-%s", id)
	svc, err := mgr.k8sClient.Clientset().CoreV1().Services(k8s.VibespaceNamespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Service not found: %v", err)
	}
	if len(svc.Spec.Ports) < 2 {
		t.Fatalf("expected at least 2 service ports, got %d", len(svc.Spec.Ports))
	}
	svcPortNames := map[string]int32{}
	for _, p := range svc.Spec.Ports {
		svcPortNames[p.Name] = p.Port
	}
	if svcPortNames["ttyd"] != 7681 {
		t.Errorf("service ttyd port = %d, want 7681", svcPortNames["ttyd"])
	}
	if svcPortNames["ssh"] != 22 {
		t.Errorf("service ssh port = %d, want 22", svcPortNames["ssh"])
	}
}

func TestScaleDeployment(t *testing.T) {
	mgr := newTestDeploymentManager(t)
	ctx := context.Background()

	id := uuid.New().String()[:8]
	req := testDeploymentRequest(id)

	t.Cleanup(func() {
		_ = mgr.DeleteDeployment(context.Background(), id)
	})

	if err := mgr.CreateDeployment(ctx, req); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	// Scale to 0
	if err := mgr.ScaleAllDeploymentsForVibespace(ctx, id, 0); err != nil {
		t.Fatalf("ScaleAllDeploymentsForVibespace(0): %v", err)
	}
	deploy, err := mgr.GetDeployment(ctx, id)
	if err != nil {
		t.Fatalf("GetDeployment after scale down: %v", err)
	}
	if got := *deploy.Spec.Replicas; got != 0 {
		t.Errorf("replicas after scale down = %d, want 0", got)
	}

	// Scale back to 1
	if err := mgr.ScaleAllDeploymentsForVibespace(ctx, id, 1); err != nil {
		t.Fatalf("ScaleAllDeploymentsForVibespace(1): %v", err)
	}
	deploy, err = mgr.GetDeployment(ctx, id)
	if err != nil {
		t.Fatalf("GetDeployment after scale up: %v", err)
	}
	if got := *deploy.Spec.Replicas; got != 1 {
		t.Errorf("replicas after scale up = %d, want 1", got)
	}
}

func TestDeleteDeployment(t *testing.T) {
	mgr := newTestDeploymentManager(t)
	ctx := context.Background()

	id := uuid.New().String()[:8]
	req := testDeploymentRequest(id)

	if err := mgr.CreateDeployment(ctx, req); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	if err := mgr.DeleteDeployment(ctx, id); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}

	// Verify Deployment is gone
	_, err := mgr.GetDeployment(ctx, id)
	if err == nil {
		t.Fatal("expected error getting deleted deployment, got nil")
	}

	// Verify Service is gone
	svcName := fmt.Sprintf("vibespace-%s", id)
	_, err = mgr.k8sClient.Clientset().CoreV1().Services(k8s.VibespaceNamespace).Get(ctx, svcName, metav1.GetOptions{})
	if err == nil {
		t.Fatal("expected error getting deleted service, got nil")
	}
	if !k8serrors.IsNotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestListByLabel(t *testing.T) {
	mgr := newTestDeploymentManager(t)
	ctx := context.Background()

	id1 := uuid.New().String()[:8]
	id2 := uuid.New().String()[:8]
	req1 := testDeploymentRequest(id1)
	req2 := testDeploymentRequest(id2)

	t.Cleanup(func() {
		_ = mgr.DeleteDeployment(context.Background(), id1)
		_ = mgr.DeleteDeployment(context.Background(), id2)
	})

	if err := mgr.CreateDeployment(ctx, req1); err != nil {
		t.Fatalf("CreateDeployment(1): %v", err)
	}
	if err := mgr.CreateDeployment(ctx, req2); err != nil {
		t.Fatalf("CreateDeployment(2): %v", err)
	}

	// List all deployments — both should appear
	all, err := mgr.ListDeployments(ctx)
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	found := map[string]bool{}
	for _, d := range all {
		vid := d.Labels["vibespace.dev/id"]
		if vid == id1 || vid == id2 {
			found[vid] = true
		}
	}
	if !found[id1] || !found[id2] {
		t.Errorf("ListDeployments missing test deployments: found=%v", found)
	}

	// List agents for id1 only — should return exactly 1
	agents, err := mgr.ListAgentsForVibespace(ctx, id1)
	if err != nil {
		t.Fatalf("ListAgentsForVibespace: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent for id1, got %d", len(agents))
	}
	for _, a := range agents {
		if a.DeploymentName != fmt.Sprintf("vibespace-%s", id1) {
			t.Errorf("unexpected agent deployment: %s", a.DeploymentName)
		}
	}
}

package vibespace

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
	"github.com/yagizdagabak/vibespace/pkg/k8s"
	"github.com/yagizdagabak/vibespace/pkg/model"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestService(t *testing.T) (*Service, *k8s.Client) {
	t.Helper()
	client, err := k8s.NewClient()
	if err != nil {
		t.Skip("k8s not available:", err)
	}
	ctx := context.Background()
	if err := client.EnsureNamespace(ctx); err != nil {
		t.Fatalf("EnsureNamespace: %v", err)
	}
	return NewService(client), client
}

func uniqueName(t *testing.T) string {
	t.Helper()
	return "t-" + uuid.New().String()[:8]
}

func testCreateRequest(name string) *model.CreateVibespaceRequest {
	return &model.CreateVibespaceRequest{
		Name:       name,
		Persistent: true,
		Resources: &model.Resources{
			CPU:     "100m",
			Memory:  "64Mi",
			Storage: "1Gi",
		},
	}
}

func TestCreateVibespace(t *testing.T) {
	svc, client := newTestService(t)
	ctx := context.Background()
	name := uniqueName(t)

	t.Cleanup(func() {
		_ = svc.Delete(context.Background(), name, nil)
	})

	vs, err := svc.Create(ctx, testCreateRequest(name))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if vs.Name != name {
		t.Errorf("Name = %q, want %q", vs.Name, name)
	}
	if len(vs.ID) != 8 {
		t.Errorf("ID length = %d, want 8", len(vs.ID))
	}
	if vs.Status != "creating" {
		t.Errorf("Status = %q, want %q", vs.Status, "creating")
	}
	if vs.Resources.CPU != "100m" {
		t.Errorf("CPU = %q, want %q", vs.Resources.CPU, "100m")
	}
	if vs.Resources.Memory != "64Mi" {
		t.Errorf("Memory = %q, want %q", vs.Resources.Memory, "64Mi")
	}

	// Verify deployment exists via k8s client directly
	deployName := fmt.Sprintf("vibespace-%s", vs.ID)
	deploy, err := client.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Get(ctx, deployName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Deployment not found in k8s: %v", err)
	}
	if got := deploy.Labels["vibespace.dev/id"]; got != vs.ID {
		t.Errorf("deployment label vibespace.dev/id = %q, want %q", got, vs.ID)
	}
}

func TestListVibespaces(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	name1 := uniqueName(t)
	name2 := uniqueName(t)

	t.Cleanup(func() {
		_ = svc.Delete(context.Background(), name1, nil)
		_ = svc.Delete(context.Background(), name2, nil)
	})

	vs1, err := svc.Create(ctx, testCreateRequest(name1))
	if err != nil {
		t.Fatalf("Create(1): %v", err)
	}
	vs2, err := svc.Create(ctx, testCreateRequest(name2))
	if err != nil {
		t.Fatalf("Create(2): %v", err)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := map[string]bool{}
	for _, v := range list {
		if v.ID == vs1.ID || v.ID == vs2.ID {
			found[v.ID] = true
		}
	}
	if !found[vs1.ID] || !found[vs2.ID] {
		t.Errorf("List missing test vibespaces: found=%v", found)
	}
}

func TestGetVibespace(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	name := uniqueName(t)

	t.Cleanup(func() {
		_ = svc.Delete(context.Background(), name, nil)
	})

	created, err := svc.Create(ctx, testCreateRequest(name))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Get by name
	got, err := svc.Get(ctx, name)
	if err != nil {
		t.Fatalf("Get(name): %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("Get(name) ID = %q, want %q", got.ID, created.ID)
	}

	// Get by ID
	got, err = svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get(ID): %v", err)
	}
	if got.Name != name {
		t.Errorf("Get(ID) Name = %q, want %q", got.Name, name)
	}

	// Get nonexistent — should wrap ErrVibespaceNotFound
	_, err = svc.Get(ctx, "nonexistent-vs")
	if err == nil {
		t.Fatal("expected error for nonexistent vibespace, got nil")
	}
	if !errors.Is(err, vserrors.ErrVibespaceNotFound) {
		t.Errorf("expected ErrVibespaceNotFound, got: %v", err)
	}
}

func TestDeleteVibespace(t *testing.T) {
	svc, client := newTestService(t)
	ctx := context.Background()
	name := uniqueName(t)

	vs, err := svc.Create(ctx, testCreateRequest(name))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	id := vs.ID

	// Cleanup in case assertions fail before manual delete
	t.Cleanup(func() {
		_ = svc.Delete(context.Background(), name, nil)
	})

	if err := svc.Delete(ctx, name, nil); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify vibespace is gone
	_, err = svc.Get(ctx, name)
	if err == nil {
		t.Error("expected error for deleted vibespace, got nil")
	}

	// Verify SSH key secret is cleaned up
	secretName := fmt.Sprintf("vibespace-%s-ssh-keys", id)
	_, err = client.Clientset().CoreV1().Secrets(k8s.VibespaceNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		t.Error("SSH key secret still exists after deletion")
	} else if !k8serrors.IsNotFound(err) {
		t.Errorf("unexpected error checking secret: %v", err)
	}

	// Verify PVC is cleaned up (may still be terminating with finalizer)
	pvcName := fmt.Sprintf("vibespace-%s-pvc", id)
	pvc, err := client.Clientset().CoreV1().PersistentVolumeClaims(k8s.VibespaceNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			t.Errorf("unexpected error checking PVC: %v", err)
		}
		// NotFound is expected
	} else if pvc.DeletionTimestamp == nil {
		t.Error("PVC still exists without deletion timestamp")
	}
}

func TestCreateAgent(t *testing.T) {
	svc, client := newTestService(t)
	ctx := context.Background()
	name := uniqueName(t)

	t.Cleanup(func() {
		_ = svc.Delete(context.Background(), name, nil)
	})

	vs, err := svc.Create(ctx, testCreateRequest(name))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	agentName, err := svc.SpawnAgent(ctx, name, nil)
	if err != nil {
		t.Fatalf("SpawnAgent: %v", err)
	}
	if agentName != "claude-2" {
		t.Errorf("SpawnAgent returned %q, want %q", agentName, "claude-2")
	}

	// Verify agent deployment has correct labels
	deployName := fmt.Sprintf("vibespace-%s-%s", vs.ID, agentName)
	deploy, err := client.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Get(ctx, deployName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("agent Deployment not found: %v", err)
	}
	if got := deploy.Labels["vibespace.dev/is-agent"]; got != "true" {
		t.Errorf("is-agent label = %q, want %q", got, "true")
	}
	if got := deploy.Labels["vibespace.dev/agent-type"]; got != "claude-code" {
		t.Errorf("agent-type label = %q, want %q", got, "claude-code")
	}
	if got := deploy.Labels["vibespace.dev/agent-num"]; got != "2" {
		t.Errorf("agent-num label = %q, want %q", got, "2")
	}
	if got := deploy.Labels["vibespace.dev/id"]; got != vs.ID {
		t.Errorf("vibespace id label = %q, want %q", got, vs.ID)
	}
}

func TestListAgents(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	name := uniqueName(t)

	t.Cleanup(func() {
		_ = svc.Delete(context.Background(), name, nil)
	})

	_, err := svc.Create(ctx, testCreateRequest(name))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Spawn one more agent
	_, err = svc.SpawnAgent(ctx, name, nil)
	if err != nil {
		t.Fatalf("SpawnAgent: %v", err)
	}

	agents, err := svc.ListAgents(ctx, name)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	var hasPrimary bool
	agentNames := map[string]bool{}
	for _, a := range agents {
		agentNames[a.AgentName] = true
		if a.IsPrimary {
			hasPrimary = true
		}
	}
	if !hasPrimary {
		t.Error("no primary agent found")
	}
	if !agentNames["claude-1"] {
		t.Error("expected agent claude-1 not found")
	}
	if !agentNames["claude-2"] {
		t.Error("expected agent claude-2 not found")
	}
}

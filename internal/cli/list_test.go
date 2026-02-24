package cli

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestDoList_ListError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, cs := newFakeService(t)

	// The service.List() calls deploymentManager.ListDeployments(), which returns an error.
	// However, the service wraps it and returns nil slice on certain errors.
	// Let me check: Service.List() returns error if deploymentManager.ListDeployments() fails.
	// Actually looking at the code, service.List() does: if err != nil { return nil, fmt.Errorf(...) }
	// BUT the service first calls ensureClients() which for fake service already has clients.
	// Then it calls deploymentManager.ListDeployments() which does list.
	// If list fails, it returns error, which propagates up.
	addReactor(cs, "list", "deployments", fmt.Errorf("connection refused"))

	got := captureStdout(t, func() {
		err := doList(svc)
		if err == nil {
			t.Fatal("expected error from list")
		}
		if !strings.Contains(err.Error(), "failed to list vibespaces") {
			t.Errorf("expected 'failed to list' error, got: %v", err)
		}
	})
	_ = got
}

func TestDoList_Empty(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, _ := newFakeService(t) // empty clientset

	got := captureStdout(t, func() {
		err := doList(svc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, `"count": 0`) {
		t.Errorf("expected count=0 in JSON output, got: %s", got)
	}
}

func TestDoList_WithVibespaces(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	got := captureStdout(t, func() {
		err := doList(svc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "test-vs") {
		t.Errorf("expected 'test-vs' in output, got: %s", got)
	}
	if !strings.Contains(got, `"count": 1`) {
		t.Errorf("expected count=1 in JSON output, got: %s", got)
	}
}

func TestDoList_ListAgentsError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	// ListAgents calls svc.ListAgents which does Get + ListAgentsForVibespace
	// Fail on later list calls
	callCount := 0
	cs.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		if callCount >= 3 {
			return true, nil, fmt.Errorf("connection refused")
		}
		return false, nil, nil
	})

	got := captureStdout(t, func() {
		err := doList(svc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Even if ListAgents fails, list should still show the vibespace with default agent count
	if !strings.Contains(got, "test-vs") {
		t.Errorf("expected 'test-vs' in output, got: %s", got)
	}
}

func TestDoList_DefaultMode_Empty(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})
	svc, _ := newFakeService(t)

	got := captureStdout(t, func() {
		err := doList(svc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "No vibespaces found") {
		t.Errorf("expected 'No vibespaces found' in output, got: %s", got)
	}
}

func TestDoList_PlainMode_Empty(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, PlainMode: true})
	svc, _ := newFakeService(t)

	got := captureStdout(t, func() {
		err := doList(svc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Plain mode should produce no output for empty list
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty output in plain mode, got: %s", got)
	}
}

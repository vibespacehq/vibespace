package cli

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestDoInfo_GetError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, cs := newFakeService(t)
	addReactor(cs, "list", "deployments", fmt.Errorf("connection refused"))

	err := doInfo(svc, "test-vs")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDoInfo_NotFound(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, _ := newFakeService(t) // empty clientset

	err := doInfo(svc, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDoInfo_ListAgentsError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	// The first list call gets the vibespace (Get → ListByName).
	// The second list call is ListAgents.
	callCount := 0
	cs.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		if callCount >= 3 {
			return true, nil, fmt.Errorf("connection refused")
		}
		return false, nil, nil
	})

	err := doInfo(svc, "test-vs")
	if err == nil {
		t.Fatal("expected error from ListAgents")
	}
}

func TestDoInfo_Success(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	got := captureStdout(t, func() {
		err := doInfo(svc, "test-vs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "test-vs") {
		t.Errorf("expected 'test-vs' in output, got: %s", got)
	}
	if !strings.Contains(got, "abc123") {
		t.Errorf("expected 'abc123' in output, got: %s", got)
	}
}

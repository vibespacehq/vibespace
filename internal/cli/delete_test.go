package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestDoDelete_GetError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, cs := newFakeService(t)
	addReactor(cs, "list", "deployments", fmt.Errorf("connection refused"))

	err := doDelete(svc, "test-vs", true, false, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDoDelete_DeleteError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)
	addReactor(cs, "delete", "deployments", fmt.Errorf("connection refused"))

	err := doDelete(svc, "test-vs", true, false, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to delete vibespace") {
		t.Errorf("expected 'failed to delete' error, got: %v", err)
	}
}

func TestDoDelete_DryRun(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	got := captureStdout(t, func() {
		err := doDelete(svc, "test-vs", true, false, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "dry_run") || !strings.Contains(got, "true") {
		t.Errorf("expected dry_run=true in output, got: %s", got)
	}
}

func TestDoDelete_NotFound(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})
	svc, _ := newFakeService(t) // empty clientset

	err := doDelete(svc, "nonexistent", true, false, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

package cli

import (
	"fmt"
	"strings"
	"testing"
)

func TestDoCreate_ServiceError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, cs := newFakeService(t)
	// Namespace creation will fail
	addReactor(cs, "create", "namespaces", fmt.Errorf("connection refused"))
	// Also make get namespace fail so it tries to create
	addReactor(cs, "get", "namespaces", fmt.Errorf("connection refused"))

	err := doCreate(svc, "test-vs", "claude-code", "", "", "250m", "1000m",
		"512Mi", "1Gi", "10Gi", false, nil, false, "", "", "", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to create vibespace") {
		t.Errorf("expected 'failed to create' error, got: %v", err)
	}
}

func TestDoCreate_DuplicateConflict(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	err := doCreate(svc, "test-vs", "claude-code", "", "", "250m", "1000m",
		"512Mi", "1Gi", "10Gi", false, nil, false, "", "", "", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestDoCreate_InvalidMount(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, _ := newFakeService(t)

	err := doCreate(svc, "test-vs", "claude-code", "", "", "250m", "1000m",
		"512Mi", "1Gi", "10Gi", false, []string{"badformat"}, false, "", "", "", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid mount") {
		t.Errorf("expected 'invalid mount' error, got: %v", err)
	}
}

func TestDoCreate_Success(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, _ := newFakeService(t)

	got := captureStdout(t, func() {
		err := doCreate(svc, "test-vs", "claude-code", "", "", "250m", "1000m",
			"512Mi", "1Gi", "10Gi", false, nil, false, "", "", "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "test-vs") {
		t.Errorf("expected 'test-vs' in JSON output, got: %s", got)
	}
	if !strings.Contains(got, `"success": true`) {
		t.Errorf("expected success=true in JSON output, got: %s", got)
	}
}

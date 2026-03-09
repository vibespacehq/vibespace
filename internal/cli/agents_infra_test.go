package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/agent"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestDoAgentList_ServiceError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, cs := newFakeService(t)
	addReactor(cs, "list", "deployments", fmt.Errorf("connection refused"))

	err := doAgentList(svc, "test-vs")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDoAgentList_ListAgentsError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	// doAgentList calls svc.ListAgents which does:
	//   1. svc.Get("test-vs") → GetDeploymentByName → list call #1
	//   2. dm.ListAgentsForVibespace(id) → list call #2
	// Fail on the second list call to simulate ListAgentsForVibespace failure.
	callCount := 0
	cs.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		if callCount >= 2 {
			return true, nil, fmt.Errorf("connection refused")
		}
		return false, nil, nil // pass through
	})

	err := doAgentList(svc, "test-vs")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to list agents") {
		t.Errorf("expected 'failed to list agents' error, got: %v", err)
	}
}

func TestDoAgentCreate_SpawnError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)
	addReactor(cs, "create", "deployments", fmt.Errorf("resource quota exceeded"))

	err := doAgentCreate(svc, "test-vs", AgentCreateOptions{AgentType: "claude-code"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to create agent") {
		t.Errorf("expected 'failed to create agent' error, got: %v", err)
	}
}

func TestDoAgentDelete_KillError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	agent2 := fakeAgentDeployment("test-vs", "abc123", "claude-2", agent.TypeClaudeCode, 2)
	svc, cs := newFakeService(t, deploy, agent2)
	addReactor(cs, "delete", "deployments", fmt.Errorf("connection refused"))

	err := doAgentDelete(svc, "test-vs", "claude-2")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to delete agent") {
		t.Errorf("expected 'failed to delete agent' error, got: %v", err)
	}
}

func TestDoAgentDelete_NotFound(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	err := doAgentDelete(svc, "test-vs", "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDoAgentStart_StartError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	// Make update fail (scale uses update)
	addReactor(cs, "update", "deployments", fmt.Errorf("connection refused"))

	err := doAgentStart(svc, "test-vs", "claude-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to start agent") {
		t.Errorf("expected 'failed to start agent' error, got: %v", err)
	}
}

func TestDoAgentStart_StartAllError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	addReactor(cs, "update", "deployments", fmt.Errorf("connection refused"))

	err := doAgentStart(svc, "test-vs", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to start vibespace") {
		t.Errorf("expected 'failed to start vibespace' error, got: %v", err)
	}
}

func TestDoAgentStop_StopError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	addReactor(cs, "update", "deployments", fmt.Errorf("connection refused"))

	err := doAgentStop(svc, "test-vs", "claude-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to stop agent") {
		t.Errorf("expected 'failed to stop agent' error, got: %v", err)
	}
}

func TestDoAgentStop_StopAllError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	addReactor(cs, "update", "deployments", fmt.Errorf("connection refused"))

	err := doAgentStop(svc, "test-vs", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to stop vibespace") {
		t.Errorf("expected 'failed to stop vibespace' error, got: %v", err)
	}
}

// Note: TestDoAgentCreate_InvalidType is omitted because agent.ParseType()
// defaults unknown types to TypeClaudeCode, making the "invalid agent type"
// code path in doAgentCreate unreachable.

func TestDoAgentCreate_StoppedVibespace(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeStoppedDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	err := doAgentCreate(svc, "test-vs", AgentCreateOptions{AgentType: "claude-code"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "stopped") {
		t.Errorf("expected 'stopped' error, got: %v", err)
	}
}

func TestDoAgentList_Success(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	got := captureStdout(t, func() {
		err := doAgentList(svc, "test-vs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "claude-1") {
		t.Errorf("expected 'claude-1' in output, got: %s", got)
	}
}

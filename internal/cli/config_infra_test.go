package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vibespacehq/vibespace/pkg/agent"
	k8stesting "k8s.io/client-go/testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestDoConfigShow_GetVSError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	svc, cs := newFakeService(t)
	addReactor(cs, "list", "deployments", fmt.Errorf("connection refused"))

	err := doConfigShow(svc, "test-vs", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDoConfigShow_ListAgentsError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	callCount := 0
	cs.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		if callCount >= 3 {
			return true, nil, fmt.Errorf("connection refused")
		}
		return false, nil, nil
	})

	err := doConfigShow(svc, "test-vs", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to list agents") {
		t.Errorf("expected 'failed to list agents' error, got: %v", err)
	}
}

func TestDoConfigShow_GetConfigError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	// GetAgentConfig lists deployments again. Fail on the Nth call.
	callCount := 0
	cs.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		// Calls: 1=Get(name list), 2=Get(id list), 3=ListAgents, 4=GetAgentConfig list
		// Actually the flow is complex. Let's fail on call >= 5
		if callCount >= 5 {
			return true, nil, fmt.Errorf("connection refused")
		}
		return false, nil, nil
	})

	err := doConfigShow(svc, "test-vs", "claude-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get agent config") {
		t.Errorf("expected 'failed to get agent config' error, got: %v", err)
	}
}

func TestDoConfigShow_Success(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	got := captureStdout(t, func() {
		err := doConfigShow(svc, "test-vs", "claude-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(got, "claude-1") {
		t.Errorf("expected 'claude-1' in output, got: %s", got)
	}
}

func TestDoConfigShow_NonexistentAgent(t *testing.T) {
	initOutput(OutputConfig{NoColor: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	err := doConfigShow(svc, "test-vs", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDoConfigSet_GetConfigError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	callCount := 0
	cs.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		callCount++
		if callCount >= 5 {
			return true, nil, fmt.Errorf("connection refused")
		}
		return false, nil, nil
	})

	cmd := newTestConfigSetCmd()
	cmd.Flags().Set("model", "opus")

	err := doConfigSet(svc, "test-vs", "claude-1", cmd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get agent config") {
		t.Errorf("expected 'failed to get agent config' error, got: %v", err)
	}
}

func TestDoConfigSet_UpdateError(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, cs := newFakeService(t, deploy)

	// Make update fail
	addReactor(cs, "update", "deployments", fmt.Errorf("connection refused"))

	cmd := newTestConfigSetCmd()
	cmd.Flags().Set("model", "opus")

	err := doConfigSet(svc, "test-vs", "claude-1", cmd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to update agent config") {
		t.Errorf("expected 'failed to update' error, got: %v", err)
	}
}

func TestDoConfigSet_CodexRejectsSkipPermissions(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	codexAgent := fakeAgentDeployment("test-vs", "abc123", "codex-1", agent.TypeCodex, 2)
	svc, _ := newFakeService(t, deploy, codexAgent)

	cmd := newTestConfigSetCmd()
	cmd.Flags().Set("skip-permissions", "true")

	err := doConfigSet(svc, "test-vs", "codex-1", cmd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported for Codex") {
		t.Errorf("expected 'not supported for Codex' error, got: %v", err)
	}
}

func TestDoConfigSet_ClaudeRejectsReasoningEffort(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	svc, _ := newFakeService(t, deploy)

	cmd := newTestConfigSetCmd()
	cmd.Flags().Set("reasoning-effort", "high")

	err := doConfigSet(svc, "test-vs", "claude-1", cmd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "only supported for Codex") {
		t.Errorf("expected 'only supported for Codex' error, got: %v", err)
	}
}

func TestDoConfigSet_InvalidReasoningEffort(t *testing.T) {
	initOutput(OutputConfig{NoColor: true, JSONMode: true})
	deploy := fakeVibespaceDeployment("test-vs", "abc123")
	codexAgent := fakeAgentDeployment("test-vs", "abc123", "codex-1", agent.TypeCodex, 2)
	svc, _ := newFakeService(t, deploy, codexAgent)

	cmd := newTestConfigSetCmd()
	cmd.Flags().Set("reasoning-effort", "banana")

	err := doConfigSet(svc, "test-vs", "codex-1", cmd)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid reasoning effort") {
		t.Errorf("expected 'invalid reasoning effort' error, got: %v", err)
	}
}

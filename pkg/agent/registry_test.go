package agent_test

import (
	"testing"

	"github.com/yagizdagabak/vibespace/pkg/agent"
	_ "github.com/yagizdagabak/vibespace/pkg/agent/claude"
)

func TestRegistryGetKnown(t *testing.T) {
	a, err := agent.Get(agent.TypeClaudeCode)
	if err != nil {
		t.Fatalf("Get(TypeClaudeCode) returned error: %v", err)
	}
	if a == nil {
		t.Fatal("Get(TypeClaudeCode) returned nil agent")
	}
	if a.Type() != agent.TypeClaudeCode {
		t.Errorf("agent.Type() = %q, want %q", a.Type(), agent.TypeClaudeCode)
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	_, err := agent.Get(agent.Type("nonexistent"))
	if err == nil {
		t.Fatal("Get(nonexistent) should return error")
	}
}

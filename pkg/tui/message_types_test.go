package tui

import (
	"testing"
	"time"

	"github.com/vibespacehq/vibespace/pkg/session"
)

func TestMessageTypeString(t *testing.T) {
	tests := []struct {
		mt   MessageType
		want string
	}{
		{MessageTypeUser, "user"},
		{MessageTypeAssistant, "assistant"},
		{MessageTypeToolUse, "tool_use"},
		{MessageTypeError, "error"},
		{MessageTypeThinking, "thinking"},
		{MessageTypeSystem, "system"},
		{MessageType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.mt.String(); got != tt.want {
			t.Errorf("MessageType(%d).String() = %q, want %q", tt.mt, got, tt.want)
		}
	}
}

func TestNewUserMessage(t *testing.T) {
	msg := NewUserMessage("all", "hello")
	if msg.Type != MessageTypeUser {
		t.Fatalf("expected User type, got %v", msg.Type)
	}
	if msg.Sender != "You" {
		t.Fatalf("expected sender 'You', got %q", msg.Sender)
	}
	if msg.Target != "all" {
		t.Fatalf("expected target 'all', got %q", msg.Target)
	}
	if msg.Content != "hello" {
		t.Fatalf("expected content 'hello', got %q", msg.Content)
	}
	if msg.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if msg.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
}

func TestNewAssistantMessage(t *testing.T) {
	msg := NewAssistantMessage("claude-1@test", "response text")
	if msg.Type != MessageTypeAssistant {
		t.Fatalf("expected Assistant type, got %v", msg.Type)
	}
	if msg.Sender != "claude-1@test" {
		t.Fatalf("expected sender 'claude-1@test', got %q", msg.Sender)
	}
	if msg.Content != "response text" {
		t.Fatalf("expected content 'response text', got %q", msg.Content)
	}
}

func TestNewToolUseMessage(t *testing.T) {
	msg := NewToolUseMessage("agent@vs", "Bash", "ls -la")
	if msg.Type != MessageTypeToolUse {
		t.Fatalf("expected ToolUse type, got %v", msg.Type)
	}
	if msg.ToolName != "Bash" {
		t.Fatalf("expected ToolName 'Bash', got %q", msg.ToolName)
	}
	if msg.ToolInput != "ls -la" {
		t.Fatalf("expected ToolInput 'ls -la', got %q", msg.ToolInput)
	}
}

func TestNewErrorMessage(t *testing.T) {
	msg := NewErrorMessage("agent@vs", "something failed")
	if msg.Type != MessageTypeError {
		t.Fatalf("expected Error type, got %v", msg.Type)
	}
	if msg.Content != "something failed" {
		t.Fatalf("expected content 'something failed', got %q", msg.Content)
	}
}

func TestNewSystemMessage(t *testing.T) {
	msg := NewSystemMessage("system info")
	if msg.Type != MessageTypeSystem {
		t.Fatalf("expected System type, got %v", msg.Type)
	}
	if msg.Sender != "system" {
		t.Fatalf("expected sender 'system', got %q", msg.Sender)
	}
}

func TestAgentStateSetThinking(t *testing.T) {
	state := NewAgentState(session.AgentAddress{Agent: "claude-1", Vibespace: "test"})

	state.SetThinking(true)
	if !state.IsThinking {
		t.Fatal("expected IsThinking=true")
	}
	if state.ThinkingAt.IsZero() {
		t.Fatal("expected non-zero ThinkingAt")
	}

	state.SetThinking(false)
	if state.IsThinking {
		t.Fatal("expected IsThinking=false")
	}
}

func TestThinkingIndicatorText(t *testing.T) {
	state := NewAgentState(session.AgentAddress{Agent: "claude-1", Vibespace: "test"})

	// Not thinking → empty
	if got := state.ThinkingIndicatorText(); got != "" {
		t.Fatalf("expected empty for non-thinking, got %q", got)
	}

	// Thinking → spinner frame
	state.SetThinking(true)
	state.ThinkingAt = time.Now().Add(-200 * time.Millisecond) // Force a specific frame
	got := state.ThinkingIndicatorText()
	if got == "" {
		t.Fatal("expected non-empty spinner frame for thinking state")
	}
}

func TestNewAgentState(t *testing.T) {
	addr := session.AgentAddress{Agent: "claude-1", Vibespace: "test"}
	state := NewAgentState(addr)

	if state.Address != addr {
		t.Fatalf("expected address %v, got %v", addr, state.Address)
	}
	if state.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if state.IsThinking {
		t.Fatal("expected IsThinking=false initially")
	}
}

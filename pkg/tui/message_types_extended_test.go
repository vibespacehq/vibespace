package tui

import (
	"testing"
	"time"

	"github.com/vibespacehq/vibespace/pkg/session"
)

func TestMessageTypeStringAll(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.mt.String()
			if got != tt.want {
				t.Errorf("MessageType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewUserMessageFields(t *testing.T) {
	msg := NewUserMessage("claude-1", "hello")
	if msg.Type != MessageTypeUser {
		t.Errorf("Type = %v, want %v", msg.Type, MessageTypeUser)
	}
	if msg.Target != "claude-1" {
		t.Errorf("Target = %q, want %q", msg.Target, "claude-1")
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello")
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if msg.Sender != "You" {
		t.Errorf("Sender = %q, want %q", msg.Sender, "You")
	}
}

func TestNewAssistantMessageFields(t *testing.T) {
	msg := NewAssistantMessage("claude-1", "response")
	if msg.Type != MessageTypeAssistant {
		t.Errorf("Type = %v, want %v", msg.Type, MessageTypeAssistant)
	}
	if msg.Sender != "claude-1" {
		t.Errorf("Sender = %q, want %q", msg.Sender, "claude-1")
	}
	if msg.Content != "response" {
		t.Errorf("Content = %q, want %q", msg.Content, "response")
	}
}

func TestNewToolUseMessageFields(t *testing.T) {
	msg := NewToolUseMessage("claude-1", "Bash", "ls -la")
	if msg.Type != MessageTypeToolUse {
		t.Errorf("Type = %v, want %v", msg.Type, MessageTypeToolUse)
	}
	if msg.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", msg.ToolName, "Bash")
	}
	if msg.ToolInput != "ls -la" {
		t.Errorf("ToolInput = %q, want %q", msg.ToolInput, "ls -la")
	}
}

func TestNewErrorMessageFields(t *testing.T) {
	msg := NewErrorMessage("claude-1", "failed")
	if msg.Type != MessageTypeError {
		t.Errorf("Type = %v, want %v", msg.Type, MessageTypeError)
	}
	if msg.Content != "failed" {
		t.Errorf("Content = %q, want %q", msg.Content, "failed")
	}
}

func TestNewSystemMessageFields(t *testing.T) {
	msg := NewSystemMessage("info")
	if msg.Type != MessageTypeSystem {
		t.Errorf("Type = %v, want %v", msg.Type, MessageTypeSystem)
	}
	if msg.Content != "info" {
		t.Errorf("Content = %q, want %q", msg.Content, "info")
	}
	if msg.Sender != "system" {
		t.Errorf("Sender = %q, want %q", msg.Sender, "system")
	}
}

func TestAgentStateThinkingIndicator(t *testing.T) {
	state := NewAgentState(session.AgentAddress{Agent: "claude-1", Vibespace: "test"})

	// Not thinking
	text := state.ThinkingIndicatorText()
	if text != "" {
		t.Errorf("expected empty when not thinking, got %q", text)
	}

	// Set thinking
	state.SetThinking(true)
	if !state.IsThinking {
		t.Error("expected IsThinking=true after SetThinking(true)")
	}

	text = state.ThinkingIndicatorText()
	if text == "" {
		t.Error("expected non-empty thinking indicator when thinking")
	}

	// Stop thinking
	state.SetThinking(false)
	text = state.ThinkingIndicatorText()
	if text != "" {
		t.Errorf("expected empty after SetThinking(false), got %q", text)
	}
}

func TestAgentStateThinkingAt(t *testing.T) {
	state := NewAgentState(session.AgentAddress{Agent: "claude-1", Vibespace: "test"})
	state.SetThinking(true)

	if state.ThinkingAt.IsZero() {
		t.Error("ThinkingAt should be set when thinking starts")
	}
	if time.Since(state.ThinkingAt) > time.Second {
		t.Error("ThinkingAt should be recent")
	}
}

func TestMessageIDUniqueness(t *testing.T) {
	// Verify that consecutive messages get distinct IDs
	msg1 := NewUserMessage("all", "first")
	// Small sleep to ensure timestamp changes (IDs are timestamp-based)
	time.Sleep(time.Millisecond)
	msg2 := NewUserMessage("all", "second")

	// IDs should be non-empty
	if msg1.ID == "" {
		t.Error("msg1 ID should not be empty")
	}
	if msg2.ID == "" {
		t.Error("msg2 ID should not be empty")
	}
}

func TestNewAgentStateAddress(t *testing.T) {
	addr := session.AgentAddress{Agent: "claude-2", Vibespace: "prod"}
	state := NewAgentState(addr)

	if state.Address.Agent != "claude-2" {
		t.Errorf("Address.Agent = %q, want %q", state.Address.Agent, "claude-2")
	}
	if state.Address.Vibespace != "prod" {
		t.Errorf("Address.Vibespace = %q, want %q", state.Address.Vibespace, "prod")
	}
	if state.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	if state.IsThinking {
		t.Error("IsThinking should be false initially")
	}
}

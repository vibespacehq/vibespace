package tui

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"vibespace/pkg/session"
)

// MessageType represents the type of a chat message
type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeAssistant
	MessageTypeToolUse
	MessageTypeError
	MessageTypeThinking
	MessageTypeSystem
)

// String returns the string representation of the message type
func (t MessageType) String() string {
	switch t {
	case MessageTypeUser:
		return "user"
	case MessageTypeAssistant:
		return "assistant"
	case MessageTypeToolUse:
		return "tool_use"
	case MessageTypeError:
		return "error"
	case MessageTypeThinking:
		return "thinking"
	case MessageTypeSystem:
		return "system"
	default:
		return "unknown"
	}
}

// Message represents a single message in the chat history
type Message struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	Sender    string      `json:"sender"`     // "You" or "claude-1@test"
	Target    string      `json:"target"`     // "all" or "claude-1@test" (for user messages)
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
	ToolName  string      `json:"tool_name,omitempty"`  // For tool use messages
	ToolInput string      `json:"tool_input,omitempty"` // For tool use messages (e.g., file path)
}

// NewUserMessage creates a new user message
func NewUserMessage(target, content string) *Message {
	return &Message{
		ID:        generateMessageID(),
		Type:      MessageTypeUser,
		Sender:    "You",
		Target:    target,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewAssistantMessage creates a new assistant message
func NewAssistantMessage(sender, content string) *Message {
	return &Message{
		ID:        generateMessageID(),
		Type:      MessageTypeAssistant,
		Sender:    sender,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewToolUseMessage creates a new tool use message
func NewToolUseMessage(sender, toolName, toolInput string) *Message {
	return &Message{
		ID:        generateMessageID(),
		Type:      MessageTypeToolUse,
		Sender:    sender,
		ToolName:  toolName,
		ToolInput: toolInput,
		Timestamp: time.Now(),
	}
}

// NewErrorMessage creates a new error message
func NewErrorMessage(sender, content string) *Message {
	return &Message{
		ID:        generateMessageID(),
		Type:      MessageTypeError,
		Sender:    sender,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewSystemMessage creates a new system message
func NewSystemMessage(content string) *Message {
	return &Message{
		ID:        generateMessageID(),
		Type:      MessageTypeSystem,
		Sender:    "system",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return time.Now().Format("20060102150405.000000000")
}

// AgentState tracks the current state of an agent connection
type AgentState struct {
	Address    session.AgentAddress
	SessionID  string // Claude --session-id UUID for conversation continuity
	IsThinking bool   // True when waiting for Claude response
	ThinkingAt time.Time // When thinking started (for animation)
}

// NewAgentState creates a new agent state with a generated session ID
func NewAgentState(addr session.AgentAddress) *AgentState {
	return &AgentState{
		Address:   addr,
		SessionID: generateSessionID(),
	}
}

// generateSessionID generates a valid UUID v4 for session continuity
func generateSessionID() string {
	// Generate 16 random bytes
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		// Fallback to time-based if crypto/rand fails (shouldn't happen)
		return time.Now().Format("20060102150405.000000000")
	}

	// Set version to 4 (random UUID)
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant to RFC 4122
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	// Format as UUID string
	return hex.EncodeToString(uuid[0:4]) + "-" +
		hex.EncodeToString(uuid[4:6]) + "-" +
		hex.EncodeToString(uuid[6:8]) + "-" +
		hex.EncodeToString(uuid[8:10]) + "-" +
		hex.EncodeToString(uuid[10:16])
}

// SetThinking sets the agent to thinking state
func (s *AgentState) SetThinking(thinking bool) {
	s.IsThinking = thinking
	if thinking {
		s.ThinkingAt = time.Now()
	}
}

// Spinner frames for thinking animation (braille pattern)
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ThinkingIndicatorText returns animated thinking indicator based on elapsed time
func (s *AgentState) ThinkingIndicatorText() string {
	if !s.IsThinking {
		return ""
	}
	elapsed := time.Since(s.ThinkingAt)
	frame := int(elapsed.Milliseconds()/100) % len(spinnerFrames)
	return spinnerFrames[frame]
}

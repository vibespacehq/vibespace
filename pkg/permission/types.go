// Package permission provides types and server for handling Claude Code permission requests.
package permission

import (
	"encoding/json"
	"time"
)

// Request represents a permission request from a Claude agent.
type Request struct {
	ID        string          `json:"id"`
	AgentKey  string          `json:"agent_key"` // "claude-1@project"
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Timestamp time.Time       `json:"timestamp"`
}

// Response represents a permission response sent back to the hook.
type Response struct {
	ID       string   `json:"id"`
	Decision Decision `json:"decision"`
	Reason   string   `json:"reason,omitempty"`
}

// Decision represents the user's decision on a permission request.
type Decision string

const (
	// DecisionAllow allows the tool to execute.
	DecisionAllow Decision = "allow"
	// DecisionDeny denies the tool execution.
	DecisionDeny Decision = "deny"
)

// HookInput represents the JSON input received from the Claude PermissionRequest hook.
// This maps to the structure Claude Code sends to permission hooks.
type HookInput struct {
	HookEventName string `json:"hookEventName"`
	ToolName      string `json:"toolName"`
	ToolInput     string `json:"toolInput,omitempty"`
	// Additional fields Claude might send
	SessionID string `json:"sessionId,omitempty"`
}

// HookOutput represents the JSON output returned to Claude's PermissionRequest hook.
type HookOutput struct {
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput"`
}

// HookSpecificOutput contains the permission decision for Claude.
type HookSpecificOutput struct {
	HookEventName string       `json:"hookEventName"`
	Decision      HookDecision `json:"decision"`
}

// HookDecision contains the behavior Claude should take.
type HookDecision struct {
	Behavior string `json:"behavior"` // "allow" or "deny"
}

// NewHookOutput creates a hook output with the given decision.
func NewHookOutput(decision Decision) HookOutput {
	return HookOutput{
		HookSpecificOutput: HookSpecificOutput{
			HookEventName: "PermissionRequest",
			Decision: HookDecision{
				Behavior: string(decision),
			},
		},
	}
}

// Package agent provides a pluggable abstraction for different AI coding agents.
// It allows vibespace to support multiple agent types (Claude Code, Codex, etc.)
// through a common interface.
package agent

import (
	"encoding/json"
	"strings"
)

// Type represents the type of coding agent.
type Type string

const (
	// TypeClaudeCode is Anthropic's Claude Code CLI agent.
	TypeClaudeCode Type = "claude-code"
	// TypeCodex is OpenAI's Codex CLI agent.
	TypeCodex Type = "codex"
)

// ParseType parses a string into an agent Type.
// Returns TypeClaudeCode if the string is empty or unrecognized.
func ParseType(s string) Type {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "claude-code", "claude":
		return TypeClaudeCode
	case "codex":
		return TypeCodex
	default:
		return TypeClaudeCode
	}
}

// IsValid returns true if the type is a recognized agent type.
func (t Type) IsValid() bool {
	switch t {
	case TypeClaudeCode, TypeCodex:
		return true
	default:
		return false
	}
}

// String returns the string representation of the type.
func (t Type) String() string {
	return string(t)
}

// Config holds agent-specific configuration.
type Config struct {
	// SkipPermissions disables permission prompts
	// Claude: --dangerously-skip-permissions
	// Codex: --yolo / --dangerously-bypass-approvals-and-sandbox
	SkipPermissions bool `json:"skip_permissions,omitempty"`

	// AllowedTools restricts which tools the agent can use
	// Claude: --allowedTools
	// Codex: sandbox policies
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// DisallowedTools prevents specific tools
	// Claude: --disallowedTools
	// Codex: sandbox policies
	DisallowedTools []string `json:"disallowed_tools,omitempty"`

	// Model specifies the AI model to use
	// Claude: --model claude-sonnet-4-...
	// Codex: --model gpt-5-codex
	Model string `json:"model,omitempty"`

	// MaxTurns limits conversation turns
	// Claude: --max-turns
	// Codex: config file max_turns
	MaxTurns int `json:"max_turns,omitempty"`

	// SystemPrompt adds a system prompt
	// Claude: --system-prompt
	// Codex: config file
	SystemPrompt string `json:"system_prompt,omitempty"`

	// ShareCredentials enables credential sharing via /vibespace/.vibespace
	ShareCredentials bool `json:"share_credentials,omitempty"`

	// Extra holds agent-specific extensions that don't fit the common fields
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// DefaultAllowedTools returns the default tool list for agents.
// This provides a restrictive baseline that can be customized per-agent type.
func DefaultAllowedTools() []string {
	return []string{
		"Bash(read_only:true)",
		"Read",
		"Write",
		"Edit",
		"Glob",
		"Grep",
	}
}

// GetAllowedTools returns allowed tools or default if empty.
func (c *Config) GetAllowedTools() []string {
	if c == nil || len(c.AllowedTools) == 0 {
		return DefaultAllowedTools()
	}
	return c.AllowedTools
}

// AllowedToolsString returns comma-separated allowed tools.
func (c *Config) AllowedToolsString() string {
	return strings.Join(c.GetAllowedTools(), ",")
}

// DisallowedToolsString returns comma-separated disallowed tools.
func (c *Config) DisallowedToolsString() string {
	if c == nil {
		return ""
	}
	return strings.Join(c.DisallowedTools, ",")
}

// IsEmpty returns true if the config has no custom settings.
func (c *Config) IsEmpty() bool {
	if c == nil {
		return true
	}
	return !c.SkipPermissions &&
		len(c.AllowedTools) == 0 &&
		len(c.DisallowedTools) == 0 &&
		c.Model == "" &&
		c.MaxTurns == 0 &&
		c.SystemPrompt == ""
}

// Clone creates a deep copy of the config.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := &Config{
		SkipPermissions:  c.SkipPermissions,
		Model:            c.Model,
		MaxTurns:         c.MaxTurns,
		SystemPrompt:     c.SystemPrompt,
		ShareCredentials: c.ShareCredentials,
	}
	if len(c.AllowedTools) > 0 {
		clone.AllowedTools = make([]string, len(c.AllowedTools))
		copy(clone.AllowedTools, c.AllowedTools)
	}
	if len(c.DisallowedTools) > 0 {
		clone.DisallowedTools = make([]string, len(c.DisallowedTools))
		copy(clone.DisallowedTools, c.DisallowedTools)
	}
	if len(c.Extra) > 0 {
		clone.Extra = make(map[string]interface{})
		for k, v := range c.Extra {
			clone.Extra[k] = v
		}
	}
	return clone
}

// Merge merges another config into this one. Non-zero values in other override this.
func (c *Config) Merge(other *Config) *Config {
	if other == nil {
		return c
	}
	if c == nil {
		return other.Clone()
	}

	result := c.Clone()
	if other.SkipPermissions {
		result.SkipPermissions = true
	}
	if len(other.AllowedTools) > 0 {
		result.AllowedTools = make([]string, len(other.AllowedTools))
		copy(result.AllowedTools, other.AllowedTools)
	}
	if len(other.DisallowedTools) > 0 {
		result.DisallowedTools = make([]string, len(other.DisallowedTools))
		copy(result.DisallowedTools, other.DisallowedTools)
	}
	if other.Model != "" {
		result.Model = other.Model
	}
	if other.MaxTurns > 0 {
		result.MaxTurns = other.MaxTurns
	}
	if other.SystemPrompt != "" {
		result.SystemPrompt = other.SystemPrompt
	}
	if other.ShareCredentials {
		result.ShareCredentials = true
	}
	if len(other.Extra) > 0 {
		if result.Extra == nil {
			result.Extra = make(map[string]interface{})
		}
		for k, v := range other.Extra {
			result.Extra[k] = v
		}
	}
	return result
}

// StreamMessage represents a parsed message from an agent's streaming output.
type StreamMessage struct {
	// Type of message: "text", "tool_use", "tool_result", "error", "system", "done", "session_started"
	Type string

	// Text content (for text and error types)
	Text string

	// Tool information (for tool_use type)
	ToolName  string
	ToolInput string
	ToolID    string

	// Result indicates completion (for done type)
	IsError bool
	Result  string

	// SessionID is the agent's session/thread ID (for session_started type)
	// Codex auto-generates session IDs, so we capture them from thread.started events
	SessionID string

	// Raw holds the original JSON for agent-specific parsing
	Raw json.RawMessage
}

// CodingAgent is the interface that all coding agents must implement.
// It provides methods for building commands, parsing output, and
// getting agent-specific configuration.
type CodingAgent interface {
	// Identity
	Type() Type
	DisplayName() string        // Human-readable name: "Claude Code" or "Codex"
	DefaultAgentPrefix() string // Prefix for agent naming: "claude" or "codex"

	// Container
	ContainerImage() string  // Docker image for this agent
	ConfigDirectory() string // Config dir inside container: ".claude" or ".codex"

	// Command building
	// BuildPrintModeCommand builds a command for non-interactive streaming mode.
	// sessionID is the session identifier for continuity.
	// resume indicates whether to resume an existing session.
	// config contains agent configuration.
	BuildPrintModeCommand(sessionID string, resume bool, config *Config) string

	// BuildInteractiveCommand builds a command for interactive terminal mode.
	// sessionID is optional - if provided, resumes that session.
	// config contains agent configuration.
	BuildInteractiveCommand(sessionID string, config *Config) string

	// Session handling
	// SessionIDFlag returns the flag used to specify a new session ID.
	// For Claude: "--session-id", for Codex: "" (auto-generated)
	SessionIDFlag() string

	// ResumeFlag returns the flag/command used to resume a session.
	// For Claude: "--resume", for Codex: "resume" (subcommand arg)
	ResumeFlag() string

	// Output parsing
	// ParseStreamLine parses a single line of streaming output.
	// Returns a StreamMessage and whether the line was valid JSON.
	ParseStreamLine(line string) (*StreamMessage, bool)

	// SupportedTools returns the list of tools this agent supports.
	// Used for TUI display and validation.
	SupportedTools() []string

	// Configuration
	// DefaultConfig returns the default configuration for this agent type.
	DefaultConfig() *Config

	// ValidateConfig validates agent-specific configuration.
	// Returns an error if the config is invalid.
	ValidateConfig(config *Config) error
}

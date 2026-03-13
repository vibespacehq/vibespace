// Package agent provides a pluggable abstraction for different AI coding agents.
// It allows vibespace to support multiple agent types (Claude Code, Codex, etc.)
// through a common interface.
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vibespacehq/vibespace/pkg/config"
)

// DoubleQuoteForBash escapes a string with double quotes for safe embedding inside
// an existing single-quoted bash -c context. It escapes characters that are special
// inside double quotes ($, `, ", \, !) and wraps the result in double quotes.
// This is safe at level 1 (outer shell) because no single quotes are produced,
// and safe at level 2 (inner bash) because special chars are escaped.
func DoubleQuoteForBash(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		"`", "\\`",
		`!`, `\!`,
	)
	return `"` + r.Replace(s) + `"`
}

// JoinArgsForBash joins command args with proper double-quote escaping for embedding
// inside a bash -c single-quoted context. Known-safe args (flags starting with -)
// are not quoted; all other values are double-quoted.
func JoinArgsForBash(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") || arg == "claude" || arg == "codex" || arg == "resume" || arg == "exec" {
			// Known-safe flag/command names and subcommands — pass through unquoted
			parts[i] = arg
		} else {
			parts[i] = DoubleQuoteForBash(arg)
		}
	}
	return strings.Join(parts, " ")
}

// ShellQuote escapes a string for safe embedding in a single-quoted shell context.
// Uses the standard approach: replace ' with '\” (end quote, literal quote, start quote).
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ShellQuoteArgs joins a list of arguments into a properly shell-quoted command string.
// Each argument is individually single-quote-escaped, making the result safe
// to interpolate into a shell command without injection risk.
func ShellQuoteArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = ShellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

// WrapForSSHRemote builds a shell command suitable for SSH remote execution.
// Wraps the given args in `bash -l -c 'cd "$VIBESPACE_WORKDIR" && <quoted_args>'`
// with proper escaping to prevent command injection.
// Uses double-quoting inside the outer single-quote context since single quotes
// cannot nest in bash.
func WrapForSSHRemote(args []string) string {
	innerCmd := JoinArgsForBash(args)
	return fmt.Sprintf(`bash -l -c 'cd "$VIBESPACE_WORKDIR" && %s'`, innerCmd)
}

// WrapForTmuxSSH builds a shell command for launching via tmux over SSH.
// Uses tmux new-session -A to attach or create, with proper escaping.
func WrapForTmuxSSH(tmuxSession string, args []string) string {
	innerCmd := JoinArgsForBash(args)
	// Build the bash command that tmux will run.
	// Uses single quotes around bash -c arg (protects $ and " from premature expansion).
	// ShellQuote wraps the whole thing so tmux receives it as a single argument,
	// escaping the inner single quotes via the '\'' idiom.
	bashCmd := fmt.Sprintf(`bash -l -c 'cd "$VIBESPACE_WORKDIR" && %s'`, innerCmd)
	return fmt.Sprintf(`TERM=xterm-256color tmux new-session -A -s %s %s`,
		ShellQuote(tmuxSession),
		ShellQuote(bashCmd))
}

// Type represents the type of coding agent.
type Type string

const (
	// TypeClaudeCode is Anthropic's Claude Code CLI agent.
	TypeClaudeCode Type = "claude-code"
	// TypeCodex is OpenAI's Codex CLI agent.
	TypeCodex Type = "codex"
)

// AllTypes returns all valid agent types.
func AllTypes() []Type {
	return []Type{TypeClaudeCode, TypeCodex}
}

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

	// ReasoningEffort controls how much the model "thinks" before responding
	// Codex: -c model_reasoning_effort=<low|medium|high|xhigh>
	// Claude: not supported (uses extended thinking automatically)
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// ShareCredentials enables credential sharing via /vibespace/.vibespace
	ShareCredentials bool `json:"share_credentials,omitempty"`

	// Extra holds agent-specific extensions that don't fit the common fields
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// DefaultAllowedTools returns the default tool list for agents.
// This provides a restrictive baseline that can be customized per-agent type.
func DefaultAllowedTools() []string {
	return config.Global().Agent.AllowedTools
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
		ReasoningEffort:  c.ReasoningEffort,
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
	if other.ReasoningEffort != "" {
		result.ReasoningEffort = other.ReasoningEffort
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
	// Type of message: "text", "text_delta", "tool_use", "tool_result", "error", "system", "done", "session_started"
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
	// streaming enables token-by-token output (e.g. --include-partial-messages for Claude).
	BuildPrintModeCommand(sessionID string, resume bool, config *Config, streaming bool) string

	// BuildInteractiveCommand builds a command for interactive terminal mode.
	// Returns the argument list (not a shell string) to prevent command injection.
	// Callers must use ShellQuoteArgs or WrapForSSHRemote to convert to a shell string.
	// sessionID is optional - if provided, resumes that session.
	// config contains agent configuration.
	BuildInteractiveCommand(sessionID string, config *Config) []string

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

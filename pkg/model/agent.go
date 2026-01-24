package model

import "strings"

// ClaudeConfig holds Claude Code-specific configuration for an agent.
// Used when spawning agents and building claude commands.
type ClaudeConfig struct {
	// SkipPermissions disables permission prompts (--dangerously-skip-permissions)
	SkipPermissions bool `json:"skip_permissions,omitempty"`

	// AllowedTools restricts which tools Claude can use (--allowedTools)
	// Empty = use default: Bash(read_only:true),Read,Write,Edit,Glob,Grep
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// DisallowedTools prevents specific tools (--disallowedTools)
	DisallowedTools []string `json:"disallowed_tools,omitempty"`

	// Model specifies the Claude model (--model)
	Model string `json:"model,omitempty"`

	// MaxTurns limits conversation turns (--max-turns)
	MaxTurns int `json:"max_turns,omitempty"`

	// SystemPrompt adds a system prompt (--system-prompt)
	SystemPrompt string `json:"system_prompt,omitempty"`

	// ShareCredentials enables credential sharing via /vibespace/.vibespace
	ShareCredentials bool `json:"share_credentials,omitempty"`
}

// DefaultAllowedTools returns the default tool list for Claude agents.
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
func (c *ClaudeConfig) GetAllowedTools() []string {
	if c == nil || len(c.AllowedTools) == 0 {
		return DefaultAllowedTools()
	}
	return c.AllowedTools
}

// AllowedToolsString returns comma-separated allowed tools.
func (c *ClaudeConfig) AllowedToolsString() string {
	return strings.Join(c.GetAllowedTools(), ",")
}

// DisallowedToolsString returns comma-separated disallowed tools.
func (c *ClaudeConfig) DisallowedToolsString() string {
	if c == nil {
		return ""
	}
	return strings.Join(c.DisallowedTools, ",")
}

// IsEmpty returns true if the config has no custom settings.
func (c *ClaudeConfig) IsEmpty() bool {
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
func (c *ClaudeConfig) Clone() *ClaudeConfig {
	if c == nil {
		return nil
	}
	clone := &ClaudeConfig{
		SkipPermissions: c.SkipPermissions,
		Model:           c.Model,
		MaxTurns:        c.MaxTurns,
		SystemPrompt:    c.SystemPrompt,
	}
	if len(c.AllowedTools) > 0 {
		clone.AllowedTools = make([]string, len(c.AllowedTools))
		copy(clone.AllowedTools, c.AllowedTools)
	}
	if len(c.DisallowedTools) > 0 {
		clone.DisallowedTools = make([]string, len(c.DisallowedTools))
		copy(clone.DisallowedTools, c.DisallowedTools)
	}
	return clone
}

// Merge merges another config into this one. Non-zero values in other override this.
func (c *ClaudeConfig) Merge(other *ClaudeConfig) *ClaudeConfig {
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
	return result
}

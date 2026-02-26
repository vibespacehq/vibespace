// Package claude implements the Claude Code agent for vibespace.
package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vibespacehq/vibespace/pkg/agent"
	vsconfig "github.com/vibespacehq/vibespace/pkg/config"
)

func init() {
	agent.Register(agent.TypeClaudeCode, func() agent.CodingAgent {
		return &Agent{}
	})
}

// Agent implements agent.CodingAgent for Claude Code CLI.
type Agent struct{}

// Ensure Agent implements CodingAgent.
var _ agent.CodingAgent = (*Agent)(nil)

// Type returns the agent type.
func (a *Agent) Type() agent.Type {
	return agent.TypeClaudeCode
}

// DisplayName returns the human-readable name.
func (a *Agent) DisplayName() string {
	return "Claude Code"
}

// DefaultAgentPrefix returns the prefix used for agent naming.
func (a *Agent) DefaultAgentPrefix() string {
	return vsconfig.Global().Agent.Prefixes.Claude
}

// ContainerImage returns the Docker image for Claude Code.
func (a *Agent) ContainerImage() string {
	return vsconfig.Global().Images.Claude
}

// ConfigDirectory returns the config directory inside the container.
func (a *Agent) ConfigDirectory() string {
	return ".claude"
}

// BuildPrintModeCommand builds the Claude print-mode command for streaming output.
func (a *Agent) BuildPrintModeCommand(sessionID string, resume bool, config *agent.Config) string {
	args := []string{"claude", "-p", "--verbose", "--output-format", "stream-json"}

	// Session handling
	if resume {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}

	// Apply configuration
	args = a.applyConfig(args, config)

	// Wrap in bash -l -c to ensure proper shell environment and cd to /vibespace
	// Inject permission hook settings into project-level .claude/settings.json
	// so the hook only runs when the TUI permission server is reachable via reverse tunnel.
	// Clean up on exit so direct SSH sessions aren't affected.
	hookTimeout := int(vsconfig.Global().Timeouts.PermissionHook.Duration.Milliseconds())
	setupHook := fmt.Sprintf(`mkdir -p /vibespace/.claude && cat > /vibespace/.claude/settings.json << 'HOOKEOF'
{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/home/user/.local/bin/vibespace-permission-hook","timeout":%d}]}]}}
HOOKEOF
`, hookTimeout)
	cleanupHook := `rm -f /vibespace/.claude/settings.json`
	claudeCmd := strings.Join(args, " ")
	return fmt.Sprintf(`bash -l -c '%s trap "%s" EXIT; cd /vibespace && %s'`, setupHook, cleanupHook, claudeCmd)
}

// BuildInteractiveCommand builds a Claude command for interactive terminal mode.
func (a *Agent) BuildInteractiveCommand(sessionID string, config *agent.Config) string {
	args := []string{"claude"}

	// Session handling - interactive mode uses --resume for existing sessions
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}

	// Apply configuration
	args = a.applyConfig(args, config)

	return strings.Join(args, " ")
}

// applyConfig adds configuration flags to the command arguments.
func (a *Agent) applyConfig(args []string, config *agent.Config) []string {
	if config == nil {
		// Fallback: no config available, use restrictive defaults
		args = append(args, "--allowedTools", `"Bash(read_only:true),Read,Write,Edit,Glob,Grep"`)
		return args
	}

	if config.SkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	if len(config.AllowedTools) > 0 {
		// Explicit allowed tools always take precedence
		args = append(args, "--allowedTools", fmt.Sprintf(`"%s"`, config.AllowedToolsString()))
	} else if !config.SkipPermissions {
		// Only use restrictive defaults if NOT skipping permissions
		// With skip_permissions, omit --allowedTools for full access
		args = append(args, "--allowedTools", fmt.Sprintf(`"%s"`, strings.Join(agent.DefaultAllowedTools(), ",")))
	}

	if len(config.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", fmt.Sprintf(`"%s"`, config.DisallowedToolsString()))
	}

	if config.Model != "" {
		args = append(args, "--model", config.Model)
	}

	if config.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", config.MaxTurns))
	}

	return args
}

// SessionIDFlag returns the flag used to specify a new session ID.
func (a *Agent) SessionIDFlag() string {
	return "--session-id"
}

// ResumeFlag returns the flag used to resume a session.
func (a *Agent) ResumeFlag() string {
	return "--resume"
}

// ParseStreamLine parses a line of Claude's stream-json output.
func (a *Agent) ParseStreamLine(line string) (*agent.StreamMessage, bool) {
	if line == "" {
		return nil, false
	}

	var msg claudeStreamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		// Not valid JSON - return as raw text
		return &agent.StreamMessage{
			Type: "text",
			Text: line,
		}, false
	}

	result := &agent.StreamMessage{
		Raw: json.RawMessage(line),
	}

	switch msg.Type {
	case "assistant":
		// Extract text and tool_use from content blocks
		for _, block := range msg.Message.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					result.Type = "text"
					result.Text = block.Text
					return result, true
				}
			case "tool_use":
				result.Type = "tool_use"
				result.ToolName = block.Name
				result.ToolID = block.ID
				result.ToolInput = extractToolInput(block.Name, block.Input)
				return result, true
			}
		}

	case "tool_use":
		if msg.Tool.Name != "" {
			result.Type = "tool_use"
			result.ToolName = msg.Tool.Name
			result.ToolID = msg.Tool.ID
			result.ToolInput = extractToolInput(msg.Tool.Name, msg.Tool.Input)
			return result, true
		}
		if msg.ContentBlock.Type == "tool_use" && msg.ContentBlock.Name != "" {
			result.Type = "tool_use"
			result.ToolName = msg.ContentBlock.Name
			result.ToolID = msg.ContentBlock.ID
			result.ToolInput = extractToolInput(msg.ContentBlock.Name, msg.ContentBlock.Input)
			return result, true
		}

	case "content_block_start":
		if msg.ContentBlock.Type == "tool_use" && msg.ContentBlock.Name != "" {
			result.Type = "tool_use"
			result.ToolName = msg.ContentBlock.Name
			result.ToolID = msg.ContentBlock.ID
			result.ToolInput = extractToolInput(msg.ContentBlock.Name, msg.ContentBlock.Input)
			return result, true
		}

	case "content_block_delta":
		if msg.ContentBlock.Text != "" {
			result.Type = "text"
			result.Text = msg.ContentBlock.Text
			return result, true
		}

	case "result":
		result.Type = "done"
		result.IsError = msg.IsError
		result.Result = msg.Result
		return result, true

	case "error":
		result.Type = "error"
		result.Text = msg.Error
		result.IsError = true
		return result, true

	case "system":
		if msg.Subtype != "init" && msg.Result != "" {
			result.Type = "system"
			result.Text = msg.Result
			return result, true
		}
	}

	return nil, true
}

// SupportedTools returns the list of tools Claude Code supports.
func (a *Agent) SupportedTools() []string {
	return []string{
		"Read", "Write", "Edit",
		"Bash", "Bash(read_only:true)",
		"Glob", "Grep",
		"Task", "WebFetch", "WebSearch", "NotebookEdit",
	}
}

// DefaultConfig returns the default configuration for Claude Code.
func (a *Agent) DefaultConfig() *agent.Config {
	return &agent.Config{
		AllowedTools: agent.DefaultAllowedTools(),
	}
}

// ValidateConfig validates Claude-specific configuration.
func (a *Agent) ValidateConfig(config *agent.Config) error {
	if config == nil {
		return nil
	}
	// Claude Code doesn't have strict validation requirements
	return nil
}

// claudeStreamMessage represents a streaming JSON message from Claude.
type claudeStreamMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Result  string `json:"result,omitempty"`

	// For assistant messages
	Message struct {
		Role    string         `json:"role"`
		Content []contentBlock `json:"content"`
	} `json:"message,omitempty"`

	// For content_block_delta events
	ContentBlock contentBlock `json:"content_block,omitempty"`

	// For tool_use events
	Tool struct {
		Name  string          `json:"name"`
		ID    string          `json:"id"`
		Input json.RawMessage `json:"input"`
	} `json:"tool,omitempty"`

	// For errors
	Error   string `json:"error,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// contentBlock represents a content block in Claude's response.
type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// extractToolInput extracts relevant input from tool parameters.
func extractToolInput(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	switch toolName {
	case "Read":
		var p struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal(input, &p) == nil {
			return p.FilePath
		}

	case "Bash":
		var p struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(input, &p) == nil {
			return truncateString(p.Command, 50)
		}

	case "Grep":
		var p struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if json.Unmarshal(input, &p) == nil {
			if p.Path != "" {
				return fmt.Sprintf("%s in %s", p.Pattern, p.Path)
			}
			return p.Pattern
		}

	case "Edit":
		var p struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal(input, &p) == nil {
			return p.FilePath
		}

	case "Write":
		var p struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal(input, &p) == nil {
			return p.FilePath
		}

	case "Glob":
		var p struct {
			Pattern string `json:"pattern"`
		}
		if json.Unmarshal(input, &p) == nil {
			return p.Pattern
		}

	case "Task":
		var p struct {
			Description string `json:"description"`
		}
		if json.Unmarshal(input, &p) == nil {
			return p.Description
		}

	case "WebFetch":
		var p struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(input, &p) == nil {
			return truncateString(p.URL, 40)
		}

	case "WebSearch":
		var p struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(input, &p) == nil {
			return p.Query
		}
	}

	return ""
}

// truncateString truncates a string to max length with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Package codex implements the OpenAI Codex CLI agent for vibespace.
package codex

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/agent"
)

func init() {
	agent.Register(agent.TypeCodex, func() agent.CodingAgent {
		return &Agent{}
	})
}

// Agent implements agent.CodingAgent for OpenAI Codex CLI.
type Agent struct{}

// Ensure Agent implements CodingAgent.
var _ agent.CodingAgent = (*Agent)(nil)

// Type returns the agent type.
func (a *Agent) Type() agent.Type {
	return agent.TypeCodex
}

// DisplayName returns the human-readable name.
func (a *Agent) DisplayName() string {
	return "Codex CLI"
}

// DefaultAgentPrefix returns the prefix used for agent naming.
func (a *Agent) DefaultAgentPrefix() string {
	return "codex"
}

// ContainerImage returns the Docker image for Codex.
func (a *Agent) ContainerImage() string {
	return "ghcr.io/yagizdagabak/vibespace/codex:latest"
}

// ConfigDirectory returns the config directory inside the container.
func (a *Agent) ConfigDirectory() string {
	return ".codex"
}

// BuildPrintModeCommand builds the Codex exec command for non-interactive mode.
// Codex uses "codex exec" for non-interactive execution with JSON output.
func (a *Agent) BuildPrintModeCommand(sessionID string, resume bool, config *agent.Config) string {
	var args []string

	if resume && sessionID != "" {
		// Resume existing session: codex exec resume <session-id> --json
		args = []string{"codex", "exec", "resume", sessionID, "--json"}
	} else {
		// New session: codex exec --json
		// The prompt will be passed via stdin
		args = []string{"codex", "exec", "--json"}
	}

	// Apply configuration
	args = a.applyConfig(args, config)

	// Wrap in bash -l -c to ensure proper shell environment
	return fmt.Sprintf(`bash -l -c '%s'`, strings.Join(args, " "))
}

// BuildInteractiveCommand builds a Codex command for interactive terminal mode.
func (a *Agent) BuildInteractiveCommand(sessionID string, config *agent.Config) string {
	var args []string

	if sessionID != "" {
		// Resume: codex resume <session-id>
		args = []string{"codex", "resume", sessionID}
	} else {
		// Interactive mode without session
		args = []string{"codex"}
	}

	// Apply configuration
	args = a.applyConfig(args, config)

	return strings.Join(args, " ")
}

// applyConfig adds configuration flags to the command arguments.
func (a *Agent) applyConfig(args []string, config *agent.Config) []string {
	if config == nil {
		// Default: require approval for safety
		args = append(args, "--ask-for-approval", "never")
		return args
	}

	if config.SkipPermissions {
		// --yolo is short for --dangerously-bypass-approvals-and-sandbox
		args = append(args, "--yolo")
	} else {
		// TUI handles approvals, so disable Codex's built-in approval
		args = append(args, "--ask-for-approval", "never")
	}

	if config.Model != "" {
		args = append(args, "--model", config.Model)
	}

	// Note: Codex handles tools via sandbox policies, not CLI flags
	// AllowedTools/DisallowedTools would need to be translated to config.toml

	return args
}

// SessionIDFlag returns the flag used to specify a new session ID.
// Codex auto-generates session IDs, so this returns empty.
func (a *Agent) SessionIDFlag() string {
	return ""
}

// ResumeFlag returns the flag/subcommand used to resume a session.
// Codex uses "resume" as a subcommand argument, not a flag.
func (a *Agent) ResumeFlag() string {
	return "resume"
}

// ParseStreamLine parses a line of Codex's JSONL output.
func (a *Agent) ParseStreamLine(line string) (*agent.StreamMessage, bool) {
	if line == "" {
		return nil, false
	}

	var msg codexStreamMessage
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

	// Codex JSONL format - adjust based on actual CLI output
	// This is a placeholder implementation based on expected format
	switch msg.Type {
	case "message":
		if msg.Role == "assistant" {
			result.Type = "text"
			result.Text = msg.Content
			return result, true
		}

	case "tool_call", "function_call":
		result.Type = "tool_use"
		result.ToolName = msg.Name
		result.ToolID = msg.ID
		result.ToolInput = extractCodexToolInput(msg.Name, msg.Arguments)
		return result, true

	case "tool_result", "function_result":
		result.Type = "tool_result"
		result.Text = msg.Output
		return result, true

	case "error":
		result.Type = "error"
		result.Text = msg.Message
		result.IsError = true
		return result, true

	case "done", "complete":
		result.Type = "done"
		result.IsError = msg.Status == "error"
		result.Result = msg.Message
		return result, true

	case "status":
		if msg.Status == "complete" || msg.Status == "done" {
			result.Type = "done"
			return result, true
		}
	}

	return nil, true
}

// SupportedTools returns the list of tools Codex CLI supports.
func (a *Agent) SupportedTools() []string {
	return []string{
		"shell", "file_read", "file_write", "file_edit",
		"web_search", "browser",
	}
}

// DefaultConfig returns the default configuration for Codex.
func (a *Agent) DefaultConfig() *agent.Config {
	return &agent.Config{
		// Codex uses sandbox policies instead of explicit tool lists
	}
}

// ValidateConfig validates Codex-specific configuration.
func (a *Agent) ValidateConfig(config *agent.Config) error {
	if config == nil {
		return nil
	}
	// Codex-specific validation could go here
	return nil
}

// codexStreamMessage represents a JSONL message from Codex.
// Structure is based on expected Codex CLI output format.
type codexStreamMessage struct {
	Type      string          `json:"type"`
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	Name      string          `json:"name,omitempty"`
	ID        string          `json:"id,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Output    string          `json:"output,omitempty"`
	Status    string          `json:"status,omitempty"`
	Message   string          `json:"message,omitempty"`
}

// extractCodexToolInput extracts relevant input from Codex tool parameters.
func extractCodexToolInput(toolName string, arguments json.RawMessage) string {
	if len(arguments) == 0 {
		return ""
	}

	switch toolName {
	case "shell":
		var p struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(arguments, &p) == nil {
			return truncateString(p.Command, 50)
		}

	case "file_read":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(arguments, &p) == nil {
			return p.Path
		}

	case "file_write", "file_edit":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(arguments, &p) == nil {
			return p.Path
		}

	case "web_search":
		var p struct {
			Query string `json:"query"`
		}
		if json.Unmarshal(arguments, &p) == nil {
			return p.Query
		}

	case "browser":
		var p struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(arguments, &p) == nil {
			return truncateString(p.URL, 40)
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

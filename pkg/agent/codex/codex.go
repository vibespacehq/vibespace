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
	// Start with codex exec
	args := []string{"codex", "exec"}

	// Add flags BEFORE resume/prompt (required by Codex CLI)
	args = append(args, "--json", "--skip-git-repo-check")

	// Apply configuration (adds --yolo, --model, etc.)
	args = a.applyConfig(args, config)

	if resume && sessionID != "" {
		// Resume existing session: codex exec [flags] resume <session-id> -
		args = append(args, "resume", sessionID)
	}

	// Add - at the end to read prompt from stdin
	args = append(args, "-")

	// Wrap in bash -l -c to ensure proper shell environment and cd to /vibespace
	return fmt.Sprintf(`bash -l -c 'cd /vibespace && %s'`, strings.Join(args, " "))
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
// Codex always runs in --yolo mode in vibespace because:
// 1. The container itself provides isolation/sandboxing
// 2. Non-interactive mode (TUI) cannot respond to approval prompts
// 3. Users can configure Codex directly in interactive mode if needed
func (a *Agent) applyConfig(args []string, config *agent.Config) []string {
	// Always use --yolo mode - container provides the sandbox
	args = append(args, "--yolo")

	if config != nil {
		if config.Model != "" {
			args = append(args, "--model", config.Model)
		}
		if config.ReasoningEffort != "" {
			// Use -c flag to override config for this run
			args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%s", config.ReasoningEffort))
		}
	}

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
// Codex outputs: thread.started, turn.started, item.completed, turn.completed
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

	// Actual Codex CLI JSONL format based on observed output
	switch msg.Type {
	case "thread.started":
		// Thread started - capture the thread_id for session resumption
		// Codex auto-generates session IDs, so we need to capture and store them
		if msg.ThreadID != "" {
			result.Type = "session_started"
			result.SessionID = msg.ThreadID
			return result, true
		}
		return nil, true

	case "turn.started":
		// Turn started - skip, no content to display
		return nil, true

	case "item.completed":
		// Item completed - contains the actual content
		if msg.Item.Type == "agent_message" {
			result.Type = "text"
			result.Text = msg.Item.Text
			return result, true
		}
		if msg.Item.Type == "reasoning" {
			// Reasoning/thinking - could skip or show differently
			// For now, skip it to match Claude behavior (thinking is hidden)
			return nil, true
		}
		if msg.Item.Type == "tool_call" || msg.Item.Type == "function_call" {
			result.Type = "tool_use"
			result.ToolName = msg.Item.Name
			result.ToolID = msg.Item.ID
			result.ToolInput = extractCodexToolInput(msg.Item.Name, msg.Item.Arguments)
			return result, true
		}
		if msg.Item.Type == "tool_result" || msg.Item.Type == "function_result" {
			result.Type = "tool_result"
			result.Text = msg.Item.Output
			return result, true
		}

	case "turn.completed":
		// Turn completed - signal done
		result.Type = "done"
		return result, true

	case "error":
		result.Type = "error"
		result.Text = msg.Message
		if msg.Item.Text != "" {
			result.Text = msg.Item.Text
		}
		result.IsError = true
		return result, true
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
// Actual format from Codex CLI:
//   {"type":"thread.started","thread_id":"..."}
//   {"type":"turn.started"}
//   {"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"..."}}
//   {"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"..."}}
//   {"type":"turn.completed","usage":{...}}
type codexStreamMessage struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id,omitempty"`
	Message  string `json:"message,omitempty"` // For errors

	// Item contains the actual content for item.completed events
	Item struct {
		ID        string          `json:"id,omitempty"`
		Type      string          `json:"type,omitempty"`      // "agent_message", "reasoning", "tool_call", etc.
		Text      string          `json:"text,omitempty"`      // For agent_message and reasoning
		Name      string          `json:"name,omitempty"`      // For tool calls
		Arguments json.RawMessage `json:"arguments,omitempty"` // For tool calls
		Output    string          `json:"output,omitempty"`    // For tool results
	} `json:"item,omitempty"`

	// Usage for turn.completed
	Usage struct {
		InputTokens       int `json:"input_tokens,omitempty"`
		CachedInputTokens int `json:"cached_input_tokens,omitempty"`
		OutputTokens      int `json:"output_tokens,omitempty"`
	} `json:"usage,omitempty"`
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

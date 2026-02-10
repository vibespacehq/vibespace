package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/vibespacehq/vibespace/pkg/agent"
	// Import agent implementations to trigger init() registration
	_ "github.com/vibespacehq/vibespace/pkg/agent/claude"
	_ "github.com/vibespacehq/vibespace/pkg/agent/codex"
	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// ContentBlock represents a content block in Claude's response
// Can be text, tool_use, or tool_result
type ContentBlock struct {
	Type  string          `json:"type"`            // "text", "tool_use", "tool_result"
	Text  string          `json:"text,omitempty"`  // For type="text"
	ID    string          `json:"id,omitempty"`    // For type="tool_use"
	Name  string          `json:"name,omitempty"`  // For type="tool_use" (tool name)
	Input json.RawMessage `json:"input,omitempty"` // For type="tool_use" (tool params)
}

// ClaudeMessage represents a streaming JSON message from Claude
type ClaudeMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Result  string `json:"result,omitempty"`

	// For assistant messages - content is an array of content blocks
	Message struct {
		Role    string         `json:"role"`
		Content []ContentBlock `json:"content"`
	} `json:"message,omitempty"`

	// For content_block_delta events (streaming)
	ContentBlock ContentBlock `json:"content_block,omitempty"`

	// For tool_use events (standalone)
	Tool struct {
		Name  string          `json:"name"`
		ID    string          `json:"id"`
		Input json.RawMessage `json:"input"`
	} `json:"tool,omitempty"`

	// For errors
	Error   string `json:"error,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// AgentConn represents a connection to a coding agent in print mode
type AgentConn struct {
	address         session.AgentAddress
	localPort       int
	sessionManager  *AgentSessionManager // Shared session manager for --session-id vs --resume
	multiSessionID  string               // Multi-session ID for session isolation
	resume          bool                 // If true, use --resume with existing session; if false, use --session-id
	forceNewSession bool                 // If true, use --session-id even if session exists (for /session @agent new)

	// Agent type and configuration
	agentType   agent.Type        // Type of agent (claude-code, codex)
	agentImpl   agent.CodingAgent // Agent implementation for building commands
	agentConfig *agent.Config     // Agent configuration

	// SSH process running agent in print mode
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader

	// State
	connected    bool
	reconnectGen uint64 // Incremented on manual reconnect to prevent stale async reconnects
	mu           sync.Mutex

	// Output channel for parsed messages (now rich Message types)
	outputCh chan *Message
	// Signals when agent has finished responding (received "result" message)
	responseDone chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewAgentConn creates a new agent connection
// multiSessionID is the multi-session ID for session isolation
// resume indicates whether to resume an existing session (--resume) or start fresh (--session-id)
// agentType specifies the type of agent (defaults to claude-code)
// config optionally provides agent-specific configuration
func NewAgentConn(addr session.AgentAddress, localPort int, sessionMgr *AgentSessionManager, multiSessionID string, resume bool, agentType agent.Type, config *agent.Config) *AgentConn {
	ctx, cancel := context.WithCancel(context.Background())

	// Default to Claude Code if not specified
	if agentType == "" {
		agentType = agent.TypeClaudeCode
	}

	// Get agent implementation
	agentImpl, err := agent.Get(agentType)
	if err != nil {
		// Fall back to Claude Code if unknown type
		slog.Warn("unknown agent type, defaulting to claude-code", "type", agentType, "error", err)
		agentType = agent.TypeClaudeCode
		agentImpl = agent.MustGet(agent.TypeClaudeCode)
	}

	return &AgentConn{
		address:        addr,
		localPort:      localPort,
		sessionManager: sessionMgr,
		multiSessionID: multiSessionID,
		resume:         resume,
		agentType:      agentType,
		agentImpl:      agentImpl,
		agentConfig:    config,
		outputCh:       make(chan *Message, 100),
		responseDone:   make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Connect establishes the SSH connection and starts the agent in print mode
func (c *AgentConn) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		slog.Debug("agent already connected", "agent", c.address.String())
		return nil
	}

	// Get the private key path
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return vserrors.ErrSSHKeyNotFound
	}

	// Get session ID and determine whether to use --session-id or --resume
	agentAddr := c.address.String()
	var sessionID string
	var useResume bool
	var sessionMode string

	// Check if a session already exists for this agent in this multi-session
	existingSessionID := c.sessionManager.GetSession(c.multiSessionID, agentAddr)
	slog.Debug("connect: checking session", "agent", agentAddr, "multiSession", c.multiSessionID,
		"existingSessionID", existingSessionID, "forceNewSession", c.forceNewSession)

	if c.forceNewSession && existingSessionID != "" {
		// Force new session (e.g., /session @agent new) - use --session-id
		// Clear the flag after using it
		c.forceNewSession = false
		sessionMode = "session-id (forced new)"
		sessionID = existingSessionID
		useResume = false
	} else if existingSessionID != "" {
		// Session exists - use --resume to continue it
		sessionMode = "resume"
		sessionID = existingSessionID
		useResume = true
	} else {
		// No existing session - create a new one with --session-id
		sessionID = c.sessionManager.GetOrCreateSession(c.multiSessionID, agentAddr)
		sessionMode = "session-id (new)"
		useResume = false
	}
	slog.Debug("connect: using mode", "agent", agentAddr, "mode", sessionMode)

	agentCmd := c.buildAgentCommand(sessionID, useResume)

	sshArgs := []string{
		"-i", keyPath,
		"-p", strconv.Itoa(c.localPort),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=~/.vibespace/known_hosts",
		"-o", "LogLevel=ERROR",
		// Reverse tunnel: pod's localhost:18080 -> host's localhost:18080 (permission server)
		"-R", "18080:localhost:18080",
		"user@localhost",
		agentCmd,
	}

	c.cmd = exec.CommandContext(c.ctx, "ssh", sshArgs...)

	// Get stdin pipe for sending messages
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	c.stdin = stdin

	// Get stdout pipe for receiving responses
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	c.stdout = stdout

	// Get stderr pipe
	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	c.stderr = stderr

	// Start the process
	if err := c.cmd.Start(); err != nil {
		stdin.Close()
		slog.Error("connect: failed to start SSH", "agent", agentAddr, "error", err)
		return fmt.Errorf("failed to start SSH: %w", err)
	}

	c.connected = true
	slog.Debug("connect: SSH process started", "agent", agentAddr, "pid", c.cmd.Process.Pid)

	// After first successful connection, always use --resume for reconnects
	// This prevents "Session ID already in use" errors
	c.resume = true

	// Start output reader goroutine
	go c.readLoop()

	// Start stderr reader goroutine
	go c.readStderr()

	return nil
}

// readLoop continuously reads and parses JSON output from Claude
func (c *AgentConn) readLoop() {
	// Capture generation at start - only modify state if we're still current
	c.mu.Lock()
	myGen := c.reconnectGen
	c.mu.Unlock()

	scanner := bufio.NewScanner(c.stdout)
	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Debug: log raw JSON to file
	homeDir, _ := os.UserHomeDir()
	debugPath := filepath.Join(homeDir, ".vibespace", "agents_debug.log")
	debugFile, _ := os.OpenFile(debugPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	defer func() {
		if debugFile != nil {
			debugFile.Close()
		}
	}()

	for scanner.Scan() {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Debug: log raw line
		if debugFile != nil {
			debugFile.WriteString(fmt.Sprintf("[%s][%s] RAW: %s\n", c.address.String(), c.agentType, line))
		}

		// Use agent-specific parsing via the abstraction
		streamMsg, _ := c.agentImpl.ParseStreamLine(line)

		// Debug: log parsed result
		if debugFile != nil {
			if streamMsg != nil {
				debugFile.WriteString(fmt.Sprintf("[%s][%s] PARSED: type=%s, text=%q, tool=%s\n",
					c.address.String(), c.agentType, streamMsg.Type, truncateString(streamMsg.Text, 50), streamMsg.ToolName))
			} else {
				debugFile.WriteString(fmt.Sprintf("[%s][%s] PARSED: nil (skipped)\n", c.address.String(), c.agentType))
			}
		}

		// Convert agent.StreamMessage to TUI Message types
		if streamMsg != nil {
			messages := c.convertStreamMessage(streamMsg)
			for _, m := range messages {
				c.sendOutput(m)
			}
		}
	}

	// Connection closed or error - only update state if we're still the current readLoop
	c.mu.Lock()
	if c.reconnectGen == myGen {
		c.connected = false
		slog.Debug("readLoop: connection closed, setting connected=false", "agent", c.address.String(), "gen", myGen)
	} else {
		slog.Debug("readLoop: stale loop exiting, not modifying state", "agent", c.address.String(), "myGen", myGen, "currentGen", c.reconnectGen)
	}
	c.mu.Unlock()
}

// convertStreamMessage converts an agent.StreamMessage to TUI Message types
func (c *AgentConn) convertStreamMessage(msg *agent.StreamMessage) []*Message {
	var messages []*Message
	sender := c.address.String()

	switch msg.Type {
	case "text":
		if msg.Text != "" {
			messages = append(messages, NewAssistantMessage(sender, msg.Text))
		}

	case "tool_use":
		messages = append(messages, NewToolUseMessage(sender, msg.ToolName, msg.ToolInput))

	case "tool_result":
		// Tool results are typically not shown in TUI
		// Could add if needed

	case "session_started":
		// Agent reported its session ID (Codex auto-generates these)
		// Update the session manager with the actual session ID
		if msg.SessionID != "" && c.sessionManager != nil {
			slog.Debug("session_started: capturing agent session ID",
				"agent", sender, "sessionID", msg.SessionID)
			c.sessionManager.UpdateSessionID(c.multiSessionID, sender, msg.SessionID)
		}

	case "done":
		// Signal response complete
		c.signalResponseDone()
		if msg.IsError && msg.Result != "" {
			messages = append(messages, NewErrorMessage(sender, msg.Result))
		}

	case "error":
		messages = append(messages, NewErrorMessage(sender, msg.Text))

	case "system":
		if msg.Text != "" {
			messages = append(messages, NewSystemMessage(msg.Text))
		}
	}

	return messages
}

// readStderr reads stderr output and displays errors
func (c *AgentConn) readStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line != "" {
			c.sendOutput(NewErrorMessage(c.address.String(), line))
		}
	}
}

// truncateString truncates a string to max length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// sendOutput sends output to the channel
func (c *AgentConn) sendOutput(msg *Message) {
	if msg == nil {
		return
	}
	select {
	case c.outputCh <- msg:
	default:
		// Channel full, skip
	}
}

// Send sends a message to the Claude agent
// Claude print mode requires EOF to process input, so we close stdin and reconnect
func (c *AgentConn) Send(msg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	slog.Debug("send: attempting", "agent", c.address.String(), "connected", c.connected, "msgLen", len(msg))

	if !c.connected {
		slog.Error("send: not connected", "agent", c.address.String())
		return vserrors.ErrNotConnected
	}

	// Send the message followed by newline
	_, err := fmt.Fprintf(c.stdin, "%s\n", msg)
	if err != nil {
		slog.Error("send: write failed", "agent", c.address.String(), "error", err)
		return err
	}

	slog.Debug("send: message written, closing stdin", "agent", c.address.String())

	// Close stdin to signal EOF - Claude print mode requires this to process
	c.stdin.Close()

	return nil
}

// SendAndReconnect sends a message and reconnects for the next one
// This is needed because Claude print mode processes on EOF
func (c *AgentConn) SendAndReconnect(msg string) error {
	// Capture current generation and cmd before sending
	c.mu.Lock()
	currentGen := c.reconnectGen
	currentCmd := c.cmd
	c.mu.Unlock()

	slog.Debug("sendAndReconnect: starting", "agent", c.address.String(), "gen", currentGen)

	if err := c.Send(msg); err != nil {
		slog.Error("sendAndReconnect: send failed", "agent", c.address.String(), "error", err)
		return err
	}

	slog.Debug("sendAndReconnect: send succeeded, starting async wait", "agent", c.address.String(), "gen", currentGen)

	// Wait for response to be processed, then reconnect
	// The readLoop will continue reading until the process exits
	go func() {
		slog.Debug("sendAndReconnect: async goroutine waiting for process", "agent", c.address.String(), "gen", currentGen)

		// Wait for current process to finish
		if currentCmd != nil {
			currentCmd.Wait()
		}

		slog.Debug("sendAndReconnect: process finished, checking gen", "agent", c.address.String(), "capturedGen", currentGen)

		// Only reconnect if no manual reconnect happened (e.g., /session @agent new)
		c.mu.Lock()
		if c.reconnectGen != currentGen {
			slog.Debug("sendAndReconnect: skipping async reconnect - manual reconnect already happened",
				"agent", c.address.String(), "oldGen", currentGen, "newGen", c.reconnectGen)
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		slog.Debug("sendAndReconnect: proceeding with async reconnect", "agent", c.address.String(), "gen", currentGen)

		// Reconnect for next message - if it fails, notify via output channel
		if err := c.Reconnect(); err != nil {
			slog.Debug("sendAndReconnect: reconnect failed, notifying TUI", "agent", c.address.String(), "error", err)
			// Send error message to trigger TUI reconnect logic
			select {
			case c.outputCh <- &Message{
				Type:    MessageTypeError,
				Content: fmt.Sprintf("Connection refused: %v", err),
			}:
			default:
				// Channel full or closed, ignore
			}
		}
	}()

	return nil
}

// OutputChan returns the channel for receiving rich Message output
func (c *AgentConn) OutputChan() <-chan *Message {
	return c.outputCh
}

// DoneChan returns a channel that's closed when the response is complete
func (c *AgentConn) DoneChan() <-chan struct{} {
	return c.responseDone
}

// signalResponseDone signals that the response is complete (non-blocking)
func (c *AgentConn) signalResponseDone() {
	select {
	case <-c.responseDone:
		// Already closed
	default:
		close(c.responseDone)
	}
}

// IsConnected returns whether the agent is connected
func (c *AgentConn) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Address returns the agent address
func (c *AgentConn) Address() session.AgentAddress {
	return c.address
}

// SessionID returns the current Claude session ID for this connection
func (c *AgentConn) SessionID() string {
	return c.sessionManager.GetCurrentSessionID(c.multiSessionID, c.address.String())
}

// MultiSessionID returns the multi-session ID for this connection
func (c *AgentConn) MultiSessionID() string {
	return c.multiSessionID
}

// SetResume sets the resume flag for this connection (used when switching sessions)
func (c *AgentConn) SetResume(resume bool) {
	c.resume = resume
}

// SetForceNewSession sets the forceNewSession flag (used by /session @agent new)
func (c *AgentConn) SetForceNewSession(force bool) {
	c.forceNewSession = force
}

// LocalPort returns the local SSH port
func (c *AgentConn) LocalPort() int {
	return c.localPort
}

// Config returns the agent configuration
func (c *AgentConn) Config() *agent.Config {
	return c.agentConfig
}

// AgentType returns the type of agent for this connection
func (c *AgentConn) AgentType() agent.Type {
	return c.agentType
}

// AgentImpl returns the agent implementation
func (c *AgentConn) AgentImpl() agent.CodingAgent {
	return c.agentImpl
}

// BuildInteractiveAgentCommand builds an interactive command for any agent type
// This is used by /focus to launch an interactive session with the agent's config
func BuildInteractiveAgentCommand(agentType agent.Type, config *agent.Config, sessionID string) string {
	agentImpl, err := agent.Get(agentType)
	if err != nil {
		// Fall back to Claude Code
		agentImpl = agent.MustGet(agent.TypeClaudeCode)
	}
	return agentImpl.BuildInteractiveCommand(sessionID, config)
}

// buildAgentCommand builds the agent's print-mode command using the agent abstraction
func (c *AgentConn) buildAgentCommand(sessionID string, useResume bool) string {
	// Use the agent implementation to build the command
	if c.agentImpl != nil {
		return c.agentImpl.BuildPrintModeCommand(sessionID, useResume, c.agentConfig)
	}

	// Fallback to default Claude Code behavior (should rarely happen)
	return c.buildClaudeCommandFallback(sessionID, useResume)
}

// buildClaudeCommandFallback is a fallback for when agentImpl is nil.
// This should rarely happen since agentImpl is always initialized.
func (c *AgentConn) buildClaudeCommandFallback(sessionID string, useResume bool) string {
	args := []string{"claude", "-p", "--verbose", "--output-format", "stream-json"}

	// Session handling
	if useResume {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}

	// Config-based flags
	if c.agentConfig != nil {
		if c.agentConfig.SkipPermissions {
			args = append(args, "--dangerously-skip-permissions")
		}
		if len(c.agentConfig.AllowedTools) > 0 {
			args = append(args, "--allowedTools", fmt.Sprintf(`"%s"`, c.agentConfig.AllowedToolsString()))
		} else if !c.agentConfig.SkipPermissions {
			args = append(args, "--allowedTools", fmt.Sprintf(`"%s"`, strings.Join(agent.DefaultAllowedTools(), ",")))
		}
		if len(c.agentConfig.DisallowedTools) > 0 {
			args = append(args, "--disallowedTools", fmt.Sprintf(`"%s"`, c.agentConfig.DisallowedToolsString()))
		}
		if c.agentConfig.Model != "" {
			args = append(args, "--model", c.agentConfig.Model)
		}
		if c.agentConfig.MaxTurns > 0 {
			args = append(args, "--max-turns", fmt.Sprintf("%d", c.agentConfig.MaxTurns))
		}
	} else {
		args = append(args, "--allowedTools", `"Bash(read_only:true),Read,Write,Edit,Glob,Grep"`)
	}

	return fmt.Sprintf(`bash -l -c '%s'`, strings.Join(args, " "))
}

// Close closes the agent connection
func (c *AgentConn) Close() {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}

	c.connected = false
}

// Reconnect attempts to reconnect to the agent
// Keeps the same output channel so listeners continue working
// Session continuity is managed by AgentSessionManager (--session-id vs --resume)
func (c *AgentConn) Reconnect() error {
	slog.Debug("reconnect: starting", "agent", c.address.String(), "forceNewSession", c.forceNewSession)

	// Save the output channel before closing
	savedCh := c.outputCh

	c.cancel()

	c.mu.Lock()
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	c.connected = false
	c.reconnectGen++ // Increment to invalidate any pending async reconnects
	c.mu.Unlock()

	// Create new context and responseDone channel, but keep the same output channel
	// Session ID is managed by sessionManager
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.outputCh = savedCh
	c.responseDone = make(chan struct{}) // New channel for next response

	err := c.Connect()
	if err != nil {
		slog.Error("reconnect: connect failed", "agent", c.address.String(), "error", err)
	} else {
		slog.Debug("reconnect: connect successful", "agent", c.address.String())
	}
	return err
}

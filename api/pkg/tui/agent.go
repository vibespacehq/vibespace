package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"vibespace/pkg/session"
	"vibespace/pkg/vibespace"
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

// AgentConn represents a connection to a Claude agent in print mode
type AgentConn struct {
	address        session.AgentAddress
	localPort      int
	sessionManager *ClaudeSessionManager // Shared session manager for --session-id vs --resume

	// SSH process running claude in print mode
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader

	// State
	connected bool
	mu        sync.Mutex

	// Output channel for parsed messages (now rich Message types)
	outputCh chan *Message
	// Signals when Claude has finished responding (received "result" message)
	responseDone chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewAgentConn creates a new agent connection
func NewAgentConn(addr session.AgentAddress, localPort int, sessionMgr *ClaudeSessionManager) *AgentConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentConn{
		address:        addr,
		localPort:      localPort,
		sessionManager: sessionMgr,
		outputCh:       make(chan *Message, 100),
		responseDone:   make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Connect establishes the SSH connection and starts Claude in print mode
func (c *AgentConn) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Get the private key path
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return fmt.Errorf("SSH key not found")
	}

	// Get session ID and determine if this is a new session or continuation
	// isNew=true: first message in session, use --session-id to create
	// isNew=false: continuing session, use --resume to continue
	sessionID, isNew := c.sessionManager.GetOrCreateSession(c.address)

	// Build SSH command to run Claude in print mode
	// Using stream-json for real-time output parsing
	// Use login shell to ensure PATH and environment are set up
	// Note: --verbose is required when using stream-json with -p
	var claudeCmd string
	if isNew {
		// First message: create new session with --session-id
		claudeCmd = fmt.Sprintf(
			`bash -l -c 'claude -p --verbose --output-format stream-json --session-id %s --allowedTools "Bash(read_only:true),Read,Write,Edit,Glob,Grep"'`,
			sessionID,
		)
	} else {
		// Continuing: resume existing session with --resume
		claudeCmd = fmt.Sprintf(
			`bash -l -c 'claude -p --verbose --output-format stream-json --resume %s --allowedTools "Bash(read_only:true),Read,Write,Edit,Glob,Grep"'`,
			sessionID,
		)
	}

	sshArgs := []string{
		"-i", keyPath,
		"-p", strconv.Itoa(c.localPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"user@localhost",
		claudeCmd,
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
		return fmt.Errorf("failed to start SSH: %w", err)
	}

	c.connected = true

	// Start output reader goroutine
	go c.readLoop()

	// Start stderr reader goroutine
	go c.readStderr()

	return nil
}

// readLoop continuously reads and parses JSON output from Claude
func (c *AgentConn) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Debug: log raw JSON to file
	debugFile, _ := os.OpenFile("/tmp/claude_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
			debugFile.WriteString(fmt.Sprintf("[%s] RAW: %s\n", c.address.String(), line))
		}

		// Try to parse as JSON
		var msg ClaudeMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Not JSON, send as raw text
			c.sendOutput(NewAssistantMessage(c.address.String(), line))
			continue
		}

		// Debug: log parsed type
		if debugFile != nil {
			debugFile.WriteString(fmt.Sprintf("[%s] TYPE: %s, ContentLen: %d\n", c.address.String(), msg.Type, len(msg.Message.Content)))
			for i, block := range msg.Message.Content {
				debugFile.WriteString(fmt.Sprintf("[%s]   Block[%d]: type=%s, name=%s, text=%q\n", c.address.String(), i, block.Type, block.Name, truncateString(block.Text, 50)))
			}
		}

		// Parse the message and send rich message types
		messages := c.parseClaudeMessage(&msg)
		for _, m := range messages {
			c.sendOutput(m)
		}
	}

	// Connection closed or error
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
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

// parseClaudeMessage converts a Claude JSON message to rich Message types
func (c *AgentConn) parseClaudeMessage(msg *ClaudeMessage) []*Message {
	var messages []*Message
	sender := c.address.String()

	switch msg.Type {
	case "assistant":
		// Extract text and tool_use from content blocks array
		for _, block := range msg.Message.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					messages = append(messages, NewAssistantMessage(sender, block.Text))
				}
			case "tool_use":
				// Tool use found in assistant message content
				toolInput := extractToolInput(block.Name, block.Input)
				messages = append(messages, NewToolUseMessage(sender, block.Name, toolInput))
			}
		}

	case "tool_use":
		// Standalone tool_use event
		if msg.Tool.Name != "" {
			toolInput := extractToolInput(msg.Tool.Name, msg.Tool.Input)
			messages = append(messages, NewToolUseMessage(sender, msg.Tool.Name, toolInput))
		}
		// Also check content_block for tool_use
		if msg.ContentBlock.Type == "tool_use" && msg.ContentBlock.Name != "" {
			toolInput := extractToolInput(msg.ContentBlock.Name, msg.ContentBlock.Input)
			messages = append(messages, NewToolUseMessage(sender, msg.ContentBlock.Name, toolInput))
		}

	case "content_block_start":
		// Tool use can come as content_block_start with tool_use type
		if msg.ContentBlock.Type == "tool_use" && msg.ContentBlock.Name != "" {
			toolInput := extractToolInput(msg.ContentBlock.Name, msg.ContentBlock.Input)
			messages = append(messages, NewToolUseMessage(sender, msg.ContentBlock.Name, toolInput))
		}

	case "result":
		// Final result - signal completion
		c.signalResponseDone()
		// Only show if it's an error
		if msg.IsError {
			messages = append(messages, NewErrorMessage(sender, msg.Result))
		}
		// Skip successful result messages - we already showed the assistant response

	case "content_block_delta":
		if msg.ContentBlock.Text != "" {
			messages = append(messages, NewAssistantMessage(sender, msg.ContentBlock.Text))
		}

	case "error":
		messages = append(messages, NewErrorMessage(sender, msg.Error))

	case "system":
		// Skip system init messages - they're verbose
		if msg.Subtype != "init" && msg.Result != "" {
			messages = append(messages, NewSystemMessage(msg.Result))
		}
	}

	return messages
}

// extractToolInput extracts relevant input from tool parameters
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

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	// Send the message followed by newline
	_, err := fmt.Fprintf(c.stdin, "%s\n", msg)
	if err != nil {
		return err
	}

	// Record message in session manager (for --session-id vs --resume logic)
	c.sessionManager.RecordMessage(c.address, msg)

	// Close stdin to signal EOF - Claude print mode requires this to process
	c.stdin.Close()

	return nil
}

// SendAndReconnect sends a message and reconnects for the next one
// This is needed because Claude print mode processes on EOF
func (c *AgentConn) SendAndReconnect(msg string) error {
	if err := c.Send(msg); err != nil {
		return err
	}

	// Wait for response to be processed, then reconnect
	// The readLoop will continue reading until the process exits
	go func() {
		// Wait for current process to finish
		if c.cmd != nil {
			c.cmd.Wait()
		}
		// Reconnect for next message
		c.Reconnect()
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
	return c.sessionManager.GetCurrentSessionID(c.address)
}

// LocalPort returns the local SSH port
func (c *AgentConn) LocalPort() int {
	return c.localPort
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
// Session continuity is managed by ClaudeSessionManager (--session-id vs --resume)
func (c *AgentConn) Reconnect() error {
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
	c.mu.Unlock()

	// Create new context and responseDone channel, but keep the same output channel
	// Session ID is managed by sessionManager
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.outputCh = savedCh
	c.responseDone = make(chan struct{}) // New channel for next response

	return c.Connect()
}

// HasContent returns true if a string has non-whitespace content
func HasContent(s string) bool {
	return strings.TrimSpace(s) != ""
}

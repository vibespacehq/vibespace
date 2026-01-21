package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"vibespace/pkg/session"
	"vibespace/pkg/vibespace"
)

// ClaudeMessage represents a streaming JSON message from Claude
type ClaudeMessage struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Result  string `json:"result,omitempty"`

	// For assistant messages - content is an array of content blocks
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message,omitempty"`

	// For content_block_delta events
	ContentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block,omitempty"`

	// For tool use
	Tool struct {
		Name  string `json:"name"`
		Input string `json:"input"`
	} `json:"tool,omitempty"`

	// For errors
	Error   string `json:"error,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// AgentConn represents a connection to a Claude agent in print mode
type AgentConn struct {
	address   session.AgentAddress
	localPort int

	// SSH process running claude in print mode
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader

	// State
	connected bool
	mu        sync.Mutex

	// Output channel for parsed messages
	outputCh chan string
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewAgentConn creates a new agent connection
func NewAgentConn(addr session.AgentAddress, localPort int) *AgentConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentConn{
		address:   addr,
		localPort: localPort,
		outputCh:  make(chan string, 100),
		ctx:       ctx,
		cancel:    cancel,
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

	// Build SSH command to run Claude in print mode
	// Using stream-json for real-time output parsing
	// Use login shell to ensure PATH and environment are set up
	// Note: --verbose is required when using stream-json with -p
	sshArgs := []string{
		"-i", keyPath,
		"-p", strconv.Itoa(c.localPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"user@localhost",
		// Run Claude in print mode with streaming JSON and pre-approved tools
		"bash", "-l", "-c",
		"claude -p --verbose --output-format stream-json --allowedTools 'Bash(read_only:true),Read,Write,Edit,Glob,Grep'",
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

		// Try to parse as JSON
		var msg ClaudeMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Not JSON, send as raw text
			c.sendOutput(line)
			continue
		}

		// Format the message based on type
		output := c.formatMessage(&msg)
		if output != "" {
			c.sendOutput(output)
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
			c.sendOutput(fmt.Sprintf("[Error] %s", line))
		}
	}
}

// formatMessage converts a Claude JSON message to display text
func (c *AgentConn) formatMessage(msg *ClaudeMessage) string {
	switch msg.Type {
	case "assistant":
		// Extract text from content blocks array
		var texts []string
		for _, block := range msg.Message.Content {
			if block.Type == "text" && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
		return ""

	case "result":
		// Final result - only show if it's different from assistant message
		// and not an error
		if msg.IsError {
			return fmt.Sprintf("[Error: %s]", msg.Result)
		}
		// Skip result messages - we already showed the assistant response
		return ""

	case "content_block_delta":
		if msg.ContentBlock.Text != "" {
			return msg.ContentBlock.Text
		}
		return ""

	case "tool_use":
		if msg.Tool.Name != "" {
			return fmt.Sprintf("[Using %s]", msg.Tool.Name)
		}
		return ""

	case "tool_result":
		return ""

	case "error":
		return fmt.Sprintf("[Error: %s]", msg.Error)

	case "system":
		// Skip system init messages - they're verbose
		if msg.Subtype == "init" {
			return ""
		}
		return ""

	default:
		// For unknown types, show result if available
		if msg.Result != "" {
			return msg.Result
		}
	}
	return ""
}

// sendOutput sends output to the channel
func (c *AgentConn) sendOutput(text string) {
	select {
	case c.outputCh <- text:
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

// OutputChan returns the channel for receiving output
func (c *AgentConn) OutputChan() <-chan string {
	return c.outputCh
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

	// Create new context but keep the same output channel
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.outputCh = savedCh

	return c.Connect()
}

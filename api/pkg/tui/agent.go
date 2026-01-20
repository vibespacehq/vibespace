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
	"sync"

	"vibespace/pkg/session"
	"vibespace/pkg/vibespace"
)

// ClaudeMessage represents a streaming JSON message from Claude
type ClaudeMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Result  string `json:"result,omitempty"`

	// For assistant messages
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message,omitempty"`

	// For tool use
	Tool struct {
		Name  string `json:"name"`
		Input string `json:"input"`
	} `json:"tool,omitempty"`

	// For errors
	Error string `json:"error,omitempty"`
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
	sshArgs := []string{
		"-i", keyPath,
		"-p", strconv.Itoa(c.localPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"user@localhost",
		// Run Claude in print mode with streaming JSON and pre-approved tools
		"bash", "-l", "-c",
		"claude -p --output-format stream-json --allowedTools 'Bash(read_only:true),Read,Write,Edit,Glob,Grep'",
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

// formatMessage converts a Claude JSON message to display text
func (c *AgentConn) formatMessage(msg *ClaudeMessage) string {
	switch msg.Type {
	case "assistant":
		if msg.Message.Content != "" {
			return msg.Message.Content
		}
		return msg.Content

	case "result":
		return msg.Result

	case "content_block_delta":
		return msg.Content

	case "tool_use":
		return fmt.Sprintf("[Using %s]", msg.Tool.Name)

	case "tool_result":
		return msg.Content

	case "error":
		return fmt.Sprintf("[Error: %s]", msg.Error)

	case "system":
		return fmt.Sprintf("[System: %s]", msg.Content)

	default:
		// For unknown types, show content if available
		if msg.Content != "" {
			return msg.Content
		}
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
func (c *AgentConn) Send(msg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	// Send the message followed by newline
	_, err := fmt.Fprintf(c.stdin, "%s\n", msg)
	return err
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
func (c *AgentConn) Reconnect() error {
	c.Close()

	// Create new context
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.outputCh = make(chan string, 100)

	return c.Connect()
}

// ConnectInteractive opens an interactive Claude session (for /focus mode)
// This hands over terminal control completely
func ConnectInteractive(localPort int) error {
	keyPath := vibespace.GetSSHPrivateKeyPath()
	if keyPath == "" {
		return fmt.Errorf("SSH key not found")
	}

	sshArgs := []string{
		"-i", keyPath,
		"-p", strconv.Itoa(localPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t", // Force PTY for interactive mode
		"user@localhost",
		"bash", "-l", "-c", "claude", // Run interactive Claude with login shell
	}

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

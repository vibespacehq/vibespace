package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"vibespace/pkg/daemon"
	"vibespace/pkg/session"
)

// HeadlessRunner handles non-interactive mode for multi-agent sessions
type HeadlessRunner struct {
	agents         map[string]*AgentConn
	agentOrder     []string
	agentStates    map[string]*AgentState
	sessionManager *ClaudeSessionManager
	daemons        map[string]*daemon.Client
	mu             sync.RWMutex

	// Configuration
	timeout time.Duration

	// History persistence
	historyStore *HistoryStore
	sessionName  string // Name for history persistence
}

// NewHeadlessRunner creates a new headless runner
func NewHeadlessRunner() *HeadlessRunner {
	historyStore, _ := NewHistoryStore() // Ignore error, history is optional
	return &HeadlessRunner{
		agents:         make(map[string]*AgentConn),
		agentOrder:     make([]string, 0),
		agentStates:    make(map[string]*AgentState),
		sessionManager: NewClaudeSessionManager(),
		daemons:        make(map[string]*daemon.Client),
		timeout:        2 * time.Minute, // Default timeout
		historyStore:   historyStore,
	}
}

// SetSessionName sets the session name for history persistence
func (r *HeadlessRunner) SetSessionName(name string) {
	r.sessionName = name
}

// SetTimeout sets the response timeout
func (r *HeadlessRunner) SetTimeout(d time.Duration) {
	r.timeout = d
}

// Connect connects to agents in the specified vibespaces
func (r *HeadlessRunner) Connect(ctx context.Context, vibespaces []string) error {
	for _, vsName := range vibespaces {
		// Ensure daemon is running
		if err := r.ensureDaemon(vsName); err != nil {
			return fmt.Errorf("failed to start daemon for %s: %w", vsName, err)
		}

		// Get agents for this vibespace
		client := r.daemons[vsName]
		status, err := client.Status()
		if err != nil {
			return fmt.Errorf("failed to get status for %s: %w", vsName, err)
		}

		// Get forwards to find SSH ports
		forwards, err := client.ListForwards()
		if err != nil {
			return fmt.Errorf("failed to list forwards for %s: %w", vsName, err)
		}

		// Build a map of agent name to SSH port
		agentPorts := make(map[string]int)
		for _, agent := range forwards.Agents {
			for _, fwd := range agent.Forwards {
				if fwd.Type == "ssh" && fwd.Status == "active" {
					agentPorts[agent.Name] = fwd.LocalPort
					break
				}
			}
		}

		// Connect to each agent
		for _, a := range status.Agents {
			port, ok := agentPorts[a.Name]
			if !ok {
				continue // No SSH forward for this agent
			}

			addr := session.AgentAddress{Agent: a.Name, Vibespace: vsName}
			conn := NewAgentConn(addr, port, r.sessionManager)
			if err := conn.Connect(); err != nil {
				return fmt.Errorf("failed to connect to %s: %w", addr.String(), err)
			}

			key := addr.String()
			r.agents[key] = conn
			r.agentOrder = append(r.agentOrder, key)
			r.agentStates[key] = NewAgentState(addr)
		}
	}

	return nil
}

// ensureDaemon ensures the daemon is running for a vibespace
func (r *HeadlessRunner) ensureDaemon(vsName string) error {
	if _, ok := r.daemons[vsName]; ok {
		return nil
	}

	if !daemon.IsRunning(vsName) {
		if err := daemon.SpawnDaemon(vsName); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
		if err := daemon.WaitForReady(vsName, 10*time.Second); err != nil {
			return fmt.Errorf("daemon failed to start: %w", err)
		}
	}

	client, err := daemon.NewClient(vsName)
	if err != nil {
		return fmt.Errorf("failed to create daemon client: %w", err)
	}

	r.daemons[vsName] = client
	return nil
}

// Close closes all connections
func (r *HeadlessRunner) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, conn := range r.agents {
		conn.Close()
	}
}

// MultiRequest represents a request to send to agents
type MultiRequest struct {
	Target  string `json:"target"`  // "all" or specific agent
	Message string `json:"message"`
}

// MultiResponse represents the response from agents
type MultiResponse struct {
	Session   string          `json:"session"`
	Request   RequestInfo     `json:"request"`
	Responses []AgentResponse `json:"responses"`
	Error     string          `json:"error,omitempty"`
}

// RequestInfo contains information about the request
type RequestInfo struct {
	Target  string `json:"target"`
	Message string `json:"message"`
}

// AgentResponse represents a response from a single agent
type AgentResponse struct {
	Agent     string    `json:"agent"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	ToolUses  []ToolUse `json:"tool_uses,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// ToolUse represents a tool that was used by Claude
type ToolUse struct {
	Tool  string `json:"tool"`
	Input string `json:"input,omitempty"`
}

// SendAndWait sends a message to the specified target and waits for all responses
func (r *HeadlessRunner) SendAndWait(ctx context.Context, target, message string) (*MultiResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	response := &MultiResponse{
		Request: RequestInfo{
			Target:  target,
			Message: message,
		},
		Responses: make([]AgentResponse, 0),
	}

	// Persist user message to history
	r.persistMessage(NewUserMessage(target, message))

	// Determine which agents to send to
	var targetAgents []*AgentConn
	if target == "all" || target == "" {
		for _, conn := range r.agents {
			targetAgents = append(targetAgents, conn)
		}
	} else {
		conn, ok := r.agents[target]
		if !ok {
			// Try to match by agent name without vibespace
			for key, c := range r.agents {
				if c.Address().Agent == target {
					conn = c
					target = key
					break
				}
			}
		}
		if conn == nil {
			return nil, fmt.Errorf("agent not found: %s", target)
		}
		targetAgents = append(targetAgents, conn)
	}

	if len(targetAgents) == 0 {
		return nil, fmt.Errorf("no agents to send to")
	}

	// Create channels to collect responses
	type agentResult struct {
		key      string
		messages []*Message
		err      error
	}
	resultCh := make(chan agentResult, len(targetAgents))

	// Create a timeout context
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Send to each agent and collect responses
	var wg sync.WaitGroup
	for _, conn := range targetAgents {
		wg.Add(1)
		go func(c *AgentConn) {
			defer wg.Done()

			key := c.Address().String()
			var messages []*Message
			var messagesMu sync.Mutex

			// Start collecting messages from output channel
			outputCh := c.OutputChan()
			doneCh := c.DoneChan() // Use the agent's done channel

			go func() {
				for {
					select {
					case msg, ok := <-outputCh:
						if !ok {
							return
						}
						messagesMu.Lock()
						messages = append(messages, msg)
						messagesMu.Unlock()
					case <-doneCh:
						// Response complete, drain remaining messages
						for {
							select {
							case msg := <-outputCh:
								messagesMu.Lock()
								messages = append(messages, msg)
								messagesMu.Unlock()
							default:
								return
							}
						}
					case <-ctx.Done():
						return
					}
				}
			}()

			// Send the message
			if err := c.SendAndReconnect(message); err != nil {
				resultCh <- agentResult{key: key, err: err}
				return
			}

			// Wait for the response (until agent signals done or timeout)
			select {
			case <-doneCh:
				// Agent finished responding (received "result" message)
			case <-time.After(r.timeout):
				// Timeout waiting for response
			case <-ctx.Done():
				// Context cancelled
			}

			// Small delay to allow message collection goroutine to finish draining
			time.Sleep(100 * time.Millisecond)

			messagesMu.Lock()
			resultCh <- agentResult{key: key, messages: messages}
			messagesMu.Unlock()
		}(conn)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	for result := range resultCh {
		agentResp := AgentResponse{
			Agent:     result.key,
			Timestamp: time.Now(),
			ToolUses:  make([]ToolUse, 0),
		}

		if result.err != nil {
			agentResp.Error = result.err.Error()
			// Persist error message
			r.persistMessage(NewErrorMessage(result.key, result.err.Error()))
		} else {
			// Combine messages into content and tool uses
			var contentParts []string
			for _, msg := range result.messages {
				// Persist each message to history
				r.persistMessage(msg)

				switch msg.Type {
				case MessageTypeAssistant:
					if msg.Content != "" {
						contentParts = append(contentParts, msg.Content)
					}
				case MessageTypeToolUse:
					agentResp.ToolUses = append(agentResp.ToolUses, ToolUse{
						Tool:  msg.ToolName,
						Input: msg.ToolInput,
					})
				case MessageTypeError:
					if agentResp.Error == "" {
						agentResp.Error = msg.Content
					} else {
						agentResp.Error += "; " + msg.Content
					}
				}
			}
			agentResp.Content = joinNonEmpty(contentParts, "\n")
		}

		response.Responses = append(response.Responses, agentResp)
	}

	return response, nil
}

// persistMessage saves a message to history if configured
func (r *HeadlessRunner) persistMessage(msg *Message) {
	if r.historyStore != nil && r.sessionName != "" {
		go r.historyStore.Append(r.sessionName, msg)
	}
}

// GetAgents returns a list of connected agent addresses
func (r *HeadlessRunner) GetAgents() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, len(r.agentOrder))
	copy(result, r.agentOrder)
	return result
}

// ToJSON converts the response to JSON
func (r *MultiResponse) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// joinNonEmpty joins non-empty strings with the separator
func joinNonEmpty(parts []string, sep string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	result := nonEmpty[0]
	for i := 1; i < len(nonEmpty); i++ {
		result += sep + nonEmpty[i]
	}
	return result
}

// RunHeadless runs a headless session with the given configuration
func RunHeadless(ctx context.Context, vibespaces []string, target, message string, jsonOutput bool) error {
	runner := NewHeadlessRunner()
	defer runner.Close()

	// Connect to agents
	if err := runner.Connect(ctx, vibespaces); err != nil {
		if jsonOutput {
			resp := &MultiResponse{
				Error: err.Error(),
			}
			data, _ := resp.ToJSON()
			fmt.Println(string(data))
			return nil
		}
		return err
	}

	// Send message and wait for responses
	response, err := runner.SendAndWait(ctx, target, message)
	if err != nil {
		if jsonOutput {
			resp := &MultiResponse{
				Error: err.Error(),
			}
			data, _ := resp.ToJSON()
			fmt.Println(string(data))
			return nil
		}
		return err
	}

	// Output response
	if jsonOutput {
		data, err := response.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(data))
	} else {
		// Plain text output
		for _, agentResp := range response.Responses {
			fmt.Printf("[%s]\n", agentResp.Agent)
			if agentResp.Error != "" {
				fmt.Printf("Error: %s\n", agentResp.Error)
			} else {
				if len(agentResp.ToolUses) > 0 {
					for _, tu := range agentResp.ToolUses {
						if tu.Input != "" {
							fmt.Printf("  [%s] %s\n", tu.Tool, tu.Input)
						} else {
							fmt.Printf("  [%s]\n", tu.Tool)
						}
					}
				}
				if agentResp.Content != "" {
					fmt.Println(agentResp.Content)
				}
			}
			fmt.Println()
		}
	}

	return nil
}

// MessageCallback is called for each message received during streaming
type MessageCallback func(agent string, msg *Message)

// StreamResponses sends a message and streams responses via callback
func (r *HeadlessRunner) StreamResponses(ctx context.Context, target, message string, callback MessageCallback) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Persist user message to history
	r.persistMessage(NewUserMessage(target, message))

	// Determine which agents to send to
	var targetAgents []*AgentConn
	if target == "all" || target == "" {
		for _, conn := range r.agents {
			targetAgents = append(targetAgents, conn)
		}
	} else {
		conn, ok := r.agents[target]
		if !ok {
			// Try to match by agent name without vibespace
			for key, c := range r.agents {
				if c.Address().Agent == target {
					conn = c
					target = key
					break
				}
			}
		}
		if conn == nil {
			return fmt.Errorf("agent not found: %s", target)
		}
		targetAgents = append(targetAgents, conn)
	}

	if len(targetAgents) == 0 {
		return fmt.Errorf("no agents to send to")
	}

	// Create a timeout context
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Track completion for all agents
	var wg sync.WaitGroup

	for _, conn := range targetAgents {
		wg.Add(1)
		go func(c *AgentConn) {
			defer wg.Done()

			key := c.Address().String()

			// Start collecting messages from output channel
			outputCh := c.OutputChan()
			doneCh := c.DoneChan()

			// Goroutine to stream messages via callback
			streamDone := make(chan struct{})
			go func() {
				defer close(streamDone)
				for {
					select {
					case msg, ok := <-outputCh:
						if !ok {
							return
						}
						r.persistMessage(msg) // Persist to history
						callback(key, msg)
					case <-doneCh:
						// Drain remaining messages
						for {
							select {
							case msg := <-outputCh:
								r.persistMessage(msg) // Persist to history
								callback(key, msg)
							default:
								return
							}
						}
					case <-ctx.Done():
						return
					}
				}
			}()

			// Send the message
			if err := c.SendAndReconnect(message); err != nil {
				errMsg := NewErrorMessage(key, err.Error())
				r.persistMessage(errMsg) // Persist error to history
				callback(key, errMsg)
				return
			}

			// Wait for streaming to complete
			select {
			case <-streamDone:
			case <-doneCh:
				// Give a moment for final messages
				time.Sleep(100 * time.Millisecond)
			case <-ctx.Done():
			}
		}(conn)
	}

	// Wait for all agents
	wg.Wait()
	return nil
}

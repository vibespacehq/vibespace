package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"vibespace/pkg/daemon"
	"vibespace/pkg/session"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LayoutMode represents the TUI layout mode
type LayoutMode int

const (
	LayoutSplit LayoutMode = iota
	LayoutFocus
)

// OutputBuffer stores agent output history
type OutputBuffer struct {
	lines      []OutputLine
	maxLines   int
	scrollPos  int
	mu         sync.Mutex
}

// OutputLine represents a single line of output
type OutputLine struct {
	Text      string
	Timestamp time.Time
}

// NewOutputBuffer creates a new output buffer
func NewOutputBuffer(maxLines int) *OutputBuffer {
	return &OutputBuffer{
		lines:    make([]OutputLine, 0),
		maxLines: maxLines,
	}
}

// Add adds a line to the buffer
func (b *OutputBuffer) Add(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, OutputLine{
		Text:      text,
		Timestamp: time.Now(),
	})

	// Trim if exceeds max
	if len(b.lines) > b.maxLines {
		b.lines = b.lines[len(b.lines)-b.maxLines:]
	}
}

// GetLines returns the last n lines
func (b *OutputBuffer) GetLines(n int) []OutputLine {
	b.mu.Lock()
	defer b.mu.Unlock()

	if n > len(b.lines) {
		n = len(b.lines)
	}
	return b.lines[len(b.lines)-n:]
}

// Len returns the number of lines
func (b *OutputBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.lines)
}

// Model is the main Bubble Tea model for the TUI
type Model struct {
	// Session
	session *session.Session
	isAdHoc bool

	// Connections
	agents     map[string]*AgentConn // "claude-1@projectA" → connection
	agentOrder []string              // display order
	agentMu    sync.RWMutex

	// Daemon clients (one per vibespace)
	daemons map[string]*daemon.Client

	// UI components
	input   textinput.Model
	outputs map[string]*OutputBuffer
	styles  Styles

	// Layout
	layout      LayoutMode
	focusAgent  string
	width       int
	height      int

	// State
	ctx        context.Context
	cancel     context.CancelFunc
	ready      bool
	quitting   bool
	err        error
	statusMsg  string

	// Default vibespace for short commands
	defaultVibespace string
}

// NewModel creates a new TUI model
func NewModel(sess *session.Session, isAdHoc bool) *Model {
	ti := textinput.New()
	ti.Prompt = "" // We render our own prompt
	ti.Placeholder = "@agent message or /help (Tab for autocomplete)"
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80
	ti.ShowSuggestions = true

	ctx, cancel := context.WithCancel(context.Background())

	// Set default vibespace to first one if available
	defaultVS := ""
	if len(sess.Vibespaces) > 0 {
		defaultVS = sess.Vibespaces[0].Name
	}

	return &Model{
		session:          sess,
		isAdHoc:          isAdHoc,
		agents:           make(map[string]*AgentConn),
		agentOrder:       make([]string, 0),
		daemons:          make(map[string]*daemon.Client),
		input:            ti,
		outputs:          make(map[string]*OutputBuffer),
		styles:           NewStyles(),
		layout:           LayoutSplit,
		ctx:              ctx,
		cancel:           cancel,
		defaultVibespace: defaultVS,
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.initConnections(),
	)
}

// initConnections starts connecting to all agents
func (m *Model) initConnections() tea.Cmd {
	return func() tea.Msg {
		var errors []error

		// Ensure daemons are running for each vibespace
		for _, vs := range m.session.Vibespaces {
			if err := m.ensureDaemon(vs.Name); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", vs.Name, err))
				continue
			}

			// Get agents for this vibespace
			agents, err := m.getAgentsForVibespace(vs)
			if err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", vs.Name, err))
				continue
			}

			// Connect to each agent
			for _, agentName := range agents {
				addr := session.AgentAddress{Agent: agentName, Vibespace: vs.Name}
				if err := m.connectAgent(addr); err != nil {
					errors = append(errors, fmt.Errorf("%s: %w", addr.String(), err))
				}
			}
		}

		return InitCompleteMsg{Errors: errors}
	}
}

// ensureDaemon ensures the daemon is running for a vibespace
func (m *Model) ensureDaemon(vsName string) error {
	// Check if already have client
	if _, ok := m.daemons[vsName]; ok {
		return nil
	}

	// Check if daemon is running, start if not
	if !daemon.IsRunning(vsName) {
		if err := daemon.SpawnDaemon(vsName); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
		// Wait for daemon to be ready
		if err := daemon.WaitForReady(vsName, 10*time.Second); err != nil {
			return fmt.Errorf("daemon failed to start: %w", err)
		}
	}

	// Create client
	client, err := daemon.NewClient(vsName)
	if err != nil {
		return fmt.Errorf("failed to create daemon client: %w", err)
	}

	m.daemons[vsName] = client
	return nil
}

// getAgentsForVibespace returns the list of agents to connect to for a vibespace
func (m *Model) getAgentsForVibespace(vs session.VibespaceEntry) ([]string, error) {
	// If specific agents are listed, use those
	if len(vs.Agents) > 0 {
		return vs.Agents, nil
	}

	// Otherwise, get all agents from the daemon
	client, ok := m.daemons[vs.Name]
	if !ok {
		return nil, fmt.Errorf("no daemon client for %s", vs.Name)
	}

	status, err := client.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get daemon status: %w", err)
	}

	agents := make([]string, 0, len(status.Agents))
	for _, a := range status.Agents {
		agents = append(agents, a.Name)
	}

	return agents, nil
}

// connectAgent connects to an agent
func (m *Model) connectAgent(addr session.AgentAddress) error {
	// Get SSH port from daemon
	client, ok := m.daemons[addr.Vibespace]
	if !ok {
		return fmt.Errorf("no daemon client for %s", addr.Vibespace)
	}

	forwards, err := client.ListForwards()
	if err != nil {
		return fmt.Errorf("failed to list forwards: %w", err)
	}

	// Find SSH port for this agent
	var sshPort int
	for _, agent := range forwards.Agents {
		if agent.Name == addr.Agent {
			for _, fwd := range agent.Forwards {
				if fwd.Type == "ssh" && fwd.Status == "active" {
					sshPort = fwd.LocalPort
					break
				}
			}
		}
	}

	if sshPort == 0 {
		return fmt.Errorf("no active SSH forward for %s", addr.String())
	}

	// Create agent connection
	conn := NewAgentConn(addr, sshPort)
	if err := conn.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Register agent
	m.agentMu.Lock()
	key := addr.String()
	m.agents[key] = conn
	m.agentOrder = append(m.agentOrder, key)
	m.outputs[key] = NewOutputBuffer(1000)
	m.agentMu.Unlock()

	return nil
}

// GetAgentCount returns the number of connected agents
func (m *Model) GetAgentCount() int {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()
	return len(m.agents)
}

// GetAgentColor returns the color for an agent
func (m *Model) GetAgentColor(addr string) lipgloss.Color {
	m.agentMu.RLock()
	defer m.agentMu.RUnlock()

	for i, key := range m.agentOrder {
		if key == addr {
			return GetAgentColor(i)
		}
	}
	return GetAgentColor(0)
}

// UpdateSuggestions updates the autocomplete suggestions based on current input
func (m *Model) UpdateSuggestions() {
	input := m.input.Value()

	// Only show suggestions when typing @mention
	if !strings.HasPrefix(input, "@") {
		m.input.SetSuggestions(nil)
		return
	}

	// Get the partial mention (everything after @, before space)
	mention := strings.TrimPrefix(input, "@")
	if idx := strings.Index(mention, " "); idx != -1 {
		// Already have a space, no more suggestions needed
		m.input.SetSuggestions(nil)
		return
	}

	// Build suggestions list - must include @ prefix and trailing space
	// because suggestions replace the entire input value
	var suggestions []string

	// Add "all" option
	if strings.HasPrefix("all", strings.ToLower(mention)) {
		suggestions = append(suggestions, "@all ")
	}

	// Add connected agents
	m.agentMu.RLock()
	for _, key := range m.agentOrder {
		// key is "agent@vibespace"
		if strings.HasPrefix(strings.ToLower(key), strings.ToLower(mention)) {
			suggestions = append(suggestions, "@"+key+" ")
		}
	}
	m.agentMu.RUnlock()

	m.input.SetSuggestions(suggestions)
}

// Close cleans up all connections
func (m *Model) Close() {
	m.cancel()

	m.agentMu.Lock()
	defer m.agentMu.Unlock()

	for _, conn := range m.agents {
		conn.Close()
	}
}

// Run starts the TUI
func Run(sess *session.Session, isAdHoc bool) error {
	m := NewModel(sess, isAdHoc)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	m.Close()
	return err
}

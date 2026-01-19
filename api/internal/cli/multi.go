package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fatih/color"
)

// AgentConnection represents a connection to a Claude agent
type AgentConnection struct {
	ID     string
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Cmd    *exec.Cmd
	Color  *color.Color
}

// MultiSession manages connections to multiple Claude agents
type MultiSession struct {
	vibespace    string
	vibespaceID  string // Internal ID for pod selection
	agents       map[string]*AgentConnection
	kubeconfig   string
	kubectlBin   string
	mu           sync.Mutex
	colorPalette []*color.Color
	colorIndex   int
}

func runMulti(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Verify vibespace exists and is running
	vs, err := svc.Get(ctx, vibespace)
	if err != nil {
		return fmt.Errorf("vibespace '%s' not found", vibespace)
	}

	if vs.Status != "running" {
		return fmt.Errorf("vibespace '%s' is not running. Start it with: vibespace %s start", vibespace, vibespace)
	}

	// Get paths
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubectlBin := filepath.Join(home, ".vibespace", "bin", "kubectl")

	session := &MultiSession{
		vibespace:   vibespace,
		vibespaceID: vs.ID,
		agents:      make(map[string]*AgentConnection),
		kubeconfig:  kubeconfig,
		kubectlBin:  kubectlBin,
		colorPalette: []*color.Color{
			color.New(color.FgCyan),
			color.New(color.FgMagenta),
			color.New(color.FgYellow),
			color.New(color.FgGreen),
			color.New(color.FgBlue),
		},
	}

	// Connect to the default agent
	if err := session.connectAgent(ctx, "claude-1"); err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}

	return session.run(ctx)
}

func (s *MultiSession) connectAgent(ctx context.Context, agentID string) error {
	// Find the pod using internal ID
	podSelector := fmt.Sprintf("vibespace.dev/id=%s", s.vibespaceID)

	findCmd := exec.CommandContext(ctx, s.kubectlBin,
		"--kubeconfig", s.kubeconfig,
		"-n", "vibespace",
		"get", "pod",
		"-l", podSelector,
		"-o", "jsonpath={.items[0].metadata.name}",
	)
	podNameBytes, err := findCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to find pod: %w", err)
	}
	podName := string(podNameBytes)
	if podName == "" {
		return fmt.Errorf("no running pod found")
	}

	// Start kubectl exec
	cmd := exec.CommandContext(ctx, s.kubectlBin,
		"--kubeconfig", s.kubeconfig,
		"-n", "vibespace",
		"exec", "-i", podName,
		"--", "/bin/bash",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	s.mu.Lock()
	s.agents[agentID] = &AgentConnection{
		ID:     agentID,
		Stdin:  stdin,
		Stdout: stdout,
		Cmd:    cmd,
		Color:  s.colorPalette[s.colorIndex%len(s.colorPalette)],
	}
	s.colorIndex++
	s.mu.Unlock()

	return nil
}

func (s *MultiSession) run(ctx context.Context) error {
	// Print instructions
	fmt.Println("Multi-agent terminal mode")
	fmt.Println("Commands:")
	fmt.Println("  @<agent> <message>  Send message to specific agent")
	fmt.Println("  @all <message>      Send message to all agents")
	fmt.Println("  /list               List connected agents")
	fmt.Println("  /quit               Exit multi-agent mode")
	fmt.Println()

	// List connected agents
	s.mu.Lock()
	fmt.Print("Connected agents: ")
	agentNames := make([]string, 0, len(s.agents))
	for name, agent := range s.agents {
		agentNames = append(agentNames, agent.Color.Sprint(name))
	}
	fmt.Println(strings.Join(agentNames, ", "))
	s.mu.Unlock()
	fmt.Println()

	// Start output readers for each agent
	var wg sync.WaitGroup
	for _, agent := range s.agents {
		wg.Add(1)
		go func(a *AgentConnection) {
			defer wg.Done()
			s.readAgentOutput(a)
		}(agent)
	}

	// Read user input
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "/") {
			// Handle command
			cmd := strings.TrimPrefix(line, "/")
			switch cmd {
			case "quit", "exit":
				s.cleanup()
				return nil
			case "list":
				s.mu.Lock()
				for name, agent := range s.agents {
					fmt.Printf("  %s\n", agent.Color.Sprint(name))
				}
				s.mu.Unlock()
			default:
				fmt.Printf("Unknown command: /%s\n", cmd)
			}
		} else if strings.HasPrefix(line, "@") {
			// Parse @target message
			parts := strings.SplitN(line[1:], " ", 2)
			if len(parts) < 2 {
				fmt.Println("Usage: @<agent> <message> or @all <message>")
			} else {
				target := parts[0]
				message := parts[1]
				s.sendMessage(target, message)
			}
		} else if line != "" {
			// Send to all by default
			s.sendMessage("all", line)
		}

		fmt.Print("> ")
	}

	s.cleanup()
	wg.Wait()
	return nil
}

func (s *MultiSession) sendMessage(target, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if target == "all" {
		for _, agent := range s.agents {
			fmt.Fprintf(agent.Stdin, "%s\n", message)
		}
	} else {
		agent, ok := s.agents[target]
		if !ok {
			fmt.Printf("Unknown agent: %s\n", target)
			return
		}
		fmt.Fprintf(agent.Stdin, "%s\n", message)
	}
}

func (s *MultiSession) readAgentOutput(agent *AgentConnection) {
	scanner := bufio.NewScanner(agent.Stdout)
	for scanner.Scan() {
		line := scanner.Text()
		// Print with agent prefix in color
		fmt.Printf("\r%s %s\n> ", agent.Color.Sprintf("[%s]", agent.ID), line)
	}
}

func (s *MultiSession) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, agent := range s.agents {
		agent.Stdin.Close()
		agent.Cmd.Process.Kill()
	}
}

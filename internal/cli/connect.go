package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/yagizdagabak/vibespace/pkg/agent"
	"github.com/yagizdagabak/vibespace/pkg/vibespace"
)

// browserFlag tracks whether to open browser instead of terminal
var connectBrowserFlag bool

func runConnect(vsName string, args []string) error {
	ctx := context.Background()

	// Parse flags from args
	browser := false
	agentName := "" // Will be determined from vibespace if not specified
	runAgent := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser", "-b":
			browser = true
		case "--agent", "-a":
			if i+1 < len(args) {
				agentName = args[i+1]
				runAgent = true
				i++
			}
		default:
			// If arg doesn't start with -, treat as agent name
			if len(args[i]) > 0 && args[i][0] != '-' {
				agentName = args[i]
				runAgent = true
			}
		}
	}

	// Also check global flag if set
	if connectBrowserFlag {
		browser = true
	}

	// Get the vibespace service to look up agent info
	svc, err := getVibespaceService()
	if err != nil {
		return fmt.Errorf("failed to get vibespace service: %w", err)
	}

	// Get the list of agents to determine the default/validate the specified one
	agents, err := svc.ListAgents(ctx, vsName)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		return fmt.Errorf("no agents found in vibespace '%s'", vsName)
	}

	// If no agent specified, use the primary agent (agent-num=1)
	if agentName == "" {
		for _, ag := range agents {
			if ag.AgentNum == 1 {
				agentName = ag.AgentName
				break
			}
		}
		// Fallback to first agent if no agent-num=1 found
		if agentName == "" {
			agentName = agents[0].AgentName
		}
	}

	// Find the specified agent's info
	var targetAgent *vibespace.AgentInfo
	for i := range agents {
		if agents[i].AgentName == agentName {
			targetAgent = &agents[i]
			break
		}
	}

	if targetAgent == nil {
		return fmt.Errorf("agent '%s' not found in vibespace '%s'", agentName, vsName)
	}

	mode := "ssh"
	if browser {
		mode = "browser"
	}
	slog.Info("connect command started", "vibespace", vsName, "mode", mode, "agent", agentName, "agent_type", targetAgent.AgentType)

	if browser {
		// For browser access, use ttyd port
		localPort, err := ensureDaemonRunningForAgent(ctx, vsName, agentName, "ttyd")
		if err != nil {
			slog.Error("failed to ensure daemon running for ttyd", "vibespace", vsName, "agent", agentName, "error", err)
			return err
		}
		url := fmt.Sprintf("http://localhost:%d", localPort)
		printStep("Opening browser for %s in %s...", agentName, vsName)
		slog.Info("connect command completed", "vibespace", vsName, "mode", "browser", "url", url)
		return openBrowser(url)
	}

	// For CLI access, use SSH port
	localPort, err := ensureDaemonRunningForAgent(ctx, vsName, agentName, "ssh")
	if err != nil {
		slog.Error("failed to ensure daemon running for ssh", "vibespace", vsName, "agent", agentName, "error", err)
		return err
	}

	// If agent was explicitly specified, run agent interactively
	// Otherwise just give a shell (all agents share the same filesystem)
	if runAgent {
		// Get agent config
		config, err := svc.GetAgentConfig(ctx, vsName, agentName)
		if err != nil {
			slog.Warn("failed to get agent config, using defaults", "error", err)
			config = nil
		}

		// Get the agent implementation based on the agent's type
		agentImpl, err := agent.Get(targetAgent.AgentType)
		if err != nil {
			// Fallback to Claude Code if unknown type
			slog.Warn("unknown agent type, falling back to claude-code", "type", targetAgent.AgentType)
			agentImpl = agent.MustGet(agent.TypeClaudeCode)
		}

		// Build the interactive command using the agent abstraction
		cmd := buildInteractiveCommand(agentImpl, config)
		printStep("Connecting to %s in %s...", agentName, vsName)
		slog.Info("connect command completed", "vibespace", vsName, "mode", "ssh", "agent", agentName, "local_port", localPort)
		return connectViaSSH(localPort, cmd)
	}

	printStep("Connecting to shell in %s...", vsName)
	slog.Info("connect command completed", "vibespace", vsName, "mode", "ssh-shell", "local_port", localPort)
	return connectViaSSH(localPort, "")
}

// buildInteractiveCommand builds the command to run the agent interactively
func buildInteractiveCommand(agentImpl agent.CodingAgent, config *agent.Config) string {
	// Use the agent's BuildInteractiveCommand method
	agentCmd := agentImpl.BuildInteractiveCommand("", config)
	// Wrap with cd to vibespace directory
	return fmt.Sprintf("cd /vibespace && %s", agentCmd)
}

// connectViaSSH connects to the vibespace via native SSH
// If remoteCmd is non-empty, it runs that command instead of a shell
func connectViaSSH(localPort int, remoteCmd string) error {
	// Get the dedicated vibespace private key
	privateKeyPath := vibespace.GetSSHPrivateKeyPath()
	if privateKeyPath == "" {
		return fmt.Errorf("no SSH key found - run 'vibespace create' first to generate keys")
	}

	sshArgs := []string{
		"-i", privateKeyPath,
		"-p", strconv.Itoa(localPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-t", // Force pseudo-terminal allocation for interactive commands
		"user@localhost",
	}

	// Append remote command if specified
	if remoteCmd != "" {
		sshArgs = append(sshArgs, remoteCmd)
	}

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// openBrowser opens the URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	printSuccess("Browser opened: %s", url)
	return nil
}

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vibespacehq/vibespace/pkg/agent"
	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

var connectCmd = &cobra.Command{
	Use:   "connect [agent]",
	Short: "Connect to an agent",
	Long:  `Connect to an agent in a vibespace via SSH (terminal) or browser (ttyd).`,
	Example: `  vibespace connect --vibespace myproject
  vibespace connect claude-2 --vibespace myproject
  vibespace connect --browser --vibespace myproject`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}

		browser, _ := cmd.Flags().GetBool("browser")
		agentFlag, _ := cmd.Flags().GetString("agent")

		agentName := ""
		runAgent := false
		if agentFlag != "" {
			agentName = agentFlag
			runAgent = true
		} else if len(args) > 0 {
			agentName = args[0]
			runAgent = true
		}

		return doConnect(vs, agentName, runAgent, browser)
	},
}

func init() {
	connectCmd.Flags().BoolP("browser", "b", false, "Open in web browser instead of terminal")
	connectCmd.Flags().StringP("agent", "a", "", "Specify agent name")
}

func doConnect(vsName, agentName string, runAgent, browser bool) error {
	ctx := context.Background()

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
		return fmt.Errorf("agent '%s' not found in vibespace '%s': %w", agentName, vsName, vserrors.ErrAgentNotFound)
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
	// Use the agent's BuildInteractiveCommand method (returns []string)
	agentArgs := agentImpl.BuildInteractiveCommand("", config)
	// Wrap with cd and bash login shell using safe shell quoting
	return agent.WrapForSSHRemote(agentArgs)
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

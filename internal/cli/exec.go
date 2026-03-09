package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

var execCmd = &cobra.Command{
	Use:   "exec [agent] -- <command>",
	Short: "Run a command in an agent's container",
	Long: `Run a command in an agent's container via SSH.

All flags (--vibespace, --json, etc.) must appear before the command arguments.
Use -- to explicitly separate flags from the command.`,
	Example: `  vibespace exec --vibespace myproject whoami
  vibespace exec --vibespace myproject claude-1 whoami
  vibespace exec --vibespace myproject -- ls -la /workspace
  vibespace exec --vibespace myproject claude-2 -- cat /etc/os-release`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doExec(vs, args)
	},
}

func init() {
	execCmd.Flags().SetInterspersed(false)
}

func doExec(vsName string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	// Get vibespace service to look up agents
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	// Get list of agents
	agents, err := svc.ListAgents(ctx, vsName)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		return fmt.Errorf("no agents found in vibespace '%s'", vsName)
	}

	// Determine agent name and command
	agentName := ""
	var cmdArgs []string

	// Check if first arg is an agent name
	for _, ag := range agents {
		if ag.AgentName == args[0] {
			agentName = args[0]
			cmdArgs = args[1:]
			break
		}
	}

	// If no agent matched, use primary agent and all args are the command
	if agentName == "" {
		for _, ag := range agents {
			if ag.AgentNum == 1 {
				agentName = ag.AgentName
				break
			}
		}
		if agentName == "" {
			agentName = agents[0].AgentName
		}
		cmdArgs = args
	}

	// Strip leading "--" if present (CLI argument separator)
	if len(cmdArgs) > 0 && cmdArgs[0] == "--" {
		cmdArgs = cmdArgs[1:]
	}

	if len(cmdArgs) == 0 {
		return fmt.Errorf("no command specified")
	}

	// Get SSH port for agent
	localPort, err := ensureDaemonRunningForAgent(ctx, vsName, agentName, "ssh")
	if err != nil {
		return err
	}

	// Build the remote command
	remoteCmd := strings.Join(cmdArgs, " ")

	// Execute via SSH
	stdout, stderr, exitCode, err := execViaSSH(localPort, remoteCmd)
	if err != nil {
		return fmt.Errorf("ssh error: %w", err)
	}

	// Output results
	if globalJSON {
		return out.JSON(NewJSONOutput(exitCode == 0, ExecOutput{
			Vibespace: vsName,
			Agent:     agentName,
			Command:   remoteCmd,
			Stdout:    stdout,
			Stderr:    stderr,
			ExitCode:  exitCode,
		}, nil))
	}

	// Plain output - just print stdout/stderr
	if stdout != "" {
		fmt.Print(stdout)
	}
	if stderr != "" {
		fmt.Print(stderr)
	}

	if exitCode != 0 {
		return fmt.Errorf("command exited with code %d", exitCode)
	}

	return nil
}

// execViaSSH executes a command via SSH and returns stdout, stderr, and exit code
func execViaSSH(localPort int, remoteCmd string) (string, string, int, error) {
	privateKeyPath := vibespace.GetSSHPrivateKeyPath()
	if privateKeyPath == "" {
		return "", "", 1, fmt.Errorf("no SSH key found")
	}

	sshArgs := []string{
		"-i", privateKeyPath,
		"-p", strconv.Itoa(localPort),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=~/.vibespace/known_hosts",
		"-o", "LogLevel=ERROR",
		"-o", "BatchMode=yes",
		"user@localhost",
		"--", // POSIX: end of options, prevents remoteCmd from being parsed as SSH flags
		remoteCmd,
	}

	cmd := exec.Command("ssh", sshArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", "", 1, err
		}
	}

	return stdout.String(), stderr.String(), exitCode, nil
}

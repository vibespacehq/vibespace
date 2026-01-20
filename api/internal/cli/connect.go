package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"vibespace/pkg/vibespace"
)

// browserFlag tracks whether to open browser instead of terminal
var connectBrowserFlag bool

func runConnect(vibespace string, args []string) error {
	ctx := context.Background()

	// Parse flags from args
	browser := false
	agent := ""           // Empty = shell only, specified = run claude
	agentSpecified := false // Track if agent was explicitly specified

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser", "-b":
			browser = true
		case "--agent", "-a":
			if i+1 < len(args) {
				agent = args[i+1]
				agentSpecified = true
				i++
			}
		default:
			// If arg doesn't start with -, treat as agent name
			if len(args[i]) > 0 && args[i][0] != '-' {
				agent = args[i]
				agentSpecified = true
			}
		}
	}

	// Also check global flag if set
	if connectBrowserFlag {
		browser = true
	}

	// For browser, default to claude-1
	if browser && agent == "" {
		agent = "claude-1"
	}

	mode := "ssh"
	if browser {
		mode = "browser"
	}
	slog.Info("connect command started", "vibespace", vibespace, "mode", mode, "agent", agent)

	if browser {
		// For browser access, use ttyd port
		localPort, err := ensureDaemonRunningTTYD(ctx, vibespace, agent)
		if err != nil {
			slog.Error("failed to ensure daemon running for ttyd", "vibespace", vibespace, "agent", agent, "error", err)
			return err
		}
		url := fmt.Sprintf("http://localhost:%d", localPort)
		printStep("Opening browser for %s in %s...", agent, vibespace)
		slog.Info("connect command completed", "vibespace", vibespace, "mode", "browser", "url", url)
		return openBrowser(url)
	}

	// For CLI access, use SSH port
	// All agents share the same PVC, so we can use any available agent for shell access
	containerAgent := agent
	if containerAgent == "" {
		// No specific agent requested - use first available
		containerAgent = "" // Will be resolved by ensureDaemonRunningSSH
	}

	localPort, err := ensureDaemonRunningSSHAnyAgent(ctx, vibespace, containerAgent)
	if err != nil {
		slog.Error("failed to ensure daemon running for ssh", "vibespace", vibespace, "agent", containerAgent, "error", err)
		return err
	}

	// If agent was specified, run claude interactively
	// If no agent specified, just give a shell
	if agentSpecified {
		printStep("Connecting to %s in %s...", agent, vibespace)
		slog.Info("connect command completed", "vibespace", vibespace, "mode", "ssh", "agent", agent, "local_port", localPort)
		// Use login shell to ensure PATH and environment are set up
		return connectViaSSH(localPort, "bash -l -c claude")
	}

	printStep("Connecting to shell in %s...", vibespace)
	slog.Info("connect command completed", "vibespace", vibespace, "mode", "ssh-shell", "local_port", localPort)
	return connectViaSSH(localPort, "")
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

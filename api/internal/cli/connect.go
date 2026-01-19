package cli

import (
	"context"
	"fmt"
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
	agent := "claude-1"

	filteredArgs := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser", "-b":
			browser = true
		case "--agent", "-a":
			if i+1 < len(args) {
				agent = args[i+1]
				i++
			}
		default:
			// If arg doesn't start with -, treat as agent name
			if len(args[i]) > 0 && args[i][0] != '-' {
				agent = args[i]
			} else {
				filteredArgs = append(filteredArgs, args[i])
			}
		}
	}

	// Also check global flag if set
	if connectBrowserFlag {
		browser = true
	}

	if browser {
		// For browser access, use ttyd port
		localPort, err := ensureDaemonRunningTTYD(ctx, vibespace, agent)
		if err != nil {
			return err
		}
		url := fmt.Sprintf("http://localhost:%d", localPort)
		printStep("Opening browser for %s in %s...", agent, vibespace)
		return openBrowser(url)
	}

	// For CLI access, use SSH port
	localPort, err := ensureDaemonRunningSSH(ctx, vibespace, agent)
	if err != nil {
		return err
	}

	printStep("Connecting to %s in %s via SSH...", agent, vibespace)
	return connectViaSSH(localPort)
}

// connectViaSSH connects to the vibespace via native SSH
func connectViaSSH(localPort int) error {
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
		"user@localhost",
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

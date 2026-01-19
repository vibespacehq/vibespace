package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	gottyclient "github.com/moul/gotty-client"
	"golang.org/x/term"
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

	// Ensure daemon is running and get the local port for this agent
	localPort, err := ensureDaemonRunning(ctx, vibespace, agent)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d", localPort)

	if browser {
		printStep("Opening browser for %s in %s...", agent, vibespace)
		return openBrowser(url)
	}

	printStep("Connecting to %s in %s...", agent, vibespace)
	return connectViaGottyClient(url)
}

// connectViaGottyClient connects to a GoTTY server using the gotty-client library
func connectViaGottyClient(url string) error {
	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("stdin is not a terminal - use --browser flag instead")
	}

	// Put terminal in raw mode for proper character handling
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	client, err := gottyclient.NewClient(url)
	if err != nil {
		return fmt.Errorf("failed to create gotty client: %w", err)
	}

	// Configure client
	client.SkipTLSVerify = true
	client.V2 = true // Enable v2 protocol for sorenisanerd/gotty fork

	// Connect to the server
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Note: Message printed before raw mode since raw mode affects output
	// The Loop() function handles all terminal I/O
	if err := client.Loop(); err != nil {
		// Check if this is a normal disconnect
		if err.Error() == "websocket: close 1000 (normal)" {
			return nil
		}
		return fmt.Errorf("connection error: %w", err)
	}

	return nil
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

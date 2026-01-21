package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"vibespace/pkg/session"
	"vibespace/pkg/tui"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// multiCmd is the top-level multi command for quick ad-hoc sessions
var multiCmd = &cobra.Command{
	Use:   "multi <vibespace>... [message]",
	Short: "Start multi-agent terminal with specified vibespaces",
	Long: `Start an ad-hoc multi-agent terminal session with one or more vibespaces.

This launches a terminal UI where you can interact with multiple Claude agents
across the specified vibespaces simultaneously.

Interactive mode (default when TTY):
  vibespace multi projectA           # Single vibespace
  vibespace multi projectA projectB  # Multiple vibespaces

Non-interactive mode (for scripting):
  vibespace multi projectA --json "list files"
  echo "hello" | vibespace multi projectA --json
  vibespace multi projectA --json --agent claude-1 "check logs"

Inside the TUI:
  @<agent> <message>                 Send to specific agent
  @<agent>@<vibespace> <message>     Send to agent in specific vibespace
  @all <message>                     Broadcast to all agents
  /list                              List connected agents
  /focus <agent>                     Focus on single agent
  /split                             Return to split view
  /save <name>                       Save as named session
  /quit                              Exit

Keyboard shortcuts:
  Up/Down                            Scroll chat history
  PgUp/PgDown                        Scroll by page
  Home/End                           Jump to top/bottom
  Tab                                Autocomplete agent names
  Ctrl+C                             Exit`,
	Args: cobra.MinimumNArgs(1),
	RunE: runMultiCmd,
}

func init() {
	// Add flags for non-interactive mode
	multiCmd.Flags().Bool("json", false, "Output JSON (auto-enabled when not TTY)")
	multiCmd.Flags().String("agent", "all", "Target agent for non-interactive mode (default: all)")
	multiCmd.Flags().Bool("batch", false, "Batch mode: read JSONL messages from stdin")
	multiCmd.Flags().Duration("timeout", 2*time.Minute, "Response timeout for non-interactive mode")
}

// runMultiCmd handles the top-level multi command
func runMultiCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse flags
	jsonFlag, _ := cmd.Flags().GetBool("json")
	agentFlag, _ := cmd.Flags().GetString("agent")
	batchFlag, _ := cmd.Flags().GetBool("batch")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Detect TTY - auto-enable JSON mode when not interactive
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if !stdinTTY || !stdoutTTY {
		jsonFlag = true
	}

	// Separate vibespaces from potential message
	vibespaces := args
	var message string

	// Check if the last argument looks like a message (doesn't exist as vibespace)
	if len(args) > 1 && jsonFlag {
		// In JSON mode, last arg might be the message
		lastArg := args[len(args)-1]
		// Check if it looks like a message (contains spaces or isn't a valid vibespace name)
		if strings.Contains(lastArg, " ") || !isValidVibespace(ctx, lastArg) {
			vibespaces = args[:len(args)-1]
			message = lastArg
		}
	}

	// Verify all vibespaces exist and are running
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		if jsonFlag {
			outputJSONError(err)
			return nil
		}
		return err
	}

	for _, vsName := range vibespaces {
		_, err := checkVibespaceRunning(ctx, svc, vsName)
		if err != nil {
			if jsonFlag {
				outputJSONError(err)
				return nil
			}
			return err
		}
	}

	// Non-interactive mode
	if jsonFlag {
		// Read message from stdin if not provided
		if message == "" && !stdinTTY {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				message = scanner.Text()
			}
		}

		// Batch mode
		if batchFlag {
			return runBatchMode(ctx, vibespaces, timeout)
		}

		// Single message mode
		if message == "" {
			return fmt.Errorf("message required in non-interactive mode")
		}

		return runNonInteractive(ctx, vibespaces, agentFlag, message, timeout)
	}

	// Interactive TUI mode
	sess := &session.Session{
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Vibespaces: make([]session.VibespaceEntry, 0, len(vibespaces)),
		Layout: session.Layout{
			Mode: session.LayoutModeSplit,
		},
	}

	for _, vs := range vibespaces {
		sess.Vibespaces = append(sess.Vibespaces, session.VibespaceEntry{
			Name: vs,
		})
	}

	// Setup TUI logging before launching (cleanup happens when TUI exits)
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	// Launch TUI
	return tui.Run(sess, true /* isAdHoc */)
}

// runNonInteractive runs in non-interactive mode with JSON output
func runNonInteractive(ctx context.Context, vibespaces []string, target, message string, timeout time.Duration) error {
	runner := tui.NewHeadlessRunner()
	runner.SetTimeout(timeout)
	defer runner.Close()

	// Connect to agents
	if err := runner.Connect(ctx, vibespaces); err != nil {
		outputJSONError(err)
		return nil
	}

	// Send message and wait for responses
	response, err := runner.SendAndWait(ctx, target, message)
	if err != nil {
		outputJSONError(err)
		return nil
	}

	// Set session name based on vibespaces
	response.Session = strings.Join(vibespaces, "-")

	// Output JSON response
	data, err := response.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	fmt.Println(string(data))

	return nil
}

// runBatchMode processes multiple messages from stdin (JSONL format)
func runBatchMode(ctx context.Context, vibespaces []string, timeout time.Duration) error {
	runner := tui.NewHeadlessRunner()
	runner.SetTimeout(timeout)
	defer runner.Close()

	// Connect to agents
	if err := runner.Connect(ctx, vibespaces); err != nil {
		outputJSONError(err)
		return nil
	}

	// Read JSONL from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse request
		var req tui.MultiRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			outputJSONError(fmt.Errorf("invalid request: %w", err))
			continue
		}

		// Send message and wait for responses
		response, err := runner.SendAndWait(ctx, req.Target, req.Message)
		if err != nil {
			outputJSONError(err)
			continue
		}

		// Set session name
		response.Session = strings.Join(vibespaces, "-")

		// Output JSON response
		data, err := response.ToJSON()
		if err != nil {
			outputJSONError(fmt.Errorf("failed to marshal response: %w", err))
			continue
		}
		fmt.Println(string(data))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stdin: %w", err)
	}

	return nil
}

// outputJSONError outputs an error in JSON format
func outputJSONError(err error) {
	response := &tui.MultiResponse{
		Error: err.Error(),
	}
	data, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(data))
}

// isValidVibespace checks if a name is a valid vibespace
func isValidVibespace(ctx context.Context, name string) bool {
	svc, err := getVibespaceService()
	if err != nil {
		return false
	}
	_, err = svc.Get(ctx, name)
	return err == nil
}

// runMulti handles the vibespace-scoped multi command: vibespace <name> multi
func runMulti(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Verify vibespace exists and is running
	_, err = checkVibespaceRunning(ctx, svc, vibespace)
	if err != nil {
		return err
	}

	// Create ad-hoc session with single vibespace
	sess := &session.Session{
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Vibespaces: []session.VibespaceEntry{
			{Name: vibespace},
		},
		Layout: session.Layout{
			Mode: session.LayoutModeSplit,
		},
	}

	// Setup TUI logging before launching (cleanup happens when TUI exits)
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	// Launch TUI
	return tui.Run(sess, true /* isAdHoc */)
}

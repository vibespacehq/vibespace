package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yagizdagabak/vibespace/pkg/session"
	"github.com/yagizdagabak/vibespace/pkg/tui"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// multiCmd is the top-level multi command
var multiCmd = &cobra.Command{
	Use:   "multi",
	Short: "Start multi-agent terminal session",
	Long: `Start a multi-agent terminal session to interact with Claude agents across vibespaces.

Interactive mode (default):
  vibespace multi                                       # New empty session, add agents in TUI
  vibespace multi --vibespaces test                     # New session with all agents from 'test'
  vibespace multi --vibespaces test,test2               # New session with multiple vibespaces
  vibespace multi --vibespaces test --agents claude-1@other  # Mix vibespace + specific agent
  vibespace multi -r                                    # Resume an existing session (picker)
  vibespace multi --name mywork --vibespaces test       # New session with explicit name
  vibespace test multi                                  # Shorthand for single vibespace

Non-interactive mode (for scripting):
  vibespace multi --vibespaces test --json "list files"
  vibespace multi --vibespaces test --plain --stream "work on task"

Inside the TUI:
  @<agent> <message>                 Send to specific agent
  @all <message>                     Broadcast to all agents
  /add <vibespace>                   Add all agents from a vibespace
  /add <agent>@<vibespace>           Add specific agent
  /remove <vibespace>                Remove vibespace from session
  /list                              List connected agents
  /focus <agent>                     Focus on single agent
  /quit                              Exit`,
	Args: cobra.ArbitraryArgs,
	RunE: runMultiCmd,
}

func init() {
	// Session selection
	multiCmd.Flags().BoolP("resume", "r", false, "Resume an existing session (shows picker)")

	// Session composition (no shortcuts for clarity)
	multiCmd.Flags().StringSlice("vibespaces", nil, "Vibespaces to include (all agents)")
	multiCmd.Flags().StringSlice("agents", nil, "Specific agents to include (format: agent@vibespace)")
	multiCmd.Flags().String("name", "", "Session name (default: auto-generated UUID)")

	// Non-interactive mode flags
	multiCmd.Flags().String("agent", "all", "Target agent for non-interactive mode")
	multiCmd.Flags().Bool("batch", false, "Batch mode: read JSONL messages from stdin")
	multiCmd.Flags().Bool("list-agents", false, "List connected agents and exit")
	multiCmd.Flags().Bool("stream", false, "Stream responses as they arrive (plain text mode)")
	multiCmd.Flags().Duration("timeout", 2*time.Minute, "Response timeout for non-interactive mode")
}

// runMultiCmd handles the multi command
func runMultiCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse flags
	resumeFlag, _ := cmd.Flags().GetBool("resume")
	vibespaces, _ := cmd.Flags().GetStringSlice("vibespaces")
	agents, _ := cmd.Flags().GetStringSlice("agents")
	nameFlag, _ := cmd.Flags().GetString("name")

	// Non-interactive flags
	jsonFlag := globalJSON
	plainFlag := globalPlain
	agentFlag, _ := cmd.Flags().GetString("agent")
	batchFlag, _ := cmd.Flags().GetBool("batch")
	listAgentsFlag, _ := cmd.Flags().GetBool("list-agents")
	streamFlag, _ := cmd.Flags().GetBool("stream")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Detect TTY
	stdinTTY := term.IsTerminal(int(os.Stdin.Fd()))
	stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// Determine if non-interactive mode
	nonInteractive := jsonFlag || plainFlag || !stdinTTY || !stdoutTTY
	if nonInteractive && !plainFlag {
		jsonFlag = true
	}

	// Handle resume flag - show session picker
	if resumeFlag {
		return runSessionPicker(ctx)
	}

	// Validate vibespaces exist
	if len(vibespaces) > 0 || len(agents) > 0 {
		svc, err := getVibespaceServiceWithCheck()
		if err != nil {
			if jsonFlag {
				outputJSONError(err)
				return nil
			}
			return err
		}

		// Validate all vibespaces
		for _, vs := range vibespaces {
			if _, err := checkVibespaceRunning(ctx, svc, vs); err != nil {
				if jsonFlag {
					outputJSONError(err)
					return nil
				}
				return err
			}
		}

		// Validate vibespaces from agent addresses
		for _, agentAddr := range agents {
			addr := session.ParseAgentAddress(agentAddr, "")
			if addr.Vibespace == "" {
				err := fmt.Errorf("agent '%s' must include vibespace (format: agent@vibespace)", agentAddr)
				if jsonFlag {
					outputJSONError(err)
					return nil
				}
				return err
			}
			if _, err := checkVibespaceRunning(ctx, svc, addr.Vibespace); err != nil {
				if jsonFlag {
					outputJSONError(err)
					return nil
				}
				return err
			}
		}
	}

	// Build session
	sess := buildSession(nameFlag, vibespaces, agents)

	// Save session (both interactive and non-interactive modes)
	store, err := session.NewStore()
	if err != nil {
		if jsonFlag {
			outputJSONError(err)
			return nil
		}
		return err
	}
	if err := store.Save(sess); err != nil {
		if jsonFlag {
			outputJSONError(err)
			return nil
		}
		return err
	}

	// Non-interactive mode needs at least one vibespace
	if nonInteractive {
		if len(sess.Vibespaces) == 0 {
			err := fmt.Errorf("non-interactive mode requires at least one vibespace (-v flag)")
			if jsonFlag {
				outputJSONError(err)
				return nil
			}
			return err
		}

		if listAgentsFlag {
			return runListAgents(ctx, sess.Vibespaces, jsonFlag, timeout)
		}

		if batchFlag {
			return runBatchMode(ctx, sess.Vibespaces, timeout, sess.Name)
		}

		// Get message from args or stdin
		var message string
		if len(args) > 0 {
			message = strings.Join(args, " ")
		} else if !stdinTTY {
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				message = scanner.Text()
			}
		}

		if message == "" {
			err := fmt.Errorf("message required in non-interactive mode")
			if jsonFlag {
				outputJSONError(err)
				return nil
			}
			return err
		}

		if streamFlag && plainFlag {
			return runStreamingPlain(ctx, sess.Vibespaces, agentFlag, message, timeout, sess.Name)
		}

		if plainFlag {
			return runPlainText(ctx, sess.Vibespaces, agentFlag, message, timeout, sess.Name)
		}

		return runNonInteractive(ctx, sess.Vibespaces, agentFlag, message, timeout, sess.Name)
	}

	// Interactive TUI mode
	// Setup TUI logging
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	return tui.Run(sess, false)
}

// buildSession creates a session from the provided flags
func buildSession(name string, vibespaces []string, agents []string) *session.Session {
	if name == "" {
		name = uuid.New().String()[:8] // Short UUID
	}

	sess := &session.Session{
		Name:       name,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		Vibespaces: []session.VibespaceEntry{},
		Layout: session.Layout{
			Mode: session.LayoutModeSplit,
		},
	}

	// Add whole vibespaces
	for _, vs := range vibespaces {
		sess.AddVibespace(vs, nil)
	}

	// Add specific agents
	for _, agentAddr := range agents {
		addr := session.ParseAgentAddress(agentAddr, "")
		if addr.Vibespace != "" {
			sess.AddVibespace(addr.Vibespace, []string{addr.Agent})
		}
	}

	return sess
}

// runSessionPicker shows an interactive session picker
func runSessionPicker(ctx context.Context) error {
	store, err := session.NewStore()
	if err != nil {
		return err
	}

	sessions, err := store.List()
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		fmt.Println()
		fmt.Println("Create a new session with:")
		fmt.Println("  vibespace multi --vibespaces <name>")
		return nil
	}

	// Build options for picker with detailed info
	options := make([]huh.Option[string], len(sessions))
	for i, sess := range sessions {
		lastUsed := formatRelativeTime(sess.LastUsed)

		// Build vibespace and agent info
		var vsInfo, agentInfo string
		if len(sess.Vibespaces) == 0 {
			vsInfo = "(empty)"
			agentInfo = "-"
		} else {
			vsNames := make([]string, len(sess.Vibespaces))
			var agentNames []string
			for j, vs := range sess.Vibespaces {
				vsNames[j] = vs.Name
				if len(vs.Agents) > 0 {
					for _, a := range vs.Agents {
						agentNames = append(agentNames, a+"@"+vs.Name)
					}
				}
			}
			vsInfo = strings.Join(vsNames, ", ")
			if len(agentNames) > 0 {
				agentInfo = strings.Join(agentNames, ", ")
			} else {
				agentInfo = "all"
			}
		}

		// Format with aligned columns
		label := fmt.Sprintf("%-14s │ %-15s │ %-18s │ %s", sess.Name, vsInfo, agentInfo, lastUsed)
		options[i] = huh.NewOption(label, sess.Name)
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Resume Session").
				Description("Select a session to continue working").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if selected == "" {
		return nil // User cancelled
	}

	// Load and start the selected session
	sess, err := store.Get(selected)
	if err != nil {
		return err
	}

	// Update last used
	sess.LastUsed = time.Now()
	_ = store.Save(sess)

	// Setup TUI logging
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	return tui.Run(sess, false)
}

// runNonInteractive runs in non-interactive mode with JSON output
func runNonInteractive(ctx context.Context, vibespaces []session.VibespaceEntry, target, message string, timeout time.Duration, sessionName string) error {
	runner := tui.NewHeadlessRunner()
	runner.SetTimeout(timeout)
	runner.SetSessionName(sessionName)
	defer runner.Close()

	if err := runner.Connect(ctx, vibespaces); err != nil {
		outputJSONError(err)
		return nil
	}

	response, err := runner.SendAndWait(ctx, target, message)
	if err != nil {
		outputJSONError(err)
		return nil
	}

	response.Session = sessionName

	data, err := response.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	fmt.Println(string(data))

	return nil
}

// runBatchMode processes multiple messages from stdin (JSONL format)
func runBatchMode(ctx context.Context, vibespaces []session.VibespaceEntry, timeout time.Duration, sessionName string) error {
	runner := tui.NewHeadlessRunner()
	runner.SetTimeout(timeout)
	runner.SetSessionName(sessionName)
	defer runner.Close()

	if err := runner.Connect(ctx, vibespaces); err != nil {
		outputJSONError(err)
		return nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req tui.MultiRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			outputJSONError(fmt.Errorf("invalid request: %w", err))
			continue
		}

		response, err := runner.SendAndWait(ctx, req.Target, req.Message)
		if err != nil {
			outputJSONError(err)
			continue
		}

		response.Session = sessionName

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

// runListAgents lists connected agents without sending a message
func runListAgents(ctx context.Context, vibespaces []session.VibespaceEntry, jsonOutput bool, timeout time.Duration) error {
	runner := tui.NewHeadlessRunner()
	runner.SetTimeout(timeout)
	defer runner.Close()

	if err := runner.Connect(ctx, vibespaces); err != nil {
		if jsonOutput {
			outputJSONError(err)
			return nil
		}
		return err
	}

	agents := runner.GetAgents()

	// Build session name from vibespace names
	vsNames := make([]string, len(vibespaces))
	for i, vs := range vibespaces {
		vsNames[i] = vs.Name
	}
	sessionName := strings.Join(vsNames, "-")

	if jsonOutput {
		response := struct {
			Session string   `json:"session"`
			Agents  []string `json:"agents"`
		}{
			Session: sessionName,
			Agents:  agents,
		}
		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("Session: %s\n", sessionName)
		fmt.Printf("Agents (%d):\n", len(agents))
		for _, agent := range agents {
			fmt.Printf("  %s\n", agent)
		}
	}

	return nil
}

// runPlainText runs in plain text mode (non-streaming)
func runPlainText(ctx context.Context, vibespaces []session.VibespaceEntry, target, message string, timeout time.Duration, sessionName string) error {
	runner := tui.NewHeadlessRunner()
	runner.SetTimeout(timeout)
	runner.SetSessionName(sessionName)
	defer runner.Close()

	if err := runner.Connect(ctx, vibespaces); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return nil
	}

	response, err := runner.SendAndWait(ctx, target, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return nil
	}

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

	return nil
}

// runMulti handles the vibespace-scoped multi command: vibespace <name> multi
// This provides backward compatibility with the old command style
func runMulti(vsName string, args []string) error {
	ctx := context.Background()

	// Verify vibespace exists and is running
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	if _, err := checkVibespaceRunning(ctx, svc, vsName); err != nil {
		return err
	}

	// Build session with single vibespace
	sess := buildSession("", []string{vsName}, nil)

	// Save session
	store, err := session.NewStore()
	if err != nil {
		return err
	}

	if err := store.Save(sess); err != nil {
		return err
	}

	// Setup TUI logging
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	return tui.Run(sess, false)
}

// runStreamingPlain runs in streaming plain text mode
func runStreamingPlain(ctx context.Context, vibespaces []session.VibespaceEntry, target, message string, timeout time.Duration, sessionName string) error {
	runner := tui.NewHeadlessRunner()
	runner.SetTimeout(timeout)
	runner.SetSessionName(sessionName)
	defer runner.Close()

	if err := runner.Connect(ctx, vibespaces); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return runner.StreamResponses(ctx, target, message, func(agent string, msg *tui.Message) {
		switch msg.Type {
		case tui.MessageTypeAssistant:
			fmt.Printf("[%s] %s\n", agent, msg.Content)
		case tui.MessageTypeToolUse:
			if msg.ToolInput != "" {
				fmt.Printf("[%s] [%s] %s\n", agent, msg.ToolName, msg.ToolInput)
			} else {
				fmt.Printf("[%s] [%s]\n", agent, msg.ToolName)
			}
		case tui.MessageTypeError:
			fmt.Fprintf(os.Stderr, "[%s] Error: %s\n", agent, msg.Content)
		}
	})
}

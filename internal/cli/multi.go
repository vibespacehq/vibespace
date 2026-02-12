package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/tui"

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
  vibespace multi -r <session-id>                       # Resume specific session directly
  vibespace multi --name mywork --vibespaces test       # New session with explicit name
  vibespace test multi                                  # Shorthand for single vibespace

Non-interactive mode (for scripting):
  vibespace multi --list-sessions --json               # List available sessions
  vibespace multi -r <session-id> --json "message"     # Resume session and send message
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
	// Session selection - accepts optional session ID
	// Using NoOptDefVal allows -r without argument to show picker
	multiCmd.Flags().StringP("resume", "r", "", "Resume a session (picker if no ID, or specify session ID)")
	multiCmd.Flag("resume").NoOptDefVal = " " // Space means "flag used but no value"

	// Session composition (no shortcuts for clarity)
	multiCmd.Flags().StringSlice("vibespaces", nil, "Vibespaces to include (all agents)")
	multiCmd.Flags().StringSlice("agents", nil, "Specific agents to include (format: agent@vibespace)")
	multiCmd.Flags().String("name", "", "Session name (default: auto-generated UUID)")

	// Non-interactive mode flags
	multiCmd.Flags().String("agent", "all", "Target agent for non-interactive mode")
	multiCmd.Flags().Bool("batch", false, "Batch mode: read JSONL messages from stdin")
	multiCmd.Flags().Bool("list-agents", false, "List connected agents and exit")
	multiCmd.Flags().Bool("list-sessions", false, "List available sessions and exit (for scripting)")
	multiCmd.Flags().Bool("stream", false, "Stream responses as they arrive (plain text mode)")
	multiCmd.Flags().Duration("timeout", 2*time.Minute, "Response timeout for non-interactive mode")
}

// runMultiCmd handles the multi command
func runMultiCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Setup logging early so it applies to all modes (interactive and non-interactive)
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	// Parse flags
	resumeFlag, _ := cmd.Flags().GetString("resume")
	resumeFlagChanged := cmd.Flags().Changed("resume")
	vibespaces, _ := cmd.Flags().GetStringSlice("vibespaces")
	agents, _ := cmd.Flags().GetStringSlice("agents")
	nameFlag, _ := cmd.Flags().GetString("name")

	// Non-interactive flags
	jsonFlag := globalJSON
	plainFlag := globalPlain
	agentFlag, _ := cmd.Flags().GetString("agent")
	batchFlag, _ := cmd.Flags().GetBool("batch")
	listAgentsFlag, _ := cmd.Flags().GetBool("list-agents")
	listSessionsFlag, _ := cmd.Flags().GetBool("list-sessions")
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

	// Handle --list-sessions flag
	if listSessionsFlag {
		return runListSessions(jsonFlag)
	}

	// Handle resume flag
	if resumeFlagChanged {
		// Trim space - NoOptDefVal uses space to indicate flag without value
		sessionID := strings.TrimSpace(resumeFlag)

		// If no session ID from flag value, check first positional arg
		if sessionID == "" && len(args) > 0 {
			store, err := session.NewStore()
			if err == nil && store.Exists(args[0]) {
				sessionID = args[0]
				args = args[1:] // Remove session ID from args
			}
		}

		return runSessionResume(ctx, sessionID, nonInteractive, jsonFlag, agentFlag, args, timeout)
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

	// Use SaveNew to prevent overwriting existing sessions
	if err := store.SaveNew(sess); err != nil {
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
	return tui.RunAppWithChat(sess, false)
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

// runListSessions lists all available sessions (for scripting)
func runListSessions(jsonOutput bool) error {
	out := getOutput()
	store, err := session.NewStore()
	if err != nil {
		if jsonOutput || out.IsJSONMode() {
			outputJSONError(err)
			return nil
		}
		return err
	}

	sessions, err := store.List()
	if err != nil {
		if jsonOutput || out.IsJSONMode() {
			outputJSONError(err)
			return nil
		}
		return err
	}

	if jsonOutput || out.IsJSONMode() {
		items := make([]MultiSessionItem, len(sessions))
		for i, sess := range sessions {
			vsNames := make([]string, len(sess.Vibespaces))
			for j, vs := range sess.Vibespaces {
				vsNames[j] = vs.Name
			}
			items[i] = MultiSessionItem{
				Name:       sess.Name,
				Vibespaces: vsNames,
				CreatedAt:  sess.CreatedAt.Format(time.RFC3339),
				LastUsed:   sess.LastUsed.Format(time.RFC3339),
			}
		}
		return out.JSON(NewJSONOutput(true, MultiListSessionsOutput{
			Sessions: items,
			Count:    len(items),
		}, nil))
	}

	if len(sessions) == 0 {
		if out.IsPlainMode() {
			return nil
		}
		fmt.Println("No sessions found.")
		return nil
	}

	// Build table
	headers := []string{"SESSION", "VIBESPACES", "LAST USED"}
	rows := make([][]string, len(sessions))
	for i, sess := range sessions {
		vsNames := make([]string, len(sess.Vibespaces))
		for j, vs := range sess.Vibespaces {
			vsNames[j] = vs.Name
		}
		vsInfo := strings.Join(vsNames, ", ")
		if vsInfo == "" {
			vsInfo = "(empty)"
		}
		rows[i] = []string{sess.Name, vsInfo, formatRelativeTime(sess.LastUsed)}
	}

	out.Table(headers, rows)
	return nil
}

// runSessionResume handles resuming a session (interactive or non-interactive)
func runSessionResume(ctx context.Context, sessionID string, nonInteractive, jsonOutput bool, agentTarget string, args []string, timeout time.Duration) error {
	store, err := session.NewStore()
	if err != nil {
		if jsonOutput {
			outputJSONError(err)
			return nil
		}
		return err
	}

	// If no session ID provided
	if sessionID == "" {
		if nonInteractive {
			// In non-interactive mode, list sessions
			return runListSessions(jsonOutput)
		}
		// In interactive mode, show picker
		return runSessionPicker(ctx)
	}

	// Load the specified session
	sess, err := store.Get(sessionID)
	if err != nil {
		if jsonOutput {
			outputJSONError(err)
			return nil
		}
		return err
	}

	// Update last used
	sess.LastUsed = time.Now()
	_ = store.Save(sess)

	if nonInteractive {
		// Non-interactive mode - need a message
		var message string
		if len(args) > 0 {
			message = strings.Join(args, " ")
		}

		if message == "" {
			err := fmt.Errorf("message required when resuming session in non-interactive mode")
			if jsonOutput {
				outputJSONError(err)
				return nil
			}
			return err
		}

		// Run with the resumed session
		runner := tui.NewHeadlessRunner()
		runner.SetTimeout(timeout)
		runner.SetSessionName(sess.Name)
		runner.SetResumeSession(true) // Tell runner to use --resume
		defer runner.Close()

		if err := runner.Connect(ctx, sess.Vibespaces); err != nil {
			if jsonOutput {
				outputJSONError(err)
				return nil
			}
			return err
		}

		response, err := runner.SendAndWait(ctx, agentTarget, message)
		if err != nil {
			if jsonOutput {
				outputJSONError(err)
				return nil
			}
			return err
		}

		response.Session = sess.Name

		if jsonOutput {
			out := getOutput()
			_ = out.JSON(NewJSONOutput(true, convertMultiResponse(response), nil))
		} else {
			for _, agentResp := range response.Responses {
				fmt.Printf("[%s] %s\n", agentResp.Agent, agentResp.Content)
			}
		}

		return nil
	}

	// Interactive mode - start TUI with resume
	return tui.RunAppWithChat(sess, true)
}

// runSessionPicker shows an interactive session picker
func runSessionPicker(ctx context.Context) error {
	out := getOutput()
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

	// Print header
	fmt.Println(out.Bold("Resume Session"))
	fmt.Println()
	fmt.Printf("  %s %s %s %s\n",
		out.Dim(fmt.Sprintf("%-5s", "#")),
		out.Dim(fmt.Sprintf("%-18s", "SESSION")),
		out.Dim(fmt.Sprintf("%-22s", "VIBESPACE(S)")),
		out.Dim("LAST USED"))

	// Print numbered list
	for i, sess := range sessions {
		vsNames := make([]string, len(sess.Vibespaces))
		for j, vs := range sess.Vibespaces {
			vsNames[j] = vs.Name
		}
		vsInfo := strings.Join(vsNames, ", ")
		if vsInfo == "" {
			vsInfo = "(empty)"
		}
		vsInfo = truncateStr(vsInfo, 20)
		lastUsed := formatRelativeTime(sess.LastUsed)

		num := fmt.Sprintf("%-5s", fmt.Sprintf("%d)", i+1))
		fmt.Printf("  %s %s %s %s\n",
			num,
			out.Bold(fmt.Sprintf("%-18s", sess.Name)),
			out.Teal(fmt.Sprintf("%-22s", vsInfo)),
			out.Dim(lastUsed))
	}

	// Prompt for selection
	fmt.Println()
	fmt.Print("Select session (number): ")

	var choice int
	if _, err := fmt.Scan(&choice); err != nil {
		return nil // User cancelled (e.g. Ctrl+C or empty input)
	}

	if choice < 1 || choice > len(sessions) {
		return fmt.Errorf("invalid selection: %d", choice)
	}

	selected := &sessions[choice-1]

	// Update last used
	selected.LastUsed = time.Now()
	_ = store.Save(selected)

	// Setup TUI logging
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	// Resume existing Claude sessions (use --resume)
	return tui.RunAppWithChat(selected, true)
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

	// Convert to standard JSON envelope
	out := getOutput()
	return out.JSON(NewJSONOutput(true, convertMultiResponse(response), nil))
}

// convertMultiResponse converts tui.MultiResponse to MultiMessageOutput for JSON envelope
func convertMultiResponse(resp *tui.MultiResponse) MultiMessageOutput {
	responses := make([]MultiAgentResponse, len(resp.Responses))
	for i, r := range resp.Responses {
		toolUses := make([]MultiToolUse, len(r.ToolUses))
		for j, tu := range r.ToolUses {
			toolUses[j] = MultiToolUse{
				Tool:  tu.Tool,
				Input: tu.Input,
			}
		}
		responses[i] = MultiAgentResponse{
			Agent:     r.Agent,
			Timestamp: r.Timestamp.Format(time.RFC3339),
			Content:   r.Content,
			ToolUses:  toolUses,
			Error:     r.Error,
		}
	}
	return MultiMessageOutput{
		Session: resp.Session,
		Request: MultiRequestInfo{
			Target:  resp.Request.Target,
			Message: resp.Request.Message,
		},
		Responses: responses,
	}
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

		// Use standard JSON envelope
		out := getOutput()
		_ = out.JSON(NewJSONOutput(true, convertMultiResponse(response), nil))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stdin: %w", err)
	}

	return nil
}

// outputJSONError outputs an error in JSON format using standard envelope
func outputJSONError(err error) {
	out := getOutput()
	exitCode, code := vserrors.ErrorCode(err)
	hint := getErrorHint(err)
	_ = out.JSON(NewJSONOutput(false, nil, &JSONError{
		Message:  err.Error(),
		Code:     code,
		ExitCode: exitCode,
		Hint:     hint,
	}))
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
		out := getOutput()
		return out.JSON(NewJSONOutput(true, MultiListAgentsOutput{
			Session: sessionName,
			Agents:  agents,
			Count:   len(agents),
		}, nil))
	}

	out := getOutput()
	if out.IsPlainMode() {
		if out.Header() {
			fmt.Println("SESSION\tAGENT")
		}
		for _, agent := range agents {
			fmt.Printf("%s\t%s\n", sessionName, agent)
		}
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

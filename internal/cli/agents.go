package cli

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/agent"
	vspkg "github.com/yagizdagabak/vibespace/pkg/vibespace"
)

// runAgent routes to agent subcommands
// Usage: vibespace <name> agent {list,create,delete}
func runAgent(vibespace string, args []string) error {
	if len(args) == 0 {
		// Default to list
		return runAgentList(vibespace, args)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "list":
		return runAgentList(vibespace, subArgs)
	case "create":
		return runAgentCreate(vibespace, subArgs)
	case "delete":
		return runAgentDelete(vibespace, subArgs)
	case "--help", "-h":
		fmt.Printf(`Manage agents in a vibespace

Usage:
  vibespace %s agent [command]

Available Commands:
  list        List all agents (default)
  create      Create a new agent
  delete      Delete an agent

Examples:
  vibespace %s agent
  vibespace %s agent list
  vibespace %s agent create
  vibespace %s agent create --agent-type codex
  vibespace %s agent delete claude-2
`, vibespace, vibespace, vibespace, vibespace, vibespace, vibespace)
		return nil
	default:
		return fmt.Errorf("unknown agent subcommand: %s", subCmd)
	}
}

func runAgentList(vibespace string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Debug("agent list command started", "vibespace", vibespace)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// List agents using the service method
	agents, err := svc.ListAgents(ctx, vibespace)
	if err != nil {
		slog.Error("failed to list agents", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to list agents: %w", err)
	}

	// Sort agents by agent number
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].AgentNum < agents[j].AgentNum
	})

	// JSON output mode
	if out.IsJSONMode() {
		items := make([]AgentListItem, len(agents))
		for i, agent := range agents {
			items[i] = AgentListItem{
				Name:      agent.AgentName,
				Type:      agent.AgentType.String(),
				Vibespace: vibespace,
				Status:    agent.Status,
			}
		}
		return out.JSON(NewJSONOutput(true, AgentsOutput{
			Vibespace: vibespace,
			Agents:    items,
			Count:     len(items),
		}, nil))
	}

	if len(agents) == 0 {
		// Plain mode - no output for empty result
		if out.IsPlainMode() {
			return nil
		}
		fmt.Printf("No agents in vibespace '%s'\n", vibespace)
		fmt.Println()
		fmt.Printf("Create one with: vibespace %s agent create\n", vibespace)
		return nil
	}

	// Build table rows
	headers := []string{"AGENT", "TYPE", "VIBESPACE", "STATUS"}
	rows := make([][]string, len(agents))
	for i, a := range agents {
		status := a.Status
		if !out.NoColor() {
			switch a.Status {
			case "running":
				status = out.Green(a.Status)
			case "stopped":
				status = out.Yellow(a.Status)
			case "creating":
				status = out.Yellow(a.Status)
			}
		}
		rows[i] = []string{a.AgentName, a.AgentType.String(), vibespace, status}
	}

	out.Table(headers, rows)

	slog.Debug("agent list command completed", "vibespace", vibespace, "count", len(agents))
	return nil
}

func runAgentCreate(vibespace string, args []string) error {
	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`Create a new agent in a vibespace

Usage:
  vibespace %s agent create [flags]

Flags:
  -n, --name string            Custom name for the agent (default: <type>-N)
  -t, --agent-type string      Agent type: claude-code, codex (default: inherit from primary)
  -s, --share-credentials      Share credentials across all agents
      --skip-permissions       Enable --dangerously-skip-permissions
      --allowed-tools string   Comma-separated allowed tools (replaces default)
      --disallowed-tools string Comma-separated disallowed tools
      --model string           Model to use (e.g., opus, sonnet)
      --max-turns int          Maximum conversation turns
  -h, --help                   Help for create

Examples:
  vibespace %s agent create                        # Inherits type from primary agent
  vibespace %s agent create --agent-type codex     # Explicit codex type
  vibespace %s agent create --name researcher
  vibespace %s agent create --share-credentials
  vibespace %s agent create --skip-permissions
  vibespace %s agent create --allowed-tools "Bash,Read,Write"
`, vibespace, vibespace, vibespace, vibespace, vibespace, vibespace, vibespace)
			return nil
		}
	}

	ctx := context.Background()
	out := getOutput()

	// Parse flags
	shareCredentials := false
	customName := ""
	agentTypeStr := "" // Empty means inherit from primary agent
	skipPermissions := false
	allowedTools := ""
	disallowedTools := ""
	modelName := ""
	maxTurns := 0

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--share-credentials" || arg == "-s":
			shareCredentials = true
		case arg == "--skip-permissions":
			skipPermissions = true
		case (arg == "--name" || arg == "-n") && i+1 < len(args):
			customName = args[i+1]
			i++
		case strings.HasPrefix(arg, "--name="):
			customName = arg[7:]
		case strings.HasPrefix(arg, "-n="):
			customName = arg[3:]
		case (arg == "--agent-type" || arg == "-t") && i+1 < len(args):
			agentTypeStr = args[i+1]
			i++
		case strings.HasPrefix(arg, "--agent-type="):
			agentTypeStr = arg[13:]
		case strings.HasPrefix(arg, "-t="):
			agentTypeStr = arg[3:]
		case arg == "--allowed-tools" && i+1 < len(args):
			allowedTools = args[i+1]
			i++
		case strings.HasPrefix(arg, "--allowed-tools="):
			allowedTools = arg[16:]
		case arg == "--disallowed-tools" && i+1 < len(args):
			disallowedTools = args[i+1]
			i++
		case strings.HasPrefix(arg, "--disallowed-tools="):
			disallowedTools = arg[19:]
		case arg == "--model" && i+1 < len(args):
			modelName = args[i+1]
			i++
		case strings.HasPrefix(arg, "--model="):
			modelName = arg[8:]
		case arg == "--max-turns" && i+1 < len(args):
			maxTurns, _ = strconv.Atoi(args[i+1])
			i++
		case strings.HasPrefix(arg, "--max-turns="):
			maxTurns, _ = strconv.Atoi(arg[12:])
		}
	}

	// Parse and validate agent type (only if explicitly specified)
	var agentType agent.Type
	if agentTypeStr != "" {
		agentType = agent.ParseType(agentTypeStr)
		if !agentType.IsValid() {
			return fmt.Errorf("invalid agent type '%s': valid types are claude-code, codex", agentTypeStr)
		}
	}

	slog.Info("agent create command started", "vibespace", vibespace, "name", customName, "agent_type", agentType, "share_credentials", shareCredentials)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Verify vibespace exists and is running
	vs, err := checkVibespaceRunning(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not running", "vibespace", vibespace, "error", err)
		return err
	}

	if customName != "" {
		printStep("Creating agent '%s' in '%s'...", customName, vibespace)
	} else if agentType != "" {
		printStep("Creating new %s agent in '%s'...", agentType, vibespace)
	} else {
		printStep("Creating new agent in '%s'...", vibespace)
	}

	// Build AgentConfig if any config flags are set
	var agentConfig *agent.Config
	if skipPermissions || allowedTools != "" || disallowedTools != "" || modelName != "" || maxTurns > 0 {
		agentConfig = &agent.Config{
			SkipPermissions: skipPermissions,
			Model:           modelName,
			MaxTurns:        maxTurns,
		}
		if allowedTools != "" {
			agentConfig.AllowedTools = strings.Split(allowedTools, ",")
		}
		if disallowedTools != "" {
			agentConfig.DisallowedTools = strings.Split(disallowedTools, ",")
		}
	}

	// Spawn the agent with options
	opts := &vspkg.SpawnAgentOptions{
		Name:             customName,
		AgentType:        agentType,
		ShareCredentials: shareCredentials,
		Config:           agentConfig,
	}
	agentName, err := svc.SpawnAgent(ctx, vs.ID, opts)
	if err != nil {
		slog.Error("failed to create agent", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// JSON output
	if out.IsJSONMode() {
		agentTypeOutput := agentType.String()
		if agentTypeOutput == "" {
			agentTypeOutput = "inherited"
		}
		return out.JSON(NewJSONOutput(true, AgentCreateOutput{
			Vibespace: vibespace,
			Agent:     agentName,
			Type:      agentTypeOutput,
		}, nil))
	}

	slog.Info("agent create command completed", "vibespace", vibespace, "agent", agentName)
	printSuccess("Agent '%s' scheduled (starting...)", agentName)
	if shareCredentials {
		fmt.Println("  Credential sharing enabled via /vibespace/.vibespace")
	}
	if agentConfig != nil {
		if agentConfig.SkipPermissions {
			fmt.Println("  Skip permissions enabled")
		}
		if len(agentConfig.AllowedTools) > 0 {
			fmt.Printf("  Allowed tools: %s\n", strings.Join(agentConfig.AllowedTools, ","))
		}
		if agentConfig.Model != "" {
			fmt.Printf("  Model: %s\n", agentConfig.Model)
		}
	}
	fmt.Println()
	fmt.Printf("Connect with: vibespace %s connect %s\n", vibespace, agentName)
	return nil
}

func runAgentDelete(vibespace string, args []string) error {
	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`Delete an agent from a vibespace

Usage:
  vibespace %s agent delete <agent>

Arguments:
  agent    Name of the agent to delete

Examples:
  vibespace %s agent delete claude-2
  vibespace %s agent delete codex-1
`, vibespace, vibespace, vibespace)
			return nil
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("agent name required. Usage: vibespace %s agent delete <agent>", vibespace)
	}

	agentID := args[0]
	ctx := context.Background()
	out := getOutput()

	slog.Info("agent delete command started", "vibespace", vibespace, "agent", agentID)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Verify vibespace exists
	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	printStep("Deleting agent '%s' from '%s'...", agentID, vibespace)

	// Kill the agent
	if err := svc.KillAgent(ctx, vs.ID, agentID); err != nil {
		slog.Error("failed to delete agent", "vibespace", vibespace, "agent", agentID, "error", err)
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, AgentDeleteOutput{
			Vibespace: vibespace,
			Agent:     agentID,
		}, nil))
	}

	slog.Info("agent delete command completed", "vibespace", vibespace, "agent", agentID)
	printSuccess("Agent '%s' deleted", agentID)

	return nil
}

// runStart scales up agents in a vibespace
// Usage:
//   vibespace foo start           # scale all agents to 1
//   vibespace foo start claude-2  # scale specific agent to 1
func runStart(vibespace string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`Start agents in a vibespace

Usage:
  vibespace %s start [agent] [flags]

Arguments:
  agent    Optional agent name to start (default: all agents)

Flags:
  -h, --help   Help for start

Examples:
  vibespace %s start           # Start all agents
  vibespace %s start claude-2  # Start specific agent
`, vibespace, vibespace, vibespace)
			return nil
		}
	}

	slog.Info("start command started", "vibespace", vibespace, "args", args)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Check vibespace exists
	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	var agentName string
	if len(args) > 0 {
		// Start specific agent
		agentName = args[0]
		printStep("Starting agent '%s' in '%s'...", agentName, vibespace)

		if err := svc.StartAgent(ctx, vs.ID, agentName); err != nil {
			slog.Error("failed to start agent", "vibespace", vibespace, "agent", agentName, "error", err)
			return fmt.Errorf("failed to start agent: %w", err)
		}

		// JSON output
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StartOutput{
				Vibespace: vibespace,
				Agent:     agentName,
			}, nil))
		}

		slog.Info("start command completed", "vibespace", vibespace, "agent", agentName)
		printSuccess("Agent '%s' started", agentName)
	} else {
		// Start all agents
		printStep("Starting all agents in '%s'...", vibespace)

		if err := svc.Start(ctx, vs.ID); err != nil {
			slog.Error("failed to start vibespace", "vibespace", vibespace, "error", err)
			return fmt.Errorf("failed to start vibespace: %w", err)
		}

		// JSON output
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StartOutput{
				Vibespace: vibespace,
			}, nil))
		}

		slog.Info("start command completed", "vibespace", vibespace)
		printSuccess("All agents started in '%s'", vibespace)
	}

	return nil
}

// runStop scales down agents in a vibespace
// Usage:
//   vibespace foo stop           # scale all agents to 0
//   vibespace foo stop claude-2  # scale specific agent to 0
func runStop(vibespace string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`Stop agents in a vibespace

Usage:
  vibespace %s stop [agent] [flags]

Arguments:
  agent    Optional agent name to stop (default: all agents)

Flags:
  -h, --help   Help for stop

Examples:
  vibespace %s stop           # Stop all agents
  vibespace %s stop claude-2  # Stop specific agent
`, vibespace, vibespace, vibespace)
			return nil
		}
	}

	slog.Info("stop command started", "vibespace", vibespace, "args", args)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Check vibespace exists
	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	var agentName string
	if len(args) > 0 {
		// Stop specific agent
		agentName = args[0]
		printStep("Stopping agent '%s' in '%s'...", agentName, vibespace)

		if err := svc.StopAgent(ctx, vs.ID, agentName); err != nil {
			slog.Error("failed to stop agent", "vibespace", vibespace, "agent", agentName, "error", err)
			return fmt.Errorf("failed to stop agent: %w", err)
		}

		// JSON output
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StopOutput{
				Stopped: true,
				Target:  agentName,
			}, nil))
		}

		slog.Info("stop command completed", "vibespace", vibespace, "agent", agentName)
		printSuccess("Agent '%s' stopped", agentName)
	} else {
		// Stop all agents
		printStep("Stopping all agents in '%s'...", vibespace)

		if err := svc.Stop(ctx, vs.ID); err != nil {
			slog.Error("failed to stop vibespace", "vibespace", vibespace, "error", err)
			return fmt.Errorf("failed to stop vibespace: %w", err)
		}

		// JSON output
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StopOutput{
				Stopped: true,
				Target:  vibespace,
			}, nil))
		}

		slog.Info("stop command completed", "vibespace", vibespace)
		printSuccess("All agents stopped in '%s'", vibespace)
	}

	return nil
}

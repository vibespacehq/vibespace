package cli

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/model"
	vspkg "github.com/yagizdagabak/vibespace/pkg/vibespace"
)

func runAgents(vibespace string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Debug("agents command started", "vibespace", vibespace)

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

	// Sort agents by claude ID
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].ClaudeID < agents[j].ClaudeID
	})

	// JSON output mode
	if out.IsJSONMode() {
		items := make([]AgentListItem, len(agents))
		for i, agent := range agents {
			items[i] = AgentListItem{
				Name:      agent.AgentName,
				Vibespace: vibespace,
				Status:    agent.Status,
			}
		}
		return out.JSON(JSONOutput{
			Success: true,
			Data: AgentsOutput{
				Vibespace: vibespace,
				Agents:    items,
				Count:     len(items),
			},
		})
	}

	if len(agents) == 0 {
		fmt.Printf("No agents in vibespace '%s'\n", vibespace)
		fmt.Println()
		fmt.Printf("Spawn one with: vibespace %s spawn\n", vibespace)
		return nil
	}

	// Plain output mode
	if out.IsPlainMode() {
		for _, agent := range agents {
			fmt.Printf("%s\t%s\t%s\n", agent.AgentName, vibespace, agent.Status)
		}
		return nil
	}

	// Print as table with fixed-width columns
	fmt.Printf("%-12s %-20s %-10s\n", "AGENT", "VIBESPACE", "STATUS")

	for _, agent := range agents {
		// Colorize status after formatting to maintain alignment
		status := fmt.Sprintf("%-10s", agent.Status)
		switch agent.Status {
		case "running":
			status = green(agent.Status) + "   " // "running" is 7 chars, pad to 10
		case "stopped":
			status = yellow(agent.Status) + "   " // "stopped" is 7 chars, pad to 10
		case "creating":
			status = yellow(agent.Status) + "  " // "creating" is 8 chars, pad to 10
		}
		fmt.Printf("%-12s %-20s %s\n", agent.AgentName, vibespace, status)
	}

	slog.Debug("agents command completed", "vibespace", vibespace, "count", len(agents))
	return nil
}

func runSpawn(vibespace string, args []string) error {
	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println(`Spawn a new Claude agent in a vibespace

Usage:
  vibespace <name> spawn [flags]

Flags:
  -n, --name string            Custom name for the agent (default: claude-N)
  -s, --share-credentials      Share Claude credentials across all agents
      --skip-permissions       Enable --dangerously-skip-permissions for Claude
      --allowed-tools string   Comma-separated allowed tools (replaces default)
      --disallowed-tools string Comma-separated disallowed tools
      --model string           Claude model to use (e.g., opus, sonnet)
      --max-turns int          Maximum conversation turns
  -h, --help                   Help for spawn

Examples:
  vibespace myproject spawn
  vibespace myproject spawn --name researcher
  vibespace myproject spawn --share-credentials
  vibespace myproject spawn --skip-permissions
  vibespace myproject spawn --allowed-tools "Bash,Read,Write"`)
			return nil
		}
	}

	ctx := context.Background()

	// Parse flags
	shareCredentials := false
	customName := ""
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

	slog.Info("spawn command started", "vibespace", vibespace, "name", customName, "share_credentials", shareCredentials)

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
		printStep("Spawning agent '%s' in '%s'...", customName, vibespace)
	} else {
		printStep("Spawning new agent in '%s'...", vibespace)
	}

	// Build ClaudeConfig if any config flags are set
	var claudeConfig *model.ClaudeConfig
	if skipPermissions || allowedTools != "" || disallowedTools != "" || modelName != "" || maxTurns > 0 {
		claudeConfig = &model.ClaudeConfig{
			SkipPermissions: skipPermissions,
			Model:           modelName,
			MaxTurns:        maxTurns,
		}
		if allowedTools != "" {
			claudeConfig.AllowedTools = strings.Split(allowedTools, ",")
		}
		if disallowedTools != "" {
			claudeConfig.DisallowedTools = strings.Split(disallowedTools, ",")
		}
	}

	// Spawn the agent with options
	opts := &vspkg.SpawnAgentOptions{
		Name:             customName,
		ShareCredentials: shareCredentials,
		ClaudeConfig:     claudeConfig,
	}
	agentName, err := svc.SpawnAgent(ctx, vs.ID, opts)
	if err != nil {
		slog.Error("failed to spawn agent", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to spawn agent: %w", err)
	}

	slog.Info("spawn command completed", "vibespace", vibespace, "agent", agentName)
	printSuccess("Agent '%s' created", agentName)
	if shareCredentials {
		fmt.Println("  Credential sharing enabled via /vibespace/.vibespace")
	}
	if claudeConfig != nil {
		if claudeConfig.SkipPermissions {
			fmt.Println("  Skip permissions enabled")
		}
		if len(claudeConfig.AllowedTools) > 0 {
			fmt.Printf("  Allowed tools: %s\n", claudeConfig.AllowedToolsString())
		}
		if claudeConfig.Model != "" {
			fmt.Printf("  Model: %s\n", claudeConfig.Model)
		}
	}
	fmt.Println()
	fmt.Printf("Connect with: vibespace %s connect %s\n", vibespace, agentName)
	return nil
}

func runKill(vibespace string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID required. Usage: vibespace %s kill <agent>", vibespace)
	}

	agentID := args[0]
	ctx := context.Background()

	slog.Info("kill command started", "vibespace", vibespace, "agent", agentID)

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

	printStep("Killing agent '%s' in '%s'...", agentID, vibespace)

	// Kill the agent
	if err := svc.KillAgent(ctx, vs.ID, agentID); err != nil {
		slog.Error("failed to kill agent", "vibespace", vibespace, "agent", agentID, "error", err)
		return fmt.Errorf("failed to kill agent: %w", err)
	}

	slog.Info("kill command completed", "vibespace", vibespace, "agent", agentID)
	printSuccess("Agent '%s' removed", agentID)

	return nil
}

// runUp scales up agents in a vibespace
// Usage:
//   vibespace foo up           # scale all agents to 1
//   vibespace foo up claude-2  # scale specific agent to 1
func runUp(vibespace string, args []string) error {
	ctx := context.Background()

	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println(`Scale up agents in a vibespace

Usage:
  vibespace <name> up [agent] [flags]

Arguments:
  agent    Optional agent name to scale up (default: all agents)

Flags:
  -h, --help   Help for up

Examples:
  vibespace myproject up           # Scale all agents to 1
  vibespace myproject up claude-2  # Scale specific agent to 1`)
			return nil
		}
	}

	slog.Info("up command started", "vibespace", vibespace, "args", args)

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

	if len(args) > 0 {
		// Scale up specific agent
		agentName := args[0]
		printStep("Scaling up agent '%s' in '%s'...", agentName, vibespace)

		if err := svc.StartAgent(ctx, vs.ID, agentName); err != nil {
			slog.Error("failed to scale up agent", "vibespace", vibespace, "agent", agentName, "error", err)
			return fmt.Errorf("failed to scale up agent: %w", err)
		}

		slog.Info("up command completed", "vibespace", vibespace, "agent", agentName)
		printSuccess("Agent '%s' scaled up", agentName)
	} else {
		// Scale up all agents (start the vibespace)
		printStep("Scaling up all agents in '%s'...", vibespace)

		if err := svc.Start(ctx, vs.ID); err != nil {
			slog.Error("failed to scale up vibespace", "vibespace", vibespace, "error", err)
			return fmt.Errorf("failed to scale up vibespace: %w", err)
		}

		slog.Info("up command completed", "vibespace", vibespace)
		printSuccess("All agents scaled up in '%s'", vibespace)
	}

	return nil
}

// runDown scales down agents in a vibespace
// Usage:
//   vibespace foo down           # scale all agents to 0
//   vibespace foo down claude-2  # scale specific agent to 0
func runDown(vibespace string, args []string) error {
	ctx := context.Background()

	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println(`Scale down agents in a vibespace

Usage:
  vibespace <name> down [agent] [flags]

Arguments:
  agent    Optional agent name to scale down (default: all agents)

Flags:
  -h, --help   Help for down

Examples:
  vibespace myproject down           # Scale all agents to 0
  vibespace myproject down claude-2  # Scale specific agent to 0`)
			return nil
		}
	}

	slog.Info("down command started", "vibespace", vibespace, "args", args)

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

	if len(args) > 0 {
		// Scale down specific agent
		agentName := args[0]
		printStep("Scaling down agent '%s' in '%s'...", agentName, vibespace)

		if err := svc.StopAgent(ctx, vs.ID, agentName); err != nil {
			slog.Error("failed to scale down agent", "vibespace", vibespace, "agent", agentName, "error", err)
			return fmt.Errorf("failed to scale down agent: %w", err)
		}

		slog.Info("down command completed", "vibespace", vibespace, "agent", agentName)
		printSuccess("Agent '%s' scaled down", agentName)
	} else {
		// Scale down all agents (stop the vibespace)
		printStep("Scaling down all agents in '%s'...", vibespace)

		if err := svc.Stop(ctx, vs.ID); err != nil {
			slog.Error("failed to scale down vibespace", "vibespace", vibespace, "error", err)
			return fmt.Errorf("failed to scale down vibespace: %w", err)
		}

		slog.Info("down command completed", "vibespace", vibespace)
		printSuccess("All agents scaled down in '%s'", vibespace)
	}

	return nil
}

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"vibespace/pkg/daemon"
	vspkg "vibespace/pkg/vibespace"
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
  -n, --name string          Custom name for the agent (default: claude-N)
  -s, --share-credentials    Share Claude credentials across all agents
  -h, --help                 Help for spawn

Examples:
  vibespace myproject spawn
  vibespace myproject spawn --name researcher
  vibespace myproject spawn --share-credentials`)
			return nil
		}
	}

	ctx := context.Background()

	// Parse flags
	shareCredentials := false
	customName := ""
	for i, arg := range args {
		if arg == "--share-credentials" || arg == "-s" {
			shareCredentials = true
		}
		if (arg == "--name" || arg == "-n") && i+1 < len(args) {
			customName = args[i+1]
		}
		// Handle --name=value format
		if len(arg) > 7 && arg[:7] == "--name=" {
			customName = arg[7:]
		}
		if len(arg) > 3 && arg[:3] == "-n=" {
			customName = arg[3:]
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

	// Spawn the agent with options
	opts := &vspkg.SpawnAgentOptions{
		Name:             customName,
		ShareCredentials: shareCredentials,
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
	fmt.Println()

	// If daemon is running, suggest restarting it to discover the new agent
	if daemon.IsRunning(vibespace) {
		printWarning("Daemon is running. Restart it to discover the new agent:")
		fmt.Printf("  vibespace %s down && vibespace %s up\n", vibespace, vibespace)
		fmt.Println()
	}

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

	// If daemon is running, suggest restarting it
	if daemon.IsRunning(vibespace) {
		fmt.Println()
		printWarning("Daemon is running. Restart it to update agent list:")
		fmt.Printf("  vibespace %s down && vibespace %s up\n", vibespace, vibespace)
	}

	return nil
}

func runStartVibespace(vibespace string) error {
	ctx := context.Background()

	slog.Info("start command started", "vibespace", vibespace)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	printStep("Starting vibespace '%s'...", vibespace)

	if err := svc.Start(ctx, vibespace); err != nil {
		slog.Error("failed to start vibespace", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to start vibespace: %w", err)
	}

	slog.Info("start command completed", "vibespace", vibespace)
	printSuccess("Vibespace '%s' started", vibespace)
	return nil
}

func runStopVibespace(vibespace string) error {
	ctx := context.Background()

	slog.Info("stop command started", "vibespace", vibespace)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	printStep("Stopping vibespace '%s'...", vibespace)

	if err := svc.Stop(ctx, vibespace); err != nil {
		slog.Error("failed to stop vibespace", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to stop vibespace: %w", err)
	}

	slog.Info("stop command completed", "vibespace", vibespace)
	printSuccess("Vibespace '%s' stopped", vibespace)
	return nil
}

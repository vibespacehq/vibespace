package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"vibespace/pkg/daemon"
)

func runAgents(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	// List agents using the service method
	agents, err := svc.ListAgents(ctx, vibespace)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		fmt.Printf("No agents in vibespace '%s'\n", vibespace)
		fmt.Println()
		fmt.Printf("Spawn one with: vibespace %s spawn\n", vibespace)
		return nil
	}

	// Sort agents by claude ID
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].ClaudeID < agents[j].ClaudeID
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tSTATUS")

	for _, agent := range agents {
		status := agent.Status
		switch status {
		case "running":
			status = green(status)
		case "stopped":
			status = yellow(status)
		case "creating":
			status = yellow(status)
		}
		fmt.Fprintf(w, "%s\t%s\n", agent.AgentName, status)
	}

	w.Flush()
	return nil
}

func runSpawn(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	// Verify vibespace exists and is running
	vs, err := checkVibespaceRunning(ctx, svc, vibespace)
	if err != nil {
		return err
	}

	printStep("Spawning new agent in '%s'...", vibespace)

	// Spawn the agent
	agentName, err := svc.SpawnAgent(ctx, vs.ID)
	if err != nil {
		return fmt.Errorf("failed to spawn agent: %w", err)
	}

	printSuccess("Agent '%s' created", agentName)
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

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	// Verify vibespace exists
	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		return err
	}

	printStep("Killing agent '%s' in '%s'...", agentID, vibespace)

	// Kill the agent
	if err := svc.KillAgent(ctx, vs.ID, agentID); err != nil {
		return fmt.Errorf("failed to kill agent: %w", err)
	}

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

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	printStep("Starting vibespace '%s'...", vibespace)

	if err := svc.Start(ctx, vibespace); err != nil {
		return fmt.Errorf("failed to start vibespace: %w", err)
	}

	printSuccess("Vibespace '%s' started", vibespace)
	return nil
}

func runStopVibespace(vibespace string) error {
	ctx := context.Background()

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	printStep("Stopping vibespace '%s'...", vibespace)

	if err := svc.Stop(ctx, vibespace); err != nil {
		return fmt.Errorf("failed to stop vibespace: %w", err)
	}

	printSuccess("Vibespace '%s' stopped", vibespace)
	return nil
}

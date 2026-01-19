package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
)

func runAgents(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Verify vibespace exists
	vs, err := svc.Get(ctx, vibespace)
	if err != nil {
		return fmt.Errorf("vibespace '%s' not found", vibespace)
	}

	// For now, each vibespace has one Claude instance
	// In the future, this will query multiple Knative services
	agents := []struct {
		ID     string
		Status string
	}{
		{ID: "claude-1", Status: vs.Status},
	}

	if len(agents) == 0 {
		fmt.Printf("No agents in vibespace '%s'\n", vibespace)
		fmt.Println()
		fmt.Printf("Spawn one with: vibespace %s spawn\n", vibespace)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tSTATUS")

	for _, agent := range agents {
		status := agent.Status
		switch status {
		case "running":
			status = green(status)
		case "stopped":
			status = yellow(status)
		}
		fmt.Fprintf(w, "%s\t%s\n", agent.ID, status)
	}

	w.Flush()
	return nil
}

func runSpawn(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Verify vibespace exists
	_, err = svc.Get(ctx, vibespace)
	if err != nil {
		return fmt.Errorf("vibespace '%s' not found", vibespace)
	}

	// TODO: Implement multi-Claude spawning
	// For now, just show a message
	printWarning("Multi-Claude spawning not yet implemented")
	fmt.Println("Each vibespace currently has one Claude instance")
	fmt.Println()
	fmt.Printf("Connect with: vibespace %s connect claude-1\n", vibespace)

	return nil
}

func runKill(vibespace string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID required")
	}

	agentID := args[0]

	// TODO: Implement agent removal
	printWarning("Agent removal not yet implemented")
	fmt.Printf("Would remove agent '%s' from vibespace '%s'\n", agentID, vibespace)

	return nil
}

func runStartVibespace(vibespace string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
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

	svc, err := getVibespaceService()
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

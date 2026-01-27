package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all vibespaces",
	Long:  `Display a list of all vibespaces and their current status.`,
	Example: `  vibespace list
  vibespace list --json
  vibespace list --plain
  vibespace list --plain --header`,
	RunE: runList,
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	out := getOutput()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	vibespaces, err := svc.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list vibespaces: %w", err)
	}

	// Helper to count agents for a vibespace
	countAgents := func(vsID string) int {
		agents, err := svc.ListAgents(ctx, vsID)
		if err != nil {
			return 1 // Default to 1 on error
		}
		if len(agents) == 0 {
			return 1 // At least 1 agent
		}
		return len(agents)
	}

	// JSON output mode
	if out.IsJSONMode() {
		items := make([]VibespaceListItem, len(vibespaces))
		for i, vs := range vibespaces {
			items[i] = VibespaceListItem{
				Name:      vs.Name,
				Status:    vs.Status,
				Agents:    countAgents(vs.ID),
				CPU:       vs.Resources.CPU,
				Memory:    vs.Resources.Memory,
				Storage:   vs.Resources.Storage,
				CreatedAt: vs.CreatedAt,
			}
		}
		return out.JSON(NewJSONOutput(true, ListOutput{
			Vibespaces: items,
			Count:      len(items),
		}, nil))
	}

	if len(vibespaces) == 0 {
		// Plain mode - no output for empty result
		if out.IsPlainMode() {
			return nil
		}
		fmt.Println("No vibespaces found")
		fmt.Println()
		fmt.Println("Create one with: vibespace create <name>")
		return nil
	}

	// Build table rows
	headers := []string{"NAME", "STATUS", "AGENTS", "CPU", "MEMORY", "STORAGE", "CREATED"}
	rows := make([][]string, len(vibespaces))
	for i, vs := range vibespaces {
		status := vs.Status
		if !out.NoColor() {
			switch vs.Status {
			case "running":
				status = out.Green(vs.Status)
			case "stopped":
				status = out.Yellow(vs.Status)
			case "error":
				status = out.Red(vs.Status)
			}
		}
		rows[i] = []string{
			vs.Name,
			status,
			fmt.Sprintf("%d", countAgents(vs.ID)),
			vs.Resources.CPU,
			vs.Resources.Memory,
			vs.Resources.Storage,
			vs.CreatedAt,
		}
	}

	out.Table(headers, rows)
	return nil
}

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
  vibespace list --plain`,
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

	// JSON output mode
	if out.IsJSONMode() {
		items := make([]VibespaceListItem, len(vibespaces))
		for i, vs := range vibespaces {
			items[i] = VibespaceListItem{
				Name:      vs.Name,
				Status:    vs.Status,
				Agents:    1, // Default to 1 agent per vibespace for now
				CPU:       vs.Resources.CPU,
				Memory:    vs.Resources.Memory,
				Storage:   vs.Resources.Storage,
				CreatedAt: vs.CreatedAt,
			}
		}
		return out.JSON(JSONOutput{
			Success: true,
			Data: ListOutput{
				Vibespaces: items,
				Count:      len(items),
			},
		})
	}

	if len(vibespaces) == 0 {
		fmt.Println("No vibespaces found")
		fmt.Println()
		fmt.Println("Create one with: vibespace create <name>")
		return nil
	}

	// Plain output mode (tab-separated, no colors)
	if out.IsPlainMode() {
		for _, vs := range vibespaces {
			fmt.Printf("%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
				vs.Name, vs.Status, 1,
				vs.Resources.CPU, vs.Resources.Memory, vs.Resources.Storage,
				vs.CreatedAt)
		}
		return nil
	}

	// Print as table with fixed-width columns (tabwriter doesn't work well with ANSI colors)
	fmt.Printf("%-16s %-10s %-8s %-8s %-8s %-8s %s\n", "NAME", "STATUS", "AGENTS", "CPU", "MEMORY", "STORAGE", "CREATED")

	for _, vs := range vibespaces {
		// Colorize status after padding to maintain alignment
		status := fmt.Sprintf("%-10s", vs.Status)
		switch vs.Status {
		case "running":
			status = green(vs.Status) + "   " // "running" is 7 chars, pad to 10
		case "stopped":
			status = yellow(vs.Status) + "   " // "stopped" is 7 chars, pad to 10
		case "error":
			status = red(vs.Status) + "     " // "error" is 5 chars, pad to 10
		}

		agents := "1" // Default to 1 agent per vibespace for now

		fmt.Printf("%-16s %s %-8s %-8s %-8s %-8s %s\n",
			vs.Name, status, agents,
			vs.Resources.CPU, vs.Resources.Memory, vs.Resources.Storage,
			vs.CreatedAt)
	}

	return nil
}

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

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
			fmt.Printf("%s\t%s\t%d\t%s\n", vs.Name, vs.Status, 1, vs.CreatedAt)
		}
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tAGENTS\tCREATED")

	for _, vs := range vibespaces {
		status := vs.Status
		switch status {
		case "running":
			status = green(status)
		case "stopped":
			status = yellow(status)
		case "error":
			status = red(status)
		}

		agents := "1" // Default to 1 agent per vibespace for now
		created := vs.CreatedAt // Already a formatted string

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", vs.Name, status, agents, created)
	}

	w.Flush()
	return nil
}

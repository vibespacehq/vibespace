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
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	vibespaces, err := svc.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list vibespaces: %w", err)
	}

	if len(vibespaces) == 0 {
		fmt.Println("No vibespaces found")
		fmt.Println()
		fmt.Println("Create one with: vibespace create <name>")
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
			status = fmt.Sprintf("\033[31m%s\033[0m", status) // red
		}

		agents := "1" // Default to 1 agent per vibespace for now
		created := vs.CreatedAt // Already a formatted string

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", vs.Name, status, agents, created)
	}

	w.Flush()
	return nil
}

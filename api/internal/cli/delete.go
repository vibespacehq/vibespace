package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a vibespace",
	Long: `Delete a vibespace and all its resources.

This will remove:
  - All Claude instances
  - Persistent storage
  - Network routes

This action cannot be undone.`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

var (
	deleteForce bool
)

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
}

func runDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	name := args[0]

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Check if vibespace exists
	vs, err := svc.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("vibespace '%s' not found", name)
	}

	// Confirm deletion unless --force
	if !deleteForce {
		fmt.Printf("Delete vibespace '%s'? This cannot be undone. [y/N] ", vs.Name)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	printStep("Deleting vibespace '%s'...", name)

	if err := svc.Delete(ctx, name); err != nil {
		return fmt.Errorf("failed to delete vibespace: %w", err)
	}

	printSuccess("Vibespace '%s' deleted", name)
	return nil
}

package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"vibespace/pkg/vibespace"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a vibespace",
	Long: `Delete a vibespace and all its resources.

This will remove:
  - All Claude instances
  - Persistent storage (PVC)
  - SSH key secrets

Use --keep-data to preserve the persistent volume claim (PVC) for data recovery.

This action cannot be undone.`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

var (
	deleteForce    bool
	deleteKeepData bool
)

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteKeepData, "keep-data", false, "Preserve persistent storage (PVC) for data recovery")
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
		msg := fmt.Sprintf("Delete vibespace '%s'?", vs.Name)
		if deleteKeepData {
			msg += " (data will be preserved)"
		} else {
			msg += " All data will be deleted."
		}
		msg += " [y/N] "
		fmt.Print(msg)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	printStep("Deleting vibespace '%s'...", name)

	opts := &vibespace.DeleteOptions{
		KeepData: deleteKeepData,
	}

	if err := svc.Delete(ctx, name, opts); err != nil {
		return fmt.Errorf("failed to delete vibespace: %w", err)
	}

	printSuccess("Vibespace '%s' deleted", name)
	if deleteKeepData {
		printStep("Storage data preserved. Clean up manually if needed.")
	}
	return nil
}

package cli

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
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

	slog.Info("delete command started", "name", name, "force", deleteForce, "keep_data", deleteKeepData)

	svc, err := getVibespaceService()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Check if vibespace exists
	vs, err := svc.Get(ctx, name)
	if err != nil {
		slog.Error("vibespace not found", "name", name, "error", err)
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
		KeepData:  deleteKeepData,
		Vibespace: vs, // Pass the already-fetched vibespace to avoid redundant lookup
	}

	if err := svc.Delete(ctx, name, opts); err != nil {
		slog.Error("failed to delete vibespace", "name", name, "error", err)
		return fmt.Errorf("failed to delete vibespace: %w", err)
	}

	slog.Info("delete command completed", "name", name)
	printSuccess("Vibespace '%s' deleted", name)
	if deleteKeepData {
		printStep("Storage data preserved. Clean up manually if needed.")
	}
	return nil
}

package cli

import (
	"context"
	"fmt"
	"log/slog"

	"vibespace/pkg/daemon"
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
Use --dry-run to see what would be deleted without actually deleting.

This action cannot be undone.`,
	Example: `  vibespace delete myproject
  vibespace delete myproject --force
  vibespace delete myproject --keep-data
  vibespace delete myproject --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

var (
	deleteForce    bool
	deleteKeepData bool
	deleteDryRun   bool
)

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteKeepData, "keep-data", false, "Preserve persistent storage (PVC) for data recovery")
	deleteCmd.Flags().BoolVarP(&deleteDryRun, "dry-run", "n", false, "Show what would be deleted without actually deleting")
}

func runDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	name := args[0]
	out := getOutput()

	slog.Info("delete command started", "name", name, "force", deleteForce, "keep_data", deleteKeepData, "dry_run", deleteDryRun)

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

	// List resources that would be deleted
	resources := []string{
		fmt.Sprintf("Deployment: vibespace-%s", vs.ID),
		fmt.Sprintf("Service: vibespace-%s", vs.ID),
	}
	if !deleteKeepData {
		resources = append(resources, fmt.Sprintf("PVC: vibespace-%s-pvc", name))
	}
	resources = append(resources, fmt.Sprintf("Secret: vibespace-%s-ssh-keys", vs.ID))

	// Dry-run mode
	if deleteDryRun {
		if out.IsJSONMode() {
			return out.JSON(JSONOutput{
				Success: true,
				Data: DeleteOutput{
					Name:      name,
					KeepData:  deleteKeepData,
					DryRun:    true,
					Resources: resources,
				},
			})
		}
		printStep("Would delete vibespace '%s':", name)
		for _, r := range resources {
			fmt.Printf("  - %s\n", r)
		}
		if deleteKeepData {
			fmt.Println()
			printStep("Storage data would be preserved")
		}
		return nil
	}

	// Confirm deletion unless --force
	if !deleteForce {
		// Check if stdin is a terminal
		if !out.CanPrompt() {
			return fmt.Errorf("cannot prompt for confirmation (stdin is not a terminal). Use --force to skip confirmation")
		}

		msg := fmt.Sprintf("Delete vibespace '%s'?", vs.Name)
		if deleteKeepData {
			msg += " (data will be preserved)"
		} else {
			msg += " All data will be deleted."
		}

		confirmed, err := out.Confirm(msg, true) // defaultNo=true
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Stop daemon if running
	if daemon.IsRunning(name) {
		slog.Info("stopping daemon before delete", "vibespace", name)
		if err := daemon.StopDaemon(name); err != nil {
			slog.Warn("failed to stop daemon", "vibespace", name, "error", err)
			// Continue with deletion anyway
		}
	}

	spinner := NewSpinner(fmt.Sprintf("Deleting vibespace '%s'...", name))
	spinner.Start()

	opts := &vibespace.DeleteOptions{
		KeepData:  deleteKeepData,
		Vibespace: vs, // Pass the already-fetched vibespace to avoid redundant lookup
	}

	if err := svc.Delete(ctx, name, opts); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to delete vibespace '%s'", name))
		slog.Error("failed to delete vibespace", "name", name, "error", err)
		return fmt.Errorf("failed to delete vibespace: %w", err)
	}
	spinner.Stop()

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(JSONOutput{
			Success: true,
			Data: DeleteOutput{
				Name:      name,
				KeepData:  deleteKeepData,
				DryRun:    false,
				Resources: resources,
			},
		})
	}

	slog.Info("delete command completed", "name", name)
	printSuccess("Vibespace '%s' deleted", name)
	if deleteKeepData {
		printStep("Storage data preserved. Clean up manually if needed.")
	}
	return nil
}

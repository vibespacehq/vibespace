package cli

import (
	"context"
	"fmt"
	"log/slog"

	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	"github.com/vibespacehq/vibespace/pkg/vibespace"

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
	return doDelete(nil, args[0], deleteForce, deleteKeepData, deleteDryRun)
}

func doDelete(svc *vibespace.Service, name string, force, keepData, dryRun bool) error {
	ctx := context.Background()
	out := getOutput()

	slog.Info("delete command started", "name", name, "force", force, "keep_data", keepData, "dry_run", dryRun)

	if svc == nil {
		var err error
		svc, err = getVibespaceService()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	// Check if vibespace exists
	vs, err := svc.Get(ctx, name)
	if err != nil {
		slog.Error("vibespace not found", "name", name, "error", err)
		return fmt.Errorf("vibespace '%s' not found: %w", name, vserrors.ErrVibespaceNotFound)
	}

	// List resources that would be deleted
	resources := []string{
		fmt.Sprintf("Deployment: vibespace-%s", vs.ID),
		fmt.Sprintf("Service: vibespace-%s", vs.ID),
	}
	if !keepData {
		resources = append(resources, fmt.Sprintf("PVC: vibespace-%s-pvc", name))
	}
	resources = append(resources, fmt.Sprintf("Secret: vibespace-%s-ssh-keys", vs.ID))

	// Dry-run mode
	if dryRun {
		if out.IsJSONMode() {
			return out.JSON(JSONOutput{
				Success: true,
				Data: DeleteOutput{
					Name:      name,
					KeepData:  keepData,
					DryRun:    true,
					Resources: resources,
				},
			})
		}
		printStep("Would delete vibespace '%s':", name)
		for _, r := range resources {
			fmt.Printf("  - %s\n", r)
		}
		if keepData {
			fmt.Println()
			printStep("Storage data would be preserved")
		}
		return nil
	}

	// Confirm deletion unless --force
	if !force {
		// Check if stdin is a terminal
		if !out.CanPrompt() {
			return fmt.Errorf("cannot prompt for confirmation (stdin is not a terminal). Use --force to skip confirmation")
		}

		msg := fmt.Sprintf("Delete vibespace '%s'?", vs.Name)
		if keepData {
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

	spinner := NewSpinner(fmt.Sprintf("Deleting vibespace '%s'...", name))
	spinner.Start()

	opts := &vibespace.DeleteOptions{
		KeepData:  keepData,
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
				KeepData:  keepData,
				DryRun:    false,
				Resources: resources,
			},
		})
	}

	slog.Info("delete command completed", "name", name)
	printSuccess("Vibespace '%s' deleted", name)
	if keepData {
		printStep("Storage data preserved. Clean up manually if needed.")
	}
	return nil
}

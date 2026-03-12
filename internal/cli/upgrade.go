package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibespacehq/vibespace/pkg/update"

	"github.com/spf13/cobra"
)

var (
	upgradeCheck bool
	upgradeForce bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade vibespace to the latest version",
	Long:  `Download and install the latest version of vibespace from GitHub Releases.`,
	Example: `  vibespace upgrade
  vibespace upgrade --check
  vibespace upgrade --force`,
	RunE: runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeCheck, "check", false, "Only check for updates, don't install")
	upgradeCmd.Flags().BoolVar(&upgradeForce, "force", false, "Re-download even if already on latest")
}

// packageManagerHint returns a hint if the binary was installed via a package
// manager, or empty string if it's a direct install.
func packageManagerHint() string {
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	execPath, _ = filepath.EvalSymlinks(execPath)

	// Homebrew: /opt/homebrew/Cellar/... or /usr/local/Cellar/...
	if strings.Contains(execPath, "/Cellar/") {
		return "This binary was installed via Homebrew. Consider using: brew upgrade vibespace"
	}

	// APT/dpkg: /usr/bin/vibespace owned by dpkg
	if execPath == "/usr/bin/vibespace" || execPath == "/usr/local/bin/vibespace" {
		if _, err := os.Stat("/var/lib/dpkg/info/vibespace.list"); err == nil {
			return "This binary was installed via APT. Consider using: sudo apt upgrade vibespace"
		}
	}

	return ""
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	out := getOutput()
	ctx := context.Background()

	if Version == "dev" {
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(false, nil, &JSONError{
				Message: "cannot upgrade a development build — build from source instead",
			}))
		}
		return fmt.Errorf("cannot upgrade a development build — build from source instead")
	}

	// Fetch latest version
	printStep("Checking for updates...")
	latest, err := update.GetLatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	updateAvailable := update.IsNewer(latest, Version)

	// --check mode: just report
	if upgradeCheck {
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, UpgradeCheckOutput{
				UpdateAvailable: updateAvailable,
				CurrentVersion:  Version,
				LatestVersion:   latest,
			}, nil))
		}
		if updateAvailable {
			fmt.Printf("Update available: %s (current: %s)\n", out.Teal(latest), Version)
			fmt.Printf("Run %s to update.\n", out.Teal("vibespace upgrade"))
		} else {
			fmt.Printf("Already up to date: %s\n", out.Teal(Version))
		}
		return nil
	}

	// Already on latest (unless --force)
	if !updateAvailable && !upgradeForce {
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, UpgradeOutput{
				Upgraded:        false,
				PreviousVersion: Version,
				NewVersion:      Version,
			}, nil))
		}
		printSuccess("Already up to date: %s", Version)
		return nil
	}

	// Warn if installed via package manager
	if hint := packageManagerHint(); hint != "" {
		printWarning("%s", hint)
	}

	// Download and replace
	spinner := NewSpinner(fmt.Sprintf("Downloading %s...", latest))
	spinner.Start()

	binaryPath, err := update.DownloadAndReplace(ctx, latest)
	if err != nil {
		spinner.Fail("Upgrade failed")
		return fmt.Errorf("upgrade failed: %w", err)
	}

	spinner.Success(fmt.Sprintf("Upgraded %s → %s", Version, latest))

	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, UpgradeOutput{
			Upgraded:        true,
			PreviousVersion: Version,
			NewVersion:      latest,
			BinaryPath:      binaryPath,
		}, nil))
	}

	return nil
}

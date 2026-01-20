package cli

import (
	"context"
	"time"

	"vibespace/pkg/session"
	"vibespace/pkg/tui"

	"github.com/spf13/cobra"
)

// multiCmd is the top-level multi command for quick ad-hoc sessions
var multiCmd = &cobra.Command{
	Use:   "multi <vibespace>...",
	Short: "Start multi-agent terminal with specified vibespaces",
	Long: `Start an ad-hoc multi-agent terminal session with one or more vibespaces.

This launches a terminal UI where you can interact with multiple Claude agents
across the specified vibespaces simultaneously.

Examples:
  vibespace multi projectA           # Single vibespace
  vibespace multi projectA projectB  # Multiple vibespaces

Inside the TUI:
  @<agent> <message>                 Send to specific agent
  @<agent>@<vibespace> <message>     Send to agent in specific vibespace
  @all <message>                     Broadcast to all agents
  /list                              List connected agents
  /focus <agent>                     Focus on single agent
  /split                             Return to split view
  /save <name>                       Save as named session
  /quit                              Exit`,
	Args: cobra.MinimumNArgs(1),
	RunE: runMultiCmd,
}

// runMultiCmd handles the top-level multi command
func runMultiCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Verify all vibespaces exist and are running
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	for _, vsName := range args {
		_, err := checkVibespaceRunning(ctx, svc, vsName)
		if err != nil {
			return err
		}
	}

	// Create ad-hoc session
	sess := &session.Session{
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Vibespaces: make([]session.VibespaceEntry, 0, len(args)),
		Layout: session.Layout{
			Mode: session.LayoutModeSplit,
		},
	}

	for _, vs := range args {
		sess.Vibespaces = append(sess.Vibespaces, session.VibespaceEntry{
			Name: vs,
		})
	}

	// Setup TUI logging before launching (cleanup happens when TUI exits)
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	// Launch TUI
	return tui.Run(sess, true /* isAdHoc */)
}

// runMulti handles the vibespace-scoped multi command: vibespace <name> multi
func runMulti(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Verify vibespace exists and is running
	_, err = checkVibespaceRunning(ctx, svc, vibespace)
	if err != nil {
		return err
	}

	// Create ad-hoc session with single vibespace
	sess := &session.Session{
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Vibespaces: []session.VibespaceEntry{
			{Name: vibespace},
		},
		Layout: session.Layout{
			Mode: session.LayoutModeSplit,
		},
	}

	// Setup TUI logging before launching (cleanup happens when TUI exits)
	cleanup := setupLogging(LogConfig{Mode: LogModeTUI})
	defer cleanup()

	// Launch TUI
	return tui.Run(sess, true /* isAdHoc */)
}

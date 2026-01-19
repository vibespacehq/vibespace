package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"
	Commit  = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "vibespace",
	Short: "Multi-Claude development environments",
	Long: `vibespace - AI-powered development environments

Create isolated development environments with multiple Claude Code instances
that can collaborate on your codebase.

Get started:
  vibespace init              Initialize the cluster
  vibespace create myproject  Create a new vibespace
  vibespace myproject agents  List Claude instances
  vibespace myproject connect Connect to a Claude instance`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ArbitraryArgs,
	// Handle unknown commands as vibespace names
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// Treat first argument as a vibespace name
		return handleVibespaceCommand(args)
	},
}

func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	return err
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)

	// Global flags
	rootCmd.PersistentFlags().String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config)")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("vibespace %s (%s)\n", Version, Commit)
	},
}

// exitWithError prints an error and exits
func exitWithError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// handleVibespaceCommand handles commands for a specific vibespace
// Usage: vibespace <name> <subcommand> [args]
func handleVibespaceCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("vibespace name required")
	}

	vibespace := args[0]
	subArgs := args[1:]

	if len(subArgs) == 0 {
		// Show help for this vibespace
		fmt.Printf("Vibespace: %s\n\n", vibespace)
		fmt.Println("Available commands:")
		fmt.Println("  agents     List Claude instances")
		fmt.Println("  spawn      Create a new Claude instance")
		fmt.Println("  kill       Remove a Claude instance")
		fmt.Println("  connect    Connect to a Claude instance")
		fmt.Println("  multi      Multi-agent terminal mode")
		fmt.Println("  ports      List detected ports")
		fmt.Println("  forward    Forward a port to localhost")
		fmt.Println("  start      Start the vibespace")
		fmt.Println("  stop       Stop the vibespace")
		return nil
	}

	subCmd := subArgs[0]
	cmdArgs := subArgs[1:]

	switch subCmd {
	case "agents":
		return runAgents(vibespace, cmdArgs)
	case "spawn":
		return runSpawn(vibespace, cmdArgs)
	case "kill":
		return runKill(vibespace, cmdArgs)
	case "connect":
		return runConnect(vibespace, cmdArgs)
	case "multi":
		return runMulti(vibespace, cmdArgs)
	case "ports":
		return runPorts(vibespace, cmdArgs)
	case "forward":
		return runForward(vibespace, cmdArgs)
	case "start":
		return runStartVibespace(vibespace)
	case "stop":
		return runStopVibespace(vibespace)
	default:
		return fmt.Errorf("unknown command: %s", subCmd)
	}
}

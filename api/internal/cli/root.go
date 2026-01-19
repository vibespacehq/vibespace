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
}

func Execute() error {
	return rootCmd.Execute()
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
	rootCmd.PersistentFlags().String("kubeconfig", "", "Path to kubeconfig file (default: ~/.vibespace/kubeconfig)")
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

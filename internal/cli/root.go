package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"
	Commit  = "unknown"
)

// Global flags
var (
	globalJSON    bool
	globalVerbose bool
	globalQuiet   bool
	globalNoColor bool
	globalPlain   bool
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
  vibespace myproject connect Connect to a Claude instance

Documentation: https://github.com/vibespace/vibespace
Report issues: https://github.com/vibespace/vibespace/issues

Environment Variables:
  VIBESPACE_DEBUG=1           Enable debug logging to ~/.vibespace/debug.log
  VIBESPACE_LOG_LEVEL         Log level: debug, info, warn, error
  VIBESPACE_CLUSTER_CPU       Default cluster CPU cores (default: 4)
  VIBESPACE_CLUSTER_MEMORY    Default cluster memory in GB (default: 8)
  VIBESPACE_CLUSTER_DISK      Default cluster disk in GB (default: 60)
  VIBESPACE_DEFAULT_CPU       Default vibespace CPU (default: 1000m)
  VIBESPACE_DEFAULT_MEMORY    Default vibespace memory (default: 1Gi)
  VIBESPACE_DEFAULT_STORAGE   Default vibespace storage (default: 10Gi)
  NO_COLOR                    Disable colored output`,
	SilenceUsage:       true,
	SilenceErrors:      true,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true, // Let subcommands handle their own flags
	// PersistentPreRunE initializes output after flags are parsed
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		initOutputFromFlags()
		return nil
	},
	// Handle unknown commands as vibespace names
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		// Treat first argument as a vibespace name
		return handleVibespaceCommand(args)
	},
}

// initOutputFromFlags initializes the output based on global flag values
func initOutputFromFlags() {
	verbosity := 0
	if globalVerbose {
		verbosity = 1
	} else if globalQuiet {
		verbosity = -1
	}
	initOutput(OutputConfig{
		JSONMode:  globalJSON,
		PlainMode: globalPlain,
		Verbosity: verbosity,
		NoColor:   globalNoColor,
	})
}

// parseGlobalFlags extracts global flags from os.Args and returns the remaining args
// This is needed because DisableFlagParsing is true on root command
func parseGlobalFlags() {
	var newArgs []string
	args := os.Args[1:] // Skip program name

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			globalJSON = true
		case "--verbose", "-v":
			globalVerbose = true
		case "--quiet", "-q":
			globalQuiet = true
		case "--no-color":
			globalNoColor = true
		case "--plain":
			globalPlain = true
		case "--help", "-h":
			// Keep help flags for cobra to handle
			newArgs = append(newArgs, arg)
		default:
			// Check for --flag=value format
			if strings.HasPrefix(arg, "--json=") ||
				strings.HasPrefix(arg, "--verbose=") ||
				strings.HasPrefix(arg, "--quiet=") ||
				strings.HasPrefix(arg, "--no-color=") ||
				strings.HasPrefix(arg, "--plain=") {
				// Parse boolean flag with value (--flag=true/false)
				parts := strings.SplitN(arg, "=", 2)
				flag := parts[0]
				value := strings.ToLower(parts[1])
				isTrue := value == "true" || value == "1" || value == "yes"
				switch flag {
				case "--json":
					globalJSON = isTrue
				case "--verbose":
					globalVerbose = isTrue
				case "--quiet":
					globalQuiet = isTrue
				case "--no-color":
					globalNoColor = isTrue
				case "--plain":
					globalPlain = isTrue
				}
			} else {
				newArgs = append(newArgs, arg)
			}
		}
	}

	// Replace os.Args with filtered args
	os.Args = append([]string{os.Args[0]}, newArgs...)
}

func Execute() error {
	// Parse global flags before cobra processes commands
	// This handles flags for dynamic vibespace commands (e.g., vibespace myproject agents --json)
	parseGlobalFlags()

	// Initialize output with global flags (for dynamic commands that bypass PersistentPreRunE)
	initOutputFromFlags()

	cleanup := setupLogging(LogConfig{Mode: LogModeCLI})
	defer cleanup()

	err := rootCmd.Execute()
	if err != nil {
		// In JSON mode, output error as JSON
		if globalJSON {
			getOutput().JSON(JSONOutput{
				Success: false,
				Error:   &JSONError{Message: err.Error()},
			})
		} else {
			printError("%v", err)
		}
	}
	return err
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(daemonCmd)  // Hidden daemon command
	rootCmd.AddCommand(sessionCmd) // Multi-agent session management
	rootCmd.AddCommand(multiCmd)   // Quick ad-hoc multi-agent sessions

	// Global flags - registered here so subcommands can parse them
	rootCmd.PersistentFlags().BoolVar(&globalJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&globalVerbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&globalQuiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&globalNoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&globalPlain, "plain", false, "Plain output for scripting")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Example: `  vibespace version
  vibespace version --json`,
	Run: func(cmd *cobra.Command, args []string) {
		out := getOutput()
		if out.IsJSONMode() {
			out.JSON(JSONOutput{
				Success: true,
				Data: VersionOutput{
					Version: Version,
					Commit:  Commit,
				},
			})
			return
		}
		fmt.Printf("vibespace %s (%s)\n", Version, Commit)
	},
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
		fmt.Println("  config     View/modify agent configuration")
		fmt.Println("  multi      Multi-agent terminal mode")
		fmt.Println("  ports      List detected ports")
		fmt.Println("  up         Scale up agents (start pods)")
		fmt.Println("  down       Scale down agents (stop pods)")
		fmt.Println("  forward    Manage port-forwards (list, add, remove, ...)")
		fmt.Println("  start      Start the vibespace (alias for 'up')")
		fmt.Println("  stop       Stop the vibespace (alias for 'down')")
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
	case "config":
		return runConfig(vibespace, cmdArgs)
	case "multi":
		return runMulti(vibespace, cmdArgs)
	case "ports":
		return runPorts(vibespace, cmdArgs)
	case "up", "start":
		return runUp(vibespace, cmdArgs)
	case "down", "stop":
		return runDown(vibespace, cmdArgs)
	case "forward":
		return runForwardCmd(vibespace, cmdArgs)
	default:
		if suggestion := suggestVibespaceCommand(subCmd); suggestion != "" {
			return fmt.Errorf("unknown command: %s\n\nDid you mean: vibespace %s %s", subCmd, vibespace, suggestion)
		}
		return fmt.Errorf("unknown command: %s", subCmd)
	}
}

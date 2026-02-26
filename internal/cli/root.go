package cli

import (
	"fmt"
	"os"

	"github.com/vibespacehq/vibespace/pkg/config"
	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	"github.com/vibespacehq/vibespace/pkg/k8s"
	"github.com/vibespacehq/vibespace/pkg/tui"
	"github.com/vibespacehq/vibespace/pkg/ui"

	"github.com/spf13/cobra"
)

var (
	// Version info - set at build time via ldflags
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Global flags
var (
	globalJSON       bool
	globalVerbose    bool
	globalQuiet      bool
	globalNoColor    bool
	globalPlain      bool
	globalHeader     bool
	globalVibespace  string
	globalConfigPath string
)

var rootCmd = &cobra.Command{
	Use:   "vibespace",
	Short: "Multi-Claude development environments",
	Long: `vibespace - AI-powered development environments

Create isolated development environments with multiple Claude Code instances
that can collaborate on your codebase.

Get started:
  vibespace init                                    Initialize the cluster
  vibespace create myproject -t claude-code         Create a new vibespace
  vibespace agent list --vibespace myproject        List agents
  vibespace connect --vibespace myproject           Connect to an agent

Tip: set VIBESPACE_NAME to avoid typing --vibespace every time:
  export VIBESPACE_NAME=myproject
  vibespace agent list

Documentation: https://github.com/vibespace/vibespace
Report issues: https://github.com/vibespace/vibespace/issues

Environment Variables:
  VIBESPACE_NAME              Default vibespace for --vibespace flag
  VIBESPACE_DEBUG=1           Enable debug logging to ~/.vibespace/debug.log
  VIBESPACE_LOG_LEVEL         Log level: debug, info, warn, error
  VIBESPACE_CLUSTER_CPU       Default cluster CPU cores (default: 4)
  VIBESPACE_CLUSTER_MEMORY    Default cluster memory in GB (default: 8)
  VIBESPACE_CLUSTER_DISK      Default cluster disk in GB (default: 60)
  VIBESPACE_DEFAULT_CPU       Default vibespace CPU (default: 1000m)
  VIBESPACE_DEFAULT_MEMORY    Default vibespace memory (default: 1Gi)
  VIBESPACE_DEFAULT_STORAGE   Default vibespace storage (default: 10Gi)
  VIBESPACE_CONFIG            Path to config file (default: ~/.vibespace/config.yaml)
  NO_COLOR                    Disable colored output`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(globalConfigPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		config.SetGlobal(cfg)
		ui.ApplyTheme(cfg.Theme)
		k8s.VibespaceNamespace = cfg.Kubernetes.Namespace
		initOutputFromFlags()
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunApp(Version, Commit, BuildDate)
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
		Header:    globalHeader,
		Verbosity: verbosity,
		NoColor:   globalNoColor,
	})
}

func Execute() error {
	cleanup := setupLogging(LogConfig{Mode: LogModeCLI})
	defer cleanup()

	err := rootCmd.Execute()
	if err != nil {
		exitCode, code := vserrors.ErrorCode(err)

		// In JSON mode, output error as JSON
		if globalJSON {
			hint := getErrorHint(err)
			getOutput().JSON(NewJSONOutput(false, nil, &JSONError{
				Message:  err.Error(),
				Code:     code,
				ExitCode: exitCode,
				Hint:     hint,
			}))
		} else {
			printError("%v", err)
		}

		os.Exit(exitCode)
	}
	return nil
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
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(multiCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(remoteCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(forwardCmd)

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&globalJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&globalVerbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&globalQuiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&globalNoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&globalPlain, "plain", false, "Plain output for scripting")
	rootCmd.PersistentFlags().BoolVar(&globalHeader, "header", false, "Include headers in plain output")

	// Config flag
	rootCmd.PersistentFlags().StringVar(&globalConfigPath, "config", "", "Path to config file (env: VIBESPACE_CONFIG)")

	// Vibespace flag
	defaultVS := os.Getenv("VIBESPACE_NAME")
	rootCmd.PersistentFlags().StringVar(&globalVibespace, "vibespace", defaultVS, "Vibespace name (env: VIBESPACE_NAME)")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Example: `  vibespace version
  vibespace version --json`,
	Run: func(cmd *cobra.Command, args []string) {
		out := getOutput()
		if out.IsJSONMode() {
			out.JSON(NewJSONOutput(true, VersionOutput{
				Version:   Version,
				Commit:    Commit,
				BuildDate: BuildDate,
			}, nil))
			return
		}
		fmt.Printf("vibespace %s\n", Version)
		fmt.Printf("  commit:  %s\n", Commit)
		fmt.Printf("  built:   %s\n", BuildDate)
	},
}

func requireVibespace(cmd *cobra.Command) (string, error) {
	if globalVibespace == "" {
		return "", fmt.Errorf("vibespace name required: use --vibespace flag or set VIBESPACE_NAME environment variable")
	}
	return globalVibespace, nil
}

package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/agent"
	"github.com/yagizdagabak/vibespace/pkg/k8s"
	"github.com/yagizdagabak/vibespace/pkg/model"
	"github.com/yagizdagabak/vibespace/pkg/vibespace"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new vibespace",
	Long:  `Create a new vibespace with an AI coding agent.`,
	Example: `  vibespace create myproject -t claude-code
  vibespace create myproject -t codex --repo https://github.com/user/repo
  vibespace create myproject -t claude-code --cpu 500m --memory 512Mi
  vibespace create myproject -t claude-code --share-credentials
  vibespace create myproject -t claude-code --mount ~/code:/workspace
  vibespace create myproject -t claude-code --mount ~/code:/workspace:ro`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

var (
	createRepo             string
	createCPU              string
	createMemory           string
	createStorage          string
	createShareCredentials bool
	createAgentType        string   // Agent type (claude-code, codex)
	createAgentName        string   // Custom name for primary agent
	createMounts           []string // Host directory mounts (host:container[:ro])
	// Agent config flags
	createSkipPermissions bool
	createAllowedTools    string
	createDisallowedTools string
	createModel           string
	createMaxTurns        int
)

// Default resource values - can be overridden via environment variables
const (
	DefaultCPU     = "1000m"
	DefaultMemory  = "1Gi"
	DefaultStorage = "10Gi"
)

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func init() {
	// Read defaults from environment variables, falling back to constants
	cpuDefault := getEnvOrDefault("VIBESPACE_DEFAULT_CPU", DefaultCPU)
	memoryDefault := getEnvOrDefault("VIBESPACE_DEFAULT_MEMORY", DefaultMemory)
	storageDefault := getEnvOrDefault("VIBESPACE_DEFAULT_STORAGE", DefaultStorage)

	createCmd.Flags().StringVar(&createRepo, "repo", "", "GitHub repository to clone")
	createCmd.Flags().StringVar(&createCPU, "cpu", cpuDefault, "CPU request/limit (e.g., 400m, 500m, 1)")
	createCmd.Flags().StringVar(&createMemory, "memory", memoryDefault, "Memory request/limit (e.g., 256Mi, 512Mi, 1Gi)")
	createCmd.Flags().StringVar(&createStorage, "storage", storageDefault, "Storage size for persistent volume (e.g., 10Gi, 20Gi)")
	createCmd.Flags().BoolVarP(&createShareCredentials, "share-credentials", "s", false, "Share credentials across all agents")
	createCmd.Flags().StringVarP(&createAgentType, "agent-type", "t", "", "Agent type: claude-code, codex (required)")
	createCmd.MarkFlagRequired("agent-type")
	createCmd.Flags().StringVarP(&createAgentName, "name", "n", "", "Custom name for the primary agent (default: <type>-1)")
	createCmd.Flags().StringArrayVarP(&createMounts, "mount", "m", nil, "Mount host directory (host:container[:ro], can be repeated)")

	// Agent configuration flags
	createCmd.Flags().BoolVar(&createSkipPermissions, "skip-permissions", false, "Enable --dangerously-skip-permissions for Claude")
	createCmd.Flags().StringVar(&createAllowedTools, "allowed-tools", "", "Comma-separated allowed tools (replaces default)")
	createCmd.Flags().StringVar(&createDisallowedTools, "disallowed-tools", "", "Comma-separated disallowed tools")
	createCmd.Flags().StringVar(&createModel, "model", "", "Claude model to use (e.g., opus, sonnet)")
	createCmd.Flags().IntVar(&createMaxTurns, "max-turns", 0, "Maximum conversation turns")
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	name := args[0]
	slog.Info("create command started", "name", name, "repo", createRepo, "agent_type", createAgentType)

	// Parse and validate agent type
	agentType := agent.ParseType(createAgentType)
	if !agentType.IsValid() {
		return fmt.Errorf("invalid agent type '%s': valid types are claude-code, codex", createAgentType)
	}

	// Get vibespace service
	svc, err := getVibespaceService()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Build AgentConfig if any config flags are set
	var agentConfig *agent.Config
	if createSkipPermissions || createAllowedTools != "" || createDisallowedTools != "" || createModel != "" || createMaxTurns > 0 {
		agentConfig = &agent.Config{
			SkipPermissions: createSkipPermissions,
			Model:           createModel,
			MaxTurns:        createMaxTurns,
		}
		if createAllowedTools != "" {
			agentConfig.AllowedTools = strings.Split(createAllowedTools, ",")
		}
		if createDisallowedTools != "" {
			agentConfig.DisallowedTools = strings.Split(createDisallowedTools, ",")
		}
	}

	// Parse and validate mounts
	var mounts []model.Mount
	for _, mountStr := range createMounts {
		mount, err := parseMount(mountStr)
		if err != nil {
			return fmt.Errorf("invalid mount '%s': %w", mountStr, err)
		}
		mounts = append(mounts, mount)
	}

	// Build create request
	req := &model.CreateVibespaceRequest{
		Name:             name,
		Persistent:       true, // Always use persistent storage for shared filesystem between agents
		ShareCredentials: createShareCredentials,
		AgentType:        agentType,
		AgentName:        createAgentName,
		AgentConfig:      agentConfig,
		Mounts:           mounts,
		Resources: &model.Resources{
			CPU:     createCPU,
			Memory:  createMemory,
			Storage: createStorage,
		},
	}
	if createRepo != "" {
		req.GithubRepo = createRepo
	}

	spinner := NewSpinner("Creating vibespace...")
	spinner.Start()

	vs, err := svc.Create(ctx, req)
	if err != nil {
		spinner.Fail("Failed to create vibespace")
		slog.Error("failed to create vibespace", "error", err)
		return fmt.Errorf("failed to create vibespace: %w", err)
	}

	slog.Info("create command completed", "name", vs.Name, "id", vs.ID)
	spinner.Success(fmt.Sprintf("Vibespace created: %s (starting...)", vs.Name))

	// JSON output
	out := getOutput()
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, CreateOutput{
			Name: vs.Name,
			ID:   vs.ID,
		}, nil))
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  vibespace %s agent     List agents\n", vs.Name)
	fmt.Printf("  vibespace %s connect   Connect to an agent\n", vs.Name)

	return nil
}

// parseMount parses a mount string in the format host:container[:ro]
func parseMount(mountStr string) (model.Mount, error) {
	parts := strings.Split(mountStr, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return model.Mount{}, fmt.Errorf("format must be host:container[:ro]")
	}

	hostPath := parts[0]
	containerPath := parts[1]
	readOnly := false

	// Check for :ro suffix
	if len(parts) == 3 {
		if parts[2] != "ro" {
			return model.Mount{}, fmt.Errorf("third part must be 'ro' for read-only mount")
		}
		readOnly = true
	}

	// Expand ~ to home directory
	if strings.HasPrefix(hostPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return model.Mount{}, fmt.Errorf("failed to expand ~: %w", err)
		}
		hostPath = filepath.Join(home, hostPath[2:])
	} else if hostPath == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return model.Mount{}, fmt.Errorf("failed to expand ~: %w", err)
		}
		hostPath = home
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(hostPath)
	if err != nil {
		return model.Mount{}, fmt.Errorf("invalid host path: %w", err)
	}
	hostPath = absPath

	// Validate host path exists
	info, err := os.Stat(hostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Mount{}, fmt.Errorf("host path does not exist: %s", hostPath)
		}
		return model.Mount{}, fmt.Errorf("cannot access host path: %w", err)
	}
	if !info.IsDir() {
		return model.Mount{}, fmt.Errorf("host path must be a directory: %s", hostPath)
	}

	// Validate container path is absolute
	if !filepath.IsAbs(containerPath) {
		return model.Mount{}, fmt.Errorf("container path must be absolute: %s", containerPath)
	}

	return model.Mount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		ReadOnly:      readOnly,
	}, nil
}

// getVibespaceService creates the vibespace service with all dependencies
func getVibespaceService() (*vibespace.Service, error) {
	// Get isolated kubeconfig path
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	kubeconfig := filepath.Join(home, ".vibespace", "kubeconfig")

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return nil, fmt.Errorf("cluster not initialized. Run 'vibespace init' first")
	}

	// Set KUBECONFIG environment variable for the k8s client
	os.Setenv("KUBECONFIG", kubeconfig)

	// Create k8s client (reads from KUBECONFIG or default location)
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Create vibespace service (it creates deployment and network managers internally)
	svc := vibespace.NewService(k8sClient)
	return svc, nil
}

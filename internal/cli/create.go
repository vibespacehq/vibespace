package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/config"
	"github.com/vibespacehq/vibespace/pkg/github"
	"github.com/vibespacehq/vibespace/pkg/k8s"
	modelPkg "github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/vibespace"

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
	createCPULimit         string
	createMemory           string
	createMemoryLimit      string
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

func init() {
	// Flag defaults use hardcoded values matching config.Default().
	// At runtime, runCreate checks cmd.Flags().Changed() and falls back to config.Global()
	// which has file + env overrides applied.
	createCmd.Flags().StringVar(&createRepo, "repo", "", "GitHub repository to clone")
	createCmd.Flags().StringVar(&createCPU, "cpu", "250m", "CPU request for scheduling (e.g., 250m, 500m)")
	createCmd.Flags().StringVar(&createCPULimit, "cpu-limit", "1000m", "CPU limit for burst (e.g., 1000m, 2000m)")
	createCmd.Flags().StringVar(&createMemory, "memory", "512Mi", "Memory request for scheduling (e.g., 256Mi, 512Mi)")
	createCmd.Flags().StringVar(&createMemoryLimit, "memory-limit", "1Gi", "Memory limit for burst (e.g., 1Gi, 2Gi)")
	createCmd.Flags().StringVar(&createStorage, "storage", "10Gi", "Storage size for persistent volume (e.g., 10Gi, 20Gi)")
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
	// Apply config.Global() for flags not explicitly set by user
	cfg := config.Global()
	if !cmd.Flags().Changed("cpu") {
		createCPU = cfg.Resources.CPU
	}
	if !cmd.Flags().Changed("cpu-limit") {
		createCPULimit = cfg.Resources.CPULimit
	}
	if !cmd.Flags().Changed("memory") {
		createMemory = cfg.Resources.Memory
	}
	if !cmd.Flags().Changed("memory-limit") {
		createMemoryLimit = cfg.Resources.MemoryLimit
	}
	if !cmd.Flags().Changed("storage") {
		createStorage = cfg.Resources.Storage
	}
	if !cmd.Flags().Changed("share-credentials") {
		createShareCredentials = cfg.Agent.ShareCredentials
	}
	if !cmd.Flags().Changed("skip-permissions") {
		createSkipPermissions = cfg.Agent.SkipPermissions
	}
	if !cmd.Flags().Changed("model") && cfg.Agent.Model != "" {
		createModel = cfg.Agent.Model
	}
	if !cmd.Flags().Changed("max-turns") && cfg.Agent.MaxTurns > 0 {
		createMaxTurns = cfg.Agent.MaxTurns
	}
	return doCreate(nil, args[0], createAgentType, createRepo, createAgentName, createCPU, createCPULimit,
		createMemory, createMemoryLimit, createStorage, createShareCredentials, createMounts,
		createSkipPermissions, createAllowedTools, createDisallowedTools, createModel, createMaxTurns)
}

func doCreate(svc *vibespace.Service, name, agentTypeStr, repo, agentName, cpu, cpuLimit,
	memory, memoryLimit, storage string, shareCredentials bool, mounts []string,
	skipPermissions bool, allowedTools, disallowedTools, model string, maxTurns int) error {
	ctx := context.Background()

	slog.Info("create command started", "name", name, "repo", repo, "agent_type", agentTypeStr)

	// Parse and validate agent type
	agentType := agent.ParseType(agentTypeStr)
	if !agentType.IsValid() {
		return fmt.Errorf("invalid agent type '%s': valid types are claude-code, codex", agentTypeStr)
	}

	// Get vibespace service
	if svc == nil {
		var err error
		svc, err = getVibespaceService()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	// Build AgentConfig if any config flags are set
	var agentConfig *agent.Config
	if skipPermissions || allowedTools != "" || disallowedTools != "" || model != "" || maxTurns > 0 {
		agentConfig = &agent.Config{
			SkipPermissions: skipPermissions,
			Model:           model,
			MaxTurns:        maxTurns,
		}
		if allowedTools != "" {
			agentConfig.AllowedTools = strings.Split(allowedTools, ",")
		}
		if disallowedTools != "" {
			agentConfig.DisallowedTools = strings.Split(disallowedTools, ",")
		}
	}

	// Parse and validate mounts
	var parsedMounts []modelPkg.Mount
	for _, mountStr := range mounts {
		mount, err := parseMount(mountStr)
		if err != nil {
			return fmt.Errorf("invalid mount '%s': %w", mountStr, err)
		}
		parsedMounts = append(parsedMounts, mount)
	}

	// Build create request
	req := &modelPkg.CreateVibespaceRequest{
		Name:             name,
		Persistent:       true, // Always use persistent storage for shared filesystem between agents
		ShareCredentials: shareCredentials,
		AgentType:        agentType,
		AgentName:        agentName,
		AgentConfig:      agentConfig,
		Mounts:           parsedMounts,
		Resources: &modelPkg.Resources{
			CPU:         cpu,
			CPULimit:    cpuLimit,
			Memory:      memory,
			MemoryLimit: memoryLimit,
			Storage:     storage,
		},
	}
	if repo != "" {
		req.GithubRepo = repo
	}

	// GitHub OAuth device flow for HTTPS repos
	if repo != "" && strings.HasPrefix(repo, "https://") {
		clientID := config.Global().GitHub.ClientID

		devResp, err := github.RequestDeviceCode(ctx, clientID, "repo")
		if err != nil {
			return fmt.Errorf("failed to start GitHub authorization: %w", err)
		}

		fmt.Println()
		fmt.Printf("  Open:  %s\n", devResp.VerificationURI)
		fmt.Printf("  Code:  %s\n\n", devResp.UserCode)

		// Best-effort browser open
		_ = openBrowser(devResp.VerificationURI)

		authSpinner := NewSpinner("Waiting for GitHub authorization...")
		authSpinner.Start()

		authCtx, authCancel := context.WithTimeout(ctx, time.Duration(devResp.ExpiresIn)*time.Second)
		defer authCancel()

		token, err := github.PollForToken(authCtx, clientID, devResp.DeviceCode, devResp.Interval)
		if err != nil {
			authSpinner.Fail("GitHub authorization failed")
			return fmt.Errorf("GitHub authorization failed: %w", err)
		}

		authSpinner.Success("GitHub authorized")
		req.GithubAccessToken = token.AccessToken
		req.GithubRefreshToken = token.RefreshToken
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
func parseMount(mountStr string) (modelPkg.Mount, error) {
	parts := strings.Split(mountStr, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return modelPkg.Mount{}, fmt.Errorf("format must be host:container[:ro]")
	}

	hostPath := parts[0]
	containerPath := parts[1]
	readOnly := false

	// Check for :ro suffix
	if len(parts) == 3 {
		if parts[2] != "ro" {
			return modelPkg.Mount{}, fmt.Errorf("third part must be 'ro' for read-only mount")
		}
		readOnly = true
	}

	// Expand ~ to home directory
	if strings.HasPrefix(hostPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return modelPkg.Mount{}, fmt.Errorf("failed to expand ~: %w", err)
		}
		hostPath = filepath.Join(home, hostPath[2:])
	} else if hostPath == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return modelPkg.Mount{}, fmt.Errorf("failed to expand ~: %w", err)
		}
		hostPath = home
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(hostPath)
	if err != nil {
		return modelPkg.Mount{}, fmt.Errorf("invalid host path: %w", err)
	}
	hostPath = absPath

	// Validate host path exists
	info, err := os.Stat(hostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return modelPkg.Mount{}, fmt.Errorf("host path does not exist: %s", hostPath)
		}
		return modelPkg.Mount{}, fmt.Errorf("cannot access host path: %w", err)
	}
	if !info.IsDir() {
		return modelPkg.Mount{}, fmt.Errorf("host path must be a directory: %s", hostPath)
	}

	// Validate container path is absolute
	if !filepath.IsAbs(containerPath) {
		return modelPkg.Mount{}, fmt.Errorf("container path must be absolute: %s", containerPath)
	}

	return modelPkg.Mount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		ReadOnly:      readOnly,
	}, nil
}

// getVibespaceService creates the vibespace service with all dependencies
func getVibespaceService() (*vibespace.Service, error) {
	kubeconfig, err := resolveKubeconfig()
	if err != nil {
		return nil, err
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

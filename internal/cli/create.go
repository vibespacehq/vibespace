package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/yagizdagabak/vibespace/pkg/k8s"
	"github.com/yagizdagabak/vibespace/pkg/model"
	"github.com/yagizdagabak/vibespace/pkg/vibespace"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new vibespace",
	Long: `Create a new vibespace with a Claude Code instance.

If no name is provided, a random name will be generated.`,
	Example: `  vibespace create
  vibespace create myproject
  vibespace create myproject --repo https://github.com/user/repo
  vibespace create myproject --cpu 500m --memory 512Mi
  vibespace create myproject --share-credentials`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreate,
}

var (
	createRepo             string
	createCPU              string
	createMemory           string
	createStorage          string
	createShareCredentials bool
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
	createCmd.Flags().BoolVarP(&createShareCredentials, "share-credentials", "s", false, "Share Claude credentials across all agents")
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	slog.Info("create command started", "name", name, "repo", createRepo)

	// Get vibespace service
	svc, err := getVibespaceService()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Build create request
	req := &model.CreateVibespaceRequest{
		Name:             name,
		Persistent:       true, // Always use persistent storage for shared filesystem between agents
		ShareCredentials: createShareCredentials,
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
	spinner.Success(fmt.Sprintf("Vibespace created: %s", vs.Name))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  vibespace %s agents    List Claude instances\n", vs.Name)
	fmt.Printf("  vibespace %s connect   Connect to Claude\n", vs.Name)

	return nil
}

// getVibespaceService creates the vibespace service with all dependencies
func getVibespaceService() (*vibespace.Service, error) {
	// Get kubeconfig path - Colima updates ~/.kube/config with the "colima" context
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	kubeconfig := filepath.Join(home, ".kube", "config")

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

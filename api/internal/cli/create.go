package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"vibespace/pkg/k8s"
	"vibespace/pkg/model"
	"vibespace/pkg/vibespace"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new vibespace",
	Long: `Create a new vibespace with a Claude Code instance.

If no name is provided, a random name will be generated.

Examples:
  vibespace create
  vibespace create myproject
  vibespace create myproject --repo https://github.com/user/repo`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreate,
}

var (
	createRepo string
)

func init() {
	createCmd.Flags().StringVar(&createRepo, "repo", "", "GitHub repository to clone")
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get vibespace service
	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Build create request
	req := &model.CreateVibespaceRequest{}
	if len(args) > 0 {
		req.Name = args[0]
	}
	if createRepo != "" {
		req.GithubRepo = createRepo
	}

	printStep("Creating vibespace...")

	vs, err := svc.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create vibespace: %w", err)
	}

	printSuccess("Vibespace created: %s", vs.Name)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  vibespace %s agents    List Claude instances\n", vs.Name)
	fmt.Printf("  vibespace %s connect   Connect to Claude\n", vs.Name)

	return nil
}

// getVibespaceService creates the vibespace service with all dependencies
func getVibespaceService() (*vibespace.Service, error) {
	// Get kubeconfig path
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

	// Create vibespace service (it creates knative and network managers internally)
	svc := vibespace.NewService(k8sClient)
	return svc, nil
}

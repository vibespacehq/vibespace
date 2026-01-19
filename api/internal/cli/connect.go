package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runConnect(vibespace string, args []string) error {
	ctx := context.Background()

	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Verify vibespace exists and is running
	vs, err := svc.Get(ctx, vibespace)
	if err != nil {
		return fmt.Errorf("vibespace '%s' not found", vibespace)
	}

	if vs.Status != "running" {
		return fmt.Errorf("vibespace '%s' is not running. Start it with: vibespace %s start", vibespace, vibespace)
	}

	// Determine which agent to connect to
	agentID := "claude-1" // Default
	if len(args) > 0 {
		agentID = args[0]
	}

	// Get paths
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubectlBin := filepath.Join(home, ".vibespace", "bin", "kubectl")

	// Build pod name
	// Format: vibespace-<vibespace-name>-<agent-id>-<hash>
	// For now, we use a simplified approach
	podSelector := fmt.Sprintf("vibespace.dev/id=%s", vibespace)

	printStep("Connecting to %s in %s...", agentID, vibespace)

	// Find the pod
	findCmd := exec.CommandContext(ctx, kubectlBin,
		"--kubeconfig", kubeconfig,
		"-n", "vibespace",
		"get", "pod",
		"-l", podSelector,
		"-o", "jsonpath={.items[0].metadata.name}",
	)
	podNameBytes, err := findCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to find pod: %w", err)
	}
	podName := string(podNameBytes)
	if podName == "" {
		return fmt.Errorf("no running pod found for vibespace '%s'", vibespace)
	}

	// Connect to the pod
	fmt.Printf("Connected to %s. Type 'exit' to disconnect.\n", agentID)
	fmt.Println()

	cmd := exec.CommandContext(ctx, kubectlBin,
		"--kubeconfig", kubeconfig,
		"-n", "vibespace",
		"exec", "-it", podName,
		"--", "/bin/bash",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

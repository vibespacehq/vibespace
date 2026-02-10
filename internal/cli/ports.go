package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
)

// DetectedPort represents a port detected by the port detector daemon
type DetectedPort struct {
	Port       int       `json:"port"`
	Process    string    `json:"process"`
	DetectedAt time.Time `json:"detected_at"`
}

// DetectedPorts represents the JSON structure from port detector
type DetectedPorts struct {
	Ports []DetectedPort `json:"ports"`
}

func runPorts(vibespace string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`List detected ports in a vibespace

Usage:
  vibespace %s ports

Ports are automatically detected when agents start dev servers.

Examples:
  vibespace %s ports
  vibespace %s ports --json
`, vibespace, vibespace, vibespace)
			return nil
		}
	}

	// Ensure daemon is running (auto-start if needed)
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	// Verify vibespace exists and is running
	vs, err := svc.Get(ctx, vibespace)
	if err != nil {
		return fmt.Errorf("vibespace '%s' not found: %w", vibespace, vserrors.ErrVibespaceNotFound)
	}

	if vs.Status != "running" {
		return fmt.Errorf("vibespace '%s' is not running", vibespace)
	}

	// Get paths - use active kubeconfig (remote or local)
	home, _ := os.UserHomeDir()
	kubeconfig, _ := resolveKubeconfig()
	kubectlBin := filepath.Join(home, ".vibespace", "bin", "kubectl")

	// Find the pod using internal ID
	podSelector := fmt.Sprintf("vibespace.dev/id=%s", vs.ID)

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

	// Read detected ports from container
	readCmd := exec.CommandContext(ctx, kubectlBin,
		"--kubeconfig", kubeconfig,
		"-n", "vibespace",
		"exec", podName,
		"--", "cat", "/tmp/vibespace-ports.json",
	)
	output, err := readCmd.Output()
	if err != nil {
		// JSON output for empty result
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, PortsOutput{
				Vibespace: vibespace,
				Ports:     []DetectedPort{},
				Count:     0,
			}, nil))
		}
		// Plain mode - no output for empty result
		if out.IsPlainMode() {
			return nil
		}
		// File might not exist yet
		fmt.Println("No ports detected yet")
		fmt.Println()
		fmt.Println("Ports are detected when Claude starts a dev server (e.g., npm run dev)")
		return nil
	}

	var ports DetectedPorts
	if err := json.Unmarshal(output, &ports); err != nil {
		return fmt.Errorf("failed to parse ports data: %w", err)
	}

	// JSON output mode
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, PortsOutput{
			Vibespace: vibespace,
			Ports:     ports.Ports,
			Count:     len(ports.Ports),
		}, nil))
	}

	if len(ports.Ports) == 0 {
		// Plain mode - no output for empty result
		if out.IsPlainMode() {
			return nil
		}
		fmt.Println("No ports detected")
		fmt.Println()
		fmt.Println("Ports are detected when Claude starts a dev server")
		return nil
	}

	// Build table rows
	headers := []string{"PORT", "PROCESS", "DETECTED"}
	rows := make([][]string, len(ports.Ports))
	for i, port := range ports.Ports {
		ago := time.Since(port.DetectedAt).Round(time.Second)
		rows[i] = []string{
			fmt.Sprintf("%d", port.Port),
			port.Process,
			fmt.Sprintf("%s ago", ago),
		}
	}

	out.Table(headers, rows)

	// Don't print footer in plain mode
	if !out.IsPlainMode() {
		fmt.Println()
		fmt.Printf("Forward a port with: vibespace %s forward add <port>\n", vibespace)
	}

	return nil
}

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"
	"time"
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
		return fmt.Errorf("vibespace '%s' not found", vibespace)
	}

	if vs.Status != "running" {
		return fmt.Errorf("vibespace '%s' is not running", vibespace)
	}

	// Get paths
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
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

	if len(ports.Ports) == 0 {
		fmt.Println("No ports detected")
		fmt.Println()
		fmt.Println("Ports are detected when Claude starts a dev server")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tPROCESS\tDETECTED")

	for _, port := range ports.Ports {
		ago := time.Since(port.DetectedAt).Round(time.Second)
		fmt.Fprintf(w, "%d\t%s\t%s ago\n", port.Port, port.Process, ago)
	}

	w.Flush()
	fmt.Println()
	fmt.Printf("Forward a port with: vibespace %s forward <port>\n", vibespace)

	return nil
}

func runForward(vibespace string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("port number required")
	}

	port := args[0]

	ctx := context.Background()

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
		return fmt.Errorf("vibespace '%s' not found", vibespace)
	}

	if vs.Status != "running" {
		return fmt.Errorf("vibespace '%s' is not running", vibespace)
	}

	// Get paths
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
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

	fmt.Printf("Forwarding %s:%s → localhost:%s\n", vibespace, port, port)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Run kubectl port-forward
	cmd := exec.CommandContext(ctx, kubectlBin,
		"--kubeconfig", kubeconfig,
		"-n", "vibespace",
		"port-forward", podName,
		fmt.Sprintf("%s:%s", port, port),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

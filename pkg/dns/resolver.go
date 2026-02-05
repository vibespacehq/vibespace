package dns

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
)

// ConfigureSystemResolver configures the system to use our DNS server for
// the vibespace.internal domain.
//
// On macOS: Creates /etc/resolver/vibespace.internal pointing to 127.0.0.1:<port>
// On Linux: Adds a systemd-resolved configuration
func ConfigureSystemResolver(port int) error {
	switch runtime.GOOS {
	case "darwin":
		return configureMacOSResolver(port)
	case "linux":
		return configureLinuxResolver(port)
	default:
		slog.Warn("DNS resolver configuration not supported on this platform", "os", runtime.GOOS)
		return nil
	}
}

// RemoveSystemResolver removes the system resolver configuration.
func RemoveSystemResolver() error {
	switch runtime.GOOS {
	case "darwin":
		return removeMacOSResolver()
	case "linux":
		return removeLinuxResolver()
	default:
		return nil
	}
}

func configureMacOSResolver(port int) error {
	resolverDir := "/etc/resolver"

	// Create resolver directory (requires sudo)
	cmd := exec.Command("sudo", "mkdir", "-p", resolverDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create resolver directory: %w", err)
	}

	// Write resolver file
	content := fmt.Sprintf("nameserver 127.0.0.1\nport %d\n", port)
	resolverFile := resolverDir + "/" + Domain

	// Write via sudo tee
	cmd = exec.Command("sudo", "tee", resolverFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Use a pipe for the content
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tee: %w", err)
	}

	pipe.Write([]byte(content))
	pipe.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to write resolver file: %w", err)
	}

	slog.Info("macOS resolver configured", "file", resolverFile, "port", port)
	return nil
}

func removeMacOSResolver() error {
	resolverFile := "/etc/resolver/" + Domain
	if _, err := os.Stat(resolverFile); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("sudo", "rm", "-f", resolverFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove resolver file: %w", err)
	}

	slog.Info("macOS resolver removed", "file", resolverFile)
	return nil
}

func configureLinuxResolver(port int) error {
	// Use systemd-resolved drop-in config
	dropinDir := "/etc/systemd/resolved.conf.d"

	cmd := exec.Command("sudo", "mkdir", "-p", dropinDir)
	if err := cmd.Run(); err != nil {
		slog.Warn("could not create resolved.conf.d (systemd-resolved may not be in use)", "error", err)
		return nil
	}

	content := fmt.Sprintf("[Resolve]\nDNS=127.0.0.1:%d\nDomains=~%s\n", port, Domain)
	dropinFile := dropinDir + "/vibespace.conf"

	cmd = exec.Command("sudo", "tee", dropinFile)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tee: %w", err)
	}

	pipe.Write([]byte(content))
	pipe.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to write resolver config: %w", err)
	}

	// Restart systemd-resolved to pick up changes
	exec.Command("sudo", "systemctl", "restart", "systemd-resolved").Run()

	slog.Info("linux resolver configured", "file", dropinFile, "port", port)
	return nil
}

func removeLinuxResolver() error {
	dropinFile := "/etc/systemd/resolved.conf.d/vibespace.conf"
	if _, err := os.Stat(dropinFile); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("sudo", "rm", "-f", dropinFile)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove resolver config: %w", err)
	}

	// Restart systemd-resolved
	exec.Command("sudo", "systemctl", "restart", "systemd-resolved").Run()

	slog.Info("linux resolver removed", "file", dropinFile)
	return nil
}

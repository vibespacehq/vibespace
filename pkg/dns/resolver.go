package dns

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const hostsMarker = "# vibespace-managed"

// ErrSudoRequired is returned when sudo credentials are not cached
// and a password is needed.
var ErrSudoRequired = errors.New("sudo authentication required")

// AddHostEntry adds "127.0.0.1 <name>.vibespace.internal" to /etc/hosts.
// If sudoPass is empty, tries non-interactive sudo (cached credentials).
// If sudoPass is provided, pipes it via sudo -S.
// Returns ErrSudoRequired if cached credentials are unavailable and no password given.
func AddHostEntry(name, sudoPass string) error {
	if HasHostEntry(name) {
		return nil
	}
	fqdn := name + "." + Domain()
	line := fmt.Sprintf("127.0.0.1 %s %s", fqdn, hostsMarker)

	if err := runSudo(sudoPass, "bash", "-c", fmt.Sprintf("echo '%s' >> /etc/hosts", line)); err != nil {
		return err
	}
	slog.Info("added /etc/hosts entry", "name", fqdn)
	return nil
}

// RemoveHostEntry removes a vibespace-managed entry from /etc/hosts.
func RemoveHostEntry(name, sudoPass string) error {
	if !HasHostEntry(name) {
		return nil
	}
	fqdn := name + "." + Domain()

	if err := runSudo(sudoPass, "sed", "-i", "", fmt.Sprintf("/%s.*%s/d", fqdn, hostsMarker), "/etc/hosts"); err != nil {
		return err
	}
	slog.Info("removed /etc/hosts entry", "name", fqdn)
	return nil
}

// RemoveAllHostEntries removes all vibespace-managed entries from /etc/hosts.
func RemoveAllHostEntries(sudoPass string) error {
	if !hasAnyHostEntries() {
		return nil
	}
	if err := runSudo(sudoPass, "sed", "-i", "", fmt.Sprintf("/%s/d", hostsMarker), "/etc/hosts"); err != nil {
		return err
	}
	slog.Info("removed all vibespace entries from /etc/hosts")
	return nil
}

// HasHostEntry checks if a vibespace-managed entry exists for the given name.
func HasHostEntry(name string) bool {
	fqdn := name + "." + Domain()
	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, fqdn) && strings.Contains(line, hostsMarker) {
			return true
		}
	}
	return false
}

func hasAnyHostEntries() bool {
	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), hostsMarker)
}

// runSudo runs a command with sudo. Empty password tries sudo -n (cached creds).
// Non-empty password uses sudo -S (piped stdin).
func runSudo(password string, args ...string) error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		slog.Warn("hosts entry management not supported", "os", runtime.GOOS)
		return nil
	}

	if password != "" {
		cmd := exec.Command("sudo", append([]string{"-S"}, args...)...)
		cmd.Stdin = strings.NewReader(password + "\n")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sudo failed: %w", err)
		}
		return nil
	}

	// Try non-interactive (cached credentials)
	cmd := exec.Command("sudo", append([]string{"-n"}, args...)...)
	if err := cmd.Run(); err != nil {
		return ErrSudoRequired
	}
	return nil
}

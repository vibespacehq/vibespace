package remote

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"golang.org/x/crypto/curve25519"
)

// WireGuard interface name
const (
	WGInterfaceName = "wg-vibespace"
)

// KeyPair represents a WireGuard key pair.
type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateKeyPair generates a new WireGuard key pair.
func GenerateKeyPair() (*KeyPair, error) {
	// Generate 32 random bytes for private key
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Clamp the private key according to Curve25519 requirements
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derive public key
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	return &KeyPair{
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey[:]),
	}, nil
}

// ClientConfig represents the configuration for a WireGuard client.
type ClientConfig struct {
	PrivateKey      string
	Address         string // e.g., "10.100.0.2/32"
	ServerPublicKey string
	ServerEndpoint  string // e.g., "vps.example.com:51820"
	ServerIP        string // e.g., "10.100.0.1"
}

// ServerConfig represents the configuration for a WireGuard server.
type ServerConfig struct {
	PrivateKey string
	Address    string // e.g., "10.100.0.1/24"
	ListenPort int
	Clients    []ServerClientConfig
}

// ServerClientConfig represents a client in the server configuration.
type ServerClientConfig struct {
	PublicKey  string
	AllowedIPs string // e.g., "10.100.0.2/32"
}

// Client config template
const clientConfigTemplate = `[Interface]
PrivateKey = {{.PrivateKey}}
Address = {{.Address}}

[Peer]
PublicKey = {{.ServerPublicKey}}
Endpoint = {{.ServerEndpoint}}
AllowedIPs = {{.ServerIP}}/32
PersistentKeepalive = 25
`

// Server config template
const serverConfigTemplate = `[Interface]
PrivateKey = {{.PrivateKey}}
Address = {{.Address}}
ListenPort = {{.ListenPort}}
{{range .Clients}}
[Peer]
PublicKey = {{.PublicKey}}
AllowedIPs = {{.AllowedIPs}}
{{end}}`

// getBinDir returns the path to the vibespace bin directory.
func getBinDir() (string, error) {
	vsHome, err := getVibespaceHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(vsHome, "bin"), nil
}

// wgBin returns the path to the bundled wg binary.
func wgBin() (string, error) {
	binDir, err := getBinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(binDir, "wg"), nil
}

// wgQuickBin returns the path to the bundled wg-quick script.
func wgQuickBin() (string, error) {
	binDir, err := getBinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(binDir, "wg-quick"), nil
}

// wireguardGoBin returns the path to the bundled wireguard-go binary (macOS only).
func wireguardGoBin() (string, error) {
	binDir, err := getBinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(binDir, "wireguard-go"), nil
}

// WriteClientConfig writes a WireGuard client configuration file.
// Returns the path to the temp config file that should be installed with InstallConfig.
func WriteClientConfig(config *ClientConfig) (string, error) {
	tmpl, err := template.New("client").Parse(clientConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse client template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return "", fmt.Errorf("failed to execute client template: %w", err)
	}

	// Write to a temp file in vibespace home
	vsHome, err := getVibespaceHome()
	if err != nil {
		return "", err
	}

	tempPath := filepath.Join(vsHome, WGInterfaceName+".conf")
	if err := os.WriteFile(tempPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write temp config: %w", err)
	}

	return tempPath, nil
}

// WriteServerConfig writes a WireGuard server configuration file.
func WriteServerConfig(config *ServerConfig) (string, error) {
	tmpl, err := template.New("server").Parse(serverConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse server template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return "", fmt.Errorf("failed to execute server template: %w", err)
	}

	// First write to a temp file in vibespace home
	vsHome, err := getVibespaceHome()
	if err != nil {
		return "", err
	}

	tempPath := filepath.Join(vsHome, WGInterfaceName+".conf")
	if err := os.WriteFile(tempPath, buf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("failed to write temp config: %w", err)
	}

	return tempPath, nil
}

// InstallConfig copies the config to /etc/wireguard (requires sudo).
func InstallConfig(tempPath string) error {
	destPath := fmt.Sprintf("/etc/wireguard/%s.conf", WGInterfaceName)

	// Ensure /etc/wireguard exists
	cmd := exec.Command("sudo", "mkdir", "-p", "/etc/wireguard")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create /etc/wireguard: %w", err)
	}

	// Use sudo to copy the file
	cmd = exec.Command("sudo", "cp", tempPath, destPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install WireGuard config (sudo required): %w", err)
	}

	// Set proper permissions
	cmd = exec.Command("sudo", "chmod", "600", destPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set config permissions: %w", err)
	}

	return nil
}

// QuickUp brings up the WireGuard interface using wg-quick.
func QuickUp() error {
	wgQuick, err := wgQuickBin()
	if err != nil {
		return err
	}

	// Build PATH to include our bin directory and wireguard-go
	binDir, _ := getBinDir()
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", binDir, currentPath)

	cmd := exec.Command("sudo", "-E", wgQuick, "up", WGInterfaceName)
	cmd.Env = append(os.Environ(), "PATH="+newPath, "WG_QUICK_USERSPACE_IMPLEMENTATION=wireguard-go")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring up WireGuard interface: %w", err)
	}
	return nil
}

// QuickDown brings down the WireGuard interface using wg-quick.
func QuickDown() error {
	wgQuick, err := wgQuickBin()
	if err != nil {
		return err
	}

	// Build PATH to include our bin directory
	binDir, _ := getBinDir()
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", binDir, currentPath)

	cmd := exec.Command("sudo", "-E", wgQuick, "down", WGInterfaceName)
	cmd.Env = append(os.Environ(), "PATH="+newPath, "WG_QUICK_USERSPACE_IMPLEMENTATION=wireguard-go")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Ignore error if interface doesn't exist
		if strings.Contains(err.Error(), "is not a WireGuard interface") {
			return nil
		}
		return fmt.Errorf("failed to bring down WireGuard interface: %w", err)
	}
	return nil
}

// IsInterfaceUp checks if the WireGuard interface is up.
func IsInterfaceUp() bool {
	wg, err := wgBin()
	if err != nil {
		return false
	}

	cmd := exec.Command("sudo", wg, "show", WGInterfaceName)
	err = cmd.Run()
	return err == nil
}

// IsWireGuardInstalled checks if WireGuard tools are installed in the bundled location.
func IsWireGuardInstalled() bool {
	wg, err := wgBin()
	if err != nil {
		return false
	}
	wgQuick, err := wgQuickBin()
	if err != nil {
		return false
	}

	if _, err := os.Stat(wg); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(wgQuick); os.IsNotExist(err) {
		return false
	}

	// On macOS, also check for wireguard-go
	if runtime.GOOS == "darwin" {
		wgGo, err := wireguardGoBin()
		if err != nil {
			return false
		}
		if _, err := os.Stat(wgGo); os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// IsWireGuardAvailable checks if WireGuard tools are available (installed and ready to use).
func IsWireGuardAvailable() bool {
	return IsWireGuardInstalled()
}

// InstallWireGuard downloads and installs WireGuard tools to the bundled location.
func InstallWireGuard(ctx context.Context) error {
	binDir, err := getBinDir()
	if err != nil {
		return err
	}

	// Ensure bin directory exists
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return installWireGuardMacOS(ctx, binDir)
	case "linux":
		return installWireGuardLinux(ctx, binDir)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// HomebrewFormula represents the Homebrew formula API response.
type HomebrewFormula struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Bottle  struct {
		Stable struct {
			Files map[string]struct {
				URL    string `json:"url"`
				SHA256 string `json:"sha256"`
			} `json:"files"`
		} `json:"stable"`
	} `json:"bottle"`
}

// installWireGuardMacOS installs WireGuard tools on macOS from Homebrew bottles.
func installWireGuardMacOS(ctx context.Context, binDir string) error {
	arch := runtime.GOARCH

	// Install wireguard-tools
	if err := downloadHomebrewBottle(ctx, "wireguard-tools", arch, binDir, []string{"wg", "wg-quick"}); err != nil {
		return fmt.Errorf("failed to install wireguard-tools: %w", err)
	}

	// Install wireguard-go (userspace implementation for macOS)
	if err := downloadHomebrewBottle(ctx, "wireguard-go", arch, binDir, []string{"wireguard-go"}); err != nil {
		return fmt.Errorf("failed to install wireguard-go: %w", err)
	}

	return nil
}

// macOSVersions maps macOS major version numbers to Homebrew bottle names.
// Order matters: index 0 is newest.
var macOSVersions = []struct {
	major int
	name  string
}{
	{26, "tahoe"},    // macOS 26
	{15, "sequoia"},  // macOS 15
	{14, "sonoma"},   // macOS 14
	{13, "ventura"},  // macOS 13
	{12, "monterey"}, // macOS 12
	{11, "big_sur"},  // macOS 11
}

// getMacOSVersion returns the major macOS version number.
func getMacOSVersion() (int, error) {
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get macOS version: %w", err)
	}

	// Parse version like "14.2.1" or "15.0"
	version := strings.TrimSpace(string(output))
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid version format: %s", version)
	}

	var major int
	if _, err := fmt.Sscanf(parts[0], "%d", &major); err != nil {
		return 0, fmt.Errorf("failed to parse major version: %w", err)
	}

	return major, nil
}

// getBottleNameForMacOS returns the Homebrew bottle name for the current macOS version and architecture.
func getBottleNameForMacOS(goArch string) (string, error) {
	major, err := getMacOSVersion()
	if err != nil {
		return "", err
	}

	// Find the bottle name for our version or the closest older version
	for _, v := range macOSVersions {
		if v.major <= major {
			if goArch == "arm64" {
				return "arm64_" + v.name, nil
			}
			return v.name, nil
		}
	}

	// Fallback to oldest known version
	oldest := macOSVersions[len(macOSVersions)-1]
	if goArch == "arm64" {
		return "arm64_" + oldest.name, nil
	}
	return oldest.name, nil
}

// downloadHomebrewBottle downloads a Homebrew bottle and extracts specific binaries.
// goArch should be "arm64" or "amd64"
func downloadHomebrewBottle(ctx context.Context, formula, goArch, destDir string, binaries []string) error {
	// Get formula info from Homebrew API
	apiURL := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", formula)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch formula info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch formula info: status %d", resp.StatusCode)
	}

	var formulaInfo HomebrewFormula
	if err := json.NewDecoder(resp.Body).Decode(&formulaInfo); err != nil {
		return fmt.Errorf("failed to parse formula info: %w", err)
	}

	// Get the bottle name for our OS version and architecture
	bottleName, err := getBottleNameForMacOS(goArch)
	if err != nil {
		return fmt.Errorf("failed to determine bottle name: %w", err)
	}

	// Look for exact match first
	bottle, ok := formulaInfo.Bottle.Stable.Files[bottleName]
	if !ok {
		// Try older versions as fallback
		major, _ := getMacOSVersion()
		for _, v := range macOSVersions {
			if v.major <= major {
				name := v.name
				if goArch == "arm64" {
					name = "arm64_" + name
				}
				if b, found := formulaInfo.Bottle.Stable.Files[name]; found {
					bottle = b
					break
				}
			}
		}
	}
	if bottle.URL == "" {
		return fmt.Errorf("no bottle found for %s", bottleName)
	}

	// Download the bottle
	req, err = http.NewRequestWithContext(ctx, "GET", bottle.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	// Homebrew bottles require this header
	req.Header.Set("Authorization", "Bearer QQ==")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download bottle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download bottle: status %d", resp.StatusCode)
	}

	// Extract the binaries we need
	return extractHomebrewBottle(resp.Body, formula, destDir, binaries)
}

// extractHomebrewBottle extracts specific binaries from a Homebrew bottle (tar.gz).
func extractHomebrewBottle(r io.Reader, formula, destDir string, binaries []string) error {
	// Homebrew bottles are gzipped tarballs
	// Structure: <formula>/<version>/bin/<binary>

	// Create a temp file to store the download
	tmpFile, err := os.CreateTemp("", "vibespace-bottle-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, r); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Extract using tar command (simpler than Go's archive/tar for nested gzip)
	// First, let's list what's in there to find the right paths
	listCmd := exec.Command("tar", "-tzf", tmpPath)
	output, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list bottle contents: %w", err)
	}

	// Find and extract each binary
	for _, binary := range binaries {
		// Look for the binary in the listing
		var binaryPath string
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasSuffix(line, "/bin/"+binary) || strings.HasSuffix(line, "/sbin/"+binary) {
				binaryPath = line
				break
			}
		}
		if binaryPath == "" {
			return fmt.Errorf("binary %s not found in bottle", binary)
		}

		// Extract just this file
		extractCmd := exec.Command("tar", "-xzf", tmpPath, "-C", destDir, "--strip-components=3", binaryPath)
		if err := extractCmd.Run(); err != nil {
			// Try with different strip count
			extractCmd = exec.Command("tar", "-xzf", tmpPath, "-C", destDir, "--strip-components=2", binaryPath)
			if err := extractCmd.Run(); err != nil {
				return fmt.Errorf("failed to extract %s: %w", binary, err)
			}
		}

		// Ensure it's executable
		destPath := filepath.Join(destDir, binary)
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("failed to chmod %s: %w", binary, err)
		}
	}

	return nil
}

// installWireGuardLinux installs WireGuard tools on Linux.
// Downloads from Homebrew Linux bottles.
func installWireGuardLinux(ctx context.Context, binDir string) error {
	arch := runtime.GOARCH

	// Download wireguard-tools from Homebrew Linux bottles
	if err := downloadHomebrewBottleLinux(ctx, "wireguard-tools", arch, binDir, []string{"wg", "wg-quick"}); err != nil {
		return fmt.Errorf("failed to install wireguard-tools: %w", err)
	}

	return nil
}

// downloadHomebrewBottleLinux downloads a Homebrew Linux bottle.
func downloadHomebrewBottleLinux(ctx context.Context, formula, goArch, destDir string, binaries []string) error {
	// Get formula info from Homebrew API
	apiURL := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", formula)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch formula info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch formula info: status %d", resp.StatusCode)
	}

	var formulaInfo HomebrewFormula
	if err := json.NewDecoder(resp.Body).Decode(&formulaInfo); err != nil {
		return fmt.Errorf("failed to parse formula info: %w", err)
	}

	// Determine bottle name for Linux
	bottleName := "x86_64_linux"
	if goArch == "arm64" {
		bottleName = "arm64_linux"
	}

	bottle, ok := formulaInfo.Bottle.Stable.Files[bottleName]
	if !ok {
		return fmt.Errorf("no Linux bottle found for architecture %s", goArch)
	}

	// Download the bottle
	req, err = http.NewRequestWithContext(ctx, "GET", bottle.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer QQ==")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download bottle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download bottle: status %d", resp.StatusCode)
	}

	return extractHomebrewBottle(resp.Body, formula, destDir, binaries)
}

// GetWireGuardStatus returns the current WireGuard interface status.
func GetWireGuardStatus() (string, error) {
	wg, err := wgBin()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("sudo", wg, "show", WGInterfaceName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get WireGuard status: %w", err)
	}
	return string(output), nil
}

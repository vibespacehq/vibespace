package remote

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DiagnosticResult represents the result of a single diagnostic check.
type DiagnosticResult struct {
	Check   string `json:"check"`
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

// RunDiagnostics runs a series of diagnostic checks on the client connection.
func RunDiagnostics(state *RemoteState) []DiagnosticResult {
	var results []DiagnosticResult

	// 1. WireGuard Interface
	results = append(results, checkWireGuardInterface())

	// 2. UDP Connectivity (try connecting to server endpoint)
	if state.ServerEndpoint != "" {
		results = append(results, checkUDPConnectivity(state.ServerEndpoint))
	}

	// 3. WireGuard Handshake
	results = append(results, checkWireGuardHandshake())

	// 4. Management API
	if state.ServerIP != "" {
		results = append(results, checkManagementAPI(state.ServerIP))
	}

	// 5. Kubeconfig
	results = append(results, checkKubeconfig())

	return results
}

func checkWireGuardInterface() DiagnosticResult {
	if IsInterfaceUp() {
		return DiagnosticResult{
			Check:   "WireGuard Interface",
			Status:  true,
			Message: "Interface is up",
		}
	}
	return DiagnosticResult{
		Check:   "WireGuard Interface",
		Status:  false,
		Message: "Interface is down. Try: vibespace remote disconnect && vibespace remote connect <token>",
	}
}

func checkUDPConnectivity(endpoint string) DiagnosticResult {
	conn, err := net.DialTimeout("udp", endpoint, 3*time.Second)
	if err != nil {
		return DiagnosticResult{
			Check:   "UDP Connectivity",
			Status:  false,
			Message: fmt.Sprintf("Cannot reach %s: %v", endpoint, err),
		}
	}
	conn.Close()
	return DiagnosticResult{
		Check:   "UDP Connectivity",
		Status:  true,
		Message: fmt.Sprintf("Can reach %s", endpoint),
	}
}

func checkWireGuardHandshake() DiagnosticResult {
	wg, err := wgBin()
	if err != nil {
		return DiagnosticResult{
			Check:   "WireGuard Handshake",
			Status:  false,
			Message: "wg binary not found",
		}
	}

	ifName := wireguardInterfaceName()
	out, err := exec.Command("sudo", wg, "show", ifName, "latest-handshakes").Output()
	if err != nil {
		return DiagnosticResult{
			Check:   "WireGuard Handshake",
			Status:  false,
			Message: "Could not read handshake info (interface may be down)",
		}
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return DiagnosticResult{
			Check:   "WireGuard Handshake",
			Status:  false,
			Message: "No handshake yet. Check that the server is running and UDP port 51820 is reachable",
		}
	}

	// Parse timestamp - format is "<pubkey>\t<unix-timestamp>"
	parts := strings.Fields(output)
	if len(parts) >= 2 && parts[1] != "0" {
		var ts int64
		fmt.Sscanf(parts[1], "%d", &ts)
		if ts > 0 {
			handshakeTime := time.Unix(ts, 0)
			ago := time.Since(handshakeTime).Round(time.Second)
			return DiagnosticResult{
				Check:   "WireGuard Handshake",
				Status:  true,
				Message: fmt.Sprintf("Last handshake %s ago", ago),
			}
		}
	}

	return DiagnosticResult{
		Check:   "WireGuard Handshake",
		Status:  false,
		Message: "No successful handshake. Ensure server is running and firewall allows UDP 51820",
	}
}

func checkManagementAPI(serverIP string) DiagnosticResult {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/health", serverIP, DefaultManagementPort))
	if err != nil {
		return DiagnosticResult{
			Check:   "Management API",
			Status:  false,
			Message: fmt.Sprintf("Cannot reach management API at %s:%d: %v", serverIP, DefaultManagementPort, err),
		}
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return DiagnosticResult{
			Check:   "Management API",
			Status:  true,
			Message: "Server API is reachable",
		}
	}
	return DiagnosticResult{
		Check:   "Management API",
		Status:  false,
		Message: fmt.Sprintf("API returned status %d", resp.StatusCode),
	}
}

func checkKubeconfig() DiagnosticResult {
	path, err := GetRemoteKubeconfigPath()
	if err != nil {
		return DiagnosticResult{
			Check:   "Kubeconfig",
			Status:  false,
			Message: "Could not determine kubeconfig path",
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return DiagnosticResult{
			Check:   "Kubeconfig",
			Status:  false,
			Message: "Kubeconfig file not found. Reconnect to fetch it",
		}
	}

	if info.Size() == 0 {
		return DiagnosticResult{
			Check:   "Kubeconfig",
			Status:  false,
			Message: "Kubeconfig file is empty",
		}
	}

	return DiagnosticResult{
		Check:   "Kubeconfig",
		Status:  true,
		Message: fmt.Sprintf("Kubeconfig exists (%d bytes)", info.Size()),
	}
}

// CheckFirewall attempts to detect if required ports are blocked.
// Called during server startup to provide actionable guidance.
func CheckFirewall() []DiagnosticResult {
	var results []DiagnosticResult

	// Check UDP 51820 (WireGuard)
	results = append(results, checkPortBindable("udp", 51820, "WireGuard"))

	// Check TCP 7781 (Registration API)
	results = append(results, checkPortBindable("tcp", DefaultRegistrationPort, "Registration API"))

	return results
}

func checkPortBindable(proto string, port int, name string) DiagnosticResult {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen(proto, addr)
	if err != nil {
		// For UDP, use ListenPacket
		if proto == "udp" {
			pc, udpErr := net.ListenPacket("udp", addr)
			if udpErr != nil {
				return DiagnosticResult{
					Check:  fmt.Sprintf("Port %d/%s (%s)", port, strings.ToUpper(proto), name),
					Status: false,
					Message: fmt.Sprintf("Cannot bind %s. Check:\n"+
						"  - ufw: sudo ufw allow %d/%s\n"+
						"  - iptables: sudo iptables -A INPUT -p %s --dport %d -j ACCEPT\n"+
						"  - firewalld: sudo firewall-cmd --add-port=%d/%s --permanent\n"+
						"  - Cloud: check security group / firewall rules",
						addr, port, proto, proto, port, port, proto),
				}
			}
			pc.Close()
			return DiagnosticResult{
				Check:   fmt.Sprintf("Port %d/%s (%s)", port, strings.ToUpper(proto), name),
				Status:  true,
				Message: fmt.Sprintf("Port %d/%s is available", port, strings.ToUpper(proto)),
			}
		}
		return DiagnosticResult{
			Check:  fmt.Sprintf("Port %d/%s (%s)", port, strings.ToUpper(proto), name),
			Status: false,
			Message: fmt.Sprintf("Cannot bind %s. Check:\n"+
				"  - ufw: sudo ufw allow %d/%s\n"+
				"  - iptables: sudo iptables -A INPUT -p %s --dport %d -j ACCEPT\n"+
				"  - firewalld: sudo firewall-cmd --add-port=%d/%s --permanent\n"+
				"  - Cloud: check security group / firewall rules",
				addr, port, proto, proto, port, port, proto),
		}
	}
	ln.Close()
	return DiagnosticResult{
		Check:   fmt.Sprintf("Port %d/%s (%s)", port, strings.ToUpper(proto), name),
		Status:  true,
		Message: fmt.Sprintf("Port %d/%s is available", port, strings.ToUpper(proto)),
	}
}

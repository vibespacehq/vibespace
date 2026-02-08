package remote

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
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
		results = append(results, checkManagementAPI(state))
	}

	// 5. Kubeconfig
	results = append(results, checkKubeconfig())

	return results
}

func checkWireGuardInterface() DiagnosticResult {
	switch InterfaceStatus() {
	case "up":
		return DiagnosticResult{
			Check:   "WireGuard Interface",
			Status:  true,
			Message: "Interface is up",
		}
	case "unknown":
		return DiagnosticResult{
			Check:   "WireGuard Interface",
			Status:  true,
			Message: "Interface status unknown (needs sudo to verify). Assuming up",
		}
	default:
		return DiagnosticResult{
			Check:   "WireGuard Interface",
			Status:  false,
			Message: "Interface is down. Try: vibespace remote disconnect && vibespace remote connect <token>",
		}
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

func checkManagementAPI(state *RemoteState) DiagnosticResult {
	client := mgmtHTTPClient(3 * time.Second)
	resp, err := client.Get(fmt.Sprintf("https://%s:%d/health", state.ServerIP, DefaultManagementPort))
	if err != nil {
		return DiagnosticResult{
			Check:   "Management API",
			Status:  false,
			Message: fmt.Sprintf("Cannot reach management API at %s:%d: %v", state.ServerIP, DefaultManagementPort, err),
		}
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		return DiagnosticResult{
			Check:   "Management API",
			Status:  true,
			Message: "Server API is reachable (HTTPS)",
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

	// On Linux, check firewall rules for required ports
	if runtime.GOOS == "linux" {
		results = append(results, checkFirewallRules()...)
	}

	return results
}

// checkFirewallRules checks ufw/iptables/firewalld for required port rules (Linux only).
func checkFirewallRules() []DiagnosticResult {
	var results []DiagnosticResult

	// Check ufw
	if _, err := exec.LookPath("ufw"); err == nil {
		out, err := exec.Command("sudo", "-n", "ufw", "status").Output()
		if err == nil {
			status := string(out)
			if strings.Contains(status, "Status: active") {
				wgOpen := strings.Contains(status, "51820") || strings.Contains(status, "WireGuard")
				regOpen := strings.Contains(status, fmt.Sprintf("%d", DefaultRegistrationPort))
				if !wgOpen {
					results = append(results, DiagnosticResult{
						Check:   "UFW: WireGuard",
						Status:  false,
						Message: "ufw is active but port 51820/udp not found. Run: sudo ufw allow 51820/udp",
					})
				}
				if !regOpen {
					results = append(results, DiagnosticResult{
						Check:   "UFW: Registration",
						Status:  false,
						Message: fmt.Sprintf("ufw is active but port %d/tcp not found. Run: sudo ufw allow %d/tcp", DefaultRegistrationPort, DefaultRegistrationPort),
					})
				}
			}
		}
	}

	// Check iptables
	if _, err := exec.LookPath("iptables"); err == nil {
		out, err := exec.Command("sudo", "-n", "iptables", "-L", "-n").Output()
		if err == nil {
			rules := string(out)
			if !strings.Contains(rules, "51820") && !strings.Contains(rules, "ACCEPT     all") {
				results = append(results, DiagnosticResult{
					Check:   "iptables: WireGuard",
					Status:  false,
					Message: "No iptables rule found for port 51820. Run: sudo iptables -A INPUT -p udp --dport 51820 -j ACCEPT",
				})
			}
		}
	}

	// Check firewalld
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		out, err := exec.Command("sudo", "-n", "firewall-cmd", "--list-ports").Output()
		if err == nil {
			ports := string(out)
			if !strings.Contains(ports, "51820/udp") {
				results = append(results, DiagnosticResult{
					Check:   "firewalld: WireGuard",
					Status:  false,
					Message: "firewalld does not allow 51820/udp. Run: sudo firewall-cmd --add-port=51820/udp --permanent && sudo firewall-cmd --reload",
				})
			}
		}
	}

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

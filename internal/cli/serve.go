package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yagizdagabak/vibespace/pkg/remote"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the remote mode server",
	Long: `Start the vibespace remote mode server.

This enables other machines to connect to this cluster via WireGuard tunnel.
Run on a VPS or any machine that has a public IP address.

Prerequisites:
  - The cluster must be initialized (vibespace init)
  - Port 51820/UDP must be open for WireGuard

The server exposes:
  - WireGuard VPN on port 51820/UDP (public)
  - Management API on port 7780/TCP (WireGuard network only)`,
	Example: `  # Start the server
  vibespace serve

  # Generate an invite token for a client
  vibespace serve --generate-token --endpoint your-server.com

  # List registered clients
  vibespace serve --list-clients

  # Remove a client
  vibespace serve --remove-client <name-or-key>`,
	RunE: runServe,
}

var (
	serveGenerateToken bool
	serveEndpoint      string
	serveAddClient     string
	serveForeground    bool
	serveTokenTTL      time.Duration
	serveListClients   bool
	serveRemoveClient  string
)

func init() {
	serveCmd.Flags().BoolVar(&serveGenerateToken, "generate-token", false, "Generate an invite token for a client")
	serveCmd.Flags().StringVar(&serveEndpoint, "endpoint", "", "Public endpoint for clients (override auto-detection)")
	serveCmd.Flags().StringVar(&serveAddClient, "add-client", "", "Add a client by their WireGuard public key")
	serveCmd.Flags().BoolVar(&serveForeground, "foreground", false, "Run in foreground (don't daemonize)")
	serveCmd.Flags().DurationVar(&serveTokenTTL, "token-ttl", remote.DefaultInviteTokenTTL, "Invite token time-to-live (e.g. 15m, 1h)")
	serveCmd.Flags().BoolVar(&serveListClients, "list-clients", false, "List all registered clients")
	serveCmd.Flags().StringVar(&serveRemoveClient, "remove-client", "", "Remove a client by name, hostname, or public key")
}

func runServe(cmd *cobra.Command, args []string) error {
	out := getOutput()
	ctx := context.Background()

	// Install WireGuard if needed
	if !remote.IsWireGuardInstalled() {
		printStep("Installing WireGuard...")
		if err := remote.InstallWireGuard(ctx); err != nil {
			return fmt.Errorf("failed to install WireGuard: %w", err)
		}
		printSuccess("WireGuard installed")
	}

	// Create server
	server, err := remote.NewServer()
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Handle --list-clients flag
	if serveListClients {
		clients := server.ListClients()
		if out.IsJSONMode() {
			var clientOutputs []ClientOutput
			for _, c := range clients {
				clientOutputs = append(clientOutputs, ClientOutput{
					Name:         c.Name,
					PublicKey:    c.PublicKey,
					AssignedIP:   c.AssignedIP,
					Hostname:     c.Hostname,
					RegisteredAt: c.RegisteredAt.Format("2006-01-02 15:04:05"),
				})
			}
			return out.JSON(NewJSONOutput(true, ClientListOutput{
				Clients: clientOutputs,
				Count:   len(clientOutputs),
			}, nil))
		}

		if len(clients) == 0 {
			fmt.Println("No registered clients")
			return nil
		}

		fmt.Printf("Registered clients (%d):\n\n", len(clients))
		for _, c := range clients {
			name := c.Name
			if c.Hostname != "" && c.Hostname != c.Name {
				name = fmt.Sprintf("%s (%s)", c.Name, c.Hostname)
			}
			fmt.Printf("  %s\n", out.Teal(name))
			fmt.Printf("    IP: %s\n", c.AssignedIP)
			fmt.Printf("    Key: %s...%s\n", c.PublicKey[:8], c.PublicKey[len(c.PublicKey)-4:])
			fmt.Printf("    Registered: %s\n", c.RegisteredAt.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
		return nil
	}

	// Handle --remove-client flag
	if serveRemoveClient != "" {
		if err := server.RemoveClient(serveRemoveClient); err != nil {
			return fmt.Errorf("failed to remove client: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, map[string]string{
				"removed": serveRemoveClient,
			}, nil))
		}

		printSuccess("Client removed")
		return nil
	}

	// Handle --add-client flag
	if serveAddClient != "" {
		assignedIP, err := server.AddClient("client", serveAddClient)
		if err != nil {
			return fmt.Errorf("failed to add client: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, map[string]string{
				"assigned_ip": assignedIP,
				"public_key":  serveAddClient,
			}, nil))
		}

		printSuccess("Client added")
		fmt.Printf("Assigned IP: %s\n", out.Teal(assignedIP))
		fmt.Println()
		fmt.Println("Give this IP to the client. They need to run:")
		fmt.Printf("  vibespace remote activate %s\n", assignedIP)
		return nil
	}

	// Handle --generate-token flag
	if serveGenerateToken {
		endpoint := serveEndpoint
		if endpoint == "" {
			// Auto-detect public IP
			detectedIP, err := remote.DetectPublicIP()
			if err != nil {
				return fmt.Errorf("failed to detect public IP (use --endpoint to specify manually): %w", err)
			}
			endpoint = detectedIP
		}

		// Add default port if not specified
		endpoint = ensureEndpointPort(endpoint, remote.DefaultWireGuardPort)
		if endpoint == "" {
			return fmt.Errorf("invalid endpoint")
		}

		token, err := server.GenerateInviteToken(endpoint, serveTokenTTL)
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, ServeTokenOutput{
				Token:     token,
				ExpiresIn: serveTokenTTL.String(),
			}, nil))
		}

		expiresAt := time.Now().Add(serveTokenTTL).Format("2006-01-02 15:04:05")
		fmt.Printf("Invite token: %s\n", out.Teal(token))
		fmt.Printf("Expires at: %s\n", out.Dim(expiresAt))
		fmt.Println()
		fmt.Println("Give this token to the client:")
		fmt.Printf("  vibespace remote connect %s\n", token)
		return nil
	}

	// Check cluster is initialized
	if err := checkClusterInitialized(); err != nil {
		return err
	}

	if serveForeground {
		// Run in foreground - start server directly
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		slog.Info("starting remote mode server in foreground")
		if err := server.Start(ctx, true); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, ServeOutput{
				Running:    true,
				ListenPort: remote.DefaultWireGuardPort,
				ServerIP:   remote.DefaultServerIP,
			}, nil))
		}

		printSuccess("Remote server started")
		fmt.Printf("WireGuard: %s\n", out.Teal(fmt.Sprintf("0.0.0.0:%d/udp", remote.DefaultWireGuardPort)))
		fmt.Printf("Management API: %s\n", out.Teal(fmt.Sprintf("%s:%d", remote.DefaultServerIP, remote.DefaultManagementPort)))
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop")

		// Wait for interrupt
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		fmt.Println()
		printStep("Shutting down...")
		if err := server.Stop(ctx); err != nil {
			slog.Warn("error stopping server", "error", err)
		}
		printSuccess("Server stopped")
	} else {
		// Daemonize - spawn a detached process
		slog.Info("spawning remote mode server as daemon")
		if err := remote.SpawnServe(); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, ServeOutput{
				Running:    true,
				ListenPort: remote.DefaultWireGuardPort,
				ServerIP:   remote.DefaultServerIP,
			}, nil))
		}

		printSuccess("Remote server started in background")
		fmt.Println()
		fmt.Println("Generate a token for clients:")
		fmt.Printf("  vibespace serve --generate-token\n")
	}

	return nil
}

// containsPort checks if a string contains a port number.
func containsPort(s string) bool {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return true
		}
		if s[i] == ']' { // IPv6
			return false
		}
	}
	return false
}

func ensureEndpointPort(endpoint string, port int) string {
	if endpoint == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(endpoint); err == nil {
		return endpoint
	}
	if net.ParseIP(endpoint) != nil || strings.Contains(endpoint, ":") {
		return net.JoinHostPort(endpoint, strconv.Itoa(port))
	}
	if containsPort(endpoint) {
		return endpoint
	}
	return fmt.Sprintf("%s:%d", endpoint, port)
}

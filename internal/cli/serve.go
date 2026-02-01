package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

  # Add a client after they give you their public key
  vibespace serve --add-client <client-public-key>`,
	RunE: runServe,
}

var (
	serveGenerateToken bool
	serveEndpoint      string
	serveAddClient     string
	serveForeground    bool
)

func init() {
	serveCmd.Flags().BoolVar(&serveGenerateToken, "generate-token", false, "Generate an invite token for a client")
	serveCmd.Flags().StringVar(&serveEndpoint, "endpoint", "", "Public endpoint for clients (override auto-detection)")
	serveCmd.Flags().StringVar(&serveAddClient, "add-client", "", "Add a client by their WireGuard public key")
	serveCmd.Flags().BoolVar(&serveForeground, "foreground", false, "Run in foreground (don't daemonize)")
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
		if !containsPort(endpoint) {
			endpoint = fmt.Sprintf("%s:%d", endpoint, remote.DefaultWireGuardPort)
		}

		token, err := server.GenerateInviteToken(endpoint)
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, ServeTokenOutput{
				Token: token,
			}, nil))
		}

		fmt.Printf("Invite token: %s\n", out.Teal(token))
		fmt.Println()
		fmt.Println("Give this token to the client:")
		fmt.Printf("  vibespace remote connect %s\n", token)
		fmt.Println()
		fmt.Println("After the client runs that command, they will give you a public key.")
		fmt.Println("Add it with:")
		fmt.Printf("  vibespace serve --add-client %s\n", out.Dim("<client-public-key>"))
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

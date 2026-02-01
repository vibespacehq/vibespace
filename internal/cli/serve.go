package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
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
  - WireGuard must be installed (wg-quick command available)
  - The cluster must be initialized (vibespace init)
  - Port 51820/UDP must be open for WireGuard

The server exposes:
  - WireGuard VPN on port 51820/UDP (public)
  - Management API on port 7780/TCP (WireGuard network only)`,
	Example: `  # Start the server
  vibespace serve

  # Start in foreground mode (don't daemonize)
  vibespace serve --foreground

  # Generate a registration token for a client
  vibespace serve --generate-token

  # Generate token with custom TTL
  vibespace serve --generate-token --token-ttl 1h`,
	RunE: runServe,
}

var (
	serveGenerateToken bool
	serveTokenTTL      string
	serveForeground    bool
)

func init() {
	serveCmd.Flags().BoolVar(&serveGenerateToken, "generate-token", false, "Generate a client registration token")
	serveCmd.Flags().StringVar(&serveTokenTTL, "token-ttl", "30m", "Token TTL (e.g., 30m, 1h)")
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

	// Handle --generate-token flag
	if serveGenerateToken {
		ttl, err := time.ParseDuration(serveTokenTTL)
		if err != nil {
			return fmt.Errorf("invalid token TTL: %w", err)
		}

		token, err := server.GenerateToken(ttl)
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, ServeTokenOutput{
				Token:     token,
				ExpiresIn: serveTokenTTL,
			}, nil))
		}

		fmt.Printf("Registration token: %s\n", out.Teal(token))
		fmt.Printf("Expires in: %s\n", serveTokenTTL)
		fmt.Println()
		fmt.Println("Give this token to the client to connect:")
		fmt.Printf("  vibespace remote connect %s --token %s\n", out.Dim("<user@this-host>"), token)
		return nil
	}

	// Check cluster is initialized
	if err := checkClusterInitialized(); err != nil {
		return err
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("starting remote mode server")
	if err := server.Start(ctx, serveForeground); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, ServeOutput{
			Running:    true,
			ListenPort: remote.DefaultWireGuardPort,
			ServerIP:   remote.DefaultServerIP,
		}, nil))
	}

	if serveForeground {
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
		printSuccess("Remote server started in background")
		fmt.Println()
		fmt.Println("Generate a token for clients:")
		fmt.Printf("  vibespace serve --generate-token\n")
	}

	return nil
}

// Hidden command for SSH registration (called by client via SSH)
var remoteRegisterCmd = &cobra.Command{
	Use:    "_remote-register",
	Hidden: true,
	Short:  "Register a remote client (internal)",
	RunE:   runRemoteRegister,
}

var (
	registerPubKey string
	registerToken  string
	registerName   string
)

func init() {
	remoteRegisterCmd.Flags().StringVar(&registerPubKey, "pubkey", "", "Client's WireGuard public key")
	remoteRegisterCmd.Flags().StringVar(&registerToken, "token", "", "Registration token")
	remoteRegisterCmd.Flags().StringVar(&registerName, "name", "", "Client name")
	remoteRegisterCmd.MarkFlagRequired("pubkey")
	remoteRegisterCmd.MarkFlagRequired("token")
}

func runRemoteRegister(cmd *cobra.Command, args []string) error {
	resp, err := remote.RegisterClient(registerPubKey, registerToken, registerName)
	if err != nil {
		// Output error as text to stderr for SSH
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Output JSON response to stdout for client to parse
	return json.NewEncoder(os.Stdout).Encode(resp)
}

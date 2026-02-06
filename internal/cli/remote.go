package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yagizdagabak/vibespace/pkg/remote"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote server connections",
	Long: `Connect to and manage remote vibespace servers.

Remote mode allows you to use a vibespace cluster running on a VPS
or another machine. The connection uses WireGuard for secure tunneling.

Connection flow:
  1. Server admin runs: vibespace serve --generate-token
  2. You run: vibespace remote connect <token>
     That's it! The token contains everything needed to register,
     set up the tunnel, and fetch the kubeconfig automatically.`,
	Example: `  # Connect using an invite token (one step!)
  vibespace remote connect vs-eyJrIjoiYWJj...

  # Check connection status
  vibespace remote status

  # Disconnect
  vibespace remote disconnect`,
}

var remoteConnectCmd = &cobra.Command{
	Use:   "connect <token>",
	Short: "Connect to a remote server",
	Long: `Connect to a remote vibespace server using an invite token.

The invite token contains the server's public key, endpoint, and
certificate fingerprint. This command handles everything automatically:
registration, WireGuard tunnel setup, and kubeconfig fetching.`,
	Example: `  vibespace remote connect vs-eyJrIjoiYWJjMTIzIiwiZSI6InZwcy5leGFtcGxlLmNvbTo1MTgyMCJ9`,
	Args:    cobra.ExactArgs(1),
	RunE:    runRemoteConnect,
}

var remoteDisconnectCmd = &cobra.Command{
	Use:     "disconnect",
	Short:   "Disconnect from the remote server",
	Long:    `Disconnect from the current remote vibespace server and tear down the WireGuard tunnel.`,
	Example: `  vibespace remote disconnect`,
	RunE:    runRemoteDisconnect,
}

var remoteWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch and auto-reconnect the remote tunnel",
	Long: `Watch the remote WireGuard tunnel and automatically reconnect if it drops.

This command monitors the tunnel health and attempts to restore the connection
if connectivity is lost. Press Ctrl-C to stop watching.`,
	Example: `  vibespace remote watch`,
	RunE:    runRemoteWatch,
}

var remoteStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show remote connection status",
	Long:    `Display the current remote connection status and details.`,
	Example: `  vibespace remote status`,
	RunE:    runRemoteStatus,
}

func init() {
	remoteCmd.AddCommand(remoteConnectCmd)
	remoteCmd.AddCommand(remoteDisconnectCmd)
	remoteCmd.AddCommand(remoteStatusCmd)
	remoteCmd.AddCommand(remoteWatchCmd)
}

func runRemoteConnect(cmd *cobra.Command, args []string) error {
	out := getOutput()
	token := args[0]
	ctx := context.Background()

	// Install WireGuard if needed
	if !remote.IsWireGuardInstalled() {
		printStep("Installing WireGuard...")
		if err := remote.InstallWireGuard(ctx); err != nil {
			return fmt.Errorf("failed to install WireGuard: %w", err)
		}
		printSuccess("WireGuard installed")
	}

	printStep("Connecting to remote server...")

	opts := remote.ConnectOptions{
		Token: token,
	}

	if err := remote.Connect(opts); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Get status to show details
	state, err := remote.GetStatus()
	if err != nil {
		return err
	}

	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, RemoteConnectOutput{
			Connected:  true,
			ServerHost: state.ServerHost,
			LocalIP:    state.LocalIP,
			ServerIP:   state.ServerIP,
		}, nil))
	}

	printSuccess("Connected to remote server")
	fmt.Println()
	fmt.Printf("Server: %s\n", out.Teal(state.ServerHost))
	fmt.Printf("Local IP: %s\n", state.LocalIP)
	fmt.Printf("Server IP: %s\n", state.ServerIP)
	fmt.Println()
	fmt.Println("All vibespace commands will now use the remote cluster.")
	fmt.Println("To disconnect: vibespace remote disconnect")

	return nil
}

func runRemoteWatch(cmd *cobra.Command, args []string) error {
	out := getOutput()

	state, err := remote.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	if !state.Connected {
		return fmt.Errorf("not connected to any remote server: use 'vibespace remote connect <token>' first")
	}

	fmt.Printf("Watching tunnel to %s ...\n", out.Teal(state.ServerHost))
	fmt.Println("Press Ctrl-C to stop.")
	fmt.Println()

	watcher := remote.NewConnectionWatcher(state.ServerIP)

	watcher.OnDisconnect(func() {
		fmt.Printf("%s Tunnel lost, attempting reconnect...\n", out.Yellow("[!!]"))
	})
	watcher.OnReconnect(func() {
		fmt.Printf("%s Tunnel restored\n", out.Green("[ok]"))
	})

	watcher.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println()
	fmt.Println("Stopping watcher...")
	watcher.Stop()
	return nil
}

func runRemoteDisconnect(cmd *cobra.Command, args []string) error {
	out := getOutput()

	printStep("Disconnecting from remote server...")

	if err := remote.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, RemoteDisconnectOutput{
			Disconnected: true,
		}, nil))
	}

	printSuccess("Disconnected from remote server")
	fmt.Println()
	fmt.Println("Vibespace will now use the local cluster.")
	fmt.Println("To initialize local cluster: vibespace init")

	return nil
}

func runRemoteStatus(cmd *cobra.Command, args []string) error {
	out := getOutput()

	state, err := remote.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	tunnelUp := remote.IsInterfaceUp()

	// Run diagnostics if connected
	var diagnostics []remote.DiagnosticResult
	if state.Connected {
		diagnostics = remote.RunDiagnostics(state)
	}

	if out.IsJSONMode() {
		connectedAt := ""
		if !state.ConnectedAt.IsZero() {
			connectedAt = state.ConnectedAt.Format("2006-01-02 15:04:05")
		}
		var diagOutput []DiagnosticOutput
		for _, d := range diagnostics {
			diagOutput = append(diagOutput, DiagnosticOutput{
				Check:   d.Check,
				Status:  d.Status,
				Message: d.Message,
			})
		}
		return out.JSON(NewJSONOutput(true, RemoteStatusOutput{
			Connected:   state.Connected,
			ServerHost:  state.ServerHost,
			LocalIP:     state.LocalIP,
			ServerIP:    state.ServerIP,
			ConnectedAt: connectedAt,
			TunnelUp:    tunnelUp,
			Diagnostics: diagOutput,
		}, nil))
	}

	if !state.Connected && state.PublicKey == "" {
		fmt.Println("Not connected to any remote server")
		fmt.Println()
		fmt.Println("To connect: vibespace remote connect <token>")
		return nil
	}

	if !state.Connected && state.PublicKey != "" {
		fmt.Printf("Remote: %s\n", out.Yellow("pending"))
		fmt.Printf("Server: %s\n", state.ServerEndpoint)
		fmt.Println()
		fmt.Println("Connection setup incomplete. Try disconnecting and reconnecting:")
		fmt.Println("  vibespace remote disconnect")
		fmt.Println("  vibespace remote connect <token>")
		return nil
	}

	// Connection info
	fmt.Printf("Remote: %s\n", out.Teal("connected"))
	fmt.Printf("Server: %s\n", state.ServerHost)
	fmt.Printf("Local IP: %s\n", state.LocalIP)
	fmt.Printf("Server IP: %s\n", state.ServerIP)
	fmt.Printf("Connected at: %s\n", state.ConnectedAt.Format("2006-01-02 15:04:05"))

	if tunnelUp {
		fmt.Printf("Tunnel: %s\n", out.Green("active"))
	} else {
		fmt.Printf("Tunnel: %s\n", out.Yellow("interface down"))
	}

	// Diagnostics
	if len(diagnostics) > 0 {
		fmt.Println()
		fmt.Println("Diagnostics:")
		allPassed := true
		for _, d := range diagnostics {
			if d.Status {
				fmt.Printf("  %s %s: %s\n", out.Green("[ok]"), d.Check, d.Message)
			} else {
				fmt.Printf("  %s %s: %s\n", out.Yellow("[!!]"), d.Check, d.Message)
				allPassed = false
			}
		}
		fmt.Println()
		if allPassed {
			fmt.Printf("Health: %s\n", out.Green("all checks passed"))
		} else {
			fmt.Printf("Health: %s\n", out.Yellow("some checks failed"))
		}
	}

	return nil
}

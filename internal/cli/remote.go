package cli

import (
	"context"
	"fmt"

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

var remoteActivateCmd = &cobra.Command{
	Use:        "activate <assigned-ip>",
	Short:      "Deprecated: use 'remote connect' instead",
	Long:       `This command is deprecated. Use 'vibespace remote connect <token>' which handles registration and activation in one step.`,
	Args:       cobra.ExactArgs(1),
	RunE:       runRemoteActivate,
	Deprecated: "use 'vibespace remote connect <token>' instead",
}

var remoteDisconnectCmd = &cobra.Command{
	Use:     "disconnect",
	Short:   "Disconnect from the remote server",
	Long:    `Disconnect from the current remote vibespace server and tear down the WireGuard tunnel.`,
	Example: `  vibespace remote disconnect`,
	RunE:    runRemoteDisconnect,
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
	remoteCmd.AddCommand(remoteActivateCmd)
	remoteCmd.AddCommand(remoteDisconnectCmd)
	remoteCmd.AddCommand(remoteStatusCmd)
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

func runRemoteActivate(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("activate is deprecated: use 'vibespace remote connect <token>' which handles registration and activation automatically")
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

	if out.IsJSONMode() {
		connectedAt := ""
		if !state.ConnectedAt.IsZero() {
			connectedAt = state.ConnectedAt.Format("2006-01-02 15:04:05")
		}
		return out.JSON(NewJSONOutput(true, RemoteStatusOutput{
			Connected:   state.Connected,
			ServerHost:  state.ServerHost,
			LocalIP:     state.LocalIP,
			ServerIP:    state.ServerIP,
			ConnectedAt: connectedAt,
		}, nil))
	}

	if !state.Connected && state.PublicKey == "" {
		fmt.Println("Not connected to any remote server")
		fmt.Println()
		fmt.Println("To connect: vibespace remote connect <token>")
		return nil
	}

	if !state.Connected && state.PublicKey != "" {
		// Pending state - this shouldn't happen with the new flow, but handle gracefully
		fmt.Printf("Remote: %s\n", out.Yellow("pending"))
		fmt.Printf("Server: %s\n", state.ServerEndpoint)
		fmt.Println()
		fmt.Println("Connection setup incomplete. Try disconnecting and reconnecting:")
		fmt.Println("  vibespace remote disconnect")
		fmt.Println("  vibespace remote connect <token>")
		return nil
	}

	fmt.Printf("Remote: %s\n", out.Teal("connected"))
	fmt.Printf("Server: %s\n", state.ServerHost)
	fmt.Printf("Local IP: %s\n", state.LocalIP)
	fmt.Printf("Server IP: %s\n", state.ServerIP)
	fmt.Printf("Connected at: %s\n", state.ConnectedAt.Format("2006-01-02 15:04:05"))

	// Check if WireGuard interface is actually up
	if remote.IsInterfaceUp() {
		fmt.Printf("Tunnel: %s\n", out.Green("active"))
	} else {
		fmt.Printf("Tunnel: %s\n", out.Yellow("interface down"))
	}

	return nil
}

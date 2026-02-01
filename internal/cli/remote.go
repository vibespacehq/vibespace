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
  1. Server admin runs: vibespace serve --generate-token --endpoint <host>
  2. You run: vibespace remote connect <token>
  3. Give the output public key to the server admin
  4. Server admin runs: vibespace serve --add-client <your-public-key>
  5. You run: vibespace remote activate`,
	Example: `  # Connect using an invite token
  vibespace remote connect vs-eyJrIjoiYWJj...

  # After server adds your key, activate the tunnel
  vibespace remote activate

  # Check connection status
  vibespace remote status

  # Disconnect
  vibespace remote disconnect`,
}

var remoteConnectCmd = &cobra.Command{
	Use:   "connect <token>",
	Short: "Set up connection to a remote server",
	Long: `Set up a connection to a remote vibespace server using an invite token.

The invite token contains the server's public key and endpoint.
After running this command, you'll receive your client public key.
Give this key to the server admin to add you as a client.`,
	Example: `  vibespace remote connect vs-eyJrIjoiYWJjMTIzIiwiZSI6InZwcy5leGFtcGxlLmNvbTo1MTgyMCJ9`,
	Args:    cobra.ExactArgs(1),
	RunE:    runRemoteConnect,
}

var remoteActivateCmd = &cobra.Command{
	Use:   "activate",
	Short: "Activate the WireGuard tunnel",
	Long: `Activate the WireGuard tunnel after the server has added your public key.

Run this after the server admin has added your client public key with:
  vibespace serve --add-client <your-public-key>`,
	Example: `  vibespace remote activate`,
	RunE:    runRemoteActivate,
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

	printStep("Setting up connection...")

	opts := remote.ConnectOptions{
		Token: token,
	}

	clientPubKey, err := remote.Connect(opts)
	if err != nil {
		return fmt.Errorf("failed to set up connection: %w", err)
	}

	// Get status to show details
	state, err := remote.GetStatus()
	if err != nil {
		return err
	}

	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, map[string]string{
			"client_public_key": clientPubKey,
			"local_ip":          state.LocalIP,
			"server_endpoint":   state.ServerEndpoint,
		}, nil))
	}

	printSuccess("Connection configured")
	fmt.Println()
	fmt.Printf("Your public key: %s\n", out.Teal(clientPubKey))
	fmt.Printf("Assigned IP: %s\n", state.LocalIP)
	fmt.Println()
	fmt.Println("Give your public key to the server admin.")
	fmt.Println("They need to run:")
	fmt.Printf("  vibespace serve --add-client %s\n", clientPubKey)
	fmt.Println()
	fmt.Println("After they add you, activate the tunnel:")
	fmt.Println("  vibespace remote activate")

	return nil
}

func runRemoteActivate(cmd *cobra.Command, args []string) error {
	out := getOutput()

	printStep("Activating tunnel...")

	if err := remote.Activate(); err != nil {
		return fmt.Errorf("failed to activate: %w", err)
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
		// Pending state - connect was run but not activate
		fmt.Printf("Remote: %s\n", out.Yellow("pending"))
		fmt.Printf("Your public key: %s\n", state.PublicKey)
		fmt.Printf("Assigned IP: %s\n", state.LocalIP)
		fmt.Printf("Server: %s\n", state.ServerEndpoint)
		fmt.Println()
		fmt.Println("Waiting for server to add your public key.")
		fmt.Println("After they add you, run: vibespace remote activate")
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

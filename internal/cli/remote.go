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

Prerequisites:
  - WireGuard must be installed (wg-quick command available)
  - The server must be running 'vibespace serve'
  - You need a registration token from the server admin`,
	Example: `  # Connect to a remote server
  vibespace remote connect user@vps.example.com --token vs-abc123

  # Check connection status
  vibespace remote status

  # Disconnect from remote server
  vibespace remote disconnect`,
}

var remoteConnectCmd = &cobra.Command{
	Use:   "connect <host>",
	Short: "Connect to a remote vibespace server",
	Long: `Connect to a remote vibespace server via WireGuard tunnel.

The host can be specified as:
  - hostname (uses current user)
  - user@hostname
  - user@hostname:port (for non-standard SSH ports)

A registration token is required for first-time connections.
The token is obtained from the server admin via:
  vibespace serve --generate-token`,
	Example: `  vibespace remote connect user@vps.example.com --token vs-abc123
  vibespace remote connect vps.example.com --token vs-abc123 --name "MacBook Pro"`,
	Args: cobra.ExactArgs(1),
	RunE: runRemoteConnect,
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

var (
	remoteToken string
	remoteName  string
)

func init() {
	remoteConnectCmd.Flags().StringVar(&remoteToken, "token", "", "Registration token from server")
	remoteConnectCmd.Flags().StringVar(&remoteName, "name", "", "Name for this client (optional)")
	remoteConnectCmd.MarkFlagRequired("token")

	remoteCmd.AddCommand(remoteConnectCmd)
	remoteCmd.AddCommand(remoteDisconnectCmd)
	remoteCmd.AddCommand(remoteStatusCmd)
}

func runRemoteConnect(cmd *cobra.Command, args []string) error {
	out := getOutput()
	host := args[0]
	ctx := context.Background()

	// Install WireGuard if needed
	if !remote.IsWireGuardInstalled() {
		printStep("Installing WireGuard...")
		if err := remote.InstallWireGuard(ctx); err != nil {
			return fmt.Errorf("failed to install WireGuard: %w", err)
		}
		printSuccess("WireGuard installed")
	}

	printStep("Connecting to %s...", host)

	opts := remote.ConnectOptions{
		Host:  host,
		Token: remoteToken,
		Name:  remoteName,
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
		return out.JSON(NewJSONOutput(true, RemoteStatusOutput{
			Connected:   state.Connected,
			ServerHost:  state.ServerHost,
			LocalIP:     state.LocalIP,
			ServerIP:    state.ServerIP,
			ConnectedAt: state.ConnectedAt.Format("2006-01-02 15:04:05"),
		}, nil))
	}

	if !state.Connected {
		fmt.Println("Not connected to any remote server")
		fmt.Println()
		fmt.Println("To connect: vibespace remote connect <host> --token <token>")
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

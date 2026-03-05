package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	vsdns "github.com/vibespacehq/vibespace/pkg/dns"
)

var forwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "Manage port-forwards",
	Example: `  vibespace forward --vibespace myproject
  vibespace forward list --vibespace myproject
  vibespace forward add 3000 --vibespace myproject
  vibespace forward remove 3000 --vibespace myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doForwardList(vs)
	},
}

var forwardListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all port-forwards",
	Aliases: []string{"ls"},
	Example: `  vibespace forward list --vibespace myproject
  vibespace forward list --vibespace myproject --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doForwardList(vs)
	},
}

var forwardAddCmd = &cobra.Command{
	Use:   "add <port>",
	Short: "Add a new port-forward",
	Example: `  vibespace forward add 3000 --vibespace myproject
  vibespace forward add 8080 --agent claude-2 --vibespace myproject
  vibespace forward add 5432 --local 15432 --vibespace myproject
  vibespace forward add 3000 --dns --vibespace myproject`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}

		remotePort, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid port number: %s", args[0])
		}

		agent, _ := cmd.Flags().GetString("agent")
		localPort, _ := cmd.Flags().GetInt("local")
		enableDNS, _ := cmd.Flags().GetBool("dns")
		dnsName, _ := cmd.Flags().GetString("dns-name")
		if cmd.Flags().Changed("dns-name") {
			enableDNS = true
		}

		return doForwardAdd(vs, agent, remotePort, localPort, enableDNS, dnsName)
	},
}

var forwardRemoveCmd = &cobra.Command{
	Use:   "remove <port>",
	Short: "Remove a port-forward",
	Example: `  vibespace forward remove 3000 --vibespace myproject
  vibespace forward remove 8080 --agent claude-2 --vibespace myproject`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}

		remotePort, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid port number: %s", args[0])
		}

		agent, _ := cmd.Flags().GetString("agent")
		return doForwardRemove(vs, agent, remotePort)
	},
}

func init() {
	forwardCmd.AddCommand(forwardListCmd)
	forwardCmd.AddCommand(forwardAddCmd)
	forwardCmd.AddCommand(forwardRemoveCmd)

	forwardAddCmd.Flags().StringP("agent", "a", "claude-1", "Agent to forward from")
	forwardAddCmd.Flags().IntP("local", "l", 0, "Local port to use (default: auto-allocate)")
	forwardAddCmd.Flags().Bool("dns", false, "Enable DNS resolution (requires sudo)")
	forwardAddCmd.Flags().String("dns-name", "", "Custom DNS name (default: agent.vibespace)")

	forwardRemoveCmd.Flags().StringP("agent", "a", "claude-1", "Agent to remove forward from")
}

func doForwardList(vibespace string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Debug("forward list command started", "vibespace", vibespace)

	// Ensure daemon is running (auto-start if needed)
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		slog.Error("failed to ensure daemon running", "vibespace", vibespace, "error", err)
		return err
	}

	client, err := daemon.NewClient()
	if err != nil {
		slog.Error("failed to create daemon client", "error", err)
		return err
	}

	result, err := client.ListForwardsForVibespace(vibespace)
	if err != nil {
		slog.Error("failed to list forwards", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to list forwards: %w", err)
	}

	// JSON output mode
	if out.IsJSONMode() {
		jsonOut := ForwardsOutput{
			Vibespace: vibespace,
			Agents:    make([]AgentForwardInfo, len(result.Agents)),
		}
		for i, agent := range result.Agents {
			jsonOut.Agents[i] = AgentForwardInfo{
				Name:     agent.Name,
				PodName:  agent.PodName,
				Forwards: make([]ForwardInfo, len(agent.Forwards)),
			}
			for j, fwd := range agent.Forwards {
				jsonOut.Agents[i].Forwards[j] = ForwardInfo{
					LocalPort:  fwd.LocalPort,
					RemotePort: fwd.RemotePort,
					Type:       fwd.Type,
					Status:     fwd.Status,
					Error:      fwd.Error,
					Reconnects: fwd.Reconnects,
					DNSName:    fwd.DNSName,
				}
			}
		}
		return out.JSON(NewJSONOutput(true, jsonOut, nil))
	}

	if len(result.Agents) == 0 {
		// Plain mode - no output for empty result
		if out.IsPlainMode() {
			return nil
		}
		fmt.Println("No port-forwards active")
		return nil
	}

	// Build table rows
	headers := []string{"AGENT", "LOCAL", "REMOTE", "TYPE", "STATUS"}
	var rows [][]string
	for _, agent := range result.Agents {
		for _, fwd := range agent.Forwards {
			status := fwd.Status
			if fwd.Error != "" {
				status = fmt.Sprintf("%s (%s)", status, fwd.Error)
			}
			if fwd.Reconnects > 0 {
				status = fmt.Sprintf("%s [%d reconnects]", status, fwd.Reconnects)
			}
			rows = append(rows, []string{
				agent.Name,
				strconv.Itoa(fwd.LocalPort),
				strconv.Itoa(fwd.RemotePort),
				fwd.Type,
				status,
			})
		}
	}

	out.Table(headers, rows)

	slog.Debug("forward list command completed", "vibespace", vibespace, "agent_count", len(result.Agents))
	return nil
}

func doForwardAdd(vibespace, agent string, remotePort, localPort int, enableDNS bool, dnsName string) error {
	out := getOutput()

	slog.Info("forward add command started", "vibespace", vibespace, "agent", agent, "remote_port", remotePort, "local_port", localPort, "dns", enableDNS)

	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		slog.Error("failed to ensure daemon running", "vibespace", vibespace, "error", err)
		return err
	}

	client, err := daemon.NewClient()
	if err != nil {
		slog.Error("failed to connect to daemon", "error", err)
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	result, err := client.AddForwardForVibespace(vibespace, agent, remotePort, localPort, enableDNS, dnsName)
	if err != nil {
		slog.Error("failed to add forward", "vibespace", vibespace, "agent", agent, "remote_port", remotePort, "error", err)
		return fmt.Errorf("failed to add forward: %w", err)
	}

	// Add /etc/hosts entry if DNS was enabled
	if result.DNSName != "" {
		if err := vsdns.AddHostEntry(result.DNSName, ""); errors.Is(err, vsdns.ErrSudoRequired) {
			// Prompt for sudo interactively, then retry
			printStep("DNS entry requires sudo...")
			sudoCmd := exec.Command("sudo", "-v")
			sudoCmd.Stdin = os.Stdin
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr
			if sudoCmd.Run() == nil {
				vsdns.AddHostEntry(result.DNSName, "")
			}
		}
	}

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, ForwardAddOutput{
			Vibespace:  vibespace,
			Agent:      agent,
			LocalPort:  result.LocalPort,
			RemotePort: result.RemotePort,
			DNSName:    result.DNSName,
		}, nil))
	}

	slog.Info("forward add command completed", "vibespace", vibespace, "agent", agent, "local_port", result.LocalPort, "remote_port", result.RemotePort, "dns_name", result.DNSName)
	printSuccess("Forward added: localhost:%d -> %d", result.LocalPort, result.RemotePort)
	if result.DNSName != "" {
		fmt.Printf("  DNS: %s.%s:%d\n", result.DNSName, vsdns.Domain(), result.LocalPort)
	}
	return nil
}

func doForwardRemove(vibespace, agent string, remotePort int) error {
	out := getOutput()

	slog.Info("forward remove command started", "vibespace", vibespace, "agent", agent, "remote_port", remotePort)

	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		slog.Error("failed to ensure daemon running", "vibespace", vibespace, "error", err)
		return err
	}

	client, err := daemon.NewClient()
	if err != nil {
		slog.Error("failed to connect to daemon", "error", err)
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	// Look up DNS name before removing so we can clean up /etc/hosts
	var dnsName string
	if list, err := client.ListForwardsForVibespace(vibespace); err == nil {
		for _, a := range list.Agents {
			if a.Name == agent {
				for _, fwd := range a.Forwards {
					if fwd.RemotePort == remotePort && fwd.DNSName != "" {
						dnsName = fwd.DNSName
					}
				}
			}
		}
	}

	if err := client.RemoveForwardForVibespace(vibespace, agent, remotePort); err != nil {
		slog.Error("failed to remove forward", "vibespace", vibespace, "agent", agent, "remote_port", remotePort, "error", err)
		return fmt.Errorf("failed to remove forward: %w", err)
	}

	// Remove /etc/hosts entry if forward had DNS
	if dnsName != "" {
		if err := vsdns.RemoveHostEntry(dnsName, ""); errors.Is(err, vsdns.ErrSudoRequired) {
			printStep("Removing DNS entry requires sudo...")
			sudoCmd := exec.Command("sudo", "-v")
			sudoCmd.Stdin = os.Stdin
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr
			if sudoCmd.Run() == nil {
				vsdns.RemoveHostEntry(dnsName, "")
			}
		}
	}

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, ForwardRemoveOutput{
			Vibespace:  vibespace,
			Agent:      agent,
			RemotePort: remotePort,
		}, nil))
	}

	slog.Info("forward remove command completed", "vibespace", vibespace, "agent", agent, "remote_port", remotePort)
	printSuccess("Forward removed: port %d", remotePort)
	return nil
}

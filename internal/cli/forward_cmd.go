package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/yagizdagabak/vibespace/pkg/daemon"
)

// runForwardCmd handles the forward subcommands
// Usage: vibespace <name> forward {list,add,remove}
func runForwardCmd(vibespace string, args []string) error {
	if len(args) == 0 {
		return runForwardList(vibespace)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "--help", "-h":
		printForwardHelp(vibespace)
		return nil
	case "list":
		return runForwardList(vibespace)
	case "add":
		return runForwardAdd(vibespace, subArgs)
	case "remove":
		return runForwardRemove(vibespace, subArgs)
	default:
		// If the argument looks like a port number, treat it as 'add'
		if _, err := strconv.Atoi(subCmd); err == nil {
			return runForwardAdd(vibespace, args)
		}
		return fmt.Errorf("unknown forward subcommand: %s", subCmd)
	}
}

// printForwardHelp prints help for the forward command
func printForwardHelp(vibespace string) {
	fmt.Printf(`Manage port-forwards for a vibespace

Usage:
  vibespace %s forward [command]

Available Commands:
  list        List all port-forwards (default)
  add         Add a new port-forward
  remove      Remove a port-forward

Flags:
  -h, --help  Help for forward

Examples:
  vibespace %s forward                    # List all forwards
  vibespace %s forward list               # List all forwards
  vibespace %s forward add 3000           # Forward port 3000 for claude-1
  vibespace %s forward add 8080 --agent claude-2 --local 9090
  vibespace %s forward remove 3000        # Remove port 3000 forward

Use "vibespace %s forward [command] --help" for more information about a command.
`, vibespace, vibespace, vibespace, vibespace, vibespace, vibespace, vibespace)
}

// runForwardList lists all port-forwards
func runForwardList(vibespace string) error {
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
	headers := []string{"AGENT", "LOCAL", "REMOTE", "TYPE", "STATUS", "DNS"}
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
			dns := fmt.Sprintf("%s.vibespace.internal:%d", agent.Name, fwd.LocalPort)
			rows = append(rows, []string{
				agent.Name,
				strconv.Itoa(fwd.LocalPort),
				strconv.Itoa(fwd.RemotePort),
				fwd.Type,
				status,
				dns,
			})
		}
	}

	out.Table(headers, rows)

	slog.Debug("forward list command completed", "vibespace", vibespace, "agent_count", len(result.Agents))
	return nil
}

// runForwardAdd adds a new port-forward
func runForwardAdd(vibespace string, args []string) error {
	out := getOutput()

	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`Add a new port-forward

Usage:
  vibespace %s forward add PORT [flags]

Arguments:
  PORT    Remote port in the container to forward

Flags:
  -a, --agent string   Agent to forward from (default: claude-1)
  -l, --local int      Local port to use (default: auto-allocate)
  -h, --help           Help for add

Examples:
  vibespace %s forward add 3000
  vibespace %s forward add 8080 --agent claude-2
  vibespace %s forward add 5432 --local 15432 --agent trusted
`, vibespace, vibespace, vibespace, vibespace)
			return nil
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("remote port required. Usage: vibespace %s forward add PORT [--agent AGENT] [--local LOCAL_PORT]", vibespace)
	}

	// Parse port
	remotePort, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid port number: %s", args[0])
	}

	// Parse optional flags
	agent := "claude-1" // Default agent
	localPort := 0      // Auto-allocate

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--agent", "-a":
			if i+1 < len(args) {
				agent = args[i+1]
				i++
			}
		case "--local", "-l":
			if i+1 < len(args) {
				localPort, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
	}

	slog.Info("forward add command started", "vibespace", vibespace, "agent", agent, "remote_port", remotePort, "local_port", localPort)

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

	result, err := client.AddForwardForVibespace(vibespace, agent, remotePort, localPort)
	if err != nil {
		slog.Error("failed to add forward", "vibespace", vibespace, "agent", agent, "remote_port", remotePort, "error", err)
		return fmt.Errorf("failed to add forward: %w\nCheck daemon status: vibespace %s forward list", err, vibespace)
	}

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, ForwardAddOutput{
			Vibespace:  vibespace,
			Agent:      agent,
			LocalPort:  result.LocalPort,
			RemotePort: result.RemotePort,
		}, nil))
	}

	slog.Info("forward add command completed", "vibespace", vibespace, "agent", agent, "local_port", result.LocalPort, "remote_port", result.RemotePort)
	printSuccess("Forward added: localhost:%d -> %d", result.LocalPort, result.RemotePort)
	fmt.Printf("  DNS: %s:%s.vibespace.internal:%d (use Safari — Chromium browsers bypass local DNS)\n", vibespace, agent, result.LocalPort)
	return nil
}

// runForwardRemove removes a port-forward
func runForwardRemove(vibespace string, args []string) error {
	out := getOutput()

	// Handle help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Printf(`Remove a port-forward

Usage:
  vibespace %s forward remove PORT [flags]

Arguments:
  PORT    Remote port to stop forwarding

Flags:
  -a, --agent string   Agent to remove forward from (default: claude-1)
  -h, --help           Help for remove

Examples:
  vibespace %s forward remove 3000
  vibespace %s forward remove 8080 --agent claude-2
`, vibespace, vibespace, vibespace)
			return nil
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("remote port required. Usage: vibespace %s forward remove PORT [--agent AGENT]", vibespace)
	}

	remotePort, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid port number: %s", args[0])
	}

	agent := "claude-1"
	for i := 1; i < len(args); i++ {
		if (args[i] == "--agent" || args[i] == "-a") && i+1 < len(args) {
			agent = args[i+1]
			break
		}
	}

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

	if err := client.RemoveForwardForVibespace(vibespace, agent, remotePort); err != nil {
		slog.Error("failed to remove forward", "vibespace", vibespace, "agent", agent, "remote_port", remotePort, "error", err)
		return fmt.Errorf("failed to remove forward: %w", err)
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

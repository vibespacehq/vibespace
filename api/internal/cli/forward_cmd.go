package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"vibespace/pkg/daemon"
)

// runForwardCmd handles the forward subcommands
// Usage: vibespace <name> forward {list,add,remove,stop,start,restart,restart-all}
func runForwardCmd(vibespace string, args []string) error {
	if len(args) == 0 {
		return runForwardList(vibespace)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "list":
		return runForwardList(vibespace)
	case "add":
		return runForwardAdd(vibespace, subArgs)
	case "remove":
		return runForwardRemove(vibespace, subArgs)
	case "stop":
		return runForwardStop(vibespace, subArgs)
	case "start":
		return runForwardStart(vibespace, subArgs)
	case "restart":
		return runForwardRestart(vibespace, subArgs)
	case "restart-all":
		return runForwardRestartAll(vibespace)
	default:
		// If the argument looks like a port number, treat it as 'add'
		if _, err := strconv.Atoi(subCmd); err == nil {
			return runForwardAdd(vibespace, args)
		}
		return fmt.Errorf("unknown forward subcommand: %s", subCmd)
	}
}

// runForwardList lists all port-forwards
func runForwardList(vibespace string) error {
	ctx := context.Background()

	// Ensure daemon is running (auto-start if needed)
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	client, err := daemon.NewClient(vibespace)
	if err != nil {
		return err
	}

	result, err := client.ListForwards()
	if err != nil {
		return fmt.Errorf("failed to list forwards: %w", err)
	}

	if len(result.Agents) == 0 {
		fmt.Println("No port-forwards active")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "AGENT\tLOCAL\tREMOTE\tTYPE\tSTATUS")

	for _, agent := range result.Agents {
		for _, fwd := range agent.Forwards {
			status := fwd.Status
			if fwd.Error != "" {
				status = fmt.Sprintf("%s (%s)", status, fwd.Error)
			}
			if fwd.Reconnects > 0 {
				status = fmt.Sprintf("%s [%d reconnects]", status, fwd.Reconnects)
			}

			fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n",
				agent.Name,
				fwd.LocalPort,
				fwd.RemotePort,
				fwd.Type,
				status,
			)
		}
	}

	w.Flush()
	return nil
}

// runForwardAdd adds a new port-forward
func runForwardAdd(vibespace string, args []string) error {
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

	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	client, err := daemon.NewClient(vibespace)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	result, err := client.AddForward(agent, remotePort, localPort)
	if err != nil {
		return fmt.Errorf("failed to add forward: %w\nCheck daemon status: vibespace %s forward list", err, vibespace)
	}

	printSuccess("Forward added: localhost:%d → %d", result.LocalPort, result.RemotePort)
	return nil
}

// runForwardRemove removes a port-forward
func runForwardRemove(vibespace string, args []string) error {
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

	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	client, err := daemon.NewClient(vibespace)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if err := client.RemoveForward(agent, remotePort); err != nil {
		return fmt.Errorf("failed to remove forward: %w", err)
	}

	printSuccess("Forward removed: port %d", remotePort)
	return nil
}

// runForwardStop stops a port-forward without removing it
func runForwardStop(vibespace string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("remote port required. Usage: vibespace %s forward stop PORT [--agent AGENT]", vibespace)
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

	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	client, err := daemon.NewClient(vibespace)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if err := client.StopForward(agent, remotePort); err != nil {
		return fmt.Errorf("failed to stop forward: %w", err)
	}

	printSuccess("Forward stopped: port %d", remotePort)
	return nil
}

// runForwardStart starts a stopped port-forward
func runForwardStart(vibespace string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("remote port required. Usage: vibespace %s forward start PORT [--agent AGENT]", vibespace)
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

	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	client, err := daemon.NewClient(vibespace)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if err := client.StartForward(agent, remotePort); err != nil {
		return fmt.Errorf("failed to start forward: %w", err)
	}

	printSuccess("Forward started: port %d", remotePort)
	return nil
}

// runForwardRestart restarts a port-forward
func runForwardRestart(vibespace string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("remote port required. Usage: vibespace %s forward restart PORT [--agent AGENT]", vibespace)
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

	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	client, err := daemon.NewClient(vibespace)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if err := client.RestartForward(agent, remotePort); err != nil {
		return fmt.Errorf("failed to restart forward: %w", err)
	}

	printSuccess("Forward restarted: port %d", remotePort)
	return nil
}

// runForwardRestartAll restarts all port-forwards
func runForwardRestartAll(vibespace string) error {
	// Ensure daemon is running (auto-start if needed)
	ctx := context.Background()
	if err := ensureDaemonRunningSimple(ctx, vibespace); err != nil {
		return err
	}

	client, err := daemon.NewClient(vibespace)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	printStep("Restarting all port-forwards...")

	if err := client.RestartAll(); err != nil {
		return fmt.Errorf("failed to restart forwards: %w", err)
	}

	printSuccess("All forwards restarted")
	return nil
}

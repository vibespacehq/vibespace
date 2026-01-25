package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/agent"
)

// runConfig routes to config subcommands
func runConfig(vibespace string, args []string) error {
	if len(args) == 0 {
		// Default to show
		return runConfigShow(vibespace, args)
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "show":
		return runConfigShow(vibespace, subArgs)
	case "set":
		return runConfigSet(vibespace, subArgs)
	case "--help", "-h":
		fmt.Println(`Manage agent configuration

Usage:
  vibespace <name> config [command]

Available Commands:
  show        Show agent configuration (default)
  set         Set agent configuration

Examples:
  vibespace myproject config
  vibespace myproject config show claude-1
  vibespace myproject config set claude-1 --skip-permissions`)
		return nil
	default:
		// If not a subcommand, treat as agent name for show
		return runConfigShow(vibespace, args)
	}
}

func runConfigShow(vibespace string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	// Check for help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println(`Show agent configuration

Usage:
  vibespace <name> config show [agent]

Arguments:
  agent     Optional agent name (shows all if not specified)

Examples:
  vibespace myproject config show
  vibespace myproject config show claude-1`)
			return nil
		}
	}

	slog.Debug("config show command started", "vibespace", vibespace, "args", args)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Verify vibespace exists
	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	// Get specific agent or all agents
	var agentName string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		agentName = args[0]
	}

	if agentName != "" {
		// Show config for specific agent
		config, err := svc.GetAgentConfig(ctx, vs.ID, agentName)
		if err != nil {
			return fmt.Errorf("failed to get agent config: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(JSONOutput{
				Success: true,
				Data: map[string]interface{}{
					"vibespace": vibespace,
					"agent":     agentName,
					"config":    config,
				},
			})
		}

		printAgentConfig(agentName, config)
	} else {
		// Show config for all agents
		agents, err := svc.ListAgents(ctx, vs.ID)
		if err != nil {
			return fmt.Errorf("failed to list agents: %w", err)
		}

		if out.IsJSONMode() {
			configs := make([]map[string]interface{}, 0, len(agents))
			for _, agent := range agents {
				config, err := svc.GetAgentConfig(ctx, vs.ID, agent.AgentName)
				if err != nil {
					slog.Warn("failed to get config for agent", "agent", agent.AgentName, "error", err)
					continue
				}
				configs = append(configs, map[string]interface{}{
					"agent":  agent.AgentName,
					"config": config,
				})
			}
			return out.JSON(JSONOutput{
				Success: true,
				Data: map[string]interface{}{
					"vibespace": vibespace,
					"agents":    configs,
				},
			})
		}

		for _, agent := range agents {
			config, err := svc.GetAgentConfig(ctx, vs.ID, agent.AgentName)
			if err != nil {
				printWarning("Failed to get config for %s: %v", agent.AgentName, err)
				continue
			}
			printAgentConfig(agent.AgentName, config)
			fmt.Println()
		}
	}

	return nil
}

func printAgentConfig(agentName string, config *agent.Config) {
	out := getOutput()

	// Header with agent name
	fmt.Printf("\n  %s %s\n", out.Bold("⬡"), out.Bold(agentName))
	fmt.Println()

	// Helper to print a config row
	printRow := func(icon, label, value string, valueColor func(string) string) {
		fmt.Printf("    %s %-20s %s\n", out.Dim(icon), out.Dim(label), valueColor(value))
	}

	// skip_permissions
	if config.SkipPermissions {
		printRow("◉", "skip_permissions", "enabled", out.Green)
	} else {
		printRow("○", "skip_permissions", "disabled", out.Dim)
	}

	// share_credentials
	if config.ShareCredentials {
		printRow("◉", "share_credentials", "enabled", out.Green)
	} else {
		printRow("○", "share_credentials", "disabled", out.Dim)
	}

	// allowed_tools
	if len(config.AllowedTools) > 0 {
		printRow("⚙", "allowed_tools", strings.Join(config.AllowedTools, ", "), func(s string) string { return s })
	} else if config.SkipPermissions {
		printRow("⚙", "allowed_tools", "all", out.Green)
	} else {
		printRow("⚙", "allowed_tools", strings.Join(agent.DefaultAllowedTools(), ", ")+" "+out.Dim("(default)"), func(s string) string { return s })
	}

	// disallowed_tools
	if len(config.DisallowedTools) > 0 {
		printRow("⊘", "disallowed_tools", strings.Join(config.DisallowedTools, ", "), out.Red)
	} else {
		printRow("⊘", "disallowed_tools", "none", out.Dim)
	}

	// model
	if config.Model != "" {
		printRow("◈", "model", config.Model, func(s string) string { return s })
	} else {
		printRow("◈", "model", "default", out.Dim)
	}

	// max_turns
	if config.MaxTurns > 0 {
		printRow("↻", "max_turns", strconv.Itoa(config.MaxTurns), func(s string) string { return s })
	} else {
		printRow("↻", "max_turns", "unlimited", out.Dim)
	}

	// system_prompt (only show if set)
	if config.SystemPrompt != "" {
		prompt := config.SystemPrompt
		if len(prompt) > 40 {
			prompt = prompt[:37] + "..."
		}
		printRow("✎", "system_prompt", prompt, func(s string) string { return s })
	}

	fmt.Println()
}

func runConfigSet(vibespace string, args []string) error {
	ctx := context.Background()

	// Check for help flag
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println(`Set agent configuration

Usage:
  vibespace <name> config set <agent> [flags]

Flags:
      --skip-permissions         Enable --dangerously-skip-permissions
      --no-skip-permissions      Disable --dangerously-skip-permissions
      --allowed-tools string     Comma-separated allowed tools
      --disallowed-tools string  Comma-separated disallowed tools
      --model string             Claude model to use
      --max-turns int            Maximum conversation turns (0 = unlimited)
  -h, --help                     Help for config set

Examples:
  vibespace myproject config set claude-1 --skip-permissions
  vibespace myproject config set claude-1 --allowed-tools "Bash,Read,Write"
  vibespace myproject config set claude-1 --model opus`)
			return nil
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("agent name required. Usage: vibespace %s config set <agent> [flags]", vibespace)
	}

	agentName := args[0]
	args = args[1:]

	slog.Info("config set command started", "vibespace", vibespace, "agent", agentName)

	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Verify vibespace exists
	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	// Get current config
	config, err := svc.GetAgentConfig(ctx, vs.ID, agentName)
	if err != nil {
		return fmt.Errorf("failed to get agent config: %w", err)
	}

	// Parse flags and update config
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--skip-permissions":
			config.SkipPermissions = true
		case arg == "--no-skip-permissions":
			config.SkipPermissions = false
		case arg == "--allowed-tools" && i+1 < len(args):
			config.AllowedTools = strings.Split(args[i+1], ",")
			i++
		case strings.HasPrefix(arg, "--allowed-tools="):
			config.AllowedTools = strings.Split(arg[16:], ",")
		case arg == "--disallowed-tools" && i+1 < len(args):
			config.DisallowedTools = strings.Split(args[i+1], ",")
			i++
		case strings.HasPrefix(arg, "--disallowed-tools="):
			config.DisallowedTools = strings.Split(arg[19:], ",")
		case arg == "--model" && i+1 < len(args):
			config.Model = args[i+1]
			i++
		case strings.HasPrefix(arg, "--model="):
			config.Model = arg[8:]
		case arg == "--max-turns" && i+1 < len(args):
			config.MaxTurns, _ = strconv.Atoi(args[i+1])
			i++
		case strings.HasPrefix(arg, "--max-turns="):
			config.MaxTurns, _ = strconv.Atoi(arg[12:])
		case arg == "--system-prompt" && i+1 < len(args):
			config.SystemPrompt = args[i+1]
			i++
		case strings.HasPrefix(arg, "--system-prompt="):
			config.SystemPrompt = arg[16:]
		}
	}

	printStep("Updating config for '%s' in '%s'...", agentName, vibespace)

	if err := svc.UpdateAgentConfig(ctx, vs.ID, agentName, config); err != nil {
		return fmt.Errorf("failed to update agent config: %w", err)
	}

	printSuccess("Config updated for '%s'", agentName)
	fmt.Println("  Note: Pod will restart to apply changes")
	fmt.Println()
	printAgentConfig(agentName, config)

	return nil
}

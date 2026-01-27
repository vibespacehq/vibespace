package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/yagizdagabak/vibespace/pkg/agent"
	vspkg "github.com/yagizdagabak/vibespace/pkg/vibespace"
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

	// Always get agents list to find agent type
	agents, err := svc.ListAgents(ctx, vs.ID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	// Helper to find agent by name
	findAgent := func(name string) *vspkg.AgentInfo {
		for i := range agents {
			if agents[i].AgentName == name {
				return &agents[i]
			}
		}
		return nil
	}

	// Helper to find agent type string by name (for JSON output)
	findAgentType := func(name string) string {
		if a := findAgent(name); a != nil {
			return a.AgentType.String()
		}
		return "unknown"
	}

	// Helper to convert config to output format (with all fields, no omitempty)
	configToOutput := func(config *agent.Config) AgentConfigOutput {
		allowedTools := config.AllowedTools
		if allowedTools == nil {
			allowedTools = []string{}
		}
		disallowedTools := config.DisallowedTools
		if disallowedTools == nil {
			disallowedTools = []string{}
		}
		return AgentConfigOutput{
			SkipPermissions:  config.SkipPermissions,
			ShareCredentials: config.ShareCredentials,
			AllowedTools:     allowedTools,
			DisallowedTools:  disallowedTools,
			Model:            config.Model,
			MaxTurns:         config.MaxTurns,
			SystemPrompt:     config.SystemPrompt,
			ReasoningEffort:  config.ReasoningEffort,
		}
	}

	if agentName != "" {
		// Show config for specific agent
		config, err := svc.GetAgentConfig(ctx, vs.ID, agentName)
		if err != nil {
			return fmt.Errorf("failed to get agent config: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, ConfigShowOutput{
				Vibespace: vibespace,
				Agent:     agentName,
				Type:      findAgentType(agentName),
				Config:    configToOutput(config),
			}, nil))
		}

		agentInfo := findAgent(agentName)
		if agentInfo == nil {
			return fmt.Errorf("agent '%s' not found", agentName)
		}
		printAgentConfig(agentName, agentInfo.AgentType, config)
	} else {
		// Show config for all agents
		if out.IsJSONMode() {
			configs := make([]AgentConfigItem, 0, len(agents))
			for _, a := range agents {
				config, err := svc.GetAgentConfig(ctx, vs.ID, a.AgentName)
				if err != nil {
					slog.Warn("failed to get config for agent", "agent", a.AgentName, "error", err)
					continue
				}
				configs = append(configs, AgentConfigItem{
					Agent:  a.AgentName,
					Type:   a.AgentType.String(),
					Config: configToOutput(config),
				})
			}
			return out.JSON(NewJSONOutput(true, ConfigShowAllOutput{
				Vibespace: vibespace,
				Agents:    configs,
			}, nil))
		}

		for _, a := range agents {
			config, err := svc.GetAgentConfig(ctx, vs.ID, a.AgentName)
			if err != nil {
				printWarning("Failed to get config for %s: %v", a.AgentName, err)
				continue
			}
			printAgentConfig(a.AgentName, a.AgentType, config)
			fmt.Println()
		}
	}

	return nil
}

func printAgentConfig(agentName string, agentType agent.Type, config *agent.Config) {
	out := getOutput()
	isCodex := agentType == agent.TypeCodex

	// Header with agent name
	fmt.Printf("\n  %s %s\n", out.Bold("⬡"), out.Bold(agentName))
	fmt.Println()

	// Helper to print a config row
	printRow := func(icon, label, value string, valueColor func(string) string) {
		fmt.Printf("    %s %-20s %s\n", out.Dim(icon), out.Dim(label), valueColor(value))
	}

	// skip_permissions (Claude only - Codex always runs in yolo mode)
	if isCodex {
		printRow("◉", "skip_permissions", "always (yolo mode)", out.Green)
	} else if config.SkipPermissions {
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

	// allowed_tools (Claude only - Codex doesn't have tool restrictions)
	if isCodex {
		printRow("⚙", "allowed_tools", "all (no restrictions)", out.Green)
	} else if len(config.AllowedTools) > 0 {
		printRow("⚙", "allowed_tools", strings.Join(config.AllowedTools, ", "), func(s string) string { return s })
	} else if config.SkipPermissions {
		printRow("⚙", "allowed_tools", "all", out.Green)
	} else {
		printRow("⚙", "allowed_tools", strings.Join(agent.DefaultAllowedTools(), ", ")+" "+out.Dim("(default)"), func(s string) string { return s })
	}

	// disallowed_tools (Claude only)
	if isCodex {
		printRow("⊘", "disallowed_tools", "n/a", out.Dim)
	} else if len(config.DisallowedTools) > 0 {
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

	// reasoning_effort (Codex only)
	if config.ReasoningEffort != "" {
		printRow("◆", "reasoning_effort", config.ReasoningEffort, func(s string) string { return s })
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
      --skip-permissions         Enable --dangerously-skip-permissions (Claude only)
      --no-skip-permissions      Disable --dangerously-skip-permissions (Claude only)
      --allowed-tools string     Comma-separated allowed tools (Claude only)
      --disallowed-tools string  Comma-separated disallowed tools (Claude only)
      --model string             Model to use (see below)
      --max-turns int            Maximum conversation turns (0 = unlimited)
      --reasoning-effort string  Reasoning effort: low, medium, high, xhigh (Codex only)
  -h, --help                     Help for config set

Claude Models:
  sonnet      Latest Sonnet (4.5) for daily coding tasks
  opus        Opus 4.5 for complex reasoning
  haiku       Fast and efficient for simple tasks
  opusplan    Opus for planning, Sonnet for execution

Codex Models:
  gpt-5.2-codex       Most advanced agentic coding model (recommended)
  gpt-5.1-codex-mini  Smaller, cost-effective
  gpt-5.1-codex-max   Optimized for long-horizon tasks
  gpt-5.2             General agentic model

Examples:
  vibespace myproject config set claude-1 --skip-permissions
  vibespace myproject config set claude-1 --model opus
  vibespace myproject config set codex-1 --model gpt-5.2-codex
  vibespace myproject config set codex-1 --reasoning-effort high`)
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

	// Check agent type to restrict certain flags
	agents, err := svc.ListAgents(ctx, vs.ID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	var agentType agent.Type
	for _, a := range agents {
		if a.AgentName == agentName {
			agentType = a.AgentType
			break
		}
	}
	isCodex := agentType == agent.TypeCodex

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
			if isCodex {
				return fmt.Errorf("--skip-permissions is not supported for Codex agents (always runs in --yolo mode)")
			}
			config.SkipPermissions = true
		case arg == "--no-skip-permissions":
			if isCodex {
				return fmt.Errorf("--no-skip-permissions is not supported for Codex agents (always runs in --yolo mode)")
			}
			config.SkipPermissions = false
		case arg == "--allowed-tools" && i+1 < len(args):
			if isCodex {
				return fmt.Errorf("--allowed-tools is not supported for Codex agents")
			}
			config.AllowedTools = strings.Split(args[i+1], ",")
			i++
		case strings.HasPrefix(arg, "--allowed-tools="):
			if isCodex {
				return fmt.Errorf("--allowed-tools is not supported for Codex agents")
			}
			config.AllowedTools = strings.Split(arg[16:], ",")
		case arg == "--disallowed-tools" && i+1 < len(args):
			if isCodex {
				return fmt.Errorf("--disallowed-tools is not supported for Codex agents")
			}
			config.DisallowedTools = strings.Split(args[i+1], ",")
			i++
		case strings.HasPrefix(arg, "--disallowed-tools="):
			if isCodex {
				return fmt.Errorf("--disallowed-tools is not supported for Codex agents")
			}
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
		case arg == "--reasoning-effort" && i+1 < len(args):
			if !isCodex {
				return fmt.Errorf("--reasoning-effort is only supported for Codex agents")
			}
			effort := strings.ToLower(args[i+1])
			if effort != "low" && effort != "medium" && effort != "high" && effort != "xhigh" {
				return fmt.Errorf("invalid reasoning effort: %s (must be low, medium, high, or xhigh)", effort)
			}
			config.ReasoningEffort = effort
			i++
		case strings.HasPrefix(arg, "--reasoning-effort="):
			if !isCodex {
				return fmt.Errorf("--reasoning-effort is only supported for Codex agents")
			}
			effort := strings.ToLower(arg[19:])
			if effort != "low" && effort != "medium" && effort != "high" && effort != "xhigh" {
				return fmt.Errorf("invalid reasoning effort: %s (must be low, medium, high, or xhigh)", effort)
			}
			config.ReasoningEffort = effort
		}
	}

	printStep("Updating config for '%s' in '%s'...", agentName, vibespace)

	if err := svc.UpdateAgentConfig(ctx, vs.ID, agentName, config); err != nil {
		return fmt.Errorf("failed to update agent config: %w", err)
	}

	// JSON output
	out := getOutput()
	if out.IsJSONMode() {
		// Convert config to output format
		allowedTools := config.AllowedTools
		if allowedTools == nil {
			allowedTools = []string{}
		}
		disallowedTools := config.DisallowedTools
		if disallowedTools == nil {
			disallowedTools = []string{}
		}
		return out.JSON(NewJSONOutput(true, ConfigSetOutput{
			Vibespace: vibespace,
			Agent:     agentName,
			Config: AgentConfigOutput{
				SkipPermissions:  config.SkipPermissions,
				ShareCredentials: config.ShareCredentials,
				AllowedTools:     allowedTools,
				DisallowedTools:  disallowedTools,
				Model:            config.Model,
				MaxTurns:         config.MaxTurns,
				SystemPrompt:     config.SystemPrompt,
				ReasoningEffort:  config.ReasoningEffort,
			},
		}, nil))
	}

	printSuccess("Config updated for '%s'", agentName)
	fmt.Println("  Note: Pod will restart to apply changes")
	fmt.Println()
	printAgentConfig(agentName, agentType, config)

	return nil
}

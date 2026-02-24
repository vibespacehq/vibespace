package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vibespacehq/vibespace/pkg/agent"
	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
	vspkg "github.com/vibespacehq/vibespace/pkg/vibespace"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage agent configuration",
	Example: `  vibespace config --vibespace myproject
  vibespace config show claude-1 --vibespace myproject
  vibespace config set claude-1 --skip-permissions --vibespace myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doConfigShow(nil, vs, "")
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show [agent]",
	Short: "Show agent configuration",
	Example: `  vibespace config show --vibespace myproject
  vibespace config show claude-1 --vibespace myproject`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		agentName := ""
		if len(args) > 0 {
			agentName = args[0]
		}
		return doConfigShow(nil, vs, agentName)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <agent>",
	Short: "Set agent configuration",
	Long: `Set agent configuration.

Claude Models:
  sonnet      Latest Sonnet (4.5) for daily coding tasks
  opus        Opus 4.5 for complex reasoning
  haiku       Fast and efficient for simple tasks
  opusplan    Opus for planning, Sonnet for execution

Codex Models:
  gpt-5.2-codex       Most advanced agentic coding model (recommended)
  gpt-5.1-codex-mini  Smaller, cost-effective
  gpt-5.1-codex-max   Optimized for long-horizon tasks
  gpt-5.2             General agentic model`,
	Example: `  vibespace config set claude-1 --skip-permissions --vibespace myproject
  vibespace config set claude-1 --model opus --vibespace myproject
  vibespace config set codex-1 --reasoning-effort high --vibespace myproject`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		vs, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doConfigSet(nil, vs, args[0], cmd)
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)

	configSetCmd.Flags().Bool("skip-permissions", false, "Enable --dangerously-skip-permissions (Claude only)")
	configSetCmd.Flags().Bool("no-skip-permissions", false, "Disable --dangerously-skip-permissions (Claude only)")
	configSetCmd.Flags().String("allowed-tools", "", "Comma-separated allowed tools (Claude only)")
	configSetCmd.Flags().String("disallowed-tools", "", "Comma-separated disallowed tools (Claude only)")
	configSetCmd.Flags().String("model", "", "Model to use")
	configSetCmd.Flags().Int("max-turns", 0, "Maximum conversation turns (0 = unlimited)")
	configSetCmd.Flags().String("system-prompt", "", "Custom system prompt")
	configSetCmd.Flags().String("reasoning-effort", "", "Reasoning effort: low, medium, high, xhigh (Codex only)")
}

func doConfigShow(svc *vspkg.Service, vibespace string, agentName string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Debug("config show command started", "vibespace", vibespace, "agent", agentName)

	if svc == nil {
		var err error
		svc, err = getVibespaceServiceWithCheck()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	// Verify vibespace exists
	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
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

	// Helper to format config as plain row
	configToPlainRow := func(name string, agentType agent.Type, config *agent.Config) string {
		isCodex := agentType == agent.TypeCodex
		skipPerm := "false"
		if isCodex {
			skipPerm = "always"
		} else if config.SkipPermissions {
			skipPerm = "true"
		}
		model := config.Model
		if model == "" {
			model = "default"
		}
		maxTurns := "unlimited"
		if config.MaxTurns > 0 {
			maxTurns = strconv.Itoa(config.MaxTurns)
		}
		reasoning := config.ReasoningEffort
		if reasoning == "" {
			reasoning = "-"
		}
		return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", name, agentType.String(), skipPerm, model, maxTurns, reasoning)
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
			return fmt.Errorf("agent '%s' not found: %w", agentName, vserrors.ErrAgentNotFound)
		}

		if out.IsPlainMode() {
			if out.Header() {
				fmt.Println("AGENT\tTYPE\tSKIP_PERMISSIONS\tMODEL\tMAX_TURNS\tREASONING_EFFORT")
			}
			fmt.Println(configToPlainRow(agentName, agentInfo.AgentType, config))
			return nil
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

		if out.IsPlainMode() {
			if out.Header() {
				fmt.Println("AGENT\tTYPE\tSKIP_PERMISSIONS\tMODEL\tMAX_TURNS\tREASONING_EFFORT")
			}
			for _, a := range agents {
				config, err := svc.GetAgentConfig(ctx, vs.ID, a.AgentName)
				if err != nil {
					continue
				}
				fmt.Println(configToPlainRow(a.AgentName, a.AgentType, config))
			}
			return nil
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

	// Header: name  type
	fmt.Printf("%s  %s\n", out.Bold(agentName), out.Dim(agentType.String()))
	fmt.Println()

	// Helper for label-value rows
	printField := func(label, value string) {
		fmt.Printf("  %s %s\n", out.Dim(fmt.Sprintf("%-19s", label)), value)
	}

	// skip_permissions
	if isCodex {
		printField("skip_permissions", out.Green("always"))
	} else if config.SkipPermissions {
		printField("skip_permissions", out.Green("true"))
	} else {
		printField("skip_permissions", "false")
	}

	// share_credentials
	if config.ShareCredentials {
		printField("share_credentials", out.Green("true"))
	} else {
		printField("share_credentials", "false")
	}

	// allowed_tools
	if isCodex {
		printField("allowed_tools", out.Green("all"))
	} else if len(config.AllowedTools) > 0 {
		printField("allowed_tools", strings.Join(config.AllowedTools, ", "))
	} else if config.SkipPermissions {
		printField("allowed_tools", out.Green("all"))
	} else {
		printField("allowed_tools", strings.Join(agent.DefaultAllowedTools(), ", ")+" "+out.Dim("(default)"))
	}

	// disallowed_tools
	if isCodex {
		printField("disallowed_tools", out.Dim("-"))
	} else if len(config.DisallowedTools) > 0 {
		printField("disallowed_tools", out.Red(strings.Join(config.DisallowedTools, ", ")))
	} else {
		printField("disallowed_tools", out.Dim("-"))
	}

	// model
	if config.Model != "" {
		printField("model", config.Model)
	} else {
		printField("model", out.Dim("default"))
	}

	// max_turns
	if config.MaxTurns > 0 {
		printField("max_turns", strconv.Itoa(config.MaxTurns))
	} else {
		printField("max_turns", out.Dim("unlimited"))
	}

	// system_prompt (only show if set)
	if config.SystemPrompt != "" {
		prompt := config.SystemPrompt
		if len(prompt) > 40 {
			prompt = prompt[:37] + "..."
		}
		printField("system_prompt", prompt)
	}

	// reasoning_effort (Codex only)
	if config.ReasoningEffort != "" {
		printField("reasoning_effort", config.ReasoningEffort)
	}
}

func doConfigSet(svc *vspkg.Service, vibespace string, agentName string, cmd *cobra.Command) error {
	ctx := context.Background()

	slog.Info("config set command started", "vibespace", vibespace, "agent", agentName)

	if svc == nil {
		var err error
		svc, err = getVibespaceServiceWithCheck()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
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

	// Apply flag changes
	if cmd.Flags().Changed("skip-permissions") {
		if isCodex {
			return fmt.Errorf("--skip-permissions is not supported for Codex agents (always runs in --yolo mode)")
		}
		config.SkipPermissions = true
	}
	if cmd.Flags().Changed("no-skip-permissions") {
		if isCodex {
			return fmt.Errorf("--no-skip-permissions is not supported for Codex agents (always runs in --yolo mode)")
		}
		config.SkipPermissions = false
	}
	if cmd.Flags().Changed("allowed-tools") {
		if isCodex {
			return fmt.Errorf("--allowed-tools is not supported for Codex agents")
		}
		v, _ := cmd.Flags().GetString("allowed-tools")
		config.AllowedTools = strings.Split(v, ",")
	}
	if cmd.Flags().Changed("disallowed-tools") {
		if isCodex {
			return fmt.Errorf("--disallowed-tools is not supported for Codex agents")
		}
		v, _ := cmd.Flags().GetString("disallowed-tools")
		config.DisallowedTools = strings.Split(v, ",")
	}
	if cmd.Flags().Changed("model") {
		config.Model, _ = cmd.Flags().GetString("model")
	}
	if cmd.Flags().Changed("max-turns") {
		config.MaxTurns, _ = cmd.Flags().GetInt("max-turns")
	}
	if cmd.Flags().Changed("system-prompt") {
		config.SystemPrompt, _ = cmd.Flags().GetString("system-prompt")
	}
	if cmd.Flags().Changed("reasoning-effort") {
		if !isCodex {
			return fmt.Errorf("--reasoning-effort is only supported for Codex agents")
		}
		effort, _ := cmd.Flags().GetString("reasoning-effort")
		effort = strings.ToLower(effort)
		if effort != "low" && effort != "medium" && effort != "high" && effort != "xhigh" {
			return fmt.Errorf("invalid reasoning effort: %s (must be low, medium, high, or xhigh)", effort)
		}
		config.ReasoningEffort = effort
	}

	printStep("Updating config for '%s' in '%s'...", agentName, vibespace)

	if err := svc.UpdateAgentConfig(ctx, vs.ID, agentName, config); err != nil {
		return fmt.Errorf("failed to update agent config: %w", err)
	}

	// JSON output
	out := getOutput()
	if out.IsJSONMode() {
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

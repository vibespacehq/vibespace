package cli

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vibespacehq/vibespace/pkg/agent"
	vspkg "github.com/vibespacehq/vibespace/pkg/vibespace"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents in a vibespace",
	Long:  `Manage agents in a vibespace. Requires --vibespace flag or VIBESPACE_NAME env var.`,
	Example: `  vibespace agent list --vibespace myproject
  vibespace agent create --vibespace myproject --agent-type codex
  vibespace agent delete claude-2 --vibespace myproject
  vibespace agent start --vibespace myproject
  vibespace agent stop claude-2 --vibespace myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vsName, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doAgentList(nil, vsName)
	},
}

var agentListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all agents",
	Aliases: []string{"ls"},
	Example: `  vibespace agent list --vibespace myproject
  vibespace agent list --vibespace myproject --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vsName, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doAgentList(nil, vsName)
	},
}

var agentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent",
	Example: `  vibespace agent create --vibespace myproject
  vibespace agent create --vibespace myproject --agent-type codex
  vibespace agent create --vibespace myproject --name researcher --share-credentials
  vibespace agent create --vibespace myproject --skip-permissions --model opus
  vibespace agent create --vibespace myproject --allowed-tools "Bash,Read,Write"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vsName, err := requireVibespace(cmd)
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		agentTypeStr, _ := cmd.Flags().GetString("agent-type")
		shareCredentials, _ := cmd.Flags().GetBool("share-credentials")
		skipPermissions, _ := cmd.Flags().GetBool("skip-permissions")
		allowedTools, _ := cmd.Flags().GetString("allowed-tools")
		disallowedTools, _ := cmd.Flags().GetString("disallowed-tools")
		model, _ := cmd.Flags().GetString("model")
		maxTurns, _ := cmd.Flags().GetInt("max-turns")

		return doAgentCreate(nil, vsName, name, agentTypeStr, shareCredentials, skipPermissions, allowedTools, disallowedTools, model, maxTurns)
	},
}

var agentDeleteCmd = &cobra.Command{
	Use:   "delete <agent>",
	Short: "Delete an agent",
	Args:  cobra.ExactArgs(1),
	Example: `  vibespace agent delete claude-2 --vibespace myproject
  vibespace agent delete codex-1 --vibespace myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vsName, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		return doAgentDelete(nil, vsName, args[0])
	},
}

var agentStartCmd = &cobra.Command{
	Use:   "start [agent]",
	Short: "Start agents in a vibespace",
	Args:  cobra.MaximumNArgs(1),
	Example: `  vibespace agent start --vibespace myproject
  vibespace agent start claude-2 --vibespace myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vsName, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		agentName := ""
		if len(args) > 0 {
			agentName = args[0]
		}
		return doAgentStart(nil, vsName, agentName)
	},
}

var agentStopCmd = &cobra.Command{
	Use:   "stop [agent]",
	Short: "Stop agents in a vibespace",
	Args:  cobra.MaximumNArgs(1),
	Example: `  vibespace agent stop --vibespace myproject
  vibespace agent stop claude-2 --vibespace myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vsName, err := requireVibespace(cmd)
		if err != nil {
			return err
		}
		agentName := ""
		if len(args) > 0 {
			agentName = args[0]
		}
		return doAgentStop(nil, vsName, agentName)
	},
}

func init() {
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentDeleteCmd)
	agentCmd.AddCommand(agentStartCmd)
	agentCmd.AddCommand(agentStopCmd)

	agentCreateCmd.Flags().StringP("name", "n", "", "Custom name for the agent (default: <type>-N)")
	agentCreateCmd.Flags().StringP("agent-type", "t", "", "Agent type: claude-code, codex (default: inherit from primary)")
	agentCreateCmd.Flags().BoolP("share-credentials", "s", false, "Share credentials across all agents")
	agentCreateCmd.Flags().Bool("skip-permissions", false, "Enable --dangerously-skip-permissions")
	agentCreateCmd.Flags().String("allowed-tools", "", "Comma-separated allowed tools (replaces default)")
	agentCreateCmd.Flags().String("disallowed-tools", "", "Comma-separated disallowed tools")
	agentCreateCmd.Flags().String("model", "", "Model to use (e.g., opus, sonnet)")
	agentCreateCmd.Flags().Int("max-turns", 0, "Maximum conversation turns")
}

// --- Business logic ---

func doAgentList(svc *vspkg.Service, vibespace string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Debug("agent list command started", "vibespace", vibespace)

	if svc == nil {
		var err error
		svc, err = getVibespaceServiceWithCheck()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	agents, err := svc.ListAgents(ctx, vibespace)
	if err != nil {
		slog.Error("failed to list agents", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to list agents: %w", err)
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].AgentNum < agents[j].AgentNum
	})

	if out.IsJSONMode() {
		items := make([]AgentListItem, len(agents))
		for i, agent := range agents {
			items[i] = AgentListItem{
				Name:      agent.AgentName,
				Type:      agent.AgentType.String(),
				Vibespace: vibespace,
				Status:    agent.Status,
			}
		}
		return out.JSON(NewJSONOutput(true, AgentsOutput{
			Vibespace: vibespace,
			Agents:    items,
			Count:     len(items),
		}, nil))
	}

	if len(agents) == 0 {
		if out.IsPlainMode() {
			return nil
		}
		fmt.Printf("No agents in vibespace '%s'\n", vibespace)
		fmt.Println()
		fmt.Printf("Create one with: vibespace agent create --vibespace %s\n", vibespace)
		return nil
	}

	headers := []string{"AGENT", "TYPE", "VIBESPACE", "STATUS"}
	rows := make([][]string, len(agents))
	for i, a := range agents {
		status := a.Status
		if !out.NoColor() {
			switch a.Status {
			case "running":
				status = out.Green(a.Status)
			case "stopped":
				status = out.Yellow(a.Status)
			case "creating":
				status = out.Yellow(a.Status)
			}
		}
		rows[i] = []string{a.AgentName, a.AgentType.String(), vibespace, status}
	}

	out.Table(headers, rows)

	slog.Debug("agent list command completed", "vibespace", vibespace, "count", len(agents))
	return nil
}

func doAgentCreate(svc *vspkg.Service, vibespace, customName, agentTypeStr string, shareCredentials, skipPermissions bool, allowedTools, disallowedTools, model string, maxTurns int) error {
	ctx := context.Background()
	out := getOutput()

	var agentType agent.Type
	if agentTypeStr != "" {
		agentType = agent.ParseType(agentTypeStr)
		if !agentType.IsValid() {
			return fmt.Errorf("invalid agent type '%s': valid types are claude-code, codex", agentTypeStr)
		}
	}

	if agentType != "" && (allowedTools != "" || disallowedTools != "") {
		impl, implErr := agent.Get(agentType)
		if implErr == nil {
			supported := impl.SupportedTools()
			supportedSet := make(map[string]bool, len(supported))
			for _, t := range supported {
				supportedSet[t] = true
			}
			if allowedTools != "" {
				for _, tool := range strings.Split(allowedTools, ",") {
					tool = strings.TrimSpace(tool)
					if !supportedSet[tool] {
						return fmt.Errorf("invalid allowed tool '%s' for %s (valid: %s)", tool, agentType, strings.Join(supported, ", "))
					}
				}
			}
			if disallowedTools != "" {
				for _, tool := range strings.Split(disallowedTools, ",") {
					tool = strings.TrimSpace(tool)
					if !supportedSet[tool] {
						return fmt.Errorf("invalid disallowed tool '%s' for %s (valid: %s)", tool, agentType, strings.Join(supported, ", "))
					}
				}
			}
		}
	}

	slog.Info("agent create command started", "vibespace", vibespace, "name", customName, "agent_type", agentType, "share_credentials", shareCredentials)

	if svc == nil {
		var err error
		svc, err = getVibespaceServiceWithCheck()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	vs, err := checkVibespaceRunning(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not running", "vibespace", vibespace, "error", err)
		return err
	}

	if customName != "" {
		printStep("Creating agent '%s' in '%s'...", customName, vibespace)
	} else if agentType != "" {
		printStep("Creating new %s agent in '%s'...", agentType, vibespace)
	} else {
		printStep("Creating new agent in '%s'...", vibespace)
	}

	var agentConfig *agent.Config
	if skipPermissions || allowedTools != "" || disallowedTools != "" || model != "" || maxTurns > 0 {
		agentConfig = &agent.Config{
			SkipPermissions: skipPermissions,
			Model:           model,
			MaxTurns:        maxTurns,
		}
		if allowedTools != "" {
			agentConfig.AllowedTools = strings.Split(allowedTools, ",")
		}
		if disallowedTools != "" {
			agentConfig.DisallowedTools = strings.Split(disallowedTools, ",")
		}
	}

	opts := &vspkg.SpawnAgentOptions{
		Name:             customName,
		AgentType:        agentType,
		ShareCredentials: shareCredentials,
		Config:           agentConfig,
	}
	agentName, err := svc.SpawnAgent(ctx, vs.ID, opts)
	if err != nil {
		slog.Error("failed to create agent", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to create agent: %w", err)
	}

	if out.IsJSONMode() {
		agentTypeOutput := agentType.String()
		if agentTypeOutput == "" {
			agentTypeOutput = "inherited"
		}
		return out.JSON(NewJSONOutput(true, AgentCreateOutput{
			Vibespace: vibespace,
			Agent:     agentName,
			Type:      agentTypeOutput,
		}, nil))
	}

	slog.Info("agent create command completed", "vibespace", vibespace, "agent", agentName)
	printSuccess("Agent '%s' scheduled (starting...)", agentName)
	if shareCredentials {
		fmt.Println("  Credential sharing enabled via /vibespace/.vibespace")
	}
	if agentConfig != nil {
		if agentConfig.SkipPermissions {
			fmt.Println("  Skip permissions enabled")
		}
		if len(agentConfig.AllowedTools) > 0 {
			fmt.Printf("  Allowed tools: %s\n", strings.Join(agentConfig.AllowedTools, ","))
			if resolvedType := agentType; resolvedType != "" {
				if impl, err := agent.Get(resolvedType); err == nil {
					excluded := excludedToolsFromAllowed(impl.SupportedTools(), agentConfig.AllowedTools)
					if len(excluded) > 0 {
						fmt.Printf("  Excluded tools: %s\n", strings.Join(excluded, ","))
					}
				}
			}
		}
		if len(agentConfig.DisallowedTools) > 0 {
			fmt.Printf("  Disallowed tools: %s\n", strings.Join(agentConfig.DisallowedTools, ","))
		}
		if agentConfig.Model != "" {
			fmt.Printf("  Model: %s\n", agentConfig.Model)
		}
	}
	fmt.Println()
	fmt.Printf("Connect with: vibespace %s connect %s\n", vibespace, agentName)
	return nil
}

func doAgentDelete(svc *vspkg.Service, vibespace, agentID string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Info("agent delete command started", "vibespace", vibespace, "agent", agentID)

	if svc == nil {
		var err error
		svc, err = getVibespaceServiceWithCheck()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	printStep("Deleting agent '%s' from '%s'...", agentID, vibespace)

	if err := svc.KillAgent(ctx, vs.ID, agentID); err != nil {
		slog.Error("failed to delete agent", "vibespace", vibespace, "agent", agentID, "error", err)
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, AgentDeleteOutput{
			Vibespace: vibespace,
			Agent:     agentID,
		}, nil))
	}

	slog.Info("agent delete command completed", "vibespace", vibespace, "agent", agentID)
	printSuccess("Agent '%s' deleted", agentID)

	return nil
}

func doAgentStart(svc *vspkg.Service, vibespace, agentName string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Info("start command started", "vibespace", vibespace, "agent", agentName)

	if svc == nil {
		var err error
		svc, err = getVibespaceServiceWithCheck()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	if agentName != "" {
		printStep("Starting agent '%s' in '%s'...", agentName, vibespace)

		if err := svc.StartAgent(ctx, vs.ID, agentName); err != nil {
			slog.Error("failed to start agent", "vibespace", vibespace, "agent", agentName, "error", err)
			return fmt.Errorf("failed to start agent: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StartOutput{
				Vibespace: vibespace,
				Agent:     agentName,
			}, nil))
		}

		slog.Info("start command completed", "vibespace", vibespace, "agent", agentName)
		printSuccess("Agent '%s' started", agentName)
	} else {
		printStep("Starting all agents in '%s'...", vibespace)

		if err := svc.Start(ctx, vs.ID); err != nil {
			slog.Error("failed to start vibespace", "vibespace", vibespace, "error", err)
			return fmt.Errorf("failed to start vibespace: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StartOutput{
				Vibespace: vibespace,
			}, nil))
		}

		slog.Info("start command completed", "vibespace", vibespace)
		printSuccess("All agents started in '%s'", vibespace)
	}

	return nil
}

func doAgentStop(svc *vspkg.Service, vibespace, agentName string) error {
	ctx := context.Background()
	out := getOutput()

	slog.Info("stop command started", "vibespace", vibespace, "agent", agentName)

	if svc == nil {
		var err error
		svc, err = getVibespaceServiceWithCheck()
		if err != nil {
			slog.Error("failed to get vibespace service", "error", err)
			return err
		}
	}

	vs, err := checkVibespaceExists(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not found", "vibespace", vibespace, "error", err)
		return err
	}

	if agentName != "" {
		printStep("Stopping agent '%s' in '%s'...", agentName, vibespace)

		if err := svc.StopAgent(ctx, vs.ID, agentName); err != nil {
			slog.Error("failed to stop agent", "vibespace", vibespace, "agent", agentName, "error", err)
			return fmt.Errorf("failed to stop agent: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StopOutput{
				Stopped: true,
				Target:  agentName,
			}, nil))
		}

		slog.Info("stop command completed", "vibespace", vibespace, "agent", agentName)
		printSuccess("Agent '%s' stopped", agentName)
	} else {
		printStep("Stopping all agents in '%s'...", vibespace)

		if err := svc.Stop(ctx, vs.ID); err != nil {
			slog.Error("failed to stop vibespace", "vibespace", vibespace, "error", err)
			return fmt.Errorf("failed to stop vibespace: %w", err)
		}

		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StopOutput{
				Stopped: true,
				Target:  vibespace,
			}, nil))
		}

		slog.Info("stop command completed", "vibespace", vibespace)
		printSuccess("All agents stopped in '%s'", vibespace)
	}

	return nil
}

func excludedToolsFromAllowed(supported, allowed []string) []string {
	allowedBase := make(map[string]bool, len(allowed))
	for _, t := range allowed {
		base := t
		if idx := strings.Index(t, "("); idx >= 0 {
			base = t[:idx]
		}
		allowedBase[base] = true
	}
	var excluded []string
	for _, t := range supported {
		if !allowedBase[t] {
			excluded = append(excluded, t)
		}
	}
	return excluded
}

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yagizdagabak/vibespace/pkg/agent"
	"github.com/yagizdagabak/vibespace/pkg/daemon"
	"github.com/yagizdagabak/vibespace/pkg/model"
	"github.com/yagizdagabak/vibespace/pkg/ui"
	"github.com/yagizdagabak/vibespace/pkg/vibespace"
)

// InfoOutput is the JSON output for the info command
type InfoOutput struct {
	Name      string            `json:"name"`
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	PVC       string            `json:"pvc"`
	CPU       string            `json:"cpu"`
	Memory    string            `json:"memory"`
	Storage   string            `json:"storage"`
	Mounts    []MountInfo       `json:"mounts,omitempty"`
	Agents    []AgentInfoOutput `json:"agents"`
	Forwards  []AgentForwardInfo `json:"forwards,omitempty"`
	CreatedAt string            `json:"created_at"`
}

// MountInfo represents a mount in JSON output
type MountInfo struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// AgentInfoOutput represents an agent with config in JSON output
type AgentInfoOutput struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Status string            `json:"status"`
	Config AgentConfigOutput `json:"config"`
}

func runInfo(vibespaceNameOrID string, args []string) error {
	ctx := context.Background()
	out := getOutput()

	// Get vibespace service
	svc, err := getVibespaceService()
	if err != nil {
		return err
	}

	// Get vibespace details
	vs, err := svc.Get(ctx, vibespaceNameOrID)
	if err != nil {
		return err
	}

	// Get agents
	agents, err := svc.ListAgents(ctx, vs.ID)
	if err != nil {
		return err
	}

	// Get agent configs
	agentConfigs := make(map[string]*agent.Config)
	for _, a := range agents {
		config, err := svc.GetAgentConfig(ctx, vs.ID, a.AgentName)
		if err == nil {
			agentConfigs[a.AgentName] = config
		}
	}

	// Try to get forwards from daemon (best-effort)
	var forwardAgents []AgentForwardInfo
	if client, err := daemon.NewClient(); err == nil {
		if result, err := client.ListForwardsForVibespace(vs.Name); err == nil {
			for _, a := range result.Agents {
				agentFwd := AgentForwardInfo{
					Name:     a.Name,
					PodName:  a.PodName,
					Forwards: make([]ForwardInfo, len(a.Forwards)),
				}
				for j, fwd := range a.Forwards {
					agentFwd.Forwards[j] = ForwardInfo{
						LocalPort:  fwd.LocalPort,
						RemotePort: fwd.RemotePort,
						Type:       fwd.Type,
						Status:     fwd.Status,
						Error:      fwd.Error,
						Reconnects: fwd.Reconnects,
					}
				}
				forwardAgents = append(forwardAgents, agentFwd)
			}
		}
	}

	// Build output data
	pvcName := fmt.Sprintf("vibespace-%s-pvc", vs.ID)
	info := InfoOutput{
		Name:      vs.Name,
		ID:        vs.ID,
		Status:    vs.Status,
		PVC:       pvcName,
		CPU:       vs.Resources.CPU,
		Memory:    vs.Resources.Memory,
		Storage:   vs.Resources.Storage,
		CreatedAt: vs.CreatedAt,
	}

	info.Forwards = forwardAgents

	// Convert mounts
	for _, m := range vs.Mounts {
		info.Mounts = append(info.Mounts, MountInfo{
			HostPath:      m.HostPath,
			ContainerPath: m.ContainerPath,
			ReadOnly:      m.ReadOnly,
		})
	}

	// Convert agents with configs
	for _, a := range agents {
		agentInfo := AgentInfoOutput{
			Name:   a.AgentName,
			Type:   a.AgentType.String(),
			Status: a.Status,
		}
		if config, ok := agentConfigs[a.AgentName]; ok {
			allowedTools := config.AllowedTools
			if allowedTools == nil {
				allowedTools = []string{}
			}
			disallowedTools := config.DisallowedTools
			if disallowedTools == nil {
				disallowedTools = []string{}
			}
			agentInfo.Config = AgentConfigOutput{
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
		info.Agents = append(info.Agents, agentInfo)
	}

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, info, nil))
	}

	// Plain output
	if out.IsPlainMode() {
		return renderInfoPlain(info, out.Header())
	}

	// Rich terminal output with lipgloss
	return renderInfoRich(info, vs, agents, agentConfigs, forwardAgents, out.NoColor())
}

func renderInfoPlain(info InfoOutput, header bool) error {
	if header {
		fmt.Println("KEY\tVALUE")
	}
	fmt.Printf("name\t%s\n", info.Name)
	fmt.Printf("id\t%s\n", info.ID)
	fmt.Printf("status\t%s\n", info.Status)
	fmt.Printf("pvc\t%s\n", info.PVC)
	fmt.Printf("agents\t%d\n", len(info.Agents))
	fmt.Printf("cpu\t%s\n", info.CPU)
	fmt.Printf("memory\t%s\n", info.Memory)
	fmt.Printf("storage\t%s\n", info.Storage)
	fmt.Printf("created\t%s\n", info.CreatedAt)
	for i, m := range info.Mounts {
		mode := "rw"
		if m.ReadOnly {
			mode = "ro"
		}
		fmt.Printf("mount.%d\t%s:%s:%s\n", i, m.HostPath, m.ContainerPath, mode)
	}
	for _, a := range info.Agents {
		fmt.Printf("agent.%s.type\t%s\n", a.Name, a.Type)
		fmt.Printf("agent.%s.status\t%s\n", a.Name, a.Status)
		fmt.Printf("agent.%s.skip_permissions\t%v\n", a.Name, a.Config.SkipPermissions)
		fmt.Printf("agent.%s.model\t%s\n", a.Name, a.Config.Model)
	}
	for _, a := range info.Forwards {
		for _, fwd := range a.Forwards {
			fmt.Printf("forward.%s.%d\t%d:%d:%s:%s\n", a.Name, fwd.RemotePort, fwd.LocalPort, fwd.RemotePort, fwd.Type, fwd.Status)
		}
	}
	return nil
}

func renderInfoRich(info InfoOutput, vs *model.Vibespace, agents []vibespace.AgentInfo, agentConfigs map[string]*agent.Config, forwards []AgentForwardInfo, noColor bool) error {
	out := getOutput()

	// Define styles
	var (
		titleStyle          lipgloss.Style
		labelStyle          lipgloss.Style
		valueStyle          lipgloss.Style
		statusStyle         lipgloss.Style
		sectionStyle        lipgloss.Style
		mountHostStyle      lipgloss.Style
		mountArrowStyle     lipgloss.Style
		mountContainerStyle lipgloss.Style
		mountModeStyle      lipgloss.Style
		dimStyle            lipgloss.Style
	)

	if noColor {
		plain := lipgloss.NewStyle()
		titleStyle = plain.Bold(true)
		labelStyle = plain.Width(12)
		valueStyle = plain
		statusStyle = plain
		sectionStyle = plain.Bold(true)
		mountHostStyle = plain
		mountArrowStyle = plain
		mountContainerStyle = plain
		mountModeStyle = plain
		dimStyle = plain
	} else {
		titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Teal)

		labelStyle = lipgloss.NewStyle().
			Foreground(ui.ColorDim).
			Width(12)

		valueStyle = lipgloss.NewStyle().
			Foreground(ui.ColorWhite)

		statusStyle = lipgloss.NewStyle()
		switch info.Status {
		case "running":
			statusStyle = statusStyle.Foreground(ui.ColorSuccess).Bold(true)
		case "stopped":
			statusStyle = statusStyle.Foreground(ui.ColorWarning)
		case "creating":
			statusStyle = statusStyle.Foreground(ui.Teal)
		default:
			statusStyle = statusStyle.Foreground(ui.ColorError)
		}

		sectionStyle = lipgloss.NewStyle().
			Foreground(ui.Orange).
			Bold(true)

		mountHostStyle = lipgloss.NewStyle().
			Foreground(ui.ColorWhite)

		mountArrowStyle = lipgloss.NewStyle().
			Foreground(ui.ColorDim)

		mountContainerStyle = lipgloss.NewStyle().
			Foreground(ui.Teal)

		mountModeStyle = lipgloss.NewStyle().
			Foreground(ui.ColorWarning)

		dimStyle = lipgloss.NewStyle().
			Foreground(ui.ColorDim)
	}

	// Build the output
	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render(info.Name))
	sb.WriteString("\n\n")

	// Main info
	sb.WriteString(renderField(labelStyle, valueStyle, "ID", info.ID))
	sb.WriteString(renderField(labelStyle, statusStyle, "Status", info.Status))
	sb.WriteString(renderField(labelStyle, valueStyle, "PVC", fmt.Sprintf("vibespace-%s-pvc", info.ID)))
	sb.WriteString("\n")

	// Resources
	sb.WriteString(sectionStyle.Render("Resources"))
	sb.WriteString("\n")
	sb.WriteString(renderField(labelStyle, valueStyle, "CPU", info.CPU))
	sb.WriteString(renderField(labelStyle, valueStyle, "Memory", info.Memory))
	sb.WriteString(renderField(labelStyle, valueStyle, "Storage", info.Storage))

	// Mounts
	if len(info.Mounts) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Mounts"))
		sb.WriteString("\n")
		for _, m := range info.Mounts {
			mode := "rw"
			if m.ReadOnly {
				mode = "ro"
			}
			hostPath := shortenPath(m.HostPath)

			line := fmt.Sprintf("  %s %s %s %s",
				mountHostStyle.Render(hostPath),
				mountArrowStyle.Render("→"),
				mountContainerStyle.Render(m.ContainerPath),
				mountModeStyle.Render(fmt.Sprintf("(%s)", mode)),
			)
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	// Agents section
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("Agents"))
	sb.WriteString("\n")

	fmt.Print(sb.String())

	// Print each agent with config (reuse the existing printAgentConfig style)
	for _, a := range agents {
		config := agentConfigs[a.AgentName]
		if config == nil {
			config = &agent.Config{}
		}
		printAgentConfigForInfo(a.AgentName, a.AgentType, a.Status, config, out, noColor)
	}

	// Forwards section (only if daemon is running and has forwards)
	hasForwards := false
	for _, a := range forwards {
		if len(a.Forwards) > 0 {
			hasForwards = true
			break
		}
	}
	if hasForwards {
		fmt.Printf("\n%s\n", sectionStyle.Render("Forwards"))
		for _, a := range forwards {
			if len(a.Forwards) == 0 {
				continue
			}
			for _, fwd := range a.Forwards {
				fwdType := fwd.Type
				if fwdType == "" {
					fwdType = "manual"
				}
				status := fwd.Status
				if fwd.Error != "" {
					status = fmt.Sprintf("%s (%s)", status, fwd.Error)
				}
				line := fmt.Sprintf("  %s %s %s %s %s",
					out.Bold(a.Name),
					dimStyle.Render(fmt.Sprintf(":%d", fwd.LocalPort)),
					mountArrowStyle.Render("→"),
					dimStyle.Render(fmt.Sprintf(":%d", fwd.RemotePort)),
					dimStyle.Render(fmt.Sprintf("[%s] %s", fwdType, status)),
				)
				fmt.Println(line)
			}
		}
	}

	// Created timestamp
	fmt.Printf("\n%s\n", dimStyle.Render(fmt.Sprintf("Created %s", info.CreatedAt)))

	return nil
}

func printAgentConfigForInfo(agentName string, agentType agent.Type, status string, config *agent.Config, out *Output, noColor bool) {
	isCodex := agentType == agent.TypeCodex

	// Status indicator
	var statusIndicator string
	if noColor {
		switch status {
		case "running":
			statusIndicator = "[running]"
		case "stopped":
			statusIndicator = "[stopped]"
		case "creating":
			statusIndicator = "[creating]"
		default:
			statusIndicator = fmt.Sprintf("[%s]", status)
		}
	} else {
		switch status {
		case "running":
			statusIndicator = out.Green("●")
		case "stopped":
			statusIndicator = out.Yellow("○")
		case "creating":
			statusIndicator = out.Teal("◐")
		default:
			statusIndicator = out.Red("●")
		}
	}

	// Header with agent name and status
	fmt.Printf("\n  %s %s %s\n", out.Bold("⬡"), out.Bold(agentName), statusIndicator)
	fmt.Println()

	// Helper to print a config row
	printRow := func(icon, label, value string, valueColor func(string) string) {
		fmt.Printf("    %s %-20s %s\n", out.Dim(icon), out.Dim(label), valueColor(value))
	}

	// skip_permissions
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

	// allowed_tools
	if isCodex {
		printRow("⚙", "allowed_tools", "all (no restrictions)", out.Green)
	} else if len(config.AllowedTools) > 0 {
		printRow("⚙", "allowed_tools", strings.Join(config.AllowedTools, ", "), func(s string) string { return s })
	} else if config.SkipPermissions {
		printRow("⚙", "allowed_tools", "all", out.Green)
	} else {
		printRow("⚙", "allowed_tools", strings.Join(agent.DefaultAllowedTools(), ", ")+" "+out.Dim("(default)"), func(s string) string { return s })
	}

	// disallowed_tools
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
}

func renderField(labelStyle, valueStyle lipgloss.Style, label, value string) string {
	return fmt.Sprintf("  %s %s\n",
		labelStyle.Render(label),
		valueStyle.Render(value),
	)
}

// shortenPath replaces home directory with ~
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

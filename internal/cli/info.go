package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vibespacehq/vibespace/pkg/agent"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/model"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// InfoOutput is the JSON output for the info command
type InfoOutput struct {
	Name      string             `json:"name"`
	ID        string             `json:"id"`
	Status    string             `json:"status"`
	PVC       string             `json:"pvc"`
	CPU       string             `json:"cpu"`
	Memory    string             `json:"memory"`
	Storage   string             `json:"storage"`
	Mounts    []MountInfo        `json:"mounts,omitempty"`
	Agents    []AgentInfoOutput  `json:"agents"`
	Forwards  []AgentForwardInfo `json:"forwards,omitempty"`
	CreatedAt string             `json:"created_at"`
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

	// Name + status on first line
	statusStr := colorStatus(out, info.Status)
	fmt.Printf("%s  %s\n", out.Bold(info.Name), statusStr)
	fmt.Println()

	// Main info
	fmt.Printf("  %-13s %s\n", "ID", info.ID)
	fmt.Printf("  %-13s %s\n", "Status", info.Status)
	fmt.Printf("  %-13s %s\n", "PVC", info.PVC)
	fmt.Println()

	// Resources
	fmt.Printf("  %-13s %s\n", "CPU", info.CPU)
	fmt.Printf("  %-13s %s\n", "Memory", info.Memory)
	fmt.Printf("  %-13s %s\n", "Storage", info.Storage)

	// Mounts
	if len(info.Mounts) > 0 {
		fmt.Println()
		fmt.Printf("  %s\n", out.Bold("Mounts"))
		for _, m := range info.Mounts {
			mode := "rw"
			if m.ReadOnly {
				mode = "ro"
			}
			hostPath := shortenPath(m.HostPath)
			fmt.Printf("    %s -> %s (%s)\n", hostPath, m.ContainerPath, mode)
		}
	}

	// Agents section
	fmt.Println()
	fmt.Printf("  %s\n", out.Bold("Agents"))
	for _, a := range agents {
		config := agentConfigs[a.AgentName]
		if config == nil {
			config = &agent.Config{}
		}
		printAgentConfigForInfo(a.AgentName, a.AgentType, a.Status, config, out)
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
		fmt.Println()
		fmt.Printf("  %s\n", out.Bold("Forwards"))
		for _, a := range forwards {
			for _, fwd := range a.Forwards {
				fwdType := fwd.Type
				if fwdType == "" {
					fwdType = "manual"
				}
				status := fwd.Status
				if fwd.Error != "" {
					status = fmt.Sprintf("%s (%s)", status, fwd.Error)
				}
				fmt.Printf("    %-10s :%d -> :%d   [%s] %s\n",
					a.Name, fwd.LocalPort, fwd.RemotePort, fwdType, status)
			}
		}
	}

	// Created timestamp
	fmt.Printf("\n  %s\n", out.Dim(fmt.Sprintf("Created %s", info.CreatedAt)))

	return nil
}

// colorStatus returns a colored status string
func colorStatus(out *Output, status string) string {
	switch status {
	case "running":
		return out.Green(status)
	case "stopped":
		return out.Yellow(status)
	case "creating":
		return out.Teal(status)
	default:
		return out.Red(status)
	}
}

func printAgentConfigForInfo(agentName string, agentType agent.Type, status string, config *agent.Config, out *Output) {
	isCodex := agentType == agent.TypeCodex

	// Build key=value pairs for compact display
	var parts []string

	// skip_permissions
	if isCodex {
		parts = append(parts, "skip_permissions=always")
	} else {
		parts = append(parts, fmt.Sprintf("skip_permissions=%v", config.SkipPermissions))
	}

	// model
	model := config.Model
	if model == "" {
		model = "default"
	}
	parts = append(parts, fmt.Sprintf("model=%s", model))

	fmt.Printf("    %-10s %-10s %s\n", agentName, colorStatus(out, status), strings.Join(parts, "  "))
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

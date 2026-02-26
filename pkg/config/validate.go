package config

import (
	"fmt"
	"regexp"
	"strings"
)

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Validate checks the config for invalid values. Returns a multi-error with field paths.
func Validate(cfg *Config) error {
	var errs []string

	// Ports: 1-65535
	validatePort := func(name string, port int) {
		if port < 1 || port > 65535 {
			errs = append(errs, fmt.Sprintf("ports.%s: %d out of range 1-65535", name, port))
		}
	}
	validatePort("ssh", cfg.Ports.SSH)
	validatePort("ttyd", cfg.Ports.TTYD)
	validatePort("permission", cfg.Ports.Permission)
	validatePort("wireguard", cfg.Ports.WireGuard)
	validatePort("management", cfg.Ports.Management)
	validatePort("registration", cfg.Ports.Registration)

	if cfg.Ports.LocalPortMultiplier < 1 {
		errs = append(errs, "ports.local_port_multiplier: must be >= 1")
	}

	// Positive durations
	if cfg.Timeouts.DaemonSocket.Duration <= 0 {
		errs = append(errs, "timeouts.daemon_socket: must be positive")
	}
	if cfg.Timeouts.PermissionHook.Duration <= 0 {
		errs = append(errs, "timeouts.permission_hook: must be positive")
	}
	if cfg.Timeouts.ClusterStartup.Duration <= 0 {
		errs = append(errs, "timeouts.cluster_startup: must be positive")
	}
	if cfg.Timeouts.ClusterPollInterval.Duration <= 0 {
		errs = append(errs, "timeouts.cluster_poll_interval: must be positive")
	}
	if cfg.TUI.Monitor.RefreshInterval.Duration <= 0 {
		errs = append(errs, "tui.monitor.refresh_interval: must be positive")
	}
	if cfg.Network.InviteTokenTTL.Duration <= 0 {
		errs = append(errs, "network.invite_token_ttl: must be positive")
	}

	// Positive integers
	if cfg.TUI.Monitor.HistoryLength < 1 {
		errs = append(errs, "tui.monitor.history_length: must be >= 1")
	}
	if cfg.Cluster.CPU < 1 {
		errs = append(errs, "cluster.cpu: must be >= 1")
	}
	if cfg.Cluster.Memory < 1 {
		errs = append(errs, "cluster.memory: must be >= 1")
	}
	if cfg.Cluster.Disk < 1 {
		errs = append(errs, "cluster.disk: must be >= 1")
	}

	// Deployment strategy
	switch cfg.Kubernetes.DeploymentStrategy {
	case "Recreate", "RollingUpdate":
		// valid
	default:
		errs = append(errs, fmt.Sprintf("kubernetes.deployment_strategy: invalid value %q (must be Recreate or RollingUpdate)", cfg.Kubernetes.DeploymentStrategy))
	}

	// Non-empty strings for critical fields
	if cfg.Kubernetes.Namespace == "" {
		errs = append(errs, "kubernetes.namespace: must not be empty")
	}
	if cfg.DNS.Domain == "" {
		errs = append(errs, "dns.domain: must not be empty")
	}
	if cfg.Images.Claude == "" {
		errs = append(errs, "images.claude: must not be empty")
	}
	if cfg.Images.Codex == "" {
		errs = append(errs, "images.codex: must not be empty")
	}
	if cfg.Images.Init == "" {
		errs = append(errs, "images.init: must not be empty")
	}

	// Hex color format
	validateColor := func(path, color string) {
		if color != "" && !hexColorRe.MatchString(color) {
			errs = append(errs, fmt.Sprintf("%s: invalid hex color %q (must be #XXXXXX)", path, color))
		}
	}
	validateColor("theme.brand.teal", cfg.Theme.Brand.Teal)
	validateColor("theme.brand.pink", cfg.Theme.Brand.Pink)
	validateColor("theme.brand.orange", cfg.Theme.Brand.Orange)
	validateColor("theme.brand.yellow", cfg.Theme.Brand.Yellow)
	validateColor("theme.semantic.success", cfg.Theme.Semantic.Success)
	validateColor("theme.semantic.error", cfg.Theme.Semantic.Error)
	validateColor("theme.semantic.warning", cfg.Theme.Semantic.Warning)
	validateColor("theme.semantic.dim", cfg.Theme.Semantic.Dim)
	validateColor("theme.semantic.muted", cfg.Theme.Semantic.Muted)
	validateColor("theme.semantic.text_light", cfg.Theme.Semantic.TextLight)
	validateColor("theme.semantic.text_dark", cfg.Theme.Semantic.TextDark)
	validateColor("theme.tui_colors.user", cfg.Theme.TUIColors.User)
	validateColor("theme.tui_colors.tool", cfg.Theme.TUIColors.Tool)
	validateColor("theme.tui_colors.timestamp", cfg.Theme.TUIColors.Timestamp)
	validateColor("theme.tui_colors.code_bg", cfg.Theme.TUIColors.CodeBg)
	validateColor("theme.tui_colors.code_fg", cfg.Theme.TUIColors.CodeFg)
	validateColor("theme.tui_colors.thinking", cfg.Theme.TUIColors.Thinking)
	for i, c := range cfg.Theme.AgentPalette {
		validateColor(fmt.Sprintf("theme.agent_palette[%d]", i), c)
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

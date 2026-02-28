// Package config provides a unified YAML configuration system for vibespace.
// It supports a priority chain: defaults -> YAML file -> env vars -> CLI flags.
package config

import (
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration for human-readable YAML marshaling (e.g., "30m", "10s").
type Duration struct {
	time.Duration
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// Config is the top-level configuration for vibespace.
type Config struct {
	Images     ImagesConfig     `yaml:"images"`
	Resources  ResourcesConfig  `yaml:"resources"`
	Agent      AgentConfig      `yaml:"agent"`
	Cluster    ClusterConfig    `yaml:"cluster"`
	Ports      PortsConfig      `yaml:"ports"`
	Network    NetworkConfig    `yaml:"network"`
	DNS        DNSConfig        `yaml:"dns"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Timeouts   TimeoutsConfig   `yaml:"timeouts"`
	TUI        TUIConfig        `yaml:"tui"`
	Theme      ThemeConfig      `yaml:"theme"`
	GitHub     GitHubConfig     `yaml:"github"`
}

// GitHubConfig holds GitHub App OAuth settings.
type GitHubConfig struct {
	ClientID string `yaml:"client_id"` // GitHub App client_id for OAuth device flow
}

// ImagesConfig holds container image references.
type ImagesConfig struct {
	Claude string `yaml:"claude"`
	Codex  string `yaml:"codex"`
	Init   string `yaml:"init"`
}

// ResourcesConfig holds default resource requests/limits.
type ResourcesConfig struct {
	CPU         string `yaml:"cpu"`
	CPULimit    string `yaml:"cpu_limit"`
	Memory      string `yaml:"memory"`
	MemoryLimit string `yaml:"memory_limit"`
	Storage     string `yaml:"storage"`
}

// AgentConfig holds agent behavior defaults.
type AgentConfig struct {
	AllowedTools     []string       `yaml:"allowed_tools"`
	SkipPermissions  bool           `yaml:"skip_permissions"`
	ShareCredentials bool           `yaml:"share_credentials"`
	Model            string         `yaml:"model"`
	MaxTurns         int            `yaml:"max_turns"`
	Prefixes         PrefixesConfig `yaml:"prefixes"`
}

// PrefixesConfig holds agent naming prefixes.
type PrefixesConfig struct {
	Claude string `yaml:"claude"`
	Codex  string `yaml:"codex"`
}

// ClusterConfig holds cluster VM resource defaults.
type ClusterConfig struct {
	CPU    int `yaml:"cpu"`
	Memory int `yaml:"memory"`
	Disk   int `yaml:"disk"`
}

// PortsConfig holds port number defaults.
type PortsConfig struct {
	SSH                 int `yaml:"ssh"`
	TTYD                int `yaml:"ttyd"`
	Permission          int `yaml:"permission"`
	WireGuard           int `yaml:"wireguard"`
	Management          int `yaml:"management"`
	Registration        int `yaml:"registration"`
	LocalPortMultiplier int `yaml:"local_port_multiplier"`
}

// NetworkConfig holds WireGuard network defaults.
type NetworkConfig struct {
	ServerIP       string   `yaml:"server_ip"`
	ClientIPStart  int      `yaml:"client_ip_start"`
	InviteTokenTTL Duration `yaml:"invite_token_ttl"`
}

// DNSConfig holds DNS defaults.
type DNSConfig struct {
	Domain string `yaml:"domain"`
}

// KubernetesConfig holds Kubernetes defaults.
type KubernetesConfig struct {
	Namespace          string   `yaml:"namespace"`
	DeploymentStrategy string   `yaml:"deployment_strategy"`
	InitContainerUID   int64    `yaml:"init_container_uid"`
	InitContainerMode  int      `yaml:"init_container_mode"`
	Capabilities       []string `yaml:"capabilities"`
}

// TimeoutsConfig holds timeout defaults.
type TimeoutsConfig struct {
	DaemonSocket        Duration `yaml:"daemon_socket"`
	PermissionHook      Duration `yaml:"permission_hook"`
	ClusterStartup      Duration `yaml:"cluster_startup"`
	ClusterPollInterval Duration `yaml:"cluster_poll_interval"`
}

// TUIConfig holds TUI behavior defaults.
type TUIConfig struct {
	Monitor     MonitorConfig `yaml:"monitor"`
	SyntaxTheme string        `yaml:"syntax_theme"`
}

// MonitorConfig holds monitor tab defaults.
type MonitorConfig struct {
	RefreshInterval Duration `yaml:"refresh_interval"`
	HistoryLength   int      `yaml:"history_length"`
}

// ThemeConfig holds all color theme settings.
type ThemeConfig struct {
	Brand        BrandColors    `yaml:"brand"`
	Semantic     SemanticColors `yaml:"semantic"`
	TUIColors    TUIColors      `yaml:"tui_colors"`
	AgentPalette []string       `yaml:"agent_palette"`
}

// BrandColors holds the brand identity colors.
type BrandColors struct {
	Teal   string `yaml:"teal"`
	Pink   string `yaml:"pink"`
	Orange string `yaml:"orange"`
	Yellow string `yaml:"yellow"`
}

// SemanticColors holds semantic/functional colors.
type SemanticColors struct {
	Success   string `yaml:"success"`
	Error     string `yaml:"error"`
	Warning   string `yaml:"warning"`
	Dim       string `yaml:"dim"`
	Muted     string `yaml:"muted"`
	TextLight string `yaml:"text_light"`
	TextDark  string `yaml:"text_dark"`
}

// TUIColors holds TUI-specific colors.
type TUIColors struct {
	User      string `yaml:"user"`
	Tool      string `yaml:"tool"`
	Timestamp string `yaml:"timestamp"`
	CodeBg    string `yaml:"code_bg"`
	CodeFg    string `yaml:"code_fg"`
	Thinking  string `yaml:"thinking"`
}

// Default returns a Config populated with all current hardcoded defaults.
func Default() *Config {
	return &Config{
		Images: ImagesConfig{
			Claude: "ghcr.io/vibespacehq/vibespace/claude-code:latest",
			Codex:  "ghcr.io/vibespacehq/vibespace/codex:latest",
			Init:   "busybox:latest",
		},
		Resources: ResourcesConfig{
			CPU:         "250m",
			CPULimit:    "1000m",
			Memory:      "512Mi",
			MemoryLimit: "1Gi",
			Storage:     "10Gi",
		},
		Agent: AgentConfig{
			AllowedTools: []string{
				"Bash(read_only:true)",
				"Read",
				"Write",
				"Edit",
				"Glob",
				"Grep",
			},
			SkipPermissions:  false,
			ShareCredentials: false,
			Model:            "",
			Prefixes: PrefixesConfig{
				Claude: "claude",
				Codex:  "codex",
			},
		},
		Cluster: ClusterConfig{
			CPU:    4,
			Memory: 8,
			Disk:   60,
		},
		Ports: PortsConfig{
			SSH:                 22,
			TTYD:                7681,
			Permission:          18080,
			WireGuard:           51820,
			Management:          7780,
			Registration:        7781,
			LocalPortMultiplier: 10000,
		},
		Network: NetworkConfig{
			ServerIP:       "10.100.0.1",
			ClientIPStart:  2,
			InviteTokenTTL: Duration{30 * time.Minute},
		},
		DNS: DNSConfig{
			Domain: "vibespace.internal",
		},
		Kubernetes: KubernetesConfig{
			Namespace:          "vibespace",
			DeploymentStrategy: "Recreate",
			InitContainerUID:   1000,
			InitContainerMode:  755,
			Capabilities: []string{
				"CHOWN",
				"DAC_OVERRIDE",
				"FOWNER",
				"SETUID",
				"SETGID",
				"NET_BIND_SERVICE",
				"KILL",
				"SYS_CHROOT",
				"AUDIT_WRITE",
			},
		},
		Timeouts: TimeoutsConfig{
			DaemonSocket:        Duration{10 * time.Second},
			PermissionHook:      Duration{300 * time.Second},
			ClusterStartup:      Duration{10 * time.Minute},
			ClusterPollInterval: Duration{5 * time.Second},
		},
		TUI: TUIConfig{
			Monitor: MonitorConfig{
				RefreshInterval: Duration{5 * time.Second},
				HistoryLength:   60,
			},
			SyntaxTheme: "monokai",
		},
		GitHub: GitHubConfig{
			ClientID: "Iv23lig2ukmBeQeInhOF",
		},
		Theme: ThemeConfig{
			Brand: BrandColors{
				Teal:   "#00ABAB",
				Pink:   "#F102F3",
				Orange: "#FF7D4B",
				Yellow: "#F5F50A",
			},
			Semantic: SemanticColors{
				Success:   "#00ABAB",
				Error:     "#FF4D4D",
				Warning:   "#FF7D4B",
				Dim:       "#666666",
				Muted:     "#444444",
				TextLight: "#1a1a1a",
				TextDark:  "#FFFFFF",
			},
			TUIColors: TUIColors{
				User:      "#00FF9F",
				Tool:      "#FF7D4B",
				Timestamp: "#555555",
				CodeBg:    "#1a1a2e",
				CodeFg:    "#87CEEB",
				Thinking:  "#F102F3",
			},
			AgentPalette: []string{
				"#F102F3",
				"#FF7D4B",
				"#00D9FF",
				"#7B61FF",
				"#F5F50A",
				"#00FF9F",
				"#FF6B6B",
			},
		},
	}
}

// Singleton
var (
	global   *Config
	globalMu sync.RWMutex
)

// Global returns the global config. If Load() hasn't been called, returns Default().
func Global() *Config {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if global == nil {
		return Default()
	}
	return global
}

// SetGlobal sets the global config.
func SetGlobal(cfg *Config) {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = cfg
}

# Configuration

## Config file

vibespace uses a YAML config file at `~/.vibespace/config.yaml`. You only need to include the values you want to override — everything else uses built-in defaults.

```bash
# Use the sample config as a starting point
cp config.sample.yaml ~/.vibespace/config.yaml
```

Alternatively, specify a custom path:

```bash
vibespace --config /path/to/config.yaml status
# or
export VIBESPACE_CONFIG=/path/to/config.yaml
```

### Priority chain

Values are resolved in this order (last wins):

1. Built-in defaults
2. Config file (`~/.vibespace/config.yaml`)
3. Environment variables (`VIBESPACE_*`)
4. CLI flags (`--cpu`, `--memory`, etc.)

### Minimal example

A config file only needs the values you want to change:

```yaml
resources:
  cpu: "500m"
  cpu_limit: "2000m"
  memory: "1Gi"
  memory_limit: "4Gi"
  storage: "20Gi"
```

### Full reference

See [`config.sample.yaml`](../config.sample.yaml) for every available option with comments and defaults.

#### Images

```yaml
images:
  claude: "ghcr.io/vibespacehq/vibespace/claude-code:latest"
  codex: "ghcr.io/vibespacehq/vibespace/codex:latest"
  init: "busybox:latest"
```

#### Resources

Default requests/limits for new vibespace pods.

```yaml
resources:
  cpu: "250m"           # CPU request (scheduling guarantee)
  cpu_limit: "1000m"    # CPU limit (max burst)
  memory: "512Mi"       # Memory request
  memory_limit: "1Gi"   # Memory limit
  storage: "10Gi"       # Persistent volume size
```

| Resource | Config key | Env var | CLI flag |
|---|---|---|---|
| CPU request | `resources.cpu` | `VIBESPACE_DEFAULT_CPU` | `--cpu` |
| CPU limit | `resources.cpu_limit` | `VIBESPACE_DEFAULT_CPU_LIMIT` | `--cpu-limit` |
| Memory request | `resources.memory` | `VIBESPACE_DEFAULT_MEMORY` | `--memory` |
| Memory limit | `resources.memory_limit` | `VIBESPACE_DEFAULT_MEMORY_LIMIT` | `--memory-limit` |
| Storage | `resources.storage` | `VIBESPACE_DEFAULT_STORAGE` | `--storage` |

#### Agent defaults

```yaml
agent:
  allowed_tools:
    - "Bash"
    - "Read"
    - "Write"
    - "Edit"
    - "Glob"
    - "Grep"
  skip_permissions: false
  share_credentials: false
  model: ""
  prefixes:
    claude: "claude"
    codex: "codex"
```

#### Cluster

VM resources for `vibespace init`.

```yaml
cluster:
  cpu: 4        # CPU cores
  memory: 8     # Memory in GB
  disk: 60      # Disk in GB
```

| Setting | Config key | Env var | CLI flag |
|---|---|---|---|
| CPU cores | `cluster.cpu` | `VIBESPACE_CLUSTER_CPU` | `--cpu` |
| Memory (GB) | `cluster.memory` | `VIBESPACE_CLUSTER_MEMORY` | `--memory` |
| Disk (GB) | `cluster.disk` | `VIBESPACE_CLUSTER_DISK` | `--disk` |

#### Ports

```yaml
ports:
  ssh: 22
  ttyd: 7681
  permission: 18080
  wireguard: 51820
  management: 7780
  registration: 7781
  local_port_multiplier: 10000
```

#### Network (remote mode)

```yaml
network:
  server_ip: "10.100.0.1"
  client_ip_start: 2
  invite_token_ttl: "30m"
```

#### DNS

```yaml
dns:
  domain: "vibespace.internal"
```

#### Kubernetes

```yaml
kubernetes:
  namespace: "vibespace"
  deployment_strategy: "Recreate"   # or "RollingUpdate"
  init_container_uid: 1000
  init_container_mode: 755
  capabilities:
    - CHOWN
    - DAC_OVERRIDE
    - FOWNER
    - SETUID
    - SETGID
    - NET_BIND_SERVICE
    - KILL
    - SYS_CHROOT
    - AUDIT_WRITE
```

#### Timeouts

```yaml
timeouts:
  daemon_socket: "10s"
  permission_hook: "300s"
  cluster_startup: "10m"
  cluster_poll_interval: "5s"
```

#### TUI

```yaml
tui:
  monitor:
    refresh_interval: "5s"
    history_length: 60
  syntax_theme: "monokai"
```

#### Theme

Customize all colors in hex format.

```yaml
theme:
  brand:
    teal: "#00ABAB"
    pink: "#F102F3"
    orange: "#FF7D4B"
    yellow: "#F5F50A"
  semantic:
    success: "#00ABAB"
    error: "#FF4D4D"
    warning: "#FF7D4B"
    dim: "#666666"
    muted: "#444444"
    text_light: "#1a1a1a"
    text_dark: "#FFFFFF"
  tui_colors:
    user: "#00FF9F"
    tool: "#FF7D4B"
    timestamp: "#555555"
    code_bg: "#1a1a2e"
    code_fg: "#87CEEB"
    thinking: "#F102F3"
  agent_palette:
    - "#F102F3"
    - "#FF7D4B"
    - "#00D9FF"
    - "#7B61FF"
    - "#F5F50A"
    - "#00FF9F"
    - "#FF6B6B"
```

## Agent configuration

Each agent has settings you can view and change at runtime:

```bash
# Show config for all agents
vibespace config show --vibespace my-project

# Show config for a specific agent
vibespace config show claude-1 --vibespace my-project

# Change settings
vibespace config set claude-1 --vibespace my-project --model sonnet --max-turns 50
```

### Available settings

| Setting | Flag | Applies to | Description |
|---|---|---|---|
| Model | `--model` | Both | Which model the agent uses |
| Max turns | `--max-turns` | Both | Limit conversation turns (0 = unlimited) |
| System prompt | `--system-prompt` | Both | Custom system prompt |
| Skip permissions | `--skip-permissions` | Claude | Run without asking for tool approval |
| Allowed tools | `--allowed-tools` | Claude | Comma-separated whitelist |
| Disallowed tools | `--disallowed-tools` | Claude | Comma-separated blacklist |
| Reasoning effort | `--reasoning-effort` | Codex | `low`, `medium`, `high`, `xhigh` |

## Resource allocation

Set resource limits when creating a vibespace:

```bash
vibespace create my-project -t claude-code \
  --cpu 500m --cpu-limit 2000m \
  --memory 1Gi --memory-limit 2Gi \
  --storage 20Gi
```

**Requests** (e.g. `--cpu`, `--memory`) control scheduling — how much the Kubernetes scheduler reserves for the pod. **Limits** (e.g. `--cpu-limit`, `--memory-limit`) cap the maximum the pod can use.

Setting requests lower than limits allows overcommit: pods are scheduled with less overhead but can burst when they need to. This is useful for running many agents since most are idle at any given time.

### Agent density guidelines

How many agents you can run depends on your cluster size and resource requests:

| Cluster | CPU Request | Memory Request | Approximate Max Agents |
|---|---|---|---|
| 4 CPU / 8GB | 250m | 512Mi | ~16 |
| 4 CPU / 8GB | 100m | 256Mi | ~30 |
| 8 CPU / 16GB | 100m | 256Mi | ~60 |
| 16 CPU / 32GB | 50m | 128Mi | ~200 |

These assume most agents are idle. Active agents use more CPU and memory — plan for your expected concurrency.

## Cluster sizing

Control the cluster VM resources at init time:

```bash
vibespace init --cpu 8 --memory 16 --disk 100
```

## Debug logging

```bash
# Log to ~/.vibespace/debug.log
VIBESPACE_DEBUG=1 vibespace status

# Set log level
VIBESPACE_LOG_LEVEL=debug vibespace create my-project -t claude-code
```

## Output control

| Variable | Effect |
|---|---|
| `NO_COLOR` | Disable all color output |
| Non-TTY stdout | Automatically switches to JSON output |

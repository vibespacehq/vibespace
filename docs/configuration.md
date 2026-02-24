# Configuration

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

### Defaults

| Resource | Request | Limit |
|---|---|---|
| CPU | 250m | 1000m |
| Memory | 512Mi | 1Gi |
| Storage | 10Gi | — |

Override defaults with environment variables:

```bash
export VIBESPACE_DEFAULT_CPU=100m
export VIBESPACE_DEFAULT_CPU_LIMIT=2000m
export VIBESPACE_DEFAULT_MEMORY=256Mi
export VIBESPACE_DEFAULT_MEMORY_LIMIT=2Gi
export VIBESPACE_DEFAULT_STORAGE=20Gi
```

## Cluster sizing

Control the cluster VM resources at init time:

```bash
vibespace init --cpu 8 --memory 16 --disk 100
```

Or with environment variables:

```bash
export VIBESPACE_CLUSTER_CPU=8
export VIBESPACE_CLUSTER_MEMORY=16
export VIBESPACE_CLUSTER_DISK=100
```

### Agent density guidelines

How many agents you can run depends on your cluster size and resource requests:

| Cluster | CPU Request | Memory Request | Approximate Max Agents |
|---|---|---|---|
| 4 CPU / 8GB | 250m | 512Mi | ~16 |
| 4 CPU / 8GB | 100m | 256Mi | ~30 |
| 8 CPU / 16GB | 100m | 256Mi | ~60 |
| 16 CPU / 32GB | 50m | 128Mi | ~200 |

These assume most agents are idle. Active agents use more CPU and memory — plan for your expected concurrency.

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

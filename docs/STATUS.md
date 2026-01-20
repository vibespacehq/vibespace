# Implementation Status

## Legend
- **Done**: Fully implemented
- **Partial**: Some functionality works, incomplete
- **TODO**: Not started

---

## Cluster Management

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace init` | Done | Downloads Colima/Lima/kubectl, starts cluster, creates namespace. Supports `--cpu`, `--memory`, `--disk` flags. Configurable via `VIBESPACE_CLUSTER_*` env vars. |
| `vibespace status` | Done | Shows Colima and namespace status |
| `vibespace stop` | Done | Stops Colima |
| `vibespace serve` | TODO | WireGuard server mode for remote clients |

## Vibespace Management

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace create <name>` | Done | Creates Deployment + Service + PVC + SSH key secret. Supports `--cpu`, `--memory`, `--storage` flags. Configurable via `VIBESPACE_DEFAULT_*` env vars. |
| `vibespace list` | Done | Lists all vibespaces |
| `vibespace delete <name>` | Done | Deletes Deployment + Service + PVC |
| `vibespace <name> start` | Done | Scales deployment to 1 replica |
| `vibespace <name> stop` | Done | Scales deployment to 0 replicas |
| `vibespace <name> status` | TODO | Per-vibespace detailed status |

## Agent Management

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace <name> agents` | Done | Lists all agents with status from Kubernetes |
| `vibespace <name> spawn` | Done | Creates additional Deployment for new agent (shared PVC) |
| `vibespace <name> kill <agent>` | Done | Deletes agent's Deployment |
| `vibespace <name> fork` | TODO | Fork vibespace with cloned filesystem |

## Connection

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace <name> connect [agent]` | Done | **SSH pivot**: Uses native `ssh` command with dedicated keypair (~/.vibespace/ssh/). Supports `--browser` flag for ttyd fallback. |
| `vibespace <name> ports` | Partial | Shows detected ports via kubectl exec. Works but uses kubectl instead of client-go. |

## Port Forwarding Daemon

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace <name> up` | Done | Starts background daemon, forwards SSH (port 22) + ttyd (port 7681) for all agents |
| `vibespace <name> down` | Done | Stops daemon via Unix socket shutdown command |
| `vibespace <name> forward list` | Done | Lists all forwards via daemon IPC |
| `vibespace <name> forward add <port>` | Done | Adds forward via daemon IPC. Supports `--agent`, `--local` flags. |
| `vibespace <name> forward remove <port>` | Done | Removes forward via daemon IPC |
| `vibespace <name> forward stop <port>` | Done | Temporarily stops forward |
| `vibespace <name> forward start <port>` | Done | Restarts stopped forward |
| `vibespace <name> forward restart <port>` | Done | Restarts specific forward |
| `vibespace <name> forward restart-all` | Done | Restarts all forwards |

## Multi-Session

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace multi <vibespace>` | Partial | TUI with SSH + Claude print mode (`claude -p --output-format stream-json`). Implements `/focus` for interactive mode. Single vibespace only, multi-vibespace support pending. |
| `vibespace session create` | TODO | |
| `vibespace session list` | TODO | |
| `vibespace session delete` | TODO | |
| `vibespace session add` | TODO | |
| `vibespace session remove` | TODO | |
| `vibespace session start` | TODO | |

## Remote Mode

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace remote connect` | TODO | WireGuard client |
| `vibespace remote disconnect` | TODO | |
| `vibespace remote status` | TODO | |

## Utility

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace version` | Done | |
| `vibespace help` | Done | |
| `vibespace <name>` (subcommand help) | Done | |

---

## Core Packages

| Package | Status | Notes |
|---------|--------|-------|
| `pkg/k8s` | Done | Kubernetes client wrapper |
| `pkg/vibespace` | Done | Business logic, SSH key management |
| `pkg/model` | Done | Data models |
| `pkg/image` | Done | Container image (sshd + ttyd + Claude Code) |
| `internal/platform` | Done | Colima/k3s management with configurable resources |
| `internal/cli/logging.go` | Done | Centralized slog config, rotation, debug mode |
| `pkg/portforward` | Done | client-go based port-forward manager with auto-reconnect |
| `pkg/daemon` | Done | Background daemon with Unix socket IPC |
| `pkg/session` | Partial | Session state management, needs multi-vibespace support |
| `pkg/tui` | Partial | bubbletea TUI with SSH + Claude print mode, `/focus` for interactive |
| `pkg/wireguard` | TODO | WireGuard integration |

---

## Container Image

The vibespace container runs:
- **sshd** on port 22 (primary CLI access)
- **ttyd** on port 7681 (browser fallback)
- **Claude Code CLI** as the default shell

SSH keys are:
- Generated per-machine in `~/.vibespace/ssh/` (ed25519)
- Injected into containers via Kubernetes Secret
- Used automatically by `vibespace connect`

---

## Environment Variables

### Logging
| Variable | Default | Description |
|----------|---------|-------------|
| `VIBESPACE_DEBUG` | (unset) | Enable debug logging (any non-empty value) |
| `VIBESPACE_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

### Vibespace Pod Resources
| Variable | Default | Description |
|----------|---------|-------------|
| `VIBESPACE_DEFAULT_CPU` | `400m` | CPU request/limit |
| `VIBESPACE_DEFAULT_MEMORY` | `256Mi` | Memory request/limit |
| `VIBESPACE_DEFAULT_STORAGE` | `10Gi` | PVC storage size |

### Cluster VM Resources (Colima)
| Variable | Default | Description |
|----------|---------|-------------|
| `VIBESPACE_CLUSTER_CPU` | `4` | CPU cores |
| `VIBESPACE_CLUSTER_MEMORY` | `8` | Memory in GB |
| `VIBESPACE_CLUSTER_DISK` | `60` | Disk in GB |

---

## Next Priority

1. **Per-vibespace status command**
   - Show detailed status, resource usage, agents

2. **Multi-session TUI improvements**
   - Support multiple vibespaces in single TUI session
   - Implement session persistence (session create/list/delete/add/remove)
   - Wire up remaining TUI commands (/add runtime support)

3. **Fork command**
   - Snapshot PVC and create new vibespace

4. **Remote mode**
   - WireGuard server (`serve`)
   - WireGuard client (`remote connect/disconnect/status`)

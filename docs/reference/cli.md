# CLI Reference

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--help` | `-h` | | Show help for any command |
| `--json` | | `false` | Output in JSON format (auto-enabled in non-TTY) |
| `--verbose` | `-v` | `false` | Enable verbose output |
| `--quiet` | `-q` | `false` | Suppress non-essential output |
| `--no-color` | | `false` | Disable colored output |
| `--plain` | | `false` | Plain output for scripting (tab-separated) |
| `--header` | | `false` | Include column headers in plain output |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `VIBESPACE_DEBUG=1` | Enable debug logging to `~/.vibespace/debug.log` |
| `VIBESPACE_LOG_LEVEL` | Log level: `debug`, `info`, `warn`, `error` |
| `VIBESPACE_CLUSTER_CPU` | Default cluster CPU cores (default: 4) |
| `VIBESPACE_CLUSTER_MEMORY` | Default cluster memory in GB (default: 8) |
| `VIBESPACE_CLUSTER_DISK` | Default cluster disk in GB (default: 60) |
| `VIBESPACE_DEFAULT_CPU` | Default vibespace CPU request (default: 250m) |
| `VIBESPACE_DEFAULT_CPU_LIMIT` | Default vibespace CPU limit (default: 1000m) |
| `VIBESPACE_DEFAULT_MEMORY` | Default vibespace memory request (default: 512Mi) |
| `VIBESPACE_DEFAULT_MEMORY_LIMIT` | Default vibespace memory limit (default: 1Gi) |
| `VIBESPACE_DEFAULT_STORAGE` | Default vibespace storage (default: 10Gi) |
| `NO_COLOR` | Disable colored output (standard convention) |

---

## Cluster Commands

### `vibespace init`

Initialize the vibespace cluster.

| Flag | Default | Description |
|------|---------|-------------|
| `--external` | `false` | Use an external Kubernetes cluster |
| `--kubeconfig` | `""` | Path to external kubeconfig |
| `--cpu` | `4` | CPU cores for cluster VM (env: `VIBESPACE_CLUSTER_CPU`) |
| `--memory` | `8` | Memory in GB for cluster VM (env: `VIBESPACE_CLUSTER_MEMORY`) |
| `--disk` | `60` | Disk in GB for cluster VM (env: `VIBESPACE_CLUSTER_DISK`) |

### `vibespace status`

Show cluster, daemon, and remote connection status. No command-specific flags.

### `vibespace stop`

Stop the cluster. Redirects to `vibespace remote disconnect` when in remote mode. No command-specific flags.

### `vibespace uninstall`

Remove vibespace and all cluster data. Interactive confirmation prompt. No command-specific flags.

---

## Vibespace Commands

### `vibespace create <name>`

Create a new vibespace.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--agent-type` | `-t` | (required) | Agent type: `claude-code`, `codex` |
| `--name` | `-n` | `""` | Custom name for primary agent |
| `--repo` | | `""` | GitHub repository to clone |
| `--mount` | `-m` | | Mount host directory (`host:container[:ro]`), repeatable |
| `--share-credentials` | `-s` | `false` | Share credentials across agents |
| `--skip-permissions` | | `false` | Enable `--dangerously-skip-permissions` |
| `--allowed-tools` | | `""` | Comma-separated allowed tools |
| `--disallowed-tools` | | `""` | Comma-separated disallowed tools |
| `--model` | | `""` | Claude model to use |
| `--max-turns` | | `0` | Maximum conversation turns |
| `--cpu` | | `"250m"` | CPU request (env: `VIBESPACE_DEFAULT_CPU`) |
| `--cpu-limit` | | `"1000m"` | CPU limit (env: `VIBESPACE_DEFAULT_CPU_LIMIT`) |
| `--memory` | | `"512Mi"` | Memory request (env: `VIBESPACE_DEFAULT_MEMORY`) |
| `--memory-limit` | | `"1Gi"` | Memory limit (env: `VIBESPACE_DEFAULT_MEMORY_LIMIT`) |
| `--storage` | | `"10Gi"` | Storage size (env: `VIBESPACE_DEFAULT_STORAGE`) |

### `vibespace list`

List all vibespaces. No command-specific flags.

**Plain columns**: NAME, STATUS, AGENTS, CPU, MEMORY, STORAGE, CREATED

### `vibespace delete <name>`

Delete a vibespace.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--force` | `-f` | `false` | Skip confirmation prompt |
| `--keep-data` | | `false` | Preserve persistent storage (PVC) |
| `--dry-run` | `-n` | `false` | Show what would be deleted |

### `vibespace <name> info`

Show vibespace details, mounts, and agents with config. No command-specific flags.

**Plain columns**: KEY, VALUE

---

## Agent Commands

All prefixed with `vibespace <name>`.

### `<name> agent list`

List agents. Default when running `<name> agent` with no subcommand.

**Plain columns**: AGENT, TYPE, VIBESPACE, STATUS

### `<name> agent create`

Create a new agent in the vibespace.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--agent-type` | `-t` | (required) | Agent type: `claude-code`, `codex` |
| `--name` | `-n` | `""` | Custom agent name |
| `--share-credentials` | `-s` | `false` | Share credentials across agents |
| `--skip-permissions` | | `false` | Enable `--dangerously-skip-permissions` |
| `--allowed-tools` | | `""` | Comma-separated allowed tools |
| `--disallowed-tools` | | `""` | Comma-separated disallowed tools |
| `--model` | | `""` | Model to use |
| `--max-turns` | | `0` | Maximum conversation turns |

### `<name> agent delete <agent>`

Delete an agent. No flags.

### `<name> start [agent]`

Start all agents or a specific agent. No flags.

### `<name> stop [agent]`

Stop all agents or a specific agent. No flags.

### `<name> connect [agent]`

Connect to an agent (interactive terminal). Requires TTY.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--browser` | `-b` | `false` | Open in web browser instead of terminal |
| `--agent` | `-a` | `""` | Specify agent name |

### `<name> exec [agent] <command>`

Execute a command in an agent container via SSH. No command-specific flags.

---

## Config Commands

### `<name> config show [agent]`

Show agent configuration. No command-specific flags.

**Plain columns**: AGENT, TYPE, SKIP_PERMISSIONS, MODEL, MAX_TURNS, REASONING_EFFORT

### `<name> config set <agent>`

Update agent configuration.

| Flag | Default | Description |
|------|---------|-------------|
| `--skip-permissions` | | Enable `--dangerously-skip-permissions` (Claude only) |
| `--no-skip-permissions` | | Disable `--dangerously-skip-permissions` (Claude only) |
| `--allowed-tools` | `""` | Comma-separated allowed tools (Claude only) |
| `--disallowed-tools` | `""` | Comma-separated disallowed tools (Claude only) |
| `--model` | `""` | Model to use |
| `--max-turns` | `0` | Maximum conversation turns |
| `--system-prompt` | `""` | Custom system prompt |
| `--reasoning-effort` | `""` | Reasoning effort: low, medium, high, xhigh (Codex only) |

---

## Port Forwarding Commands

### `<name> forward list`

List active port forwards. Default when running `<name> forward` with no subcommand.

**Plain columns**: AGENT, LOCAL, REMOTE, TYPE, STATUS

### `<name> forward add <port>`

Add a port forward.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--agent` | `-a` | `"claude-1"` | Agent to forward from |
| `--local` | `-l` | auto | Local port to use |

### `<name> forward remove <port>`

Remove a port forward.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--agent` | `-a` | `"claude-1"` | Agent to remove forward from |

### `<name> ports`

Detect listening ports inside agent containers. No command-specific flags.

**Plain columns**: PORT, PROCESS, DETECTED_AT

---

## Remote Commands

### `vibespace serve`

Start the remote mode server.

| Flag | Default | Description |
|------|---------|-------------|
| `--generate-token` | `false` | Generate an invite token for a client |
| `--endpoint` | `""` | Public endpoint for clients (override auto-detection) |
| `--foreground` | `false` | Run in foreground (don't daemonize) |
| `--token-ttl` | `30m` | Invite token time-to-live |
| `--list-clients` | `false` | List all registered clients |
| `--remove-client` | `""` | Remove a client by name, hostname, or public key |

### `vibespace remote connect <token>`

Connect to a remote server using an invite token. Requires sudo for WireGuard operations. No flags.

### `vibespace remote disconnect`

Disconnect from the remote server. No flags.

### `vibespace remote status`

Show remote connection status and run diagnostics. No flags.

### `vibespace remote watch`

Watch and auto-reconnect the remote tunnel. Blocks until SIGINT/SIGTERM. No flags.

---

## Session Commands

### `vibespace session list`

List all multi-agent sessions.

**Plain columns**: NAME, VIBESPACES, AGENTS, LAST_USED

### `vibespace session show <name>`

Show session details. No flags.

### `vibespace session delete <name>`

Delete a session. No flags.

---

## Multi (Headless) Command

### `vibespace multi [message]`

Start or interact with multi-agent sessions.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--resume` | `-r` | `""` | Resume a session (picker if no ID) |
| `--vibespaces` | | | Vibespaces to include (all agents) |
| `--agents` | | | Specific agents (`agent@vibespace`) |
| `--name` | | `""` | Session name (default: auto UUID) |
| `--agent` | | `"all"` | Target agent for non-interactive mode |
| `--batch` | | `false` | Batch mode: read JSONL from stdin |
| `--list-agents` | | `false` | List connected agents and exit |
| `--list-sessions` | | `false` | List available sessions and exit |
| `--stream` | | `false` | Stream responses as plain text |
| `--timeout` | | `2m` | Response timeout for non-interactive mode |

**Plain columns (--list-sessions)**: SESSION, VIBESPACES, LAST USED
**Plain columns (--list-agents)**: SESSION, AGENT

---

## Utility Commands

### `vibespace version`

Print version information (version, commit, build date). No flags.

### `vibespace completion <shell>`

Generate shell completion script (bash, zsh, fish, powershell). No flags.

---

## Internal Commands

### `vibespace daemon`

Hidden. Runs the port-forward daemon (spawned by `vibespace init`). Manages port forwarding, DNS resolution, pod watching, and reconciliation. Not intended for direct use.

---

## Dynamic Vibespace Help

Running `vibespace <name>` or `vibespace <name> --help` shows available subcommands:

```
Vibespace: <name>

Available commands:
  info       Show vibespace details
  agent      Manage agents (list, create, delete)
  connect    Connect to an agent
  exec       Run command in agent container
  config     View/modify agent configuration
  multi      Multi-agent terminal mode
  ports      List detected ports
  start      Start agents
  stop       Stop agents
  forward    Manage port-forwards (list, add, remove)
```

Unknown subcommands trigger a "Did you mean?" suggestion.

---

## Output Modes

| Mode | Flag | Auto-Enable | Description |
|------|------|-------------|-------------|
| Human | (default) | TTY detected | Styled tables, colors, spinners |
| JSON | `--json` | Non-TTY | Standard envelope: `{success, data, error, meta}` |
| Plain | `--plain` | Never | Tab-separated, `--header` for column names |
| Stream | `--stream` | Never | Plain text, line-by-line (multi only) |

### JSON Envelope

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": {
    "schema_version": "1",
    "cli_version": "dev",
    "timestamp": "2026-01-28T00:00:00Z"
  }
}
```

### Exit Codes

| Code | Constant | Used For |
|------|----------|----------|
| 0 | `ExitSuccess` | Success |
| 1 | `ExitInternal` | Internal errors |
| 2 | `ExitUsage` | Invalid args/flags |
| 10 | `ExitNotFound` | Resource not found |
| 11 | `ExitConflict` | Already exists |
| 12 | `ExitPermission` | Permission denied |
| 13 | `ExitTimeout` | Timeout |
| 14 | `ExitCancelled` | User cancelled |
| 15 | `ExitUnavailable` | Cluster/daemon down |

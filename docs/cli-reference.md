# CLI Reference

## Global Flags

These work on every command:

| Flag | Short | Description |
|---|---|---|
| `--json` | | JSON output. Auto-enabled when stdout is not a TTY. |
| `--plain` | | Tab-separated output for scripting. Add `--header` for column names. |
| `--verbose` | `-v` | Verbose output |
| `--quiet` | `-q` | Suppress non-essential output |
| `--no-color` | | Disable colors |
| `--help` | `-h` | Help for any command |

## Environment Variables

| Variable | Description |
|---|---|
| `VIBESPACE_NAME` | Default vibespace (used when `--vibespace` is omitted) |
| `VIBESPACE_DEBUG=1` | Write debug logs to `~/.vibespace/debug.log` |
| `VIBESPACE_LOG_LEVEL` | `debug`, `info`, `warn`, or `error` |
| `VIBESPACE_CLUSTER_CPU` | Default CPU cores for cluster VM (default: 4) |
| `VIBESPACE_CLUSTER_MEMORY` | Default RAM in GB for cluster VM (default: 8) |
| `VIBESPACE_CLUSTER_DISK` | Default disk in GB for cluster VM (default: 60) |
| `VIBESPACE_DEFAULT_CPU` | Default agent CPU request (default: 250m) |
| `VIBESPACE_DEFAULT_CPU_LIMIT` | Default agent CPU limit (default: 1000m) |
| `VIBESPACE_DEFAULT_MEMORY` | Default agent memory request (default: 512Mi) |
| `VIBESPACE_DEFAULT_MEMORY_LIMIT` | Default agent memory limit (default: 1Gi) |
| `VIBESPACE_DEFAULT_STORAGE` | Default agent storage (default: 10Gi) |
| `VIBESPACE_NO_UPDATE_CHECK` | Disable automatic update check on startup |
| `NO_COLOR` | Disable colors (standard convention) |

---

## Cluster

### `vibespace init`

Initialize the cluster. Downloads dependencies and boots a local k3s cluster.

| Flag | Default | Description |
|---|---|---|
| `--cpu` | `4` | CPU cores for the VM |
| `--memory` | `8` | RAM in GB |
| `--disk` | `60` | Disk in GB |
| `--bare-metal` | `false` | Install k3s directly on the host (Linux only) |
| `--external` | `false` | Use an existing Kubernetes cluster |
| `--kubeconfig` | | Path to external kubeconfig |

### `vibespace status`

Show cluster, daemon, and remote connection status.

### `vibespace stop`

Stop the cluster. In remote mode, prompts you to use `vibespace remote disconnect` instead.

### `vibespace uninstall`

Remove the cluster, VMs, downloaded binaries, and all state. Prompts for confirmation.

---

## Vibespaces

### `vibespace create <name>`

Create a new vibespace.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--agent-type` | `-t` | (required) | `claude-code` or `codex` |
| `--name` | `-n` | | Custom name for the primary agent |
| `--repo` | | | GitHub repo to clone into the workspace |
| `--mount` | `-m` | | Mount host dir (`host:container[:ro]`). Repeatable. |
| `--share-credentials` | `-s` | `false` | Share credentials between agents of the same type |
| `--worktree` | | `false` | Enable git worktree mode (each agent gets its own branch). Requires `--repo`. |
| `--branch` | | | Git branch for the primary agent (default: agent name). Requires `--worktree`. |
| `--skip-permissions` | | `false` | Run agent without permission prompts |
| `--allowed-tools` | | | Comma-separated list of allowed tools |
| `--disallowed-tools` | | | Comma-separated list of blocked tools |
| `--model` | | | Model to use |
| `--max-turns` | | `0` | Max conversation turns (0 = unlimited) |
| `--cpu` | | `250m` | CPU request |
| `--cpu-limit` | | `1000m` | CPU limit |
| `--memory` | | `512Mi` | Memory request |
| `--memory-limit` | | `1Gi` | Memory limit |
| `--storage` | | `10Gi` | Persistent storage size |

### `vibespace list`

List all vibespaces with status, agent count, and resource usage.

### `vibespace delete <name>`

Delete a vibespace and its resources.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--force` | `-f` | `false` | Skip confirmation |
| `--keep-data` | | `false` | Preserve persistent storage |
| `--dry-run` | `-n` | `false` | Show what would be deleted |

### `vibespace info`

Show vibespace details including agents, configuration, mounts, and port forwards.

Requires `--vibespace <name>` or `VIBESPACE_NAME`.

---

## Agents

All agent commands require `--vibespace <name>` or `VIBESPACE_NAME`.

### `vibespace agent list`

List agents in the vibespace.

### `vibespace agent create`

Add a new agent to the vibespace.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--agent-type` | `-t` | | Agent type (default: same as primary) |
| `--name` | `-n` | | Custom agent name |
| `--share-credentials` | `-s` | `false` | Use shared credential store |
| `--branch` | | | Git branch for this agent in worktree mode (default: agent name) |
| `--skip-permissions` | | `false` | Run without permission prompts |
| `--allowed-tools` | | | Comma-separated allowed tools |
| `--disallowed-tools` | | | Comma-separated blocked tools |
| `--model` | | | Model to use |
| `--max-turns` | | `0` | Max conversation turns |

### `vibespace agent delete <agent>`

Remove an agent from the vibespace.

### `vibespace agent start [agent]`

Start all agents or a specific agent.

### `vibespace agent stop [agent]`

Stop all agents or a specific agent.

### `vibespace connect [agent]`

Open an interactive terminal session to an agent via SSH.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--browser` | `-b` | `false` | Open in browser (ttyd) instead of terminal |
| `--agent` | `-a` | | Target agent name |

### `vibespace exec [agent] -- <command>`

Run a command in an agent container. Flags go before the `--` separator.

---

## Configuration

All config commands require `--vibespace <name>` or `VIBESPACE_NAME`.

### `vibespace config show [agent]`

Show agent configuration. Shows all agents if no agent name is given.

### `vibespace config set <agent>`

Update an agent's configuration.

| Flag | Default | Description |
|---|---|---|
| `--model` | | Model name |
| `--max-turns` | `0` | Max conversation turns |
| `--system-prompt` | | Custom system prompt |
| `--skip-permissions` | | Enable skip-permissions (Claude only) |
| `--no-skip-permissions` | | Disable skip-permissions (Claude only) |
| `--allowed-tools` | | Comma-separated allowed tools (Claude only) |
| `--disallowed-tools` | | Comma-separated blocked tools (Claude only) |
| `--reasoning-effort` | | `low`, `medium`, `high`, `xhigh` (Codex only) |

---

## Port Forwarding

All forward commands require `--vibespace <name>` or `VIBESPACE_NAME`.

### `vibespace forward list`

List active port forwards.

### `vibespace forward add <port>`

Forward a port from an agent container to localhost.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--agent` | `-a` | `claude-1` | Agent to forward from |
| `--local` | `-l` | (auto) | Local port number |
| `--dns` | | `false` | Enable DNS resolution (needs sudo) |
| `--dns-name` | | (auto) | Custom DNS name (default: `agent.vibespace`) |

### `vibespace forward remove <port>`

Remove a port forward.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--agent` | `-a` | `claude-1` | Agent to remove forward from |

---

## Remote Mode

### `vibespace serve`

Start the remote mode server. Runs a WireGuard VPN endpoint and management API.

| Flag | Default | Description |
|---|---|---|
| `--foreground` | `false` | Run in foreground instead of daemonizing |
| `--endpoint` | (auto-detected) | Public endpoint for clients |
| `--generate-token` | `false` | Generate a client invite token |
| `--token-ttl` | `30m` | Token expiry time |
| `--list-clients` | `false` | List registered clients |
| `--remove-client` | | Remove a client by name, hostname, or public key |

### `vibespace remote connect <token>`

Connect to a remote server using an invite token. Requires sudo for WireGuard setup.

### `vibespace remote disconnect`

Disconnect from the remote server and tear down the tunnel.

### `vibespace remote status`

Show connection status and run diagnostics.

### `vibespace remote watch`

Watch the connection and auto-reconnect on drops. Blocks until interrupted.

---

## Sessions

### `vibespace session list`

List multi-agent sessions.

### `vibespace session show <name>`

Show session details and message history.

### `vibespace session delete <name>`

Delete a session.

---

## Multi-Agent

### `vibespace multi [message]`

Start or interact with multi-agent sessions.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--vibespaces` | | | Vibespaces to include (all their agents) |
| `--agents` | | | Specific agents (`agent@vibespace`) |
| `--name` | | (auto UUID) | Session name |
| `--resume` | `-r` | | Resume a session |
| `--agent` | | `all` | Target agent for non-interactive mode |
| `--batch` | | `false` | Read JSONL from stdin |
| `--stream` | | `false` | Stream responses as plain text |
| `--timeout` | | `2m` | Response timeout |
| `--list-agents` | | `false` | List connected agents |
| `--list-sessions` | | `false` | List sessions |

---

## Utility

### `vibespace upgrade`

Download and install the latest version from GitHub Releases. Verifies SHA256 checksums before replacing the binary.

| Flag | Default | Description |
|---|---|---|
| `--check` | `false` | Only check for updates, don't install |
| `--force` | `false` | Re-download even if already on latest |

The CLI also checks for updates automatically after each command (cached for 24 hours, stderr-only). Disable with `VIBESPACE_NO_UPDATE_CHECK=1`.

### `vibespace version`

Print version, commit hash, and build date.

### `vibespace completion <shell>`

Generate shell completions for bash, zsh, fish, or powershell.

---

## Output Modes

| Mode | Flag | When | Format |
|---|---|---|---|
| Human | (default) | TTY detected | Styled tables, colors, spinners |
| JSON | `--json` | Non-TTY or explicit | `{success, data, error, meta}` envelope |
| Plain | `--plain` | Explicit only | Tab-separated values |
| Stream | `--stream` | Multi command only | Line-by-line plain text |

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Internal error |
| 2 | Invalid usage (bad flags or arguments) |
| 10 | Resource not found |
| 11 | Already exists |
| 12 | Permission denied |
| 13 | Timeout |
| 14 | Cancelled by user |
| 15 | Service unavailable (cluster or daemon not running) |

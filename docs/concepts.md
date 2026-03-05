# Concepts

The core ideas behind vibespace and how they fit together.

## Vibespace

A vibespace is an isolated stateful runtime environment with Kubernetes orchestration. Each one gets its own container, persistent storage, and SSH access. You can think of it as a sandboxed Linux machine dedicated to a project.

Vibespaces persist across restarts. Your code, agent history, and configuration survive `vibespace stop` and come back when you start again.

## Agents

An agent is an AI coding assistant running inside a vibespace. Vibespace currently supports two agent types:

- **claude-code** — Anthropic's Claude Code CLI
- **codex** — OpenAI's Codex CLI

A vibespace starts with one agent (the primary), but you can add more. By default, multiple agents share the same filesystem, so they can all work on the same codebase simultaneously.

For parallel development, you can enable **worktree mode** (`--worktree`) when creating a vibespace with a GitHub repo. This gives each agent its own git branch and working copy via git worktrees backed by a shared bare repository. Agents can work on different features simultaneously without clobbering each other's changes.

Each agent has its own configuration: model selection, tool permissions, max turns, and system prompt. You can tune these per-agent with `vibespace config set`.

### Credentials

Agents run inside containers, so they need to be logged in just like you'd log in on a new machine. Connect to the agent and run the CLI's login command (`claude login`, `codex login`, etc.).

If you create the vibespace with `--share-credentials`, you only need to do this once. Any additional agents created with the same flag share the credential store. Without it, each agent has its own login state and needs to be logged in separately.

## Daemon

The daemon is a background process that manages port forwarding, DNS resolution, and SSH tunnels. It starts automatically when you run `vibespace init` and runs as long as the cluster is up.

You don't interact with the daemon directly. It communicates with the CLI over a Unix socket at `~/.vibespace/daemon.sock`.

If the daemon dies, it auto-restarts on the next command that needs it.

## Port Forwarding

Port forwarding maps ports from agent containers to your local machine. If an agent is running a dev server on port 3000, you can access it at `localhost:<local-port>`.

```bash
vibespace forward add 3000 --vibespace my-project
```

The daemon manages these tunnels and reconnects them if they drop. You can optionally enable DNS so the forward is accessible at `<agent>.vibespace.internal` instead of remembering port numbers.

## Sessions

Sessions are multi-agent chat conversations that span one or more vibespaces. You send messages to specific agents with `@agent-name` or broadcast to all with `@all`.

Sessions persist to disk. You can resume a session later, add or remove agents, and review the conversation history.

See [Multi-Agent Sessions](multi-agent.md) for details.

## Remote Mode

By default, vibespace runs a cluster on your local machine. Remote mode moves the cluster to a VPS so you can:

- Use a more powerful machine (more CPU, RAM, storage)
- Access your environments from multiple devices
- Keep agents running while your laptop is closed

The connection is a WireGuard VPN tunnel. You run `vibespace serve` on the server and `vibespace remote connect <token>` on your client. All CLI commands work identically — the CLI detects remote mode automatically and routes through the tunnel.

See [Remote Mode](remote-mode.md) for setup instructions.

## State Directory

All local state lives in `~/.vibespace/`:

- `kubeconfig` — cluster credentials
- `bin/` — downloaded binaries (kubectl, k3s, etc.)
- `sessions/` — multi-agent session files
- `daemon.sock` — daemon IPC socket
- `debug.log` — debug output (when `VIBESPACE_DEBUG=1`)
- `remote.json`, `serve.json` — remote mode state
- WireGuard keys and TLS certs (for remote mode)

Nothing is stored outside this directory except `/etc/resolver/vibespace.internal` on macOS (for DNS) and `/etc/wireguard/wg-vibespace.conf` on Linux (for WireGuard).

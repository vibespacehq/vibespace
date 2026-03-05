# [vibespace](https://vibespace.build)

[![CI](https://github.com/vibespacehq/vibespace/actions/workflows/ci.yml/badge.svg)](https://github.com/vibespacehq/vibespace/actions/workflows/ci.yml)
[![E2E](https://github.com/vibespacehq/vibespace/actions/workflows/ci-e2e.yml/badge.svg)](https://github.com/vibespacehq/vibespace/actions/workflows/ci-e2e.yml)
[![codecov](https://codecov.io/gh/vibespacehq/vibespace/branch/main/graph/badge.svg?token=IGV80ATBGL)](https://codecov.io/gh/vibespacehq/vibespace)

Stateful runtime environments for AI agents with Kubernetes orchestration. Run Claude Code, Codex, or multiple agents collaborating on the same codebase — locally or on a remote server.

https://github.com/user-attachments/assets/991dd850-d879-4259-a87e-c005bdd55115

## What it does

- Spins up isolated Linux containers with AI coding agents inside them
- Multiple agents share the same filesystem and can work on the same project simultaneously
- Runs on your laptop (macOS/Linux) or on a VPS with WireGuard remote access
- Interactive TUI with live resource monitoring, multi-agent chat, and session management
- Port forwarding from agent containers to your host with optional DNS names
- Everything managed through a single CLI — fully scriptable with JSON output

## Quick start

```bash
# Build and install
git clone https://github.com/vibespacehq/vibespace.git
cd vibespace && ./scripts/build.sh && sudo ./scripts/install.sh

# Initialize the cluster
vibespace init

# Create a vibespace with a Claude Code agent
vibespace create my-project --agent-type claude-code --share-credentials

# Connect and log in
vibespace connect --vibespace my-project
# Inside the container: claude login

# Launch the TUI
vibespace
```

## Agent types

| Agent | Provider | Flag |
|---|---|---|
| Claude Code | Anthropic | `--agent-type claude-code` |
| Codex | OpenAI | `--agent-type codex` |

## Multi-agent

Add more agents to a vibespace and run them together:

```bash
vibespace agent create --vibespace my-project -t claude-code -s --name reviewer
vibespace multi --vibespaces my-project
```

In the session, target agents with `@claude-1 <message>` or broadcast with `@all <message>`.

## Remote mode

Run the cluster on a VPS and connect from anywhere:

```bash
# On the VPS
vibespace init --bare-metal
vibespace serve
vibespace serve --generate-token

# On your machine
sudo vibespace remote connect <token>
```

All commands work the same over the tunnel.

## Platform support

| Platform | Method |
|---|---|
| macOS (Apple Silicon / Intel) | Colima VM with k3s |
| Linux (x86_64) | Lima + QEMU, or bare metal k3s |

## Documentation

- [Getting Started](docs/getting-started.md)
- [Concepts](docs/concepts.md)
- [Installation](docs/installation.md)
- [CLI Reference](docs/cli-reference.md)
- [Multi-Agent Sessions](docs/multi-agent.md)
- [Remote Mode](docs/remote-mode.md)
- [Port Forwarding](docs/port-forwarding.md)
- [Configuration](docs/configuration.md)
- [TUI](docs/tui.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Cloud Deployment](docs/cloud-deployment.md)

## License

See [LICENSE](LICENSE).

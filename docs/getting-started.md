# Getting Started

Install vibespace, spin up your first environment, and connect to an AI coding agent.

<!-- TODO: add demo GIF of init → create → connect flow -->

## Prerequisites

- macOS (Apple Silicon or Intel) or Linux (x86_64)
- 4+ CPU cores and 8GB+ RAM available for the cluster VM

## Install

```bash
# macOS (Homebrew)
brew install vibespacehq/tap/vibespace

# Linux (APT)
curl -fsSL https://vibespacehq.github.io/apt/setup.sh | sudo bash && sudo apt install vibespace

# Any platform (curl)
curl -fsSL https://raw.githubusercontent.com/vibespacehq/vibespace/main/install.sh | bash

# Go (requires 1.25+)
go install github.com/vibespacehq/vibespace/cmd/vibespace@latest
```

See [Installation](installation.md) for all options including building from source.

Verify:

```bash
vibespace version
```

## Initialize the cluster

This downloads dependencies (k3s, kubectl, VM tooling) and boots a local Kubernetes cluster:

```bash
vibespace init
```

On macOS, this starts a Colima VM. On Linux, it uses Lima with QEMU (or `--bare-metal` for native k3s if you have root access).

First init takes a few minutes — subsequent starts are faster.

## Create a vibespace

A vibespace is an isolated stateful runtime environment with one or more AI coding agents inside it.

```bash
vibespace create my-project --agent-type claude-code --share-credentials
```

The `--share-credentials` flag sets up a shared credential store so you only need to log in once — all agents of the same type in this vibespace will share those credentials.

Other flags you might want:

```bash
# Clone a GitHub repo into the workspace
vibespace create my-project -t claude-code -s --repo https://github.com/org/repo

# Enable worktree mode for parallel development (each agent gets its own branch)
vibespace create my-project -t claude-code -s --repo https://github.com/org/repo --worktree

# Mount a local directory into the container
vibespace create my-project -t claude-code -s --mount ~/code/my-repo:/workspace

# Skip permission prompts (agent runs with full access)
vibespace create my-project -t claude-code -s --skip-permissions

# Use Codex instead
vibespace create my-project -t codex -s
```

## Connect and log in

Connect to the agent's terminal:

```bash
vibespace connect --vibespace my-project
```

This opens an interactive SSH session inside the container. On first connect, log in to the agent CLI:

```bash
# Inside the container — for Claude Code:
claude login

# For Codex:
codex login
```

Once logged in, the agent is ready to use. If you created the vibespace with `--share-credentials`, any additional agents you add later will pick up the same credentials automatically.

## Run commands

To run a one-off command without an interactive session:

```bash
vibespace exec --vibespace my-project -- ls /vibespace
```

## Check status

```bash
# Cluster and daemon status
vibespace status

# List all vibespaces
vibespace list

# Details on a specific vibespace
vibespace info --vibespace my-project
```

## Stop and clean up

```bash
# Stop the cluster (preserves everything)
vibespace stop

# Delete a specific vibespace
vibespace delete my-project

# Remove everything (cluster, data, binaries)
vibespace uninstall
```

## Next steps

- [Concepts](concepts.md) — understand vibespaces, agents, sessions, and remote mode
- [CLI Reference](cli-reference.md) — full command documentation
- [Multi-Agent Sessions](multi-agent.md) — run multiple agents on the same project
- [Remote Mode](remote-mode.md) — run your cluster on a VPS and connect from anywhere

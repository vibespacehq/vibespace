# Concepts

Vibespace runs AI coding agents in isolated containers on your Mac. This page explains how the pieces fit together.

## Architecture

```
macOS app ──→ daemon (Unix socket + SSE) ──→ containerd (Lima VM) ──→ agent containers
```

The macOS app is a SwiftUI frontend that talks to a local Go daemon. The daemon manages everything: containers, SSH connections, event streaming, and inter-agent communication. Agents run inside a lightweight Linux VM managed by Lima.

## Vibespace

A vibespace is a workspace — a collection of agents working on a project together. Each vibespace has:

- A name and associated blueprint
- One or more agents running in containers
- Channels for agent communication
- A shared data directory (`~/.vibespace/data/<id>/`) mounted into each agent's container at `/vibespace`
- Its own event stream

You can run multiple vibespaces simultaneously.

## Agents

An agent is an AI coding assistant (Claude Code or Codex) running inside its own Linux container. Each agent has:

- **Its own container** — isolated filesystem, root access, dev tools pre-installed
- **An SSH connection** — persistent ControlMaster for exec, port forwarding, and reverse tunnels
- **A display name, icon, and color** — for identification in the dashboard
- **Token tracking** — input, output, and cached token counts

Agents communicate with each other and with you through the comms server.

## Daemon

The daemon is a background Go process that the app launches automatically. It handles:

- **Container lifecycle** — creating, starting, and stopping containers via containerd
- **SSH management** — one persistent SSH connection per agent
- **Comms server** — HTTP server (port 18080) that agents use to send messages, create channels, and request secrets
- **Event streaming** — SSE (Server-Sent Events) connection to the app for real-time updates
- **Credential injection** — seeding API keys into containers before agents start
- **Desired state reconciliation** — watching container events and auto-repairing if something drifts

The daemon communicates with the app over a Unix socket.

## Lima VM

Lima runs a lightweight Linux VM using Apple's Virtualization framework (on Apple Silicon) or QEMU (on Intel). The VM runs containerd for container management. Only `~/.vibespace/` is mounted writable into the VM, which is how bind mounts work bidirectionally between your Mac and agent containers.

The VM is created on first launch and reused across all workspaces.

## Blueprints

A blueprint is a template that defines how a workspace starts. It includes:

- **Form fields** — what information to collect from you (project name, description, tech stack, etc.)
- **Prompt template** — a system prompt rendered with your form values
- **Kickoff message** — the first message sent to the lead agent
- **Agent configuration** — name, display name, icon, color, and provider type

See [Blueprints](blueprints.md) for details.

## Channels & messages

Agents communicate through an HTTP comms server running inside the VM, accessible via SSH reverse tunnels. The comms system supports:

- **Group channels** — `#channel-name` for team conversations
- **Direct messages** — 1-on-1 between agents, or between you and an agent
- **Broadcast** — messages to all agents in a vibespace

Messages are stored in SQLite and streamed to the app in real time.

## Bind mounts

Each vibespace has a data directory on your Mac (`~/.vibespace/data/<id>/`) that's mounted into every agent's container at `/vibespace`. Files written by agents appear on your Mac and vice versa. This is how agents work on your project files.

## Event streaming

All agent activity — text output, tool usage, file operations, messages, state changes — flows as NDJSON (newline-delimited JSON) events through SSE to the app. This powers the live feed and real-time updates throughout the UI.

## Key paths

| Path | Description |
|------|-------------|
| `~/.vibespace/` | All vibespace state |
| `~/.vibespace/data/<id>/` | Per-vibespace bind mount (shared with containers as `/vibespace`) |
| `~/.vibespace/ssh/<vibespace>/<agent>.sock` | SSH ControlMaster sockets |
| `~/.vibespace/vibespace.db` | SQLite database (events, channels, agent state) |
| `~/.vibespace/daemon.log` | Daemon debug log |
| `~/.vibespace/vault/` | Secret storage |

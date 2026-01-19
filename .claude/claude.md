# vibespace - AI Assistant Context

**Project**: vibespace - multi-Claude development environments
**Stack**: Go CLI + Colima/k3s + Knative

---

## What This Project Does

vibespace is a CLI tool for managing AI-powered development environments. Each vibespace runs Claude Code instances in Kubernetes pods, accessible via terminal or browser through port-forwarding.

```
vibespace init                        # Start local Kubernetes cluster
vibespace create myproject            # Create a vibespace
vibespace myproject up                # Start port-forward daemon
vibespace myproject connect           # Connect to Claude in terminal
vibespace myproject spawn             # Add another Claude agent
vibespace multi myproject otherproj   # Multi-agent terminal UI
```

**Key Features**:
- **CLI-first**: No desktop app, pure terminal experience
- **Multi-Claude orchestration**: Multiple AI agents per project, shared filesystem
- **Terminal UI**: Talk to multiple Claudes with @mentions
- **Port-forwarding**: Access dev servers via localhost (no DNS magic)
- **Remote access**: Connect to another machine's cluster via WireGuard
- **Scale-to-zero**: Knative-based, pods spin down when idle

---

## Architecture

### Local Mode

```
┌─────────────────────────────────────────────────────────────────────────┐
│  User Terminal                                                          │
│  └── vibespace CLI                                                      │
│       ├── Cluster mgmt (init, status, stop)                            │
│       ├── Vibespace mgmt (create, list, delete)                        │
│       ├── Port-forward daemon (background)                              │
│       └── Multi-session TUI (ttyd websocket → Claude)                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ client-go / port-forward
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Colima VM (k3s on macOS) or native k3s (Linux)                        │
│  ├── Knative Serving (scale-to-zero)                                   │
│  └── vibespace namespace                                                │
│       ├── vibespace-<id> (Knative Service)                             │
│       │    ├── Pod: claude-1 (ttyd:7681 + Claude Code CLI)             │
│       │    └── Pod: claude-2 (ttyd:7681 + Claude Code CLI)             │
│       └── PVC: shared /vibespace directory                             │
└─────────────────────────────────────────────────────────────────────────┘
```

### Remote Mode (WireGuard)

```
Machine B (client)                    Machine A (server)
├── vibespace CLI                     ├── vibespace CLI
├── WireGuard tunnel    ──────────→   ├── vibespace serve (WireGuard server)
└── kubeconfig → A's cluster          └── Colima/k3s cluster
```

### Multi-Session TUI

```
┌─────────────────────────────────────────────────────────────────────────┐
│ vibespace multi projectA projectB                                       │
├─────────────────────────────────────────────────────────────────────────┤
│ [claude-1@projectA] Working on the auth module...                       │
│ [claude-2@projectA] Tests are passing now                               │
│ [claude-1@projectB] Found the bug in line 42                            │
├─────────────────────────────────────────────────────────────────────────┤
│ > @claude-1@projectA can you also add rate limiting?                    │
└─────────────────────────────────────────────────────────────────────────┘

Commands: @agent message, @all broadcast, /add, /remove, /focus, /quit
```

---

## Repository Structure

```
vibespace/
├── api/                       # Go CLI and core packages
│   ├── cmd/vibespace/         # CLI entry point
│   ├── internal/
│   │   ├── cli/               # Cobra commands
│   │   └── platform/          # Colima/k3s management
│   └── pkg/
│       ├── k8s/               # Kubernetes client
│       ├── knative/           # Knative service management
│       ├── vibespace/         # Vibespace business logic
│       ├── model/             # Data models
│       └── image/             # Container image (ttyd + Claude Code)
├── docs/
│   └── CLI_SPEC.md            # Complete CLI specification
└── archive/                   # Old code (Tauri app, HTTP server, etc.)
```

---

## Key Concepts

### Vibespace
A project environment running as Knative Service(s):
- **ttyd**: Terminal over websocket (port 7681)
- **Claude Code CLI**: AI coding agent
- **Persistent volume**: `/vibespace` directory shared across all agents

### Multi-Claude
Multiple Claude instances work on the same project:
- Each Claude runs in its own Kubernetes pod
- All pods mount the same PVC (shared filesystem)
- User can message specific Claudes or broadcast to all

### Port-Forward Daemon
Background process managing connections:
- Auto-reconnects on connection drops
- Handles multiple agents (port offset: claude-2 → 17681)
- Detects and forwards dev server ports

### Sessions
Group agents from multiple vibespaces:
- Named sessions persist across restarts
- Quick ad-hoc sessions with `vibespace multi`
- @agent@vibespace addressing

---

## External Dependencies

| Component | Purpose |
|-----------|---------|
| **Colima** | Lima-based container runtime for macOS |
| **k3s** | Lightweight Kubernetes |
| **Knative Serving** | Scale-to-zero, serverless workloads |
| **ttyd** | Share terminal over websocket |
| **Claude Code** | AI coding agent CLI |
| **WireGuard** | VPN for remote access |

---

## Development

### Prerequisites
- Go 1.21+
- Docker (for building container image)

### Building

```bash
cd api
go build -o vibespace ./cmd/vibespace
```

### Testing

```bash
cd api && go test ./...
```

### Installing Locally

```bash
cd api
go build -o vibespace ./cmd/vibespace
sudo mv vibespace /usr/local/bin/
```

---

## CLI Commands Overview

See `docs/CLI_SPEC.md` for complete reference.

### Cluster Management
```bash
vibespace init                   # Initialize cluster
vibespace status                 # Show status
vibespace stop                   # Stop cluster
vibespace serve                  # Server mode for remote clients
```

### Vibespace Management
```bash
vibespace create <name>          # Create vibespace
vibespace list                   # List all
vibespace delete <name>          # Delete vibespace
vibespace <name> start/stop      # Start/stop vibespace
```

### Agent Management
```bash
vibespace <name> agents          # List agents
vibespace <name> spawn           # Add agent
vibespace <name> kill <agent>    # Remove agent
```

### Connection
```bash
vibespace <name> up              # Start port-forward daemon
vibespace <name> down            # Stop daemon
vibespace <name> connect [agent] # Connect to agent terminal
vibespace <name> forward list    # List forwards
```

### Multi-Session
```bash
vibespace session create <name>  # Create session
vibespace session add <s> <v>    # Add vibespace to session
vibespace session start <name>   # Launch TUI
vibespace multi <v1> <v2>        # Quick ad-hoc session
```

### Remote
```bash
vibespace remote connect <host>  # Connect to remote
vibespace remote disconnect      # Disconnect
```

---

## State Files

```
~/.vibespace/
├── bin/                         # Bundled binaries (colima, lima, kubectl)
├── daemons/                     # Port-forward daemon state
│   ├── <vibespace>.pid
│   ├── <vibespace>.sock
│   └── <vibespace>.json
├── sessions/                    # Multi-session state
│   └── <session>.json
├── remote/                      # Remote connection config
│   ├── kubeconfig
│   ├── wireguard.conf
│   └── connection.json
└── config.json                  # Global config
```

---

## Naming Conventions

### Code Style
- **Go**: Standard conventions, singular package names

### Kubernetes Resources
- **Namespace**: `vibespace`
- **Labels**: `vibespace.dev/id`, `vibespace.dev/project-name`
- **Service names**: `vibespace-<id>`
- **PVC names**: `vibespace-<project>-pvc`

---

## Git Workflow

### Branch Naming
```
feature/#<issue>-<short-description>
fix/#<issue>-<short-description>
```

### Commit Message Format
```
<type>(#<issue>): <description>
```

**Types**: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

---

## Security Considerations

1. **Local-first**: All components run on user's machine
2. **No cloud auth**: No external authentication
3. **API keys**: User provides Claude API key, stored in container
4. **Non-root containers**: Run as UID 1000

---

## Troubleshooting

### Cluster Issues
```bash
vibespace status                 # Check component status
~/.vibespace/bin/kubectl get pods -n vibespace
```

### Port-Forward Issues
```bash
vibespace <name> forward list    # Check forward status
vibespace <name> forward restart-all
```

### Connection Issues
```bash
vibespace <name> down && vibespace <name> up  # Restart daemon
```

---

## Important Files

- `docs/CLI_SPEC.md` - Complete CLI command reference
- `api/internal/cli/` - All CLI command implementations
- `api/pkg/vibespace/service.go` - Core vibespace logic
- `api/pkg/knative/service.go` - Knative service management
- `api/internal/platform/colima.go` - Colima VM management

---

## Current Status

**Pivot in Progress**: Transitioning from Tauri desktop app to CLI-first approach.

Implemented:
- Cluster management (init, status, stop)
- Vibespace CRUD (create, list, delete, start, stop)
- Basic connect command
- Platform detection (Colima on macOS)

To Implement:
- Port-forward daemon with auto-reconnect
- Multi-agent spawning
- Multi-session TUI
- Remote mode (WireGuard)
- Forward management commands

See `docs/CLI_SPEC.md` for the full target feature set.

---

**Last Updated**: 2026-01-19

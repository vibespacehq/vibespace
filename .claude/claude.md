# vibespace - AI Assistant Context

**Project**: vibespace - multi-Claude development environments
**Stack**: Go CLI + Colima/k3s + plain Kubernetes Deployments

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

---

## Architecture

### Local Mode

```
┌─────────────────────────────────────────────────────────────────────────┐
│  User Terminal                                                          │
│  └── vibespace CLI                                                      │
│       ├── Cluster mgmt (init, status, stop)                            │
│       ├── Vibespace mgmt (create, list, delete)                        │
│       ├── Port-forward daemon (background, forwards SSH + ttyd)        │
│       └── Multi-session TUI (SSH → Claude print mode)                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ client-go / port-forward
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Colima VM (k3s on macOS) or native k3s (Linux)                        │
│  └── vibespace namespace                                                │
│       ├── vibespace-<id> (Deployment)                                  │
│       │    ├── Pod: claude-1 (SSH:22 + ttyd:7681 + Claude Code CLI)   │
│       │    └── Pod: claude-2 (SSH:22 + ttyd:7681 + Claude Code CLI)   │
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
│   │   ├── cli/               # Cobra commands + logging setup
│   │   └── platform/          # Colima/k3s management
│   └── pkg/
│       ├── k8s/               # Kubernetes client
│       ├── vibespace/         # Vibespace business logic
│       ├── model/             # Data models
│       ├── session/           # Multi-session management
│       ├── tui/               # Terminal UI (bubbletea)
│       └── image/             # Container image (ttyd + Claude Code)
├── docs/
│   └── CLI_SPEC.md            # Complete CLI specification
└── archive/                   # Old code (Tauri app, HTTP server, etc.)
```

---

## Key Concepts

### Vibespace
A project environment running as Kubernetes Deployment(s):
- **SSH (port 22)**: Primary access for CLI and TUI (Claude print mode)
- **ttyd (port 7681)**: Browser terminal access (fallback)
- **Claude Code CLI**: AI coding agent
- **Persistent volume**: `/vibespace` directory shared across all agents

### Multi-Claude
Multiple Claude instances work on the same project:
- Each Claude runs in its own Kubernetes pod
- All pods mount the same PVC (shared filesystem)
- User can message specific Claudes or broadcast to all
- TUI uses Claude's print mode (`claude -p --output-format stream-json`)
- `/focus` command switches to interactive Claude session

### Port-Forward Daemon
Background process managing connections:
- Forwards both SSH (primary) and ttyd (browser fallback)
- Auto-reconnects on connection drops
- Handles multiple agents (port offset: claude-2 → 10022 for SSH, 17681 for ttyd)
- Detects and forwards dev server ports

### Sessions
Group agents from multiple vibespaces:
- Named sessions persist across restarts
- Quick ad-hoc sessions with `vibespace multi`
- @agent@vibespace addressing

### Claude Session Management
Each agent maintains conversation continuity via Claude CLI sessions:
- **First message**: Uses `--session-id <uuid>` to create a new Claude session
- **Subsequent messages**: Uses `--resume <uuid>` to continue the conversation
- **Per-agent tracking**: Each agent has its own session history
- **Persistence**: Sessions stored in `~/.vibespace/claude_sessions.json`

**TUI Commands**:
```
/session @agent new         # Start fresh Claude session
/session @agent list        # Show session history
/session @agent resume <id> # Switch to a different session
/session @agent info        # Show current session details
```

**Architecture**:
```
ClaudeSessionManager (shared)
├── claude-1@projectA
│   ├── currentSessionID: "abc-123"
│   ├── messageCount: 5  (0 = --session-id, >0 = --resume)
│   └── history: ["abc-123", "def-456", ...]
├── claude-2@projectA
│   └── ...
```

---

## External Dependencies

| Component | Purpose |
|-----------|---------|
| **Colima** | Lima-based container runtime for macOS |
| **k3s** | Lightweight Kubernetes |
| **ttyd** | Share terminal over websocket |
| **Claude Code** | AI coding agent CLI |
| **WireGuard** | VPN for remote access |
| **lumberjack** | Log rotation for daemon and debug logs |

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
├── lima/                        # Lima binaries (limactl, etc.)
├── daemons/                     # Port-forward daemon state and logs
│   ├── <vibespace>.pid
│   ├── <vibespace>.sock
│   ├── <vibespace>.json
│   └── <vibespace>.log          # Daemon logs (JSON, rotated)
├── sessions/                    # Multi-session state
│   └── <session>.json
├── history/                     # TUI chat history (JSONL format)
│   └── <session>.jsonl
├── claude_sessions.json         # Claude CLI session tracking per agent
├── remote/                      # Remote connection config
│   ├── kubeconfig
│   ├── wireguard.conf
│   └── connection.json
├── debug.log                    # CLI debug log (when VIBESPACE_DEBUG=1)
├── tui-debug.log                # TUI debug log (when VIBESPACE_DEBUG=1)
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

### Debug Mode
```bash
VIBESPACE_DEBUG=1 vibespace init          # Enable debug logging
VIBESPACE_LOG_LEVEL=debug vibespace init  # Set log level explicitly
```

Debug logs are written to `~/.vibespace/debug.log` (CLI) or `~/.vibespace/tui-debug.log` (TUI).

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

### Viewing Daemon Logs
```bash
tail -f ~/.vibespace/daemons/<name>.log  # Follow daemon logs (JSON format)
```

---

## Important Files

- `docs/CLI_SPEC.md` - Complete CLI command reference
- `api/internal/cli/` - All CLI command implementations
- `api/internal/cli/logging.go` - Centralized logging configuration
- `api/internal/cli/output.go` - Output handling (TTY, colors, JSON, verbosity)
- `api/internal/cli/spinner.go` - Progress spinners for long operations
- `api/internal/cli/json_types.go` - JSON output type definitions
- `api/internal/cli/suggestions.go` - Typo suggestions (Levenshtein distance)
- `api/pkg/vibespace/service.go` - Core vibespace logic
- `api/internal/platform/colima.go` - Colima VM management
- `api/pkg/tui/model.go` - TUI main model (Bubble Tea)
- `api/pkg/tui/agent.go` - Agent connections (SSH to Claude print mode)
- `api/pkg/tui/session_manager.go` - Claude CLI session tracking (--session-id/--resume)
- `api/pkg/tui/view.go` - TUI rendering (chat view, messages)
- `api/pkg/tui/update.go` - TUI event handling and commands

---

## Current Status

**CLI-first approach**: Transitioned from Tauri desktop app to pure CLI.

### Implemented
- **Cluster management**: init, status, stop (Colima on macOS)
- **Vibespace CRUD**: create, list, delete, start, stop
- **Basic connect command**: SSH (primary) or ttyd browser (with `--browser` flag)
- **Platform detection**: Colima on macOS, k3s on Linux
- **Production-ready logging**:
  - Centralized `slog` configuration in `logging.go`
  - Three modes: CLI (discard by default), TUI (discard by default), Daemon (always log)
  - Debug mode via `VIBESPACE_DEBUG=1` environment variable
  - Log level config via `VIBESPACE_LOG_LEVEL` (debug/info/warn/error)
  - JSON format for daemon logs, text format for CLI/TUI debug logs
  - Log rotation via lumberjack (10MB max, 3 backups, 7 days retention)
  - Request ID correlation for tracing operations
  - Subprocess output capture in debug mode
- **Multi-session TUI**: Bubbletea UI with viewport scrolling, syntax highlighting
  - Viewport component for proper scrolling (input stays fixed)
  - Syntax highlighting for code blocks (chroma library, monokai theme)
  - Improved tool display with colored icons
  - Thinking indicator (braille spinner, agent-colored)
- **Non-interactive multi-session mode** (scripting support):
  - Auto-detects non-TTY and switches to JSON output
  - `--json` flag for explicit JSON output
  - `--plain` flag for structured plain text output
  - `--stream` flag for real-time streaming in plain text mode
  - `--list-agents` flag to discover agents without sending a message
  - `--batch` flag for JSONL batch processing from stdin
  - `--agent` flag to target specific agent
  - `--timeout` flag for response timeout
  - Session history persisted for both TUI and non-TTY modes
- **Claude session management**:
  - Per-agent session tracking with `--session-id` (first msg) / `--resume` (subsequent)
  - Session history persistence (`~/.vibespace/claude_sessions.json`)
  - TUI commands: `/session @agent new|list|resume|info`
  - Conversation continuity across TUI restarts
- **CLI UX improvements** (clig.dev guidelines):
  - Global flags: --json, --plain, --quiet, --verbose, --no-color
  - JSON output for scripting (list, status, agents, forward list, session)
  - Spinners for long operations (init, create, delete, stop)
  - Typo suggestions for mistyped commands
  - TTY detection (disables colors when piping, refuses prompts without TTY)
  - TTY guard in tui.Run() prevents crashes in non-interactive contexts
  - Double Ctrl-C handling in daemon (graceful then force quit)
  - Examples in all command help text

### To Implement
- Port-forward daemon with auto-reconnect
- Multi-agent spawning
- TUI command handlers (currently parsed but not wired up)
- Remote mode (WireGuard)
- Forward management commands

### Log Locations
- **CLI/TUI debug logs**: `~/.vibespace/debug.log` or `~/.vibespace/tui-debug.log`
- **Daemon logs**: `~/.vibespace/daemons/<vibespace-name>.log`

See `docs/CLI_SPEC.md` for the full target feature set.

---

**Last Updated**: 2026-01-22

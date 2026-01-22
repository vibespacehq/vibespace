# vibespace CLI Specification

Complete command reference for the vibespace CLI tool.

## Command Overview

```
vibespace - Multi-Claude Development Environments

CLUSTER MANAGEMENT
──────────────────────────────────────────────────────────────────────────
  vibespace init                     Initialize local cluster (Colima/k3s)
  vibespace status                   Show cluster and component status
  vibespace stop                     Stop the local cluster
  vibespace serve                    Start server mode for remote clients
                                     (WireGuard + exposes cluster)

VIBESPACE MANAGEMENT
──────────────────────────────────────────────────────────────────────────
  vibespace create <name>            Create a new vibespace
      --image <img>                  Custom container image
      --cpu <cores>                  CPU allocation (default: 2)
      --memory <size>                Memory allocation (default: 4Gi)

  vibespace list                     List all vibespaces
  vibespace delete <name>            Delete a vibespace

VIBESPACE OPERATIONS (vibespace <name> ...)
──────────────────────────────────────────────────────────────────────────
  vibespace <name> start             Start vibespace
  vibespace <name> stop              Stop vibespace
  vibespace <name> status            Show vibespace status and details

AGENT MANAGEMENT (vibespace <name> ...)
──────────────────────────────────────────────────────────────────────────
  vibespace <name> agents            List Claude agents in vibespace
  vibespace <name> spawn             Create additional Claude agent (shared filesystem)
      --name <agent-name>            Custom agent name (default: claude-N)
  vibespace <name> fork              Fork vibespace with cloned filesystem (independent copy)
      --name <fork-name>             Name for the forked vibespace
                                     Agent works on snapshot of current state
  vibespace <name> kill <agent>      Remove a Claude agent

CONNECTION (vibespace <name> ...)
──────────────────────────────────────────────────────────────────────────
  vibespace <name> connect [agent]   Connect to agent's terminal via SSH
                                     Default: claude-1
      --browser                      Open in browser via ttyd instead of SSH

  vibespace <name> ports             List detected dev server ports

PORT FORWARDING DAEMON (vibespace <name> ...)
──────────────────────────────────────────────────────────────────────────
  vibespace <name> up                Start port-forward daemon for vibespace
                                     (forwards ttyd + detected ports, runs in background)
      --agent <agent>                Only forward specific agent (default: all)

  vibespace <name> down              Stop port-forward daemon

PORT FORWARD MANAGEMENT (vibespace <name> forward ...)
──────────────────────────────────────────────────────────────────────────
  vibespace <name> forward list      List all active port forwards and status

  vibespace <name> forward add <port>
                                     Add a new port forward
      --local <port>                 Custom local port (default: same as remote)
      --agent <agent>                Agent to forward from (default: claude-1)

  vibespace <name> forward remove <port>
                                     Remove/stop a port forward
      --agent <agent>                Specify which agent's forward to remove

  vibespace <name> forward restart <port>
                                     Restart a specific port forward
      --agent <agent>                Specify which agent's forward to restart

  vibespace <name> forward restart-all
                                     Restart all port forwards for this vibespace

  vibespace <name> forward stop <port>
                                     Temporarily stop a forward (can restart later)
      --agent <agent>                Specify which agent's forward to stop

  vibespace <name> forward start <port>
                                     Start a previously stopped forward
      --agent <agent>                Specify which agent's forward to start

MULTI-SESSION (Terminal UI for multiple agents)
──────────────────────────────────────────────────────────────────────────
  vibespace session create <name>    Create a named session
  vibespace session list             List all sessions
  vibespace session delete <name>    Delete a session

  vibespace session add <session> <vibespace> [agent]
                                     Add vibespace/agent to session

  vibespace session remove <session> <vibespace> [agent]
                                     Remove from session

  vibespace session start <name>     Launch TUI for session

  # Quick ad-hoc session (no persistence)
  vibespace multi <vibespace>...     Start TUI with specified vibespaces
                                     Example: vibespace multi projectA projectB

  # Non-interactive mode (for scripting, auto-detected when not TTY)
  vibespace multi <vibespace>... [message]
      --json                         JSON output (default for non-TTY)
      --plain                        Plain text output
      --stream                       Stream responses in real-time (plain mode)
      --agent <name>                 Target specific agent (default: all)
      --list-agents                  List connected agents and exit
      --batch                        Batch mode: read JSONL from stdin
      --timeout <duration>           Response timeout (default: 2m)

REMOTE MODE (Client connecting to another machine's cluster)
──────────────────────────────────────────────────────────────────────────
  vibespace remote connect <host>    Connect to remote vibespace server
      --token <token>                Authentication token

  vibespace remote disconnect        Disconnect from remote server
  vibespace remote status            Show remote connection status

UTILITY
──────────────────────────────────────────────────────────────────────────
  vibespace version                  Show version information
  vibespace help                     Show help
  vibespace <name>                   Show help for vibespace subcommands
```

## TUI Commands

Commands available inside `vibespace session start` or `vibespace multi`:

```
MESSAGING
──────────────────────────────────────────────────────────────────────────
  @<agent> <message>                 Send to specific agent (e.g., @claude-1)
  @<agent>@<vibespace> <message>     Send to agent in specific vibespace
  @all <message>                     Broadcast to all agents in session
  @all@<vibespace> <message>         Broadcast to all agents in one vibespace

SESSION MANAGEMENT
──────────────────────────────────────────────────────────────────────────
  /add <vibespace> [agent]           Add vibespace/agent to current session
  /remove <agent>[@vibespace]        Remove agent from session
  /list                              List connected agents with status

VIEW MODES
──────────────────────────────────────────────────────────────────────────
  /focus <agent>[@vibespace]         Full-screen single agent view
  /split                             Return to split/multiplexed view
  /scroll <agent>                    Scroll history for specific agent

PORT MANAGEMENT (within TUI)
──────────────────────────────────────────────────────────────────────────
  /ports                             Show all forwarded ports across session
  /forward <port>[@vibespace]        Add a port forward
  /unforward <port>[@vibespace]      Remove a port forward
  /open <port>[@vibespace]           Open port in browser

CLAUDE SESSION MANAGEMENT (conversation continuity)
──────────────────────────────────────────────────────────────────────────
  /session @<agent> new              Start fresh Claude session for agent
  /session @<agent> list             Show session history for agent
  /session @<agent> resume <id>      Switch to a different session
  /session @<agent> info             Show current session details

SESSION CONTROL
──────────────────────────────────────────────────────────────────────────
  /save [name]                       Save current session (for ad-hoc sessions)
  /quit                              Exit TUI
  /help                              Show TUI help
```

## Global Flags

These flags work with any command:

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (for scripting) |
| `--plain` | Plain tab-separated output (for scripting) |
| `--quiet`, `-q` | Suppress non-essential output |
| `--verbose`, `-v` | Enable verbose output |
| `--no-color` | Disable colored output |

```bash
# JSON output for scripting
vibespace --json list
vibespace --json status

# Plain output for parsing
vibespace --plain list | awk -F'\t' '{print $1}'

# Quiet mode
vibespace --quiet create myproject
```

## Environment Variables

| Variable | Description | Values |
|----------|-------------|--------|
| `VIBESPACE_DEBUG` | Enable debug logging for CLI/TUI | Any non-empty value |
| `VIBESPACE_LOG_LEVEL` | Set log level | `debug`, `info`, `warn`, `error` |
| `VIBESPACE_CLUSTER_CPU` | Default CPU cores for cluster | Integer (default: 4) |
| `VIBESPACE_CLUSTER_MEMORY` | Default memory (GB) for cluster | Integer (default: 8) |
| `VIBESPACE_CLUSTER_DISK` | Default disk (GB) for cluster | Integer (default: 60) |
| `NO_COLOR` | Disable colored output | Any non-empty value |

### Debug Mode

```bash
# Enable debug logging
VIBESPACE_DEBUG=1 vibespace init

# Set specific log level
VIBESPACE_LOG_LEVEL=debug vibespace status
```

Debug logs are written to:
- CLI: `~/.vibespace/debug.log`
- TUI: `~/.vibespace/tui-debug.log`
- Daemon: `~/.vibespace/daemons/<name>.log` (always logged, JSON format)

## State Files

```
~/.vibespace/
├── bin/                             # Bundled binaries
│   ├── colima
│   └── kubectl
│
├── lima/                            # Lima binaries (limactl, etc.)
│   └── bin/
│
├── daemons/                         # Port-forward daemons (per vibespace)
│   ├── <vibespace>.pid              # Daemon process ID
│   ├── <vibespace>.sock             # Unix socket for IPC
│   ├── <vibespace>.json             # Forward state and config
│   └── <vibespace>.log              # Daemon logs (JSON, rotated)
│
├── sessions/                        # Multi-session state
│   └── <session-name>.json          # Session config (vibespaces, agents, layout)
│
├── history/                         # TUI and non-TTY chat history (JSONL)
│   └── <session>.jsonl              # Message history per session
│
├── claude_sessions.json             # Claude CLI session tracking per agent
│                                    # (--session-id vs --resume logic)
│
├── remote/                          # Remote mode config
│   ├── kubeconfig                   # Remote cluster kubeconfig
│   ├── wireguard.conf               # WireGuard client config
│   └── connection.json              # Remote server info (host, token, status)
│
├── debug.log                        # CLI debug log (when VIBESPACE_DEBUG=1)
├── tui-debug.log                    # TUI debug log (when VIBESPACE_DEBUG=1)
│
└── config.json                      # Global config
                                     # - mode (local/remote)
                                     # - default preferences
                                     # - color scheme
```

## Architecture

### Local Mode (Machine A - has cluster)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  User Terminal                                                          │
│  └── vibespace CLI                                                      │
│       ├── Cluster mgmt (init, status, stop)                            │
│       ├── Vibespace mgmt (create, list, delete)                        │
│       ├── Port-forward daemon (background)                              │
│       │    └── Forwards SSH (primary) + ttyd (browser fallback)        │
│       └── Multi-session TUI                                             │
│            └── SSH → Claude print mode (stream-json output)            │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ kubectl / client-go
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Colima VM (k3s)                                                        │
│  └── vibespace namespace                                                │
│       ├── vibespace-abc123 (Deployment)                                │
│       │    ├── Pod: claude-1                                           │
│       │    │    └── Container: SSH:22 + ttyd:7681 + Claude Code CLI   │
│       │    └── Pod: claude-2                                           │
│       │         └── Container: SSH:22 + ttyd:7681 + Claude Code CLI   │
│       └── PVC: shared /vibespace directory                             │
└─────────────────────────────────────────────────────────────────────────┘
```

### Remote Mode (Machine B - client only)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Machine B (client)                                                     │
│  └── vibespace CLI                                                      │
│       ├── WireGuard client                                             │
│       ├── Remote kubeconfig → Machine A's cluster                      │
│       └── Same commands, tunneled through WireGuard                    │
└─────────────────────────────────────────────────────────────────────────┘
            │
            │ WireGuard tunnel
            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Machine A (server)                                                     │
│  ├── vibespace serve (WireGuard server + API)                          │
│  └── Colima VM (k3s) - same as local mode                              │
└─────────────────────────────────────────────────────────────────────────┘
```

### Multi-Session TUI Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Multi-Session TUI (bubbletea)                                          │
│  ├── Session Manager                                                    │
│  │    └── Tracks agents, vibespaces, addressing                        │
│  ├── Connection Pool                                                    │
│  │    └── SSH connections running Claude in print mode                 │
│  ├── Input Router                                                       │
│  │    └── Parses @mentions, routes messages to agent stdin             │
│  └── Output Multiplexer                                                 │
│       └── Parses stream-json output, applies colors/prefixes           │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
            ┌───────────┐   ┌───────────┐   ┌───────────┐
            │    SSH    │   │    SSH    │   │    SSH    │
            │ claude-1  │   │ claude-2  │   │ claude-1  │
            │ @projectA │   │ @projectA │   │ @projectB │
            └───────────┘   └───────────┘   └───────────┘
                    │               │               │
                    ▼               ▼               ▼
            ┌───────────┐   ┌───────────┐   ┌───────────┐
            │  claude   │   │  claude   │   │  claude   │
            │ -p (JSON) │   │ -p (JSON) │   │ -p (JSON) │
            └───────────┘   └───────────┘   └───────────┘

/focus mode: Exits TUI, launches interactive SSH session with `claude`
```

## Port Forward State File Format

`~/.vibespace/daemons/<vibespace>.json`:

```json
{
  "vibespace": "myproject",
  "started_at": "2024-01-15T10:30:00Z",
  "agents": {
    "claude-1": {
      "pod_name": "vibespace-abc123-xxx",
      "forwards": [
        {
          "local_port": 10022,
          "remote_port": 22,
          "type": "ssh",
          "status": "active",
          "started_at": "2024-01-15T10:30:00Z"
        },
        {
          "local_port": 7681,
          "remote_port": 7681,
          "type": "ttyd",
          "status": "active",
          "started_at": "2024-01-15T10:30:00Z"
        },
        {
          "local_port": 3000,
          "remote_port": 3000,
          "type": "detected",
          "status": "active",
          "process": "node",
          "started_at": "2024-01-15T10:35:00Z"
        }
      ]
    },
    "claude-2": {
      "pod_name": "vibespace-abc123-yyy",
      "forwards": [
        {
          "local_port": 20022,
          "remote_port": 22,
          "type": "ssh",
          "status": "active",
          "started_at": "2024-01-15T10:30:00Z"
        },
        {
          "local_port": 17681,
          "remote_port": 7681,
          "type": "ttyd",
          "status": "active",
          "started_at": "2024-01-15T10:30:00Z"
        }
      ]
    }
  }
}
```

## Session State File Format

`~/.vibespace/sessions/<session>.json`:

```json
{
  "name": "my-work",
  "created_at": "2024-01-15T10:00:00Z",
  "vibespaces": [
    {
      "name": "projectA",
      "agents": ["claude-1", "claude-2"],
      "mode": "local"
    },
    {
      "name": "projectB",
      "agents": ["claude-1"],
      "mode": "remote",
      "remote_host": "192.168.1.100"
    }
  ],
  "layout": {
    "mode": "split",
    "focused_agent": null
  },
  "last_used": "2024-01-15T14:30:00Z"
}
```

## Port Allocation Strategy

To avoid conflicts when forwarding multiple agents:

```
Agent         SSH Port    ttyd Port    Dev Ports (offset)
─────────────────────────────────────────────────────────
claude-1      10022       7681         3000, 8080, ...
claude-2      20022       17681        13000, 18080, ...
claude-3      30022       27681        23000, 28080, ...
claude-N      N*10000+22  N*10000+7681 N*10000+port
```

For cross-vibespace sessions, vibespace index is added:

```
projectA/claude-1    10022        7681         3000
projectA/claude-2    20022        17681        13000
projectB/claude-1    110022       107681       103000  (vibespace offset: 100000)
```

Or use dynamic allocation with user-configurable local ports.

## Example Workflows

### Basic Local Usage

```bash
# First time setup
vibespace init
# ✓ Colima started
# ✓ Cluster ready
# ✓ Ready

# Create a vibespace
vibespace create myproject
# ✓ Created vibespace 'myproject'

# Start working
vibespace myproject up
# ✓ Port-forward daemon started
# ✓ claude-1 → localhost:7681

vibespace myproject connect
# Connected to claude-1 via terminal
```

### Multi-Agent Workflow

```bash
# Spawn additional agent
vibespace myproject spawn
# ✓ Created claude-2

# Check agents
vibespace myproject agents
# AGENT      STATUS    PORT
# claude-1   running   localhost:7681
# claude-2   running   localhost:17681

# Start multi-agent TUI
vibespace multi myproject
# > @claude-1 work on the auth module
# > @claude-2 write tests for the API
# > @all what's your status?
```

### Non-Interactive / Scripting Mode

```bash
# Auto-detection: non-TTY defaults to JSON output
echo "list files in current directory" | vibespace multi myproject
# {
#   "session": "myproject",
#   "request": {"target": "all", "message": "list files..."},
#   "responses": [{"agent": "claude-1@myproject", ...}]
# }

# Explicit JSON mode with inline message
vibespace multi myproject --json "what is your status?"

# Plain text output
vibespace multi myproject --plain "list files"
# [claude-1@myproject]
# Here are the files in the current directory:
# - main.go
# - go.mod

# Streaming plain text (real-time output)
vibespace multi myproject --plain --stream "work on the auth module"

# Target specific agent
vibespace multi myproject --json --agent claude-1 "run the tests"

# List agents without sending a message
vibespace multi myproject --list-agents
# {
#   "session": "myproject",
#   "agents": ["claude-1@myproject", "claude-2@myproject"]
# }

# Batch mode: process multiple messages from JSONL
cat <<EOF | vibespace multi myproject --batch
{"target": "claude-1", "message": "check the logs"}
{"target": "claude-2", "message": "run the tests"}
{"target": "all", "message": "what is your status?"}
EOF

# Custom timeout for long-running tasks
vibespace multi myproject --json --timeout 5m "run full test suite"
```

### Remote Access

```bash
# On Machine A (has cluster)
vibespace serve
# ✓ WireGuard server started on :51820
# Connect with: vibespace remote connect 192.168.1.100
# Token: abc123xyz

# On Machine B (client)
vibespace remote connect 192.168.1.100 --token abc123xyz
# ✓ Connected to Machine A

vibespace list
# NAME        STATUS    AGENTS
# myproject   running   2

vibespace myproject connect
# Connected via WireGuard tunnel
```

### Cross-Vibespace Session

```bash
# Create session with multiple vibespaces
vibespace session create fullstack
vibespace session add fullstack frontend
vibespace session add fullstack backend
vibespace session add fullstack infra claude-1

# Start session TUI
vibespace session start fullstack
# > @claude-1@frontend build the login page
# > @claude-1@backend create the auth API
# > @all@frontend @all@backend coordinate on the API contract
```

### Forking for Parallel Exploration

```bash
# Fork a vibespace to explore different approaches independently
vibespace myproject fork --name myproject-experiment
# ✓ Snapshot created from myproject
# ✓ Created vibespace 'myproject-experiment' with cloned filesystem

# Now two independent vibespaces exist:
vibespace list
# NAME                   STATUS    AGENTS
# myproject              running   1
# myproject-experiment   running   1

# Work on original
vibespace myproject connect
# > Try approach A...

# Work on fork (completely independent filesystem)
vibespace myproject-experiment connect
# > Try approach B...

# If experiment succeeds, can merge changes back via git
```

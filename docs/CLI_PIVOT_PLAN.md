# Vibespace CLI Pivot Plan

## Overview

Pivot vibespace from a Tauri desktop app to a CLI-first tool. The core value prop remains: managed multi-Claude development environments with automatic port detection. The difference is interaction model - terminal-native instead of GUI.

---

## What We're Keeping

### Infrastructure (unchanged)
- **Bundled Kubernetes**: Colima (macOS) / k3s (Linux)
- **Knative**: Scale-to-zero workloads
- **Traefik**: Internal routing between services
- **PVC**: Shared storage across Claude instances

### Go Packages (refactored into CLI)
- `pkg/vibespace/` - Business logic for vibespace CRUD
- `pkg/knative/` - Knative service management
- `pkg/k8s/` - Kubernetes client and manifests
- `pkg/network/` - Traefik IngressRoute management
- `pkg/model/` - Data types

### Container Image
- ttyd (web terminal - kept for optional web access)
- Claude Code CLI pre-installed
- Port detector daemon (re-enabled)
- Node.js 20 + Python 3 + build tools

---

## What We're Removing

### Tauri App (`app/` directory)
- `src-tauri/` - Rust backend (~3,500 lines)
  - `dns_manager.rs` - DNS/TLS/port-forward setup
  - `k8s_manager.rs` - K8s lifecycle (port to Go)
  - `main.rs` - Tauri commands
- `src/` - React frontend (~2,000 lines)
  - Setup wizard
  - Vibespace management UI

### Go HTTP Server
- `api/cmd/server/` - HTTP server entry point
- `api/pkg/handler/` - HTTP handlers
- CORS, REST endpoints (replaced by direct CLI calls)

### DNS Magic
- Custom DNS server (dnsd)
- `/etc/resolver` configuration
- TLS certificate generation
- Port forwarding daemons (launchd/systemd)

---

## Cross-Platform Support

### OS Detection Strategy

```go
switch runtime.GOOS {
case "darwin":
    // macOS → Colima (Lima VM running k3s)
    // Supports both arm64 (Apple Silicon) and amd64 (Intel)
case "linux":
    // Linux → k3s native (no VM needed)
    // Supports amd64 and arm64
default:
    // Windows → Not supported, recommend WSL2
    return errors.New("Windows not supported. Please use WSL2.")
}
```

### Architecture Detection

```go
arch := runtime.GOARCH  // "amd64" or "arm64"
```

### Binary Download on Init

The CLI downloads required binaries on first run rather than bundling them:

```bash
vibespace init
# Detecting platform... macOS (arm64)
# Downloading Colima v0.8.1...     [=====>] 100%
# Downloading Lima v1.0.0...       [=====>] 100%
# Downloading kubectl v1.29.0...   [=====>] 100%
# Starting Colima VM...
# Installing Knative...            [=====>] 100%
# Installing Traefik...            [=====>] 100%
# ✓ Cluster ready
```

### Download Sources

| Binary | macOS | Linux |
|--------|-------|-------|
| Colima | GitHub releases | N/A |
| Lima | GitHub releases | N/A |
| k3s | N/A (inside Colima) | GitHub releases |
| kubectl | dl.k8s.io | dl.k8s.io |

### Version Pinning

Store versions in CLI or config:

```go
var DefaultVersions = map[string]string{
    "colima":  "0.8.1",
    "lima":    "1.0.0",
    "k3s":     "1.29.0",
    "kubectl": "1.29.0",
}
```

User can override in `~/.vibespace/config.yaml`:

```yaml
versions:
  colima: "0.8.1"
  k3s: "1.30.0"
```

### External Cluster Support

Power users can skip bundled k8s:

```bash
vibespace init --external                    # Use ~/.kube/config
vibespace init --kubeconfig /path/to/config  # Use specific kubeconfig
```

### Directory Structure

```
~/.vibespace/
├── bin/
│   ├── colima           # macOS only
│   ├── lima             # macOS only
│   ├── limactl          # macOS only
│   ├── qemu-system-*    # macOS only (if needed)
│   └── kubectl          # Both platforms
├── kubeconfig           # Generated kubeconfig
├── config.yaml          # User configuration
├── credentials.enc      # Encrypted credentials
└── cache/
    └── downloads/       # Cached binary downloads
```

---

## New CLI Structure

```
vibespace/
├── cmd/
│   └── vibespace/           # CLI entry point
│       └── main.go
├── internal/
│   ├── cli/                 # CLI commands (cobra)
│   │   ├── root.go
│   │   ├── init.go          # vibespace init
│   │   ├── status.go        # vibespace status
│   │   ├── create.go        # vibespace create <name>
│   │   ├── list.go          # vibespace list
│   │   ├── delete.go        # vibespace delete <name>
│   │   ├── agents.go        # vibespace <name> agents
│   │   ├── spawn.go         # vibespace <name> spawn
│   │   ├── connect.go       # vibespace <name> connect <agent>
│   │   ├── ports.go         # vibespace <name> ports
│   │   ├── forward.go       # vibespace <name> forward <port>
│   │   └── multi.go         # vibespace <name> multi (multi-agent mode)
│   ├── platform/            # Cross-platform support
│   │   ├── detect.go        # OS/arch detection
│   │   ├── download.go      # Binary downloader
│   │   ├── colima.go        # macOS Colima management
│   │   └── k3s.go           # Linux k3s management
│   ├── config/              # Configuration management
│   │   ├── config.go        # ~/.vibespace/config.yaml
│   │   └── credentials.go   # Encrypted credential storage
│   └── terminal/            # Terminal UI utilities
│       ├── spinner.go       # Progress spinners
│       ├── table.go         # Table output
│       └── multiplexer.go   # Multi-agent terminal
├── pkg/                     # Existing packages (minimal changes)
│   ├── vibespace/
│   ├── knative/
│   ├── k8s/
│   ├── network/
│   └── model/
└── Makefile
```

---

## CLI Commands

### Cluster Management
```bash
vibespace init                    # Download binaries, start k8s, install components
vibespace init --external         # Use existing kubeconfig
vibespace status                  # Show cluster and component status
vibespace stop                    # Stop k8s cluster (preserves data)
vibespace uninstall               # Remove k8s and all data
```

### Vibespace Management
```bash
vibespace create <name>           # Create new vibespace
vibespace list                    # List all vibespaces
vibespace delete <name>           # Delete vibespace
vibespace start <name>            # Scale up from zero
vibespace stop <name>             # Scale down to zero
```

### Agent Management
```bash
vibespace <name> agents           # List Claude instances
vibespace <name> spawn            # Spawn new Claude instance
vibespace <name> kill <agent-id>  # Remove Claude instance
```

### Interaction
```bash
vibespace <name> connect <agent>  # Connect to single agent
vibespace <name> multi            # Multi-agent terminal mode
vibespace <name> ports            # List detected ports
vibespace <name> forward <port>   # Forward port to localhost
```

---

## Multi-Agent Terminal Mode

### Design: Interleaved Output with @ Routing

```
$ vibespace myproject multi
Connected to: claude-abc, claude-def
Type @<agent> to direct message, @all to broadcast

[claude-abc] Ready to help with the project.
[claude-def] Ready to help with the project.

> @abc work on the API endpoint for user registration
[claude-abc] I'll start working on the user registration API...

> @def write tests for the user service
[claude-def] I'll write tests for UserService...

> @all what's the current status?
[claude-abc] API endpoint is 80% complete, working on validation.
[claude-def] Unit tests done, starting integration tests.
```

### Implementation

```go
// internal/terminal/multiplexer.go

type MultiSession struct {
    vibespace string
    agents    map[string]*AgentConn
    input     chan UserInput
    output    chan AgentOutput
    colors    map[string]string  // agent -> ANSI color
}

type UserInput struct {
    Target  string  // "abc", "def", "all"
    Message string
}

type AgentOutput struct {
    AgentID   string
    Text      string
    Streaming bool
}

func (m *MultiSession) Run(ctx context.Context) error {
    // 1. Connect to all agents via kubectl exec
    // 2. Spawn goroutine per agent to read output
    // 3. Main loop: read user input, parse @ prefix, route message
    // 4. Display output with color-coded agent prefix
}
```

### Connection via kubectl exec

```go
func (m *MultiSession) connectAgent(agentID string) (*AgentConn, error) {
    pod := fmt.Sprintf("vibespace-%s-%s", m.vibespace, agentID)

    cmd := exec.CommandContext(ctx, "kubectl",
        "exec", "-i", pod,
        "-n", "vibespace",
        "--", "claude", "--chat")

    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()

    return &AgentConn{
        ID:     agentID,
        Stdin:  stdin,
        Stdout: stdout,
        Cmd:    cmd,
    }, cmd.Start()
}
```

---

## Port Detection & Forwarding

### Port Detector in Container

Re-enable port detector daemon. Writes to file:

```json
// /tmp/vibespace-ports.json
{
  "ports": [
    {"port": 3000, "detected_at": "2024-01-15T10:30:00Z", "process": "node"},
    {"port": 5173, "detected_at": "2024-01-15T10:31:00Z", "process": "vite"}
  ]
}
```

### CLI Reads Ports

```bash
$ vibespace myproject ports
PORT   PROCESS   DETECTED
3000   node      2 minutes ago
5173   vite      30 seconds ago
```

Implementation:
```go
func getPorts(vibespace, agentID string) ([]Port, error) {
    pod := getPodName(vibespace, agentID)
    out, err := exec.Command("kubectl", "exec", pod, "-n", "vibespace",
        "--", "cat", "/tmp/vibespace-ports.json").Output()
    // Parse JSON, return ports
}
```

### Port Forwarding

```bash
$ vibespace myproject forward 3000
Forwarding myproject:3000 → localhost:3000
Press Ctrl+C to stop
```

Implementation:
```go
func forwardPort(vibespace string, port int) error {
    svc := fmt.Sprintf("svc/vibespace-%s", vibespace)
    cmd := exec.Command("kubectl", "port-forward", svc,
        fmt.Sprintf("%d:%d", port, port), "-n", "vibespace")
    return cmd.Run()
}
```

---

## Configuration

### ~/.vibespace/config.yaml

```yaml
# Cluster settings
cluster:
  provider: auto          # auto, colima, k3s, external
  kubeconfig: ~/.vibespace/kubeconfig

# Version overrides (optional)
versions:
  colima: "0.8.1"
  k3s: "1.29.0"

# Default settings for new vibespaces
defaults:
  resources:
    cpu: "250m"
    memory: "512Mi"
    storage: "10Gi"

# Credentials stored separately (encrypted)
credentials_file: ~/.vibespace/credentials.enc

# Remote clusters (future feature)
remotes:
  my-vps:
    host: vps.example.com
    user: ubuntu
    ssh_key: ~/.vibespace/keys/my-vps
```

---

## Implementation Phases

### Phase 1: Core CLI Structure (3-4 days)
- [ ] Set up cobra CLI in `cmd/vibespace/`
- [ ] Implement platform detection (`internal/platform/detect.go`)
- [ ] Implement binary downloader (`internal/platform/download.go`)
- [ ] Port Colima management from Rust (`internal/platform/colima.go`)
- [ ] Port k3s management from Rust (`internal/platform/k3s.go`)
- [ ] Implement `init`, `status`, `stop` commands

### Phase 2: Vibespace CRUD (2-3 days)
- [ ] Wire up existing `pkg/vibespace` to CLI
- [ ] Implement `create`, `list`, `delete` commands
- [ ] Implement `start`, `stop` commands (scale up/down)
- [ ] Implement `<name> agents`, `spawn`, `kill` commands

### Phase 3: Connection & Ports (2-3 days)
- [ ] Implement `connect` command (single agent via kubectl exec)
- [ ] Re-enable port detector in container image
- [ ] Implement `ports` command
- [ ] Implement `forward` command

### Phase 4: Multi-Agent Mode (3-4 days)
- [ ] Build terminal multiplexer (`internal/terminal/multiplexer.go`)
- [ ] Implement `multi` command
- [ ] Add @ prefix routing
- [ ] Color-coded output per agent
- [ ] Handle agent disconnect/reconnect

### Phase 5: Polish (2-3 days)
- [ ] Config file loading/saving
- [ ] Encrypted credential storage
- [ ] Shell completions (bash/zsh/fish)
- [ ] Error messages and help text
- [ ] Integration testing

---

## File Migration Plan

### Keep (move if needed)
```
api/pkg/vibespace/    →  pkg/vibespace/     (already there)
api/pkg/knative/      →  pkg/knative/       (already there)
api/pkg/k8s/          →  pkg/k8s/           (already there)
api/pkg/network/      →  pkg/network/       (already there)
api/pkg/model/        →  pkg/model/         (already there)
api/pkg/image/        →  pkg/image/         (container Dockerfile)
```

### Delete
```
app/                  # Entire Tauri app
api/cmd/server/       # HTTP server
api/pkg/handler/      # HTTP handlers
demiurg-backend/      # Reference only, not needed
demiurg-frontend-next/ # Reference only, not needed
```

### Create
```
cmd/vibespace/main.go
internal/cli/
internal/platform/
internal/config/
internal/terminal/
```

---

## Documentation Updates Required

> **Note for next LLM**: This section describes documentation changes needed after the CLI pivot is complete. Do NOT update docs until the implementation is done and tested.

### Files to Update

#### 1. `docs/SPEC.md`
**Current state**: Describes Tauri app architecture with React frontend, Go API server, DNS management, etc.

**Required changes**:
- Remove Section 3 (Frontend Architecture) - no more React
- Remove Section 4.2 (HTTP API) - no more REST endpoints
- Rewrite Section 4 (Backend Architecture) to describe CLI structure
- Remove Section 6 (DNS/TLS) entirely
- Update Section 7 (Container Image) - re-enable port detector
- Rewrite Section 9 (User Flows) for CLI commands
- Update all architecture diagrams

**Key sections to rewrite**:
- "System Architecture" diagram → CLI + K8s only
- "API Endpoints" → CLI commands table
- "Setup Flow" → `vibespace init` flow
- "Vibespace Creation" → `vibespace create` flow

#### 2. `docs/ROADMAP.md`
**Current state**: 4-phase roadmap targeting Tauri desktop app

**Required changes**:
- Reframe Phase 1 around CLI MVP
- Update Phase 2 for multi-agent terminal mode
- Remove references to React frontend, setup wizard
- Add new milestones for CLI features

#### 3. `.claude/CLAUDE.md` (project instructions)
**Current state**: Describes Tauri + React + Go architecture

**Required changes**:
- Update "What This Project Does" section
- Rewrite "Architecture" diagram for CLI
- Update "Repository Structure" section
- Remove frontend-related conventions
- Update "Development" section with CLI build/run commands
- Remove demiurg references (no longer porting chat UI)
- Update troubleshooting for CLI issues

#### 4. `docs/adr/` (Architecture Decision Records)
**New ADR needed**: `0016-cli-pivot.md`

**Content for new ADR**:
```markdown
# ADR 0016: CLI Pivot from Tauri Desktop App

## Status
Accepted

## Context
The original vibespace was a Tauri desktop app with React frontend.
This required complex DNS/TLS setup and GUI-based interactions.

## Decision
Pivot to CLI-first tool that:
- Removes DNS magic (use port-forward instead)
- Removes React frontend (terminal-native UI)
- Embeds Go API logic directly in CLI (no HTTP server)
- Adds multi-agent terminal mode for parallel Claude interaction

## Consequences
- Simpler installation (no DNS/TLS setup)
- Works in headless/SSH environments
- Better for power users and CI/CD
- Loses GUI discoverability (acceptable tradeoff)
```

**ADRs to mark superseded**:
- `0006-bundled-kubernetes-runtime.md` - Update to reflect CLI download approach
- Any DNS-related ADRs - Mark as superseded

#### 5. `README.md` (root)
**Required changes**:
- Update installation instructions for CLI
- Replace screenshots with terminal examples
- Update quick start guide
- Remove Tauri/React build instructions

### Documentation Update Checklist

When updating docs after implementation:

- [ ] Read each doc file completely before editing
- [ ] Update architecture diagrams (can use ASCII art for CLI)
- [ ] Replace all React/Tauri references with CLI equivalents
- [ ] Add CLI command examples with expected output
- [ ] Update troubleshooting for common CLI issues
- [ ] Ensure CLAUDE.md stays concise (it's context for AI)
- [ ] Create ADR 0016 documenting the pivot decision
- [ ] Remove references to demiurg codebases
- [ ] Update any CI/CD references if present

### What NOT to Document Yet

- Remote mode (future feature)
- NATS messaging (deferred)
- Web UI (not planned)

---

## Open Questions

1. **Binary caching**: Cache downloads across `vibespace uninstall`/`init` cycles?
   - Recommendation: Yes, keep in `~/.vibespace/cache/`

2. **Colima resource allocation**: How much CPU/RAM for the VM?
   - Default: 2 CPU, 4GB RAM (configurable in config.yaml)

3. **Multi-agent limit**: Max Claude instances per vibespace?
   - Start with no limit, add if resource issues arise

4. **Container registry**: Keep using GHCR or switch?
   - Keep GHCR for now, simpler

---

## Success Criteria

- [ ] `vibespace init` works on macOS (arm64 + amd64)
- [ ] `vibespace init` works on Linux (amd64)
- [ ] `vibespace create foo` creates a vibespace with Claude
- [ ] `vibespace foo agents` shows running Claude instances
- [ ] `vibespace foo connect claude-abc` provides interactive Claude session
- [ ] `vibespace foo multi` allows chatting with multiple agents via @ routing
- [ ] `vibespace foo ports` shows auto-detected ports
- [ ] `vibespace foo forward 3000` exposes port locally
- [ ] Full flow works without any DNS/TLS configuration

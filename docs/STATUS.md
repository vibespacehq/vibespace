# Implementation Status

## Legend
- **Done**: Fully implemented
- **Stub**: Command exists but prints "not implemented" or returns hardcoded data
- **Partial**: Some functionality works, incomplete
- **TODO**: Not started

---

## Cluster Management

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace init` | Done | Downloads Colima/Lima/kubectl, starts cluster, installs Knative |
| `vibespace status` | Done | Shows Colima, Knative, namespace status |
| `vibespace stop` | Done | Stops Colima |
| `vibespace serve` | TODO | WireGuard server mode |

## Vibespace Management

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace create <name>` | Done | Creates Knative service + PVC |
| `vibespace list` | Done | Lists all vibespaces |
| `vibespace delete <name>` | Done | Deletes Knative service |
| `vibespace <name> start` | Done | Sets minScale=1 |
| `vibespace <name> stop` | Done | Sets minScale=0 |
| `vibespace <name> status` | TODO | Per-vibespace detailed status |

## Agent Management

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace <name> agents` | Stub | Returns hardcoded "claude-1" |
| `vibespace <name> spawn` | Stub | Prints "not implemented" |
| `vibespace <name> kill` | Stub | Prints "not implemented" |

## Connection

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace <name> connect` | Partial | Shells out to kubectl, opens browser. Needs: client-go + ttyd websocket |
| `vibespace <name> ports` | Partial | Shells out to kubectl exec. Needs: client-go |
| `vibespace <name> forward <port>` | Partial | Shells out to kubectl port-forward (blocking). Needs: client-go daemon |

## Port Forwarding Daemon

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace <name> up` | TODO | Start background daemon |
| `vibespace <name> down` | TODO | Stop daemon |
| `vibespace <name> forward list` | TODO | |
| `vibespace <name> forward add` | TODO | |
| `vibespace <name> forward remove` | TODO | |
| `vibespace <name> forward stop` | TODO | |
| `vibespace <name> forward start` | TODO | |
| `vibespace <name> forward restart` | TODO | |
| `vibespace <name> forward restart-all` | TODO | |

## Multi-Session

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace multi` | Partial | Has TUI structure, uses kubectl exec to bash (not ttyd websocket) |
| `vibespace session create` | TODO | |
| `vibespace session list` | TODO | |
| `vibespace session delete` | TODO | |
| `vibespace session add` | TODO | |
| `vibespace session remove` | TODO | |
| `vibespace session start` | TODO | |

## Remote Mode

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace remote connect` | TODO | |
| `vibespace remote disconnect` | TODO | |
| `vibespace remote status` | TODO | |

## Utility

| Command | Status | Notes |
|---------|--------|-------|
| `vibespace version` | Done | |
| `vibespace help` | Done | |
| `vibespace <name>` (subcommand help) | Done | |

---

## Core Packages

| Package | Status | Notes |
|---------|--------|-------|
| `pkg/k8s` | Done | Kubernetes client wrapper |
| `pkg/knative` | Done | Knative service CRUD |
| `pkg/vibespace` | Done | Business logic (uses Knative) |
| `pkg/model` | Done | Data models |
| `pkg/image` | Done | Container image (ttyd + Claude) |
| `internal/platform` | Done | Colima management |
| `pkg/portforward` | TODO | client-go based port-forward manager |
| `pkg/daemon` | TODO | Background daemon + IPC |
| `pkg/session` | TODO | Multi-session state management |
| `pkg/tui` | TODO | Terminal UI (ttyd websocket) |
| `pkg/wireguard` | TODO | WireGuard integration |

---

## Next Priority

1. **Port-forward daemon** (`pkg/portforward`, `pkg/daemon`)
   - client-go based forwarding with auto-reconnect
   - Background process with Unix socket IPC
   - `up`, `down`, `forward *` commands

2. **Multi-agent support**
   - Implement `spawn` (create additional Knative services per agent)
   - Implement `agents` (query actual pods)
   - Implement `kill`

3. **ttyd websocket connection**
   - Replace kubectl exec with websocket to ttyd
   - Enable proper terminal passthrough in `connect`
   - Update `multi` to use websocket connections

4. **Session management**
   - Session state persistence
   - `session *` commands
   - Cross-vibespace sessions

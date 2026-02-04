# Vibespace vs Netclode: Comprehensive Architecture Comparison

## 1. Project Overview

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Purpose** | Multi-agent AI dev environments for collaboration | Self-hosted remote coding agent for mobile access |
| **Primary Use Case** | Multiple AI agents working together on projects | On-the-go prompting from phone/tablet |
| **Target User** | Developers at their workstations | Developers away from keyboard |
| **Philosophy** | CLI-native, developer ergonomics | Mobile-first, polished UX |
| **License** | (Your project) | Open source (GitHub) |

---

## 2. Tech Stack Comparison

### Languages

| Layer | Vibespace | Netclode |
|-------|-----------|----------|
| **Control Plane** | Go | Go |
| **Agent Service** | Go (SSH to containers) | TypeScript/Node.js |
| **Client** | Go (TUI) | Swift (SwiftUI) |
| **CLI** | Go (Cobra) | Go (debug only) |

### Core Dependencies

| Category | Vibespace | Netclode |
|----------|-----------|----------|
| **CLI Framework** | spf13/cobra | — |
| **TUI Framework** | charmbracelet/bubbletea | — |
| **TUI Styling** | charmbracelet/lipgloss | — |
| **K8s Client** | k8s.io/client-go | k8s.io/client-go |
| **API Protocol** | SSH + Unix sockets + HTTP | Connect RPC (protobuf) |
| **Code Generation** | — | Buf (proto → Go/TS/Swift) |
| **Crypto** | golang.org/x/crypto (ED25519) | — |

### Infrastructure

| Component | Vibespace | Netclode |
|-----------|-----------|----------|
| **Orchestration** | k3s (inside Lima VM) | k3s (on host) |
| **VM Manager** | Colima (macOS) / Lima (Linux) | Kata Containers + Cloud Hypervisor |
| **Container Runtime** | containerd (standard) | containerd + Kata runtime |
| **Storage** | Local PVCs | JuiceFS → S3 |
| **State Store** | JSON files | Redis (Streams, Hashes, Sets) |
| **Networking** | WireGuard (DIY) | Tailscale (managed) |
| **Provisioning** | CLI auto-downloads binaries | Ansible playbooks |

---

## 3. Architecture Diagrams

### Vibespace Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│ Host (macOS / Linux)                                                │
│                                                                     │
│  ┌────────────────┐    ┌────────────────┐    ┌──────────────────┐  │
│  │ vibespace CLI  │    │ Port Forward   │    │ Remote Server    │  │
│  │ (Cobra + TUI)  │    │ Daemon         │    │ (WireGuard)      │  │
│  └───────┬────────┘    └───────┬────────┘    └────────┬─────────┘  │
│          │ SSH                 │ Unix socket          │ HTTP       │
│          │                     │                      │            │
│  ┌───────┴─────────────────────┴──────────────────────┴─────────┐  │
│  │ Lima VM (Colima on macOS)                                    │  │
│  │  ┌────────────────────────────────────────────────────────┐  │  │
│  │  │ k3s Cluster                                            │  │  │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │  │  │
│  │  │  │ Vibespace A │  │ Vibespace B │  │ Vibespace C │    │  │  │
│  │  │  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │    │  │  │
│  │  │  │ │Claude   │ │  │ │Claude   │ │  │ │Codex    │ │    │  │  │
│  │  │  │ │Agent    │ │  │ │Agent    │ │  │ │Agent    │ │    │  │  │
│  │  │  │ └─────────┘ │  │ ├─────────┤ │  │ └─────────┘ │    │  │  │
│  │  │  │             │  │ │Codex    │ │  │             │    │  │  │
│  │  │  │             │  │ │Agent    │ │  │             │    │  │  │
│  │  │  │ ┌─────────┐ │  │ └─────────┘ │  │ ┌─────────┐ │    │  │  │
│  │  │  │ │ PVC     │ │  │ ┌─────────┐ │  │ │ PVC     │ │    │  │  │
│  │  │  │ │ (local) │ │  │ │ PVC     │ │  │ │ (local) │ │    │  │  │
│  │  │  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │    │  │  │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘    │  │  │
│  │  └────────────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### Netclode Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│ Linux VPS (Bare Metal / Cloud)                                      │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ k3s Cluster (directly on host)                               │  │
│  │                                                              │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │  │
│  │  │ Control     │  │ Tailscale   │  │ Ollama (GPU)        │  │  │
│  │  │ Plane (Go)  │  │ Operator    │  │ (optional local LLM)│  │  │
│  │  └──────┬──────┘  └─────────────┘  └─────────────────────┘  │  │
│  │         │                                                    │  │
│  │         │ Connect RPC                                        │  │
│  │         │                                                    │  │
│  │  ┌──────┴────────────────────────────────────────────────┐  │  │
│  │  │ Warm Pool (agent-sandbox controller)                  │  │  │
│  │  │  ┌───────────────┐  ┌───────────────┐                 │  │  │
│  │  │  │ Kata Pod      │  │ Kata Pod      │  (pre-booted)   │  │  │
│  │  │  │ ┌───────────┐ │  │ ┌───────────┐ │                 │  │  │
│  │  │  │ │ microVM   │ │  │ │ microVM   │ │                 │  │  │
│  │  │  │ │ (Cloud    │ │  │ │ (Cloud    │ │                 │  │  │
│  │  │  │ │ Hyperv.)  │ │  │ │ Hyperv.)  │ │                 │  │  │
│  │  │  │ │ ┌───────┐ │ │  │ │ ┌───────┐ │ │                 │  │  │
│  │  │  │ │ │Agent  │ │ │  │ │ │Agent  │ │ │                 │  │  │
│  │  │  │ │ │(Node) │ │ │  │ │ │(Node) │ │ │                 │  │  │
│  │  │  │ │ ├───────┤ │ │  │ │ ├───────┤ │ │                 │  │  │
│  │  │  │ │ │Docker │ │ │  │ │ │Docker │ │ │                 │  │  │
│  │  │  │ │ └───────┘ │ │  │ │ └───────┘ │ │                 │  │  │
│  │  │  │ └───────────┘ │  │ └───────────┘ │                 │  │  │
│  │  │  └───────┬───────┘  └───────┬───────┘                 │  │  │
│  │  └──────────┼──────────────────┼─────────────────────────┘  │  │
│  │             │ virtiofs         │ virtiofs                   │  │
│  │             ▼                  ▼                            │  │
│  │  ┌─────────────────────────────────────────────────────┐   │  │
│  │  │ JuiceFS CSI Driver                                  │   │  │
│  │  └─────────────────────────┬───────────────────────────┘   │  │
│  └────────────────────────────┼────────────────────────────────┘  │
│                               │                                    │
└───────────────────────────────┼────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼                       ▼
             ┌────────────┐          ┌────────────┐
             │ Redis      │          │ S3 (DO     │
             │ (metadata) │          │ Spaces)    │
             └────────────┘          └────────────┘
```

---

## 4. Isolation Model

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Isolation Boundary** | Lima VM (shared by all pods) | Kata microVM (per pod) |
| **Kernel** | Shared Linux kernel in VM | Separate kernel per sandbox |
| **Container Runtime** | Standard containerd | Kata runtime + containerd |
| **Privileged Containers** | Risky (shared kernel) | Safe (VM boundary) |
| **Docker-in-Docker** | Not recommended | Full support (safe) |
| **Sudo Access** | Limited | Full (confined to VM) |
| **Network Isolation** | K8s NetworkPolicy | NetworkPolicy + VM boundary |
| **Escape Impact** | Access to all pods in VM | Access to single VM only |

### Security Trade-offs

```
Vibespace Security Model:
┌─────────────────────────────────────┐
│ Lima VM                             │
│  ┌───────┐ ┌───────┐ ┌───────┐     │
│  │ Pod A │ │ Pod B │ │ Pod C │     │  ← Shared kernel
│  └───────┘ └───────┘ └───────┘     │    Container escape = all pods
└─────────────────────────────────────┘
     ↓ VM escape = host access

Netclode Security Model:
┌───────────┐ ┌───────────┐ ┌───────────┐
│ Kata VM A │ │ Kata VM B │ │ Kata VM C │  ← Separate kernels
│  ┌─────┐  │ │  ┌─────┐  │ │  ┌─────┐  │    Container escape = 1 VM
│  │ Pod │  │ │  │ Pod │  │ │  │ Pod │  │
│  └─────┘  │ │  └─────┘  │ │  └─────┘  │
└───────────┘ └───────────┘ └───────────┘
     ↓ VM escape = host access (harder)
```

---

## 5. Storage Architecture

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **PV Backend** | Local disk (hostPath / local-path) | JuiceFS → S3 |
| **Data Location** | Inside Lima VM disk | Object storage (S3/DO Spaces) |
| **Metadata** | — | Redis |
| **Pause Cost** | PVC stays on disk | Zero (only S3 storage) |
| **Resume Speed** | Fast (local) | Slower (network mount) |
| **Multi-node** | Not supported | Supported (shared storage) |
| **Snapshots** | Not implemented | CoW snapshots per turn |
| **Snapshot Restore** | — | Full workspace + tools + Docker |

### Storage Flow

```
Vibespace:
  Pod → PVC → Local Provisioner → Lima VM disk → Host disk

Netclode:
  Pod → PVC → JuiceFS CSI → ┬→ Redis (metadata)
                            └→ S3 (data chunks)
```

### Snapshot Capability (Netclode only)

```
Turn 1: User prompt → Agent response → Snapshot S1
Turn 2: User prompt → Agent response → Snapshot S2
Turn 3: User prompt → Agent response → Snapshot S3
                                            ↓
                              Restore to S2: workspace, mise tools,
                                            Docker images, SDK state
```

---

## 6. Networking

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Remote Access** | WireGuard (self-managed) | Tailscale (managed) |
| **VPN Setup** | Manual key generation | Automatic via operator |
| **Authentication** | ED25519 signed tokens | Tailscale ACLs |
| **Web Previews** | Port forwarding daemon | Tailscale Service exposure |
| **Preview URLs** | `localhost:<port>` | `sandbox-xyz.ts.net:<port>` |
| **Tailnet Access** | Not supported | Optional (100.64.0.0/10) |
| **Sandbox Egress** | Full internet | Filtered (no private ranges) |

### Remote Access Flow

```
Vibespace Remote Mode:
┌────────────┐     WireGuard      ┌────────────────┐
│ Local CLI  │◄──────────────────►│ Remote Server  │
│            │   10.100.0.0/24    │ (VPS)          │
└────────────┘                    └───────┬────────┘
                                          │ kubectl proxy
                                          ▼
                                  ┌────────────────┐
                                  │ k3s API Server │
                                  └────────────────┘

Netclode Remote Mode:
┌────────────┐     Tailscale      ┌────────────────┐
│ iOS App    │◄──────────────────►│ Control Plane  │
│            │   MagicDNS         │ (Tailscale     │
└────────────┘                    │  Ingress)      │
                                  └───────┬────────┘
                                          │ Connect RPC
                                          ▼
                                  ┌────────────────┐
                                  │ Sandbox Pods   │
                                  └────────────────┘
```

---

## 7. State Management

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Session Storage** | JSON files | Redis Hash |
| **Event Log** | Not persisted | Redis Streams |
| **Real-time Sync** | — | XREAD BLOCK |
| **Multi-client** | Single client | Multiple clients (cursor per client) |
| **Reconnect** | Start fresh | Resume from cursor |
| **Crash Recovery** | Manual | Automatic reconciliation |

### State Structures

```
Vibespace (~/.vibespace/):
├── forwards/
│   └── {vibespace}.json     # Port forward state
├── sessions/
│   └── {session}.json       # Session metadata
└── remote/
    └── state.json           # WireGuard config

Netclode (Redis):
├── session:{id}             # HASH - metadata
├── sessions:all             # SET - all session IDs
├── session:{id}:stream      # STREAM - all events
├── session:{id}:snapshots   # SORTED SET - snapshot IDs
└── session:{id}:snapshot:{snapId}  # HASH - snapshot metadata
```

### Event Streaming (Netclode)

```protobuf
message StreamEntry {
  string id = 1;
  google.protobuf.Timestamp timestamp = 2;
  bool partial = 3;  // streaming delta vs final

  oneof payload {
    AgentEvent event = 4;
    TerminalOutput terminal_output = 5;
    Session session_update = 6;
    Error error = 7;
  }
}
```

---

## 8. API & Communication

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Agent Connection** | SSH (interactive PTY) | Connect RPC stream |
| **Daemon IPC** | Unix socket | — |
| **Remote API** | HTTP + WireGuard | Connect RPC + Tailscale |
| **Protocol Format** | JSON / Raw | Protobuf |
| **Streaming** | SSH PTY stream | Bidirectional RPC stream |
| **Code Generation** | — | Buf (Go, TS, Swift) |
| **Type Safety** | Runtime | Compile-time (protobuf) |

### Communication Patterns

```
Vibespace:
┌─────────┐  SSH   ┌─────────┐
│   CLI   │───────►│  Agent  │   Interactive PTY
└─────────┘        └─────────┘

┌─────────┐  Unix  ┌─────────┐
│   CLI   │───────►│ Daemon  │   JSON messages
└─────────┘ socket └─────────┘

Netclode:
┌─────────┐  Connect RPC  ┌─────────────┐  Connect RPC  ┌─────────┐
│ iOS App │◄─────────────►│Control Plane│◄─────────────►│  Agent  │
└─────────┘  bidirectional└─────────────┘  bidirectional└─────────┘
             stream                        stream
```

---

## 9. Client & User Experience

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Primary Client** | Terminal TUI | iOS/macOS App |
| **Framework** | BubbleTea + Lipgloss | SwiftUI |
| **Multi-agent View** | Split/focused layout | Single session |
| **Message Routing** | `@agent`, `@all` | — |
| **Session Commands** | `/add`, `/remove`, `/focus` | — |
| **Git Diff View** | — | Built-in (word-level highlight) |
| **Live Terminal** | SSH connect | SwiftTerm embedded |
| **Markdown Render** | — | MarkdownUI + Highlightr |
| **Offline Support** | — | Queue + replay |
| **Output Modes** | Human, JSON, Plain, Stream | — |

### Multi-agent UX (Vibespace)

```
┌─────────────────────────────────────────────────────────────┐
│ vibespace multi mysession                                   │
├─────────────────────────────┬───────────────────────────────┤
│ claude@project              │ codex@project                 │
│                             │                               │
│ > Analyzing the codebase... │ > I'll help with the tests... │
│                             │                               │
│ The main entry point is     │ Running pytest...             │
│ cmd/main.go which...        │ ========================      │
│                             │ 42 passed, 2 failed           │
├─────────────────────────────┴───────────────────────────────┤
│ @all: explain the auth flow                                 │
└─────────────────────────────────────────────────────────────┘
```

### Mobile UX (Netclode)

```
┌─────────────────────────┐
│ ← Sessions    myproject │
├─────────────────────────┤
│ 🧑 run the tests        │
├─────────────────────────┤
│ 🤖 I'll run the tests   │
│    for you...           │
│                         │
│ ┌─────────────────────┐ │
│ │ 🔧 bash             │ │
│ │ pytest tests/       │ │
│ │ ──────────────────  │ │
│ │ 42 passed ✓         │ │
│ └─────────────────────┘ │
│                         │
│ All tests passing!      │
├─────────────────────────┤
│ [Type a message...]     │
└─────────────────────────┘
```

---

## 10. Agent Support

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Claude Code** | ✓ (native) | ✓ (via SDK) |
| **Codex** | ✓ (native) | ✓ (via SDK) |
| **OpenCode** | — | ✓ |
| **Copilot** | — | ✓ |
| **Multi-agent Session** | ✓ (first-class) | — |
| **Agent Config** | Shared interface + Extra map | SDK adapters |
| **Local LLM** | — | ✓ (Ollama) |

### Agent Abstraction

```go
// Vibespace: pkg/agent/agent.go
type Config interface {
    SkipPermissions() bool
    AllowedTools() []string
    DisallowedTools() []string
    Model() string
    MaxTurns() int
    ReasoningEffort() string
    Extra() map[string]any
}
```

```typescript
// Netclode: SDK adapter interface
interface SDKAdapter {
  initialize(config: SDKConfig): Promise<void>;
  executePrompt(sessionId: string, text: string): AsyncGenerator<PromptEvent>;
  setInterruptSignal(): void;
}
```

---

## 11. Features Comparison

| Feature | Vibespace | Netclode |
|---------|-----------|----------|
| **Create workspace** | ✓ | ✓ |
| **Multiple agents per workspace** | ✓ | — |
| **Agent collaboration** | ✓ (`@agent` routing) | — |
| **Session persistence** | ✓ (JSON) | ✓ (Redis) |
| **Pause/Resume** | ✓ (stop/start pods) | ✓ (delete/recreate pods) |
| **Zero-cost pause** | — | ✓ (JuiceFS offload) |
| **Snapshots** | — | ✓ (CoW per turn) |
| **Snapshot restore** | — | ✓ (full state) |
| **Port forwarding** | ✓ (daemon) | ✓ (Tailscale) |
| **Web previews** | localhost only | Tailnet URLs |
| **Git integration** | Mount host dirs | GitHub App tokens |
| **Multi-repo** | — | ✓ |
| **Docker in sandbox** | — | ✓ |
| **Live terminal** | ✓ (SSH) | ✓ (PTY + SwiftTerm) |
| **Warm pool** | — | ✓ (instant start) |
| **Local LLM** | — | ✓ (Ollama + GPU) |
| **Cross-platform** | ✓ (macOS + Linux) | Linux only |
| **Mobile client** | — | ✓ (iOS/macOS) |

---

## 12. Deployment & Operations

| Aspect | Vibespace | Netclode |
|--------|-----------|----------|
| **Installation** | `vibespace init` (auto-downloads) | Ansible playbook |
| **Dependencies** | Colima/Lima, kubectl (bundled) | KVM, Kata, JuiceFS, Redis, Tailscale |
| **Platform** | macOS, Linux | Linux only (KVM required) |
| **Nested Virt** | Not needed | Required for Kata |
| **Cloud Providers** | Any (runs locally) | OVH, home server (not Hetzner) |
| **GPU Support** | — | ✓ (NVIDIA Operator) |
| **Scaling** | Single node | Multi-node ready |
| **Resource Management** | K8s requests/limits | Kata VM + K8s + overcommit |

### Resource Configuration

```yaml
# Vibespace: via CLI flags
vibespace create myproject \
  --cpu 2 \
  --memory 4Gi \
  --storage 10Gi

# Netclode: Kata + K8s + iOS app
# VM resources in kata config
# Pod resources in deployment
# Overcommit ratios for scheduling
# Per-session override via annotations
```

---

## 13. Strengths & Weaknesses

### Vibespace

| Strengths | Weaknesses |
|-----------|------------|
| ✓ Multi-agent collaboration (unique) | ✗ No per-pod isolation (shared kernel) |
| ✓ Cross-platform (macOS + Linux) | ✗ No mobile client |
| ✓ Developer-friendly CLI/TUI | ✗ No distributed storage |
| ✓ Simpler stack (easier to understand) | ✗ No snapshots |
| ✓ Port forwarding daemon with reconciliation | ✗ No warm pool |
| ✓ Works on laptops | ✗ No Docker-in-sandbox |
| ✓ Lightweight (no microVMs) | ✗ Single-client oriented |

### Netclode

| Strengths | Weaknesses |
|-----------|------------|
| ✓ Strong isolation (Kata microVMs) | ✗ Linux only |
| ✓ Docker inside sandbox | ✗ Complex stack |
| ✓ JuiceFS + S3 (zero-cost pause) | ✗ Requires nested virt |
| ✓ CoW snapshots | ✗ Single-agent per session |
| ✓ Polished mobile app | ✗ No multi-agent collaboration |
| ✓ Redis Streams (real-time sync) | ✗ Heavier resource usage |
| ✓ Warm pool (instant start) | ✗ More operational complexity |
| ✓ Local LLM support (Ollama) | ✗ Slower storage (network) |
| ✓ Multiple SDK support | |

---

## 14. What Each Could Learn

### Vibespace Could Adopt from Netclode

| Feature | Benefit | Complexity |
|---------|---------|------------|
| **JuiceFS storage** | Zero-cost pause, multi-node | Medium |
| **CoW snapshots** | Restore any turn's full state | Medium |
| **Redis Streams** | Multi-client sync, reconnect | Low |
| **Warm pool** | Instant session start | Medium (needs agent-sandbox) |
| **Kata isolation** | Safe Docker/sudo in sandbox | High |
| **Mobile companion app** | On-the-go prompting | High |
| **Tailscale integration** | Simpler networking, web previews | Low |
| **Connect RPC** | Type-safe streaming, multi-lang | Medium |

### Netclode Could Adopt from Vibespace

| Feature | Benefit | Complexity |
|---------|---------|------------|
| **Multi-agent sessions** | Agent collaboration | Medium |
| **`@agent` routing** | Direct agent messages | Low |
| **Cross-platform (Lima)** | macOS support | High |
| **Terminal TUI** | Developer-friendly interface | Medium |
| **Port forward daemon** | Background reconciliation | Low |
| **Simpler local mode** | Development without infra | Medium |

---

## 15. Architectural Decision Summary

| Decision | Vibespace Choice | Netclode Choice | Trade-off |
|----------|------------------|-----------------|-----------|
| **Isolation** | Shared VM | Per-pod VM | Security vs simplicity |
| **Storage** | Local PVC | JuiceFS + S3 | Speed vs flexibility |
| **State** | JSON files | Redis | Simplicity vs features |
| **API** | SSH + sockets | Connect RPC | Familiarity vs type-safety |
| **Client** | Terminal TUI | Native mobile | Developer vs mobile UX |
| **Networking** | DIY WireGuard | Managed Tailscale | Control vs convenience |
| **Platform** | Cross-platform | Linux only | Reach vs capabilities |
| **Focus** | Multi-agent | Single-agent mobile | Collaboration vs mobility |

---

## 16. Conclusion

Both projects solve the "remote AI coding agent" problem but optimize for different scenarios:

**Vibespace** is ideal for:
- Developers who work primarily at their workstation
- Use cases requiring multiple agents collaborating
- Teams wanting cross-platform support
- Those preferring simpler, understandable architectures

**Netclode** is ideal for:
- Developers who want mobile access to coding agents
- Use cases requiring strong sandbox isolation
- Those needing Docker/sudo capabilities in sandboxes
- Users wanting polished UX over raw power

The architectures reflect these priorities: Vibespace invests in multi-agent orchestration and developer ergonomics, while Netclode invests in isolation, state management, and mobile experience.


Great question. Let me give you my honest take on what's worth adopting vs. what's
  not, given Vibespace's identity and goals.

  My Recommendations

  1. Tailscale → YES, adopt it

  Why: Your DIY WireGuard implementation is reinventing what Tailscale already does
  better.
  ┌─────────────────────────────┬───────────────────────────────────┐
  │    Your current approach    │             Tailscale             │
  ├─────────────────────────────┼───────────────────────────────────┤
  │ Manual key generation       │ Automatic                         │
  ├─────────────────────────────┼───────────────────────────────────┤
  │ DIY invite tokens (ED25519) │ Built-in device auth              │
  ├─────────────────────────────┼───────────────────────────────────┤
  │ Manual IP management        │ MagicDNS                          │
  ├─────────────────────────────┼───────────────────────────────────┤
  │ Port forwarding daemon      │ tailscale.com/expose annotation   │
  ├─────────────────────────────┼───────────────────────────────────┤
  │ Web previews on localhost   │ sandbox.ts.net:3000 from anywhere │
  └─────────────────────────────┴───────────────────────────────────┘
  Effort: Low. Tailscale Operator is a Helm chart. Your remote mode becomes:
  vibespace init --tailscale  # joins tailnet
  # done. accessible from any device on your tailnet

  Keep your differentiator: Multi-agent still works, just with simpler networking.

  ---
  2. Redis (Streams) → YES, adopt it

  Why: Enables features you'll want eventually:
  - Multi-client sync (TUI + future mobile/web)
  - Proper reconnect semantics
  - Event replay / history
  - Crash recovery

  Effort: Low-Medium. Redis is simple to deploy (single binary or K8s pod).

  // Instead of JSON files:
  // session:{id}:stream for events
  // XREAD BLOCK for real-time
  // Last cursor for reconnect

  Don't need full Netclode complexity — just Streams for events + Hash for session
  metadata.

  ---
  3. JuiceFS → MAYBE, for remote mode only

  Why it's cool:
  - Zero-cost pause (delete pod, PVC stays in S3)
  - CoW snapshots per turn
  - Multi-node scaling

  Why I'm hesitant:
  - Adds latency (network storage vs local NVMe)
  - More infra (Redis metadata + S3)
  - Vibespace is local-first — fast local storage matters

  My suggestion: Make it optional:
  vibespace init                      # local PVCs (fast)
  vibespace init --storage=juicefs    # S3-backed (pausable)

  Or only use JuiceFS in remotie mode where you actually need the decoupling.

  ---
  4. Snapshots → YES, if you do JuiceFS

  Why: Being able to restore to any turn (workspace + tools + Docker state) is genuinely
   powerful. Git only saves code, not the full environment.

  Depends on: JuiceFS (for CoW cloning). Without it, snapshots are expensive full
  copies.

  ---
  5. Kata / MicroVMs → NO, skip it

  Why not:
  ┌───────────────────┬──────────────────────────┬──────────────┐
  │                   │        With Kata         │ Without Kata │
  ├───────────────────┼──────────────────────────┼──────────────┤
  │ macOS support     │ ❌ Lost                  │ ✅ Works     │
  ├───────────────────┼──────────────────────────┼──────────────┤
  │ Linux desktop     │ ⚠️ Needs KVM/nested virt │ ✅ Works     │
  ├───────────────────┼──────────────────────────┼──────────────┤
  │ Complexity        │ High                     │ Low          │
  ├───────────────────┼──────────────────────────┼──────────────┤
  │ Docker-in-sandbox │ ✅ Safe                  │ ⚠️ Risky     │
  └───────────────────┴──────────────────────────┴──────────────┘
  The real question: Do you need Docker inside the sandbox?

  - If YES → Consider Kata, but accept Linux-only
  - If NO → Skip it entirely

  My take: Your agents don't need Docker. They're coding agents, not CI runners. The
  Lima VM already provides host isolation. Kata is overkill for single-tenant.

  ---
  6. Warm Pool → NICE TO HAVE, low priority

  Why it's less critical for Vibespace:
  - Vibespace = long-lived workspaces (create once, use many times)
  - Netclode = ephemeral sessions (create per prompt)

  Your sessions persist, so cold start only happens once per workspace. Not a big pain
  point.

  ---
  7. Connect RPC / Protobuf → LOW PRIORITY

  Why: Only matters if you're building multi-language clients (mobile app, web UI).

  Your SSH + Unix socket approach works fine for CLI. If you add Redis Streams, you get
  the event model benefits without the protocol migration.

  Revisit if: You decide to build a mobile companion app.

  ---
  Summary: What I'd Actually Do
  ┌───────────────┬───────────────┬──────────┬─────────┐
  │    Feature    │    Adopt?     │ Priority │ Effort  │
  ├───────────────┼───────────────┼──────────┼─────────┤
  │ Tailscale     │ ✅ Yes        │ P0       │ Low     │
  ├───────────────┼───────────────┼──────────┼─────────┤
  │ Redis Streams │ ✅ Yes        │ P1       │ Low-Med │
  ├───────────────┼───────────────┼──────────┼─────────┤
  │ JuiceFS       │ ⚠️ Optional   │ P2       │ Medium  │
  ├───────────────┼───────────────┼──────────┼─────────┤
  │ Snapshots     │ ✅ If JuiceFS │ P2       │ Medium  │
  ├───────────────┼───────────────┼──────────┼─────────┤
  │ Kata microVMs │ ❌ No         │ —        │ —       │
  ├───────────────┼───────────────┼──────────┼─────────┤
  │ Warm pool     │ ⚠️ Later      │ P3       │ Medium  │
  ├───────────────┼───────────────┼──────────┼─────────┤
  │ Connect RPC   │ ❌ Not now    │ —        │ —       │
  └───────────────┴───────────────┴──────────┴─────────┘
  ---
  The Strategic View

  Vibespace's moat: Multi-agent collaboration + cross-platform + developer ergonomics

  Netclode's moat: Isolation + mobile UX + state management

  Don't try to out-Netclode Netclode on isolation. Instead:

  1. Simplify networking (Tailscale) — removes friction
  2. Improve state (Redis) — enables future clients
  3. Optional distributed storage (JuiceFS) — for power users / remote deployments
  4. Double down on multi-agent — that's your unique value

  The goal: Make Vibespace the best place to run multiple AI agents collaborating on
  your code, accessible from anywhere on your tailnet, with optional cloud storage for
  remote deployments.

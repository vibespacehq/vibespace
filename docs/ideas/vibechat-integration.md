# Vibechat Integration

Vibechat is an **optional add-on** that provides a web/mobile GUI for users who prefer graphical interfaces over the terminal. Vibespace CLI remains the core product and first-class citizen.

## Philosophy

```
┌─────────────────────────────────────────────────────────────────────┐
│                           VIBESPACE                                 │
│                                                                     │
│   100% scriptable AI coding development environment management      │
│                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐  │
│   │                          CLI                                 │  │
│   │                                                              │  │
│   │  vibespace create myproject --agent claude-code             │  │
│   │  vibespace list --json | jq '.[] | .name'                   │  │
│   │  vibespace exec myproject "npm test" | tee results.log      │  │
│   │  vibespace multi mysession --agents claude,codex            │  │
│   │  vibespace forward myproject --port 3000                    │  │
│   │                                                              │  │
│   │  • Fully scriptable                                         │  │
│   │  • Pipeable (JSON output)                                   │  │
│   │  • Automatable (CI/CD, cron, scripts)                       │  │
│   │  • Interactive TUI for multi-agent sessions                 │  │
│   │                                                              │  │
│   └─────────────────────────────────────────────────────────────┘  │
│                                                                     │
│                    This IS the product.                             │
│                    Works perfectly standalone.                      │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                │ optional
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      VIBECHAT (Add-on)                              │
│                                                                     │
│   Web/mobile GUI for users who prefer graphical interfaces          │
│                                                                     │
│   • Browser-based agent interaction                                │
│   • Mobile app for on-the-go prompting                             │
│   • Rich message rendering (markdown, diffs, tool calls)           │
│   • Same capabilities, different interface                          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Analogy

| Core Product | Optional GUI |
|--------------|--------------|
| kubectl | Kubernetes Dashboard |
| docker CLI | Portainer |
| git | GitHub Desktop |
| terraform | Terraform Cloud UI |
| **Vibespace CLI** | **Vibechat** |

The CLI is always the first-class citizen. The GUI is a convenience layer.

---

## What Vibechat Provides

### Already Built (in vibechat repo)

| Component | Description |
|-----------|-------------|
| **Web App** | Next.js 15, React 19, Tailwind, shadcn/ui |
| **Backend** | Express, TypeScript, NATS integration |
| **Real-time** | WebSocket + NATS JetStream |
| **Auth** | NextAuth v5 (Auth.js) |
| **Database** | PostgreSQL + Drizzle ORM |
| **Messages** | Storage, history, search |
| **Channels** | Group conversations, direct messages |
| **Mobile** | (Planned/In progress) |

### What It Adds to Vibespace

- **Web UI** for multi-agent sessions (alternative to TUI)
- **Mobile access** for prompting agents on-the-go
- **Rich rendering** of agent responses (markdown, code, diffs)
- **Message persistence** and history search
- **Multi-user** access to shared workspaces

---

## Architecture

### Standalone Vibespace (No Vibechat)

```
┌──────────────────────────────────────────────────────────────┐
│ User's Machine                                               │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Vibespace CLI                                          │ │
│  │                                                         │ │
│  │  vibespace multi mysession                             │ │
│  │  ┌─────────────────────────────────────────────────┐   │ │
│  │  │ TUI (BubbleTea)                                 │   │ │
│  │  │                                                  │   │ │
│  │  │  @claude: explain the auth flow                 │   │ │
│  │  │  @codex: write tests for auth                   │   │ │
│  │  │                                                  │   │ │
│  │  └─────────────────────────────────────────────────┘   │ │
│  └───────────────────────────┬────────────────────────────┘ │
│                              │ SSH                           │
└──────────────────────────────┼───────────────────────────────┘
                               │
┌──────────────────────────────┼───────────────────────────────┐
│ k3s Cluster                  │                               │
│                              ▼                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Claude      │  │ Codex       │  │ Other       │         │
│  │ Agent Pod   │  │ Agent Pod   │  │ Agents...   │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### With Vibechat Add-on

```
┌──────────────────────────────────────────────────────────────┐
│ User Interfaces                                              │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ CLI / TUI    │  │ Web App      │  │ Mobile App   │       │
│  │ (always      │  │ (optional)   │  │ (optional)   │       │
│  │  available)  │  │              │  │              │       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                 │                 │                │
└─────────┼─────────────────┼─────────────────┼────────────────┘
          │                 │                 │
          │ SSH             │ WebSocket       │ WebSocket
          │                 │                 │
┌─────────┼─────────────────┼─────────────────┼────────────────┐
│ k3s Cluster               │                 │                │
│         │                 ▼                 ▼                │
│         │        ┌─────────────────────────────────┐        │
│         │        │ Vibechat Backend                │        │
│         │        │ (optional deployment)           │        │
│         │        └───────────────┬─────────────────┘        │
│         │                        │                           │
│         │                        │ NATS                      │
│         │                        ▼                           │
│         │        ┌─────────────────────────────────┐        │
│         │        │ NATS JetStream                  │        │
│         │        │ (event bus)                     │        │
│         │        └───────────────┬─────────────────┘        │
│         │                        │                           │
│         ▼                        ▼                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Claude      │  │ Codex       │  │ Other       │         │
│  │ Agent Pod   │  │ Agent Pod   │  │ Agents...   │         │
│  │ + NATS      │  │ + NATS      │  │             │         │
│  │   Adapter   │  │   Adapter   │  │             │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## Integration Approach

### Principle: Vibespace Doesn't Change for Vibechat

Vibechat adapts to Vibespace, not the other way around.

### The Integration Point: NATS

NATS serves as the event bus that Vibechat can optionally subscribe to:

```
Vibespace Agent ──publishes──► NATS ──subscribes──► Vibechat
                                 │
                                 └──subscribes──► TUI (optional)
                                 │
                                 └──subscribes──► Other clients
```

**Why NATS:**
- Vibechat already uses NATS (no new tech to learn)
- Pub/sub model fits the use case
- JetStream provides persistence for reconnect/replay
- Hierarchical subjects for flexible routing

---

## What Changes

### Vibespace Changes (Minimal)

| Change | Effort | Description |
|--------|--------|-------------|
| Add NATS to cluster | Low | Deploy NATS alongside existing workloads |
| NATS adapter for agents | Low | Small wrapper that publishes agent I/O to NATS |
| Optional flag | Low | `vibespace init --with-vibechat` |

**Total new code:** ~300 lines of Go

### Vibechat Changes (Minimal)

| Change | Effort | Description |
|--------|--------|-------------|
| "Vibespace" channel type | Low | New channel type that links to a session |
| Agent message rendering | Medium | Components for tool calls, diffs, thinking |
| Session management UI | Low | List/create vibespace sessions |

**Total:** Mostly frontend components

### What Stays the Same

| Component | Status |
|-----------|--------|
| Vibespace CLI | Unchanged |
| Vibespace TUI | Unchanged (can optionally use NATS too) |
| Vibespace scripting | Unchanged |
| Vibespace remote mode | Unchanged |
| Vibechat auth | Unchanged |
| Vibechat message storage | Unchanged |
| Vibechat WebSocket | Unchanged |

---

## NATS Subject Design

```
vibespace.
├── session.{sessionId}.
│   ├── agent.{agentName}.
│   │   ├── input              # User message to agent
│   │   ├── output             # Agent response (streamed)
│   │   └── status             # Agent status (ready, busy, error)
│   │
│   └── events.
│       ├── created            # Session created
│       ├── paused             # Session paused
│       ├── resumed            # Session resumed
│       └── deleted            # Session deleted
```

**Example flow:**

```
1. User types in Vibechat: "explain the auth flow"
2. Vibechat publishes: vibespace.session.abc123.agent.claude.input
3. NATS Adapter in Claude pod receives message
4. Claude processes, streams response
5. Adapter publishes chunks: vibespace.session.abc123.agent.claude.output
6. Vibechat receives, renders in real-time
7. Message stored in Vibechat PostgreSQL for history
```

---

## NATS Adapter

A small Go service that runs alongside the agent process:

```go
// pkg/nats/adapter.go

type Adapter struct {
    nc       *nats.Conn
    js       nats.JetStreamContext
    session  string
    agent    string
    process  *AgentProcess
}

func (a *Adapter) Start(ctx context.Context) error {
    // Subscribe to input from any client (Vibechat, TUI, etc.)
    subject := fmt.Sprintf("vibespace.session.%s.agent.%s.input", a.session, a.agent)

    _, err := a.js.Subscribe(subject, func(msg *nats.Msg) {
        // Forward to agent process
        a.process.SendInput(msg.Data)
    })
    if err != nil {
        return err
    }

    // Stream agent output to NATS
    go a.streamOutput(ctx)

    return nil
}

func (a *Adapter) streamOutput(ctx context.Context) {
    subject := fmt.Sprintf("vibespace.session.%s.agent.%s.output", a.session, a.agent)

    for {
        select {
        case <-ctx.Done():
            return
        case chunk := <-a.process.Output():
            a.js.Publish(subject, chunk)
        }
    }
}
```

---

## Deployment

### Without Vibechat (Default)

```bash
vibespace init
# Deploys: k3s, local-path-provisioner
# No NATS, no Vibechat
```

### With Vibechat (Optional)

```bash
vibespace init --with-vibechat
# Deploys: k3s, local-path-provisioner, NATS, PostgreSQL, Vibechat
```

Or add later:

```bash
vibespace addon install vibechat
# Deploys: NATS, PostgreSQL, Vibechat backend, Vibechat web
# Updates agent pods to include NATS adapter
```

### Kubernetes Resources (when enabled)

```yaml
# Vibechat add-on deploys:
Deployments:
  - vibechat-backend
  - vibechat-web (or static hosting)

StatefulSets:
  - nats
  - postgresql

Services:
  - vibechat-api (ClusterIP)
  - vibechat-ws (ClusterIP)
  - nats (ClusterIP)
  - postgresql (ClusterIP)

Ingress (optional):
  - vibechat.local → vibechat-web
  - api.vibechat.local → vibechat-api
```

---

## User Experience

### CLI User (No Vibechat)

```bash
# Everything works exactly as before
$ vibespace create myproject --agent claude-code
$ vibespace multi mysession
# TUI opens, full functionality
```

### CLI User (With Vibechat Installed)

```bash
# CLI still works exactly the same
$ vibespace create myproject --agent claude-code
$ vibespace multi mysession
# TUI opens, full functionality

# Additionally, can open web UI
$ vibespace web
# Opens browser to Vibechat
# Same session visible in both TUI and web
```

### Web/Mobile User

```
1. Open Vibechat in browser/app
2. Login
3. See list of Vibespace sessions
4. Click to open session
5. Chat with agents (same as TUI, different interface)
6. Rich rendering of responses
```

---

## Implementation Path

### Phase 1: NATS Foundation (Week 1)

**Goal:** Vibespace agents publish to NATS

Tasks:
- [ ] Add NATS deployment to cluster init (optional flag)
- [ ] Create NATS adapter package
- [ ] Update agent pod to include adapter (when NATS enabled)
- [ ] Test: CLI/TUI still works without NATS
- [ ] Test: With NATS, messages appear in NATS

**Deliverable:** `vibespace init --with-nats` works

### Phase 2: Vibechat Channel Type (Week 2)

**Goal:** Vibechat can display Vibespace sessions

Tasks:
- [ ] Add "vibespace" channel type to Vibechat
- [ ] Subscribe to `vibespace.session.{id}.agent.*.output`
- [ ] Basic message rendering (plain text/markdown)
- [ ] Session list from Vibespace API

**Deliverable:** Vibechat shows agent messages

### Phase 3: Rich Rendering (Week 3)

**Goal:** Agent responses look good in Vibechat

Tasks:
- [ ] Tool call component (collapsible, shows input/output)
- [ ] Code block component (syntax highlighting)
- [ ] Diff view component
- [ ] Thinking/reasoning block component
- [ ] File tree component

**Deliverable:** Full-featured agent message rendering

### Phase 4: Session Management (Week 4)

**Goal:** Create/manage Vibespace sessions from Vibechat

Tasks:
- [ ] Create session UI (select agents, configure)
- [ ] Session controls (pause, resume, delete)
- [ ] Agent selection per message (@claude, @codex, @all)
- [ ] Port forwarding UI (optional)

**Deliverable:** Full Vibechat integration

### Phase 5: Mobile (Future)

**Goal:** Mobile app for Vibechat

Tasks:
- [ ] iOS app (Swift/SwiftUI)
- [ ] Android app (Kotlin)
- [ ] Push notifications for agent responses

---

## What We're NOT Doing

| Anti-pattern | Why Avoid |
|--------------|-----------|
| Requiring Vibechat | CLI must always work standalone |
| Changing CLI for Vibechat | Vibechat adapts to Vibespace |
| Shared database | Keep data ownership separate |
| Complex auth integration | Vibechat owns auth, simple JWT validation |
| Breaking scripting | JSON output, pipes, automation must work |

---

## Success Criteria

### Vibespace Standalone

- [ ] All CLI commands work without Vibechat
- [ ] TUI works without Vibechat
- [ ] Scripting/automation works without Vibechat
- [ ] No performance impact when Vibechat not installed

### With Vibechat

- [ ] Web UI shows same information as TUI
- [ ] Real-time updates (no polling)
- [ ] Message history persisted
- [ ] Multiple users can view same session
- [ ] Mobile access works

---

## Configuration

### Vibespace Config (when Vibechat enabled)

```yaml
# ~/.vibespace/config.yaml
addons:
  vibechat:
    enabled: true
    nats:
      url: nats://nats.vibespace.svc:4222
    web:
      url: http://localhost:3000
```

### Vibechat Config (connecting to Vibespace)

```yaml
# vibechat config
vibespace:
  enabled: true
  nats:
    url: nats://nats.vibespace.svc:4222
  api:
    url: http://vibespace-api.vibespace.svc:8080
```

---

## Summary

| Aspect | Approach |
|--------|----------|
| **Relationship** | Vibechat is optional add-on to Vibespace |
| **Core product** | Vibespace CLI (always works standalone) |
| **Integration** | NATS event bus (Vibechat subscribes) |
| **Changes to Vibespace** | Minimal (~300 LOC NATS adapter) |
| **Changes to Vibechat** | Channel type + message rendering |
| **Deployment** | `vibespace addon install vibechat` |
| **User choice** | CLI, TUI, Web, Mobile — all work |

# vibespace - Technical Specification

**Version**: 2.0.0
**Date**: 2026-01-08
**Status**: Multi-Claude Architecture

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Architecture](#2-architecture)
3. [Technical Stack](#3-technical-stack)
4. [Container Image](#4-container-image)
5. [API Specification](#5-api-specification)
6. [NATS Messaging](#6-nats-messaging)
7. [Networking](#7-networking)
8. [Frontend](#8-frontend)
9. [Implementation Phases](#9-implementation-phases)
10. [Non-Functional Requirements](#10-non-functional-requirements)

---

## 1. Project Overview

### 1.1 Vision

A desktop app for managing AI-powered development environments where multiple Claude instances collaborate on the same codebase with real-time chat communication.

### 1.2 Key Concepts

| Concept | Description |
|---------|-------------|
| **Vibespace** | A project environment with shared filesystem and multiple Claude instances |
| **Multi-Claude** | Multiple Claude Code instances per vibespace, communicating via NATS |
| **Dynamic Ports** | Any port Claude starts automatically becomes accessible via subdomain |
| **Chat UI** | Real-time conversation interface (adapted from demiurg) |

### 1.3 Naming Conventions

**Project Structure**:
```
vibespace/
├── app/           # Tauri desktop application
├── api/           # Go API server
│   └── pkg/image/ # Container image (ttyd + Claude Code)
└── docs/          # Documentation
```

**Go Packages**: Singular names (`vibespace/`, `network/`, `credential/`)

**Kubernetes Resources**:
- Namespace: `vibespace`
- Labels: `vibespace.dev/id`, `vibespace.dev/project`
- Service names: `vibespace-{project}-claude-{id}`

**Domains** (Dynamic Port Exposure):
```
{project}.vibe.space              # Main (ttyd terminal)
{port}.{project}.vibe.space       # Any port Claude opens
3000.my-project.vibe.space        # Example: dev server on 3000
5173.my-project.vibe.space        # Example: Vite on 5173
```

**NATS Subjects**:
```
vibespace.{project}.claude.{id}.in      # Messages TO Claude
vibespace.{project}.claude.{id}.out     # Messages FROM Claude
vibespace.{project}.claude.{id}.status  # Status (thinking, idle)
vibespace.{project}.claude.all          # Broadcast to all Claudes
vibespace.{project}.ports.register      # Port opened
vibespace.{project}.ports.unregister    # Port closed
```

---

## 2. Architecture

### 2.1 System Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                      Desktop App (Tauri)                          │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Chat Interface + Vibespace Management                      │  │
│  └─────────────────────────────┬──────────────────────────────┘  │
└────────────────────────────────┼─────────────────────────────────┘
                                 │ WebSocket
┌────────────────────────────────┼─────────────────────────────────┐
│                        Go API Server                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ REST API │  │ WebSocket│  │  NATS    │  │ K8s Client       │  │
│  │ /api/v1  │  │   Hub    │  │ Bridge   │  │ (Knative/Traefik)│  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘  │
└────────────────────────────────┼─────────────────────────────────┘
                                 │ NATS
┌────────────────────────────────┼─────────────────────────────────┐
│                     Kubernetes (k3s)                              │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                          NATS                               │  │
│  │   Pub/Sub messaging for Claude ↔ API ↔ Frontend            │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌──────────────────┐  ┌──────────────────┐                      │
│  │  Vibespace Pod   │  │  Vibespace Pod   │                      │
│  │  project-1       │  │  project-1       │                      │
│  │  claude-1        │  │  claude-2        │                      │
│  │                  │  │                  │                      │
│  │ ttyd + Claude    │  │ ttyd + Claude    │                      │
│  │ Port Detector    │  │ Port Detector    │                      │
│  │ NATS Client      │  │ NATS Client      │                      │
│  └────────┬─────────┘  └────────┬─────────┘                      │
│           └──────────┬──────────┘                                 │
│                      │ Shared PVC (/vibespace)                    │
│                      ▼                                            │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                        Traefik                              │  │
│  │  *.project-1.vibe.space → pods (wildcard routing)          │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Interaction

```
1. User creates vibespace via Tauri UI
2. Tauri → Go API: POST /api/v1/vibespaces
3. Go API → k3s: Create PVC + Knative Service
4. Go API → Traefik: Create IngressRoute
5. Go API → Tauri: Return vibespace with URL
6. User chats via WebSocket → NATS → Claude
7. Claude starts server → Port detector → NATS → API creates IngressRoute
```

### 2.3 Deployment Modes

**Local Mode** (MVP):
- Bundled Kubernetes (Colima on macOS, k3s on Linux)
- One-click installation (~3-5 minutes)
- All components run locally
- See ADR 0006 for details

**Remote Mode** (Post-MVP):
- Desktop app connects to remote API
- Vibespaces run on VPS
- WireGuard tunnel for secure access

---

## 3. Technical Stack

### 3.1 Core Components

| Component | Technology | Version | Purpose |
|-----------|------------|---------|---------|
| Desktop App | Tauri + React | 2.x | Cross-platform UI |
| API Server | Go + Gin | 1.21+ | REST, WebSocket, NATS bridge |
| Container Runtime | k3s | 1.27+ | Lightweight Kubernetes |
| Serverless | Knative Serving | 1.15.2 | Scale-to-zero |
| Ingress | Traefik | 3.5.3 | Wildcard routing |
| Messaging | NATS | 2.10 | Real-time pub/sub |
| Terminal | ttyd | 1.7.x | Web terminal |
| AI Agent | Claude Code | latest | AI coding assistant |
| Registry | registry:2.8.3 | - | Local image storage |
| Builder | BuildKit | 0.17.3 | Image building |

*Version selection rationale: ADR 0004*

### 3.2 Key Dependencies

**Go API**:
```go
github.com/gin-gonic/gin          // Web framework
k8s.io/client-go                  // Kubernetes client
knative.dev/serving/pkg/client    // Knative client
github.com/nats-io/nats.go        // NATS client
github.com/gorilla/websocket      // WebSocket server
github.com/miekg/dns              // DNS server
```

**Frontend**:
```json
{
  "react": "^19.x",
  "@tauri-apps/api": "^2.x",
  "zustand": "^5.x",
  "immer": "^10.x"
}
```

---

## 4. Container Image

### 4.1 Single Image Philosophy

One container image for all vibespaces. Claude can install whatever it needs.

**Benefits**:
- Simpler maintenance 
- Faster iteration
- Smaller storage footprint
- Claude has full flexibility

### 4.2 Dockerfile

```dockerfile
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl wget git vim nano \
    build-essential ca-certificates openssh-client \
    nodejs npm python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install ttyd for web terminal
RUN curl -L https://github.com/tsl0922/ttyd/releases/download/1.7.4/ttyd.x86_64 \
    -o /usr/bin/ttyd && chmod +x /usr/bin/ttyd

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Install port detector daemon (Go binary)
COPY port-detector /usr/bin/port-detector

# Install NATS bridge client
COPY nats-bridge /usr/bin/nats-bridge

# Create non-root user
RUN useradd -m -s /bin/bash -u 1000 coder
USER coder
WORKDIR /vibespace

# Pre-configure Claude Code
RUN mkdir -p ~/.config/claude-code && \
    echo '{"autoApprove": true}' > ~/.config/claude-code/config.json

EXPOSE 7681

# Start supervisor for multi-process
COPY supervisord.conf /etc/supervisor/conf.d/
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
```

### 4.3 Port Detector

Go binary monitoring listening ports:

```go
func main() {
    nc, _ := nats.Connect(os.Getenv("NATS_URL"))
    project := os.Getenv("VIBESPACE_PROJECT")
    knownPorts := make(map[int]bool)

    // System ports to ignore
    ignorePorts := map[int]bool{22: true, 53: true, 7681: true}

    for {
        currentPorts := scanListeningPorts() // Parse /proc/net/tcp

        for port := range currentPorts {
            if !knownPorts[port] && port > 1024 && !ignorePorts[port] {
                nc.Publish(fmt.Sprintf("vibespace.%s.ports.register", project),
                    []byte(fmt.Sprintf(`{"port":%d}`, port)))
                knownPorts[port] = true
            }
        }

        for port := range knownPorts {
            if !currentPorts[port] {
                nc.Publish(fmt.Sprintf("vibespace.%s.ports.unregister", project),
                    []byte(fmt.Sprintf(`{"port":%d}`, port)))
                delete(knownPorts, port)
            }
        }

        time.Sleep(2 * time.Second)
    }
}

func scanListeningPorts() map[int]bool {
    // Parse /proc/net/tcp for LISTEN sockets (state 0A)
    ports := make(map[int]bool)
    data, _ := os.ReadFile("/proc/net/tcp")
    for _, line := range strings.Split(string(data), "\n") {
        // ... parse hex port from local_address column
        // ... check if state == "0A" (LISTEN)
    }
    return ports
}
```

---

## 5. API Specification

### 5.1 Endpoints

**Base URL**: `http://localhost:8090/api/v1`

#### Vibespaces

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /vibespaces | Create vibespace |
| GET | /vibespaces | List vibespaces |
| GET | /vibespaces/:id | Get vibespace |
| DELETE | /vibespaces/:id | Delete vibespace |
| POST | /vibespaces/:id/claudes | Add Claude instance |
| DELETE | /vibespaces/:id/claudes/:cid | Remove Claude |

#### Services (Port Exposure)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /vibespaces/:id/services | Register port |
| DELETE | /vibespaces/:id/services/:port | Unregister port |
| GET | /vibespaces/:id/services | List ports |

#### Cluster

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /cluster/status | Cluster health |
| POST | /cluster/setup | Install components (SSE) |

### 5.2 Request/Response Examples

**Create Vibespace**:
```http
POST /api/v1/vibespaces
Content-Type: application/json

{ "name": "my-project" }
```

```json
{
  "id": "abc123",
  "name": "my-project",
  "project_name": "brave-eagle-7421",
  "status": "creating",
  "claudes": [{ "id": "1", "status": "starting" }],
  "urls": { "main": "https://brave-eagle-7421.vibe.space" },
  "created_at": "2026-01-08T10:30:00Z"
}
```

**Add Claude**:
```http
POST /api/v1/vibespaces/abc123/claudes
```

```json
{
  "id": "2",
  "status": "starting",
  "nats_subject": "vibespace.brave-eagle-7421.claude.2"
}
```

### 5.3 Error Responses

```json
{
  "error": "Resource conflict",
  "details": "Vibespace with name 'my-project' already exists",
  "code": "VIBESPACE_EXISTS"
}
```

Error codes: `TEMPLATE_NOT_FOUND`, `VIBESPACE_EXISTS`, `CLUSTER_DOWN`, `INVALID_REQUEST`

---

## 6. NATS Messaging

### 6.1 Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nats
  namespace: default
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: nats
        image: nats:2.10-alpine
        ports:
        - containerPort: 4222
---
apiVersion: v1
kind: Service
metadata:
  name: nats
spec:
  ports:
  - port: 4222
  selector:
    app: nats
```

### 6.2 Subject Schema

```
vibespace.
├── {project}.
│   ├── claude.
│   │   ├── {id}.
│   │   │   ├── in      # Messages TO Claude
│   │   │   ├── out     # Messages FROM Claude (streaming)
│   │   │   └── status  # Status updates
│   │   └── all         # Broadcast to all Claudes
│   └── ports.
│       ├── register    # Port opened
│       └── unregister  # Port closed
```

### 6.3 Message Types

**Chat Message (to Claude)**:
```json
{
  "type": "message",
  "content": "Create a React component for user login",
  "timestamp": "2026-01-08T10:30:00Z"
}
```

**Claude Response (streaming)**:
```json
{
  "type": "response",
  "content": "I'll create a login component...",
  "streaming": true,
  "timestamp": "2026-01-08T10:30:01Z"
}
```

**Status Update**:
```json
{
  "type": "status",
  "status": "thinking" | "idle" | "error",
  "timestamp": "2026-01-08T10:30:00Z"
}
```

**Port Registration**:
```json
{
  "port": 3000,
  "protocol": "http"
}
```

**Claude-to-Claude Handoff**:
```json
{
  "type": "handoff",
  "from_claude": "1",
  "message": "Backend API complete. Please write tests.",
  "context": {
    "files_changed": ["api/handlers.go"]
  }
}
```

---

## 7. Networking

### 7.1 DNS Resolution

Custom DNS server (miekg/dns) resolving `*.vibe.space` to 127.0.0.1:

```go
func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
    domain := r.Question[0].Name

    if strings.HasSuffix(domain, ".vibe.space.") {
        rr := &dns.A{
            Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Ttl: 0},
            A:   net.ParseIP("127.0.0.1"),
        }
        m := new(dns.Msg)
        m.SetReply(r)
        m.Answer = append(m.Answer, rr)
        w.WriteMsg(m)
    }
}
```

**Platform Setup**:
- **macOS**: `/etc/resolver/vibe.space`
- **Linux**: systemd-resolved configuration

### 7.2 Traefik IngressRoutes

**Main Access (ttyd)**:
```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-{project}-main
  namespace: vibespace
spec:
  entryPoints: [web]
  routes:
  - match: Host(`{project}.vibe.space`)
    kind: Rule
    services:
    - name: vibespace-{project}-claude-1
      port: 7681
```

**Dynamic Port** (created on-demand):
```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-{project}-port-{port}
  namespace: vibespace
spec:
  entryPoints: [web]
  routes:
  - match: Host(`{port}.{project}.vibe.space`)
    kind: Rule
    services:
    - name: vibespace-{project}-claude-1
      port: {port}
```

### 7.3 Knative Service

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: vibespace-{project}-claude-{id}
  namespace: vibespace
  labels:
    vibespace.dev/project: {project}
    vibespace.dev/claude: {id}
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/min-scale: "0"
        autoscaling.knative.dev/max-scale: "1"
        autoscaling.knative.dev/scale-down-delay: "10m"
    spec:
      containerConcurrency: 1
      timeoutSeconds: 600
      containers:
      - name: vibespace
        image: ghcr.io/yagizdagabak/vibespace/vibespace:latest
        ports:
        - containerPort: 7681
          name: http1
        env:
        - name: NATS_URL
          value: "nats://nats.default.svc.cluster.local:4222"
        - name: VIBESPACE_PROJECT
          value: "{project}"
        - name: VIBESPACE_CLAUDE_ID
          value: "{id}"
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: vibespace-{project}-secrets
              key: anthropic-api-key
        volumeMounts:
        - name: data
          mountPath: /vibespace
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: vibespace-{project}-pvc
```

---

## 8. Frontend

### 8.1 Component Structure

```
app/src/
├── components/
│   ├── chat/              # Chat UI (from demiurg)
│   │   ├── ChatWindow.tsx
│   │   ├── MessageList.tsx
│   │   ├── MessageInput.tsx
│   │   └── ClaudeStatus.tsx
│   ├── vibespace/         # Vibespace management
│   │   ├── VibespaceList.tsx
│   │   ├── VibespaceCard.tsx
│   │   └── CreateModal.tsx
│   └── shared/
│       └── TitleBar.tsx
├── stores/
│   ├── useChatStore.ts
│   └── useVibespaceStore.ts
├── hooks/
│   ├── useWebSocket.ts
│   └── useVibespaces.ts
└── lib/
    ├── api.ts
    └── types.ts
```

### 8.2 State Management

**useChatStore** (Zustand + Immer):
```typescript
interface ChatStore {
  messages: Message[];
  activeClaudeId: string | null;
  isStreaming: boolean;

  sendMessage: (content: string) => void;
  setActiveClaudeId: (id: string) => void;
  addMessage: (message: Message) => void;
  updateStreamingMessage: (content: string) => void;
}
```

**useVibespaceStore**:
```typescript
interface VibespaceStore {
  vibespaces: Vibespace[];
  activeVibespace: Vibespace | null;

  createVibespace: (name: string) => Promise<Vibespace>;
  addClaude: (vibespaceId: string) => Promise<Claude>;
  removeClaude: (vibespaceId: string, claudeId: string) => Promise<void>;
}
```

### 8.3 WebSocket Hook

```typescript
export function useWebSocket(vibespaceId: string) {
  const ws = useRef<WebSocket | null>(null);
  const chatStore = useChatStore();

  useEffect(() => {
    ws.current = new WebSocket(`ws://localhost:8090/ws/${vibespaceId}`);

    ws.current.onmessage = (event) => {
      const data = JSON.parse(event.data);

      switch (data.type) {
        case 'response':
          if (data.streaming) {
            chatStore.updateStreamingMessage(data.content);
          } else {
            chatStore.addMessage(data);
          }
          break;
        case 'status':
          chatStore.setClaudeStatus(data.claudeId, data.status);
          break;
        case 'port_register':
          // Update available ports in UI
          break;
      }
    };

    return () => ws.current?.close();
  }, [vibespaceId]);

  const sendMessage = (content: string, claudeId: string) => {
    ws.current?.send(JSON.stringify({ type: 'message', content, claudeId }));
  };

  return { sendMessage };
}
```

### 8.4 Design System

**Colors**:
```css
--bg-primary:       #000000;
--bg-secondary:     #0a0a0a;
--bg-elevated:      #0f0f0f;
--text-primary:     #ffffff;
--text-secondary:   #a0a0a0;
--accent-teal:      #00ABAB;
--accent-orange:    #FF7D4B;
--accent-pink:      #F102F3;
--accent-yellow:    #F5F50A;
--success:          #3fb950;
--error:            #f85149;
```

**Typography**:
- UI: Space Grotesk
- Code: JetBrains Mono

**Icons**: Lucide React

---

## 9. Implementation Phases

### Phase 1: POC

**Goal**: Create vibespace, access via browser

| Task | Status |
|------|--------|
| Single Docker image (ttyd + Claude Code) | Pending |
| GitHub Actions → GHCR | Pending |
| Go API creates Knative Service | Pending |
| Traefik routes `{project}.vibe.space` | Pending |

**Success Criteria**:
```bash
curl -X POST localhost:8090/api/v1/vibespaces -d '{"name":"test"}'
open https://brave-eagle-7421.vibe.space
# See: ttyd terminal with Claude Code available
```

### Phase 2: NATS + Dynamic Ports

**Goal**: Ports Claude opens become accessible automatically

| Task | Status |
|------|--------|
| NATS deployment in cluster | Pending |
| Port detector daemon | Pending |
| API subscribes to port events | Pending |
| Auto-create/delete IngressRoutes | Pending |

**Success Criteria**:
```bash
# In ttyd terminal:
npm create vite@latest myapp && cd myapp && npm run dev
# Accessible at https://5173.{project}.vibe.space
```

### Phase 3: Chat Interface

**Goal**: Chat with Claude through desktop app

| Task | Status |
|------|--------|
| WebSocket server in Go API | Pending |
| NATS bridge (WebSocket ↔ NATS) | Pending |
| NATS client in container | Pending |
| Port chat UI from demiurg | Pending |

**Success Criteria**:
```
1. Type: "Create a hello world Express server"
2. See Claude's streaming response
3. Claude creates files, starts server
4. Port auto-registers
```

### Phase 4: Multi-Claude

**Goal**: Multiple Claude instances per vibespace

| Task | Status |
|------|--------|
| Multiple Knative Services per vibespace | Pending |
| Shared PVC (ReadWriteMany) | Pending |
| Claude management API | Pending |
| Claude-to-Claude messaging | Pending |
| UI for managing Claudes | Pending |

**Success Criteria**:
```
1. Create vibespace with one Claude
2. Click "Add Claude" → second Claude spawns
3. Message Claude #1: "Build the backend API"
4. Message Claude #2: "Write tests for the API"
5. Both work simultaneously on shared codebase
```

---

## 10. Non-Functional Requirements

### 10.1 Performance

| Metric | Target |
|--------|--------|
| Vibespace start (cold) | < 30s |
| Scale-to-zero | < 10s idle |
| Message latency (NATS) | < 100ms |
| Port detection | < 3s |
| API response (p95) | < 200ms |

### 10.2 Resource Usage

| Component | RAM |
|-----------|-----|
| k3s idle | < 512MB |
| NATS | ~50MB |
| Vibespace (per Claude) | ~500MB |
| API server | ~100MB |
| Minimum total | 8GB RAM, 50GB disk |

### 10.3 Security

- Containers run as non-root (UID 1000)
- API keys via Kubernetes Secrets
- Network isolation between vibespaces (NetworkPolicy)
- Local-first (no cloud dependency by default)
- Credentials encrypted at rest (AES-256)

### 10.4 Usability

| Metric | Target |
|--------|--------|
| First-time setup | < 5 minutes |
| Create vibespace | < 3 clicks |
| Learning curve | No k8s knowledge required |

---

## References

- [demiurg-backend](../demiurg-backend/): NATS patterns
- [demiurg-frontend-next](../demiurg-frontend-next/): Chat UI
- [ADR 0006](./adr/0006-bundled-kubernetes-runtime.md): Bundled Kubernetes
- [ADR 0013](./adr/0013-multi-claude-architecture.md): Multi-Claude decision
- [ADR 0014](./adr/0014-nats-messaging.md): NATS choice
- [ADR 0015](./adr/0015-dynamic-port-exposure.md): Port exposure

**External**:
- [Knative Serving](https://knative.dev/docs/serving/)
- [NATS](https://nats.io/)
- [ttyd](https://github.com/tsl0922/ttyd)
- [Claude Code](https://docs.anthropic.com/claude-code)

---

**End of Specification**

*Version 2.0.0 - 2026-01-08*

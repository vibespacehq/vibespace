# vibespace - AI Assistant Context

**Project**: vibespace - multi-Claude development environments
**Stack**: Tauri + React + Go + k3s + Knative + NATS

---

## What This Project Does

vibespace is a desktop app for managing AI-powered development environments. Each vibespace runs Claude Code instances that collaborate on the same codebase, with a real-time chat interface for communication.

```
User creates vibespace "my-project"
  → Container starts (ttyd + Claude Code + port detector)
  → User chats with Claude through desktop app
  → Claude writes code, starts dev servers
  → Dev servers auto-exposed at https://3000.my-project.vibe.space
  → User can spawn additional Claudes for parallel work
```

**Key Features**:
- Multi-Claude orchestration (multiple AI agents per project)
- Real-time chat interface (adapted from demiurg codebase)
- Dynamic port exposure (any port Claude starts becomes accessible)
- NATS-based messaging (Claude ↔ API ↔ Frontend)
- Scale-to-zero with Knative

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Tauri Desktop App                            │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Chat Interface (from demiurg) + Vibespace Management     │  │
│  └─────────────────────────────────┬─────────────────────────┘  │
│                                    │ WebSocket                   │
└────────────────────────────────────┼────────────────────────────┘
                                     │
┌────────────────────────────────────┼────────────────────────────┐
│                        Go API Server                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │ REST API │  │ WebSocket│  │  NATS    │  │ K8s Client       │ │
│  │ /api/v1  │  │   Hub    │  │ Subscriber│ │ (Knative, Traefik)│ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘ │
└────────────────────────────────────┼────────────────────────────┘
                                     │
┌────────────────────────────────────┼────────────────────────────┐
│                    Local Kubernetes (k3s)                        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                         NATS                              │   │
│  │   vibespace.{project}.claude.{id}.in/out  (messages)     │   │
│  │   vibespace.{project}.ports.register      (port events)  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  Vibespace   │  │  Vibespace   │  │  Vibespace   │          │
│  │  project-1   │  │  project-1   │  │  project-2   │          │
│  │  claude-1    │  │  claude-2    │  │  claude-1    │          │
│  │              │  │              │  │              │          │
│  │ ttyd + Claude│  │ ttyd + Claude│  │ ttyd + Claude│          │
│  │ Port Detector│  │ Port Detector│  │ Port Detector│          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│         │                 │                                     │
│         └────────┬────────┘                                     │
│                  │ Shared PVC (/vibespace)                      │
│                  ▼                                               │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                       Traefik                             │   │
│  │  *.project-1.vibe.space → vibespace pods                 │   │
│  │  3000.project-1.vibe.space → port 3000                   │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Repository Structure

```
vibespace/
├── app/                    # Tauri desktop application
│   ├── src-tauri/         # Rust backend
│   └── src/               # React frontend
│       ├── features/      # Feature modules
│       │   ├── chat/      # Chat UI (from demiurg)
│       │   └── vibespace/ # Vibespace management
│       └── lib/           # Utilities
├── api/                   # Go API server
│   ├── cmd/server/        # Entry point
│   └── pkg/
│       ├── handler/       # HTTP handlers
│       ├── vibespace/     # Vibespace management
│       ├── nats/          # NATS client
│       ├── websocket/     # WebSocket hub
│       ├── network/       # IngressRoute management
│       ├── k8s/           # Kubernetes client
│       └── image/         # Container image (ttyd + Claude Code)
├── demiurg-backend/       # Reference: NATS patterns (to be refactored)
├── demiurg-frontend-next/ # Reference: Chat UI (to be ported)
└── docs/                  # Documentation
```

---

## Key Concepts

### Vibespace
A project environment running as Knative Service(s):
- **ttyd**: Web terminal for direct access
- **Claude Code CLI**: AI coding agent
- **Port detector**: Monitors for new listening ports
- **Persistent volume**: `/vibespace` directory shared across Claudes

### Multi-Claude
Multiple Claude instances can work on the same project:
- Each Claude runs in its own Kubernetes pod
- All pods mount the same PVC (shared filesystem)
- Communication happens via NATS message subjects
- User can message specific Claudes or broadcast to all

### Dynamic Port Exposure
When Claude starts a dev server (e.g., `npm run dev` on port 3000):
1. Port detector daemon notices new listening port via `/proc/net/tcp`
2. Publishes event to NATS: `vibespace.{project}.ports.register`
3. Go API receives event and creates Traefik IngressRoute
4. Server becomes accessible at `https://3000.{project}.vibe.space`

### NATS Subject Schema
```
vibespace.{project}.claude.{id}.in     # Messages TO a specific Claude
vibespace.{project}.claude.{id}.out    # Messages FROM a specific Claude
vibespace.{project}.claude.{id}.status # Claude status (thinking, idle, error)
vibespace.{project}.claude.all         # Broadcast to all Claudes in project
vibespace.{project}.ports.register     # Port opened event
vibespace.{project}.ports.unregister   # Port closed event
```

---

## External Dependencies

| Component | Purpose |
|-----------|---------|
| **k3s** | Lightweight Kubernetes |
| **Knative Serving** | Scale-to-zero, serverless workloads |
| **Traefik** | Ingress controller, wildcard routing |
| **NATS** | Real-time pub/sub messaging |
| **ttyd** | Share terminal over web |
| **Claude Code** | AI coding agent CLI |

---

## Development

### Prerequisites
- Node.js 20+
- Go 1.21+
- Rust 1.70+
- Docker
- kubectl

### Bundled Kubernetes Access

Vibespace bundles its own Colima/Lima/kubectl in `~/.vibespace/`. Use these wrapper scripts (in `~/.local/bin/`) to access the cluster:

- `vibectl` - kubectl with vibespace PATH
- `vibecolima` - colima with vibespace PATH

Examples:
```bash
vibectl get pods -A              # List all pods
vibectl logs -n vibespace <pod>  # View pod logs
vibecolima status                # Check VM status
```

### Running Locally

```bash
# Desktop app
cd app && npm install && npm run dev

# API server
cd api && go run cmd/server/main.go

# Build container image
cd api/pkg/image && docker build -t vibespace:latest .
```

### Testing

```bash
# Go tests
cd api && go test ./...

# Frontend tests
cd app && npm run test:frontend

# Dead code check
make deadcode
```

---

## Naming Conventions

### Code Style
- **Go**: Standard conventions, singular package names (`vibespace`, `nats`)
- **TypeScript**: camelCase variables, PascalCase components
- **Files**: kebab-case for non-components, PascalCase for React components

### Kubernetes Resources
- **Namespace**: `vibespace`
- **Labels**: `vibespace.dev/id`, `vibespace.dev/project`
- **Service names**: `vibespace-{project}-claude-{id}`
- **PVC names**: `vibespace-{project}-pvc`

### Domain Names
- **Main access**: `{project}.vibe.space`
- **Port-specific**: `{port}.{project}.vibe.space`
- **Example**: `3000.brave-fox-42.vibe.space`

### API Endpoints
```
GET    /api/v1/vibespaces
POST   /api/v1/vibespaces
GET    /api/v1/vibespaces/:id
DELETE /api/v1/vibespaces/:id
POST   /api/v1/vibespaces/:id/claudes
DELETE /api/v1/vibespaces/:id/claudes/:claudeId
POST   /api/v1/vibespaces/:id/services  (port registration)
```

---

## Design System

**Philosophy**: Terminal-inspired, dark theme, vibrant accents

**Colors**:
- Background: Pure black (#000000)
- Primary: Teal (#00ABAB)
- Active: Pink (#F102F3)
- Accent: Orange (#FF7D4B)
- Highlight: Yellow (#F5F50A)

**Typography**:
- UI: Space Grotesk
- Code: JetBrains Mono

**Icons**: Lucide React

---

## Git Workflow

### Branch Naming
```
feature/#<issue>-<short-description>
fix/#<issue>-<short-description>
docs/#<issue>-<short-description>
refactor/#<issue>-<short-description>
```

### Commit Message Format
```
<type>(#<issue>): <description>

[optional body]
```

**Types**: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

### Pull Request Process
1. Create GitHub issue for the work
2. Create branch referencing issue number
3. Work in small, logical commits
4. Push and create PR with description
5. Address review feedback
6. Squash merge when approved

### Example Workflow
```bash
# Create issue
gh issue create --title "Add NATS subscriber for port events" --label "feature"
# Issue #42 created

# Create branch
git checkout -b feature/#42-nats-port-subscriber

# Work in commits
git commit -m "feat(#42): add NATS connection manager"
git commit -m "feat(#42): implement port event subscriber"
git commit -m "test(#42): add subscriber unit tests"

# Push and create PR
git push origin feature/#42-nats-port-subscriber
gh pr create --title "feat: Add NATS subscriber for port events (#42)" --body "Closes #42"

# After approval
gh pr merge 42 --squash --delete-branch
```

---

## Testing Strategy

### Frontend Testing (Vitest + React Testing Library)

```bash
npm run test:frontend          # Run all tests
npm run test:frontend:watch    # Watch mode
npm run test:frontend:coverage # Coverage report
```

**Test file location**: Co-located with component (`Component.test.tsx`)

**What to test**:
- Component rendering with different props
- User interactions (clicks, typing)
- State changes and async operations
- Error handling and edge cases

### Backend Testing (Go)

```bash
go test ./...                  # Run all tests
go test -v ./pkg/nats          # Specific package
go test -cover ./...           # With coverage
```

**What to test**:
- API handler behavior
- NATS message processing
- Kubernetes resource creation
- Service business logic

### Dead Code Detection

```bash
make deadcode
```

Run before every commit. Removes unused exports, functions, and dependencies.

---

## Reference Codebases

### demiurg-backend (TypeScript/Node.js)
Located at `demiurg-backend/` - patterns to adapt:
- NATS integration: `src/core/nats/nats.service.ts`
- WebSocket handlers: `src/core/websocket/`
- Claude streaming: `src/features/claude-code/`
- Message flow patterns

### demiurg-frontend-next (Next.js/React)
Located at `demiurg-frontend-next/` - components to port:
- Chat UI: `src/features/chat/`
- Zustand stores: `src/features/chat/store/`
- WebSocket manager: `src/lib/websocket/`
- Message rendering: `src/features/chat/components/`

**Porting notes**:
- Remove Clerk auth (not needed for local app)
- Replace `next/navigation` with react-router or Tauri navigation
- Connect WebSocket to Go backend instead of Node
- Zustand/Immer work unchanged in Tauri

---

## Security Considerations

1. **Local-first**: All components run on user's machine
2. **No cloud auth**: No external authentication service
3. **API keys**: User provides Claude API key, stored locally
4. **Network isolation**: Vibespaces isolated via Kubernetes NetworkPolicy
5. **Non-root containers**: Vibespace containers run as UID 1000

---

## Troubleshooting

### Vibespace won't start
```bash
kubectl get pods -n vibespace
kubectl describe pod <pod-name> -n vibespace
kubectl logs <pod-name> -n vibespace
```

### Port not accessible
```bash
# Check IngressRoute exists
kubectl get ingressroute -n vibespace

# Check Traefik logs
kubectl logs -n traefik deployment/traefik
```

### NATS connection issues
```bash
# Check NATS pod
kubectl get pods -n default -l app=nats
kubectl logs deployment/nats
```

---

## Important Files

- `docs/SPEC.md` - Complete technical specification
- `docs/ROADMAP.md` - Product roadmap (4 phases)
- `docs/adr/README.md` - Architecture Decision Records index
- `docs/adr/0006-bundled-kubernetes-runtime.md` - Bundled k8s approach
- `docs/adr/0013-multi-claude-architecture.md` - Multi-Claude decision
- `docs/adr/0014-nats-messaging.md` - NATS choice rationale
- `docs/adr/0015-dynamic-port-exposure.md` - Port exposure design

---

## Current Phase

**Phase 1: POC** - Prove the infrastructure works

Goal: Create a vibespace and access it via browser with ttyd + Claude Code.

See `docs/ROADMAP.md` for full phase breakdown and `docs/SPEC.md` Section 9 for implementation details.

---

**Last Updated**: 2026-01-08

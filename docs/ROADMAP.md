# vibespace - Product Roadmap

## Vision

The easiest way to work with multiple AI coding agents on the same project.

---

## Phase 1: POC - Prove the Infrastructure

**Goal**: Create a vibespace and access it via browser

### Deliverables
- [ ] Single Docker image (Alpine + ttyd + Claude Code CLI)
- [ ] GitHub Actions workflow to build and push to GHCR
- [ ] Go API creates Knative Service
- [ ] Traefik routes `{project}.vibe.space` to service
- [ ] User can access ttyd terminal in browser

### Success Criteria
```
1. Run: curl -X POST localhost:8080/api/v1/vibespaces -d '{"name":"test"}'
2. Wait for pod to start
3. Open: https://test-abc123.vibe.space
4. See: ttyd terminal with Claude Code available
```

### Out of Scope
- Chat UI
- NATS messaging
- Multi-Claude
- Port auto-detection

---

## Phase 2: NATS + Dynamic Ports

**Goal**: Ports that Claude opens become automatically accessible

### Deliverables
- [ ] NATS deployment in cluster
- [ ] Port detector daemon in container image
- [ ] Go API subscribes to port events via NATS
- [ ] Auto-create Traefik IngressRoute when port registered
- [ ] Auto-delete IngressRoute when port unregistered

### Success Criteria
```
1. Create vibespace
2. In ttyd: npm create vite@latest myapp && cd myapp && npm run dev
3. Vite starts on port 5173
4. Automatically accessible at https://5173.{project}.vibe.space
```

### Out of Scope
- Chat UI
- Multi-Claude

---

## Phase 3: Chat Interface

**Goal**: Talk to Claude through the desktop app instead of ttyd

### Deliverables
- [ ] WebSocket server in Go API
- [ ] NATS bridge (WebSocket ↔ NATS subjects)
- [ ] Claude NATS client in container (receives/sends messages)
- [ ] Port chat UI components from demiurg-frontend
- [ ] Zustand stores for chat state
- [ ] Message streaming display

### Success Criteria
```
1. Create vibespace
2. Open desktop app
3. Type: "Create a hello world Express server"
4. See Claude's streaming response in chat
5. Claude creates files, starts server
6. Port auto-registers, accessible via subdomain
```

### Out of Scope
- Multi-Claude (only one Claude per vibespace)

---

## Phase 4: Multi-Claude

**Goal**: Multiple AI agents working on the same project

### Deliverables
- [ ] Multiple Knative Services per vibespace (shared PVC)
- [ ] Claude management API (spawn, stop)
- [ ] NATS subjects for Claude-to-Claude communication
- [ ] UI for managing multiple Claudes
- [ ] Ability to direct messages to specific Claude or broadcast

### Success Criteria
```
1. Create vibespace with one Claude
2. Click "Add Claude" → second Claude spawns
3. Message Claude #1: "Build the backend API"
4. Message Claude #2: "Write tests for the API"
5. Both Claudes work simultaneously, see each other's file changes
6. Claude #1 can hand off to Claude #2 via NATS
```

---

## Future Considerations

### Not Planned for MVP

**Cloud Deployment**
- Remote Kubernetes clusters
- VPS deployment mode
- Cloud provider integrations

**Team Features**
- Multi-user collaboration
- Shared vibespaces
- Authentication/authorization

**Advanced Features**
- Custom templates
- Snapshot/restore
- Git integration
- CI/CD pipelines

### Technical Debt to Address
- Migrate existing codebase to new architecture
- Remove old template system
- Clean up unused k8s manifests
- Update all tests

---

## Architecture Decisions

### Why Single Image?
- Simpler to maintain than multiple templates
- Claude can install whatever it needs
- Faster iteration on changes
- Smaller GHCR storage

### Why NATS?
- Already proven in demiurg
- Lightweight, fast, easy to deploy
- Perfect for pub/sub messaging
- Good Go and container support

### Why ttyd?
- Lightweight (~3MB binary)
- Pure web terminal
- Fallback access even without chat UI
- Can be replaced later

### Why Knative?
- Scale-to-zero saves resources
- Managed service lifecycle
- Built-in revision management
- Industry standard

---

## Timeline

No fixed dates. Phases complete when deliverables are done and success criteria are met.

**Current Focus**: Phase 1 (POC)

---

**Last Updated**: 2026-01-08

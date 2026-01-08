# ADR 0013: Multi-Claude Architecture

**Status**: Accepted
**Date**: 2026-01-08

## Context

vibespace originally aimed to provide template-based development environments (Next.js, Vue, Python, etc.) with various AI agents (Claude Code, OpenAI Codex, Gemini). This approach had several issues:

1. **Template proliferation**: 12+ images to maintain (3 stacks × 4 agents)
2. **Agent fragmentation**: Supporting multiple AI providers added complexity
3. **Limited collaboration**: Single agent per vibespace
4. **Static ports**: Required predefined port mappings

Meanwhile, the demiurg project demonstrated a working real-time chat interface with NATS messaging that could be adapted.

## Decision

Pivot to a **Multi-Claude Architecture**:

1. **Single container image**: One image with ttyd + Claude Code CLI (no templates)
2. **Claude-only**: Remove support for other AI agents (Codex, Gemini)
3. **Multi-Claude per vibespace**: Multiple Claude instances sharing same filesystem
4. **Dynamic port exposure**: Auto-register ports as Claude starts servers
5. **NATS messaging**: Real-time communication between all components
6. **Chat UI**: Port from demiurg for desktop app integration

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      Desktop App (Tauri)                          │
│                         Chat Interface                            │
└───────────────────────────────┬──────────────────────────────────┘
                                │ WebSocket
┌───────────────────────────────┼──────────────────────────────────┐
│                       Go API Server                               │
│              REST + WebSocket + NATS Subscriber                   │
└───────────────────────────────┼──────────────────────────────────┘
                                │ NATS
┌───────────────────────────────┼──────────────────────────────────┐
│                    Kubernetes (k3s)                               │
│                                                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   Claude 1  │  │   Claude 2  │  │   Claude 3  │              │
│  │   (Pod)     │  │   (Pod)     │  │   (Pod)     │              │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │
│         └────────────────┼────────────────┘                      │
│                          │                                        │
│                   Shared PVC (/vibespace)                        │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Traefik: *.{project}.vibe.space → Pods                  │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

## Consequences

### Positive

- **Simpler maintenance**: One image instead of 12+
- **Better user experience**: Chat UI instead of terminal-only
- **True collaboration**: Multiple Claudes can work together
- **Dynamic**: Ports auto-register, no static configuration
- **Proven patterns**: NATS/chat from demiurg already works

### Negative

- **Claude lock-in**: No support for other AI providers
- **Migration work**: Significant codebase changes required
- **Complexity**: NATS adds moving parts
- **Learning curve**: New architecture to understand

### Neutral

- **ttyd as fallback**: Still have terminal access if chat fails
- **Existing infrastructure**: Knative, Traefik remain unchanged

## Implementation

### Phase 1: POC
- Single container image
- Basic Knative service
- Traefik routing
- No NATS, no chat

### Phase 2: NATS + Ports
- NATS deployment
- Port detector daemon
- Dynamic IngressRoutes

### Phase 3: Chat UI
- WebSocket server
- Port chat from demiurg
- NATS bridge

### Phase 4: Multi-Claude
- Multiple pods per vibespace
- Shared PVC
- Claude-to-Claude messaging

## Alternatives Considered

### Keep Template Approach
- Pro: Already built
- Con: Too many images, complex to maintain

### Use Different Messaging (Redis Pub/Sub, WebSocket-only)
- Pro: Simpler deployment
- Con: NATS already proven in demiurg, better patterns

### Support Multiple AI Providers
- Pro: User choice
- Con: Massive complexity, different APIs/behaviors

## References

- demiurg-backend: NATS patterns at `demiurg-backend/src/core/nats/`
- demiurg-frontend-next: Chat UI at `demiurg-frontend-next/src/features/chat/`

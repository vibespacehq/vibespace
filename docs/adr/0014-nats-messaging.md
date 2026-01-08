# ADR 0014: NATS for Real-Time Messaging

**Status**: Accepted
**Date**: 2026-01-08

## Context

vibespace needs real-time communication between:
1. **User ↔ Claude**: Chat messages and responses
2. **Claude ↔ API**: Status updates, port registration
3. **Claude ↔ Claude**: Handoffs and coordination (multi-Claude)

Options considered:
- Direct WebSocket (no message broker)
- Redis Pub/Sub
- NATS
- RabbitMQ/Kafka (heavyweight)

## Decision

Use **NATS** as the central message broker.

## Rationale

1. **Proven in demiurg**: Already have working implementation
2. **Lightweight**: Single binary, minimal resources
3. **Fast**: Sub-millisecond latency for pub/sub
4. **Simple**: No complex configuration needed
5. **Good Go support**: Official nats.go client
6. **Container-friendly**: Easy to run in Kubernetes

## Subject Schema

```
vibespace.{project}.claude.{id}.in      # Messages TO specific Claude
vibespace.{project}.claude.{id}.out     # Messages FROM specific Claude
vibespace.{project}.claude.{id}.status  # Status updates (thinking, idle)
vibespace.{project}.claude.all          # Broadcast to all Claudes
vibespace.{project}.ports.register      # Port opened by Claude
vibespace.{project}.ports.unregister    # Port closed
```

## Message Flow

### User Chat Message
```
User types message
  → Frontend WebSocket → Go API
  → API publishes: vibespace.myproject.claude.1.in
  → Container NATS client receives
  → Passes to Claude Code CLI
  → Claude streams response
  → Container publishes chunks: vibespace.myproject.claude.1.out
  → API receives, forwards via WebSocket
  → Frontend renders streaming response
```

### Port Registration
```
Claude runs: npm run dev (port 3000)
  → Port detector sees new LISTEN socket
  → Publishes: vibespace.myproject.ports.register { port: 3000 }
  → API receives, creates IngressRoute
  → https://3000.myproject.vibe.space now works
```

### Claude-to-Claude Handoff
```
Claude 1 finishes API
  → Publishes: vibespace.myproject.claude.2.in
    { type: "handoff", message: "API done, please write tests" }
  → Claude 2 receives, starts working on tests
```

## Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nats
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: nats
          image: nats:2.10-alpine
          ports:
            - containerPort: 4222
```

## Consequences

### Positive
- Decoupled components
- Scalable message routing
- Easy to add new subscribers
- Reliable delivery

### Negative
- Additional service to run
- Network hop for every message
- Need to handle disconnections

## Alternatives Rejected

### Direct WebSocket
- Con: Can't fan-out to multiple consumers
- Con: Tight coupling between components

### Redis Pub/Sub
- Con: No persistent connections
- Con: Less suited for high-frequency messaging

### Kafka/RabbitMQ
- Con: Overkill for this use case
- Con: Complex deployment

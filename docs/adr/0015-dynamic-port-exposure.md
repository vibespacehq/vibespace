# ADR 0015: Dynamic Port Exposure

**Status**: Accepted
**Date**: 2026-01-08

## Context

When Claude starts a development server (e.g., `npm run dev` on port 3000), users need to access it via browser.

Previous approach: Predefined ports (code-server:8080, preview:3000, prod:3001) with static IngressRoutes.

Problem: Limits flexibility. What if Claude starts server on port 5173? Or multiple servers?

## Decision

Implement **dynamic port exposure**:

1. **Port detector daemon** in container monitors `/proc/net/tcp`
2. On new LISTEN port → publish to NATS
3. Go API creates Traefik IngressRoute on-the-fly
4. URL pattern: `https://{port}.{project}.vibe.space`

## Implementation

### Port Detector (Go binary in container)

```go
func main() {
    nc, _ := nats.Connect(os.Getenv("NATS_URL"))
    project := os.Getenv("VIBESPACE_PROJECT")

    knownPorts := make(map[int]bool)

    for {
        currentPorts := scanListeningPorts()

        // New ports
        for port := range currentPorts {
            if !knownPorts[port] {
                nc.Publish(fmt.Sprintf("vibespace.%s.ports.register", project),
                    []byte(fmt.Sprintf(`{"port":%d}`, port)))
                knownPorts[port] = true
            }
        }

        // Closed ports
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
    // Return map of port numbers
}
```

### Go API Subscriber

```go
nc.Subscribe("vibespace.*.ports.register", func(msg *nats.Msg) {
    // Parse project from subject: vibespace.{project}.ports.register
    // Parse port from message body
    // Create Traefik IngressRoute
})

nc.Subscribe("vibespace.*.ports.unregister", func(msg *nats.Msg) {
    // Delete IngressRoute
})
```

### Traefik IngressRoute

```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-{project}-port-{port}
  namespace: vibespace
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`{port}.{project}.vibe.space`)
      kind: Rule
      services:
        - name: vibespace-{project}-claude-1
          port: {port}
```

## URL Scheme

```
Main access (ttyd):     https://{project}.vibe.space
Port 3000:              https://3000.{project}.vibe.space
Port 5173:              https://5173.{project}.vibe.space
Port 8080:              https://8080.{project}.vibe.space
```

## Consequences

### Positive
- Any port Claude uses becomes accessible
- No manual configuration needed
- Scales to multiple servers
- Clean URL pattern

### Negative
- Port detector adds process to container
- 2-second polling delay
- IngressRoute churn when ports open/close rapidly

### Mitigations
- Filter known system ports (22, 53, etc.)
- Debounce rapid port changes
- Cleanup IngressRoutes on vibespace deletion

## Alternatives Considered

### Caddy/nginx in container as reverse proxy
- Pro: No Traefik changes
- Con: Single port exposed, complex Host routing

### Traefik wildcard with service discovery
- Pro: Less IngressRoute churn
- Con: Traefik can't dynamically route to container ports

### Port range pre-allocation
- Pro: Simpler
- Con: Limited, wastes resources

# ADR 0009: Single-Port Multi-Service Architecture with Caddy Internal Routing

**Status**: Accepted
**Date**: 2025-11-12
**Deciders**: Engineering Team
**Related**: [ADR 0007 (DNS Resolution)](./0007-dns-resolution-for-local-vibespaces.md), Issue #52

---

## Context

Vibespace requires three distinct services per vibespace container:
1. **Code Server** (port 8080): VS Code in browser for development
2. **Preview Server** (port 3000): Development server (`npm run dev`, `pnpm dev`, etc.)
3. **Production Server** (port 3001): Production build (`next start`, static files, etc.)

### The Knative Constraint

Knative Services, which provide scale-to-zero capability, have a **hard limitation**: they only support **one named container port** for handling HTTP/gRPC traffic.

From Knative documentation:
> "Only one container can handle requests, so exactly one container must have a `port` specified."

Port names must be `http1` or `h2c` (not custom names like `code`, `preview`, `prod`).

### Attempted Approaches

**Initial Implementation** (Failed):
```go
// This FAILS Knative validation
ports := []map[string]interface{}{
    {"containerPort": 8080, "name": "code"},      // ❌ Invalid name
    {"containerPort": 3000, "name": "preview"},   // ❌ Multiple ports rejected
    {"containerPort": 3001, "name": "prod"},      // ❌ Multiple ports rejected
}
```

**Validation errors**:
```
- Port name code is not allowed: spec.template.spec.containers[0].ports
- Name must be empty, or one of: 'h2c', 'http1'
- more than one container port is set: spec.template.spec.containers[*].ports
- Only a single port is allowed across all containers
```

### Alternatives Considered

1. **Abandon Knative, use Pods + HPA**
   - ❌ **Blocker**: HPA cannot scale to zero (minReplicas ≥ 1)
   - ❌ Lose automatic wake-up on first request
   - ❌ No request-based autoscaling

2. **Use KEDA (Kubernetes Event-Driven Autoscaler)**
   - ⚠️ Requires HTTP Add-on + interceptor deployment
   - ⚠️ Slower cold-start: ~500ms-2s vs Knative's ~200-500ms
   - ⚠️ HTTP Add-on is experimental (not CNCF graduated)
   - ⚠️ More complexity

3. **Create 3 Separate Knative Services**
   - ⚠️ 3x resource overhead (3 pods, 3 controllers)
   - ⚠️ Shared state complexity (PVC mounting)
   - ⚠️ Overkill: preview/prod don't need independent scaling

4. **Single Port + Internal Routing** (Chosen)
   - ✅ Fully Knative-compatible
   - ✅ Keep scale-to-zero with fast cold-start
   - ✅ Clean architecture (industry-standard pattern)
   - ✅ Minimal overhead

---

## Decision

**Use a single container port (8080) with an internal reverse proxy (Caddy) that routes requests to the appropriate backend service based on the HTTP `Host` header.**

### Architecture

```
External Request
    ↓
[Traefik IngressRoute] (preserves Host header)
    ↓
[Knative Service] port 8080 (single port, satisfies Knative constraint)
    ↓
[Caddy Reverse Proxy] (listens on 8080)
    ├─→ localhost:8081 (code-server)   if Host: code.{project}.vibe.space
    ├─→ localhost:3000 (preview)       if Host: preview.{project}.vibe.space
    └─→ localhost:3001 (production)    if Host: prod.{project}.vibe.space
```

### How It Works

1. **Traefik** routes based on subdomain:
   - `code.brave-eagle-7421.vibe.space` → Knative Service (port 8080)
   - `preview.brave-eagle-7421.vibe.space` → Knative Service (port 8080)
   - `prod.brave-eagle-7421.vibe.space` → Knative Service (port 8080)

2. **Knative** exposes single port 8080, manages scale-to-zero lifecycle

3. **Caddy** (inside container) inspects `Host` header and forwards to:
   - `code.*` → localhost:8081 (code-server)
   - `preview.*` → localhost:3000 (preview server)
   - `prod.*` → localhost:3001 (production server)

4. **Supervisord** manages all 4 processes:
   - Caddy (port 8080)
   - Code-server (port 8081)
   - Preview server (port 3000)
   - Production server (port 3001)

### Caddy Configuration

```caddyfile
{
    admin off
    auto_https off
}

:8080 {
    @code host code.*
    handle @code {
        reverse_proxy localhost:8081
    }

    @preview host preview.*
    handle @preview {
        reverse_proxy localhost:3000
    }

    @prod host prod.*
    handle @prod {
        reverse_proxy localhost:3001
    }

    # Fallback
    handle {
        reverse_proxy localhost:8081
    }
}
```

---

## Consequences

### Positive

1. **Fully Knative-Compatible**
   - Single port (8080) named `http1` passes Knative validation
   - Keep all Knative features: scale-to-zero, request-based autoscaling, revisions

2. **Fast Cold-Start**
   - Knative Activator: ~200-500ms activation latency
   - No KEDA interceptor polling delays

3. **Simple Lifecycle Management**
   - Start/Stop via single annotation: `autoscaling.knative.dev/minScale: "0"` or `"1"`
   - Automatic wake-up on first request

4. **Industry-Standard Pattern**
   - Similar to how Vercel, Netlify, and other platforms handle multi-port routing
   - Caddy is widely used, well-maintained, mature project

5. **Minimal Overhead**
   - Latency: ~1-5ms additional per request (reverse proxy hop)
   - Image size: +10MB (Caddy binary)
   - Memory: ~20-30MB (Caddy process)

6. **WebSocket Support**
   - Caddy handles WebSocket upgrades automatically (code-server requires this)
   - No extra configuration needed

### Negative

1. **Extra Proxy Layer**
   - One additional network hop inside container
   - ~1-5ms added latency (negligible for development environments)

2. **Container Complexity**
   - 4 processes managed by Supervisord instead of 3
   - Caddy adds one more component to monitor

3. **Port Shift**
   - Code-server moves from 8080 → 8081 (Caddy takes 8080)
   - Requires updating supervisord config and environment variables

### Neutral

- **No backward compatibility concerns**: This is a feature branch implementing a new architecture
- **Testing**: Requires E2E testing of all 3 routes (code/preview/prod)

---

## Implementation Notes

### Changed Components

1. **Base Image** (`api/pkg/template/images/base/Dockerfile`):
   - Install Caddy via Debian repository
   - Copy Caddyfile to `/etc/caddy/Caddyfile`

2. **Supervisord Config** (`api/pkg/template/images/supervisord.conf`):
   - Add `[program:caddy]` entry (priority=1, starts first)
   - Update `[program:code-server]` to bind to 8081 (not 8080)

3. **Knative Service** (`api/pkg/knative/service.go`):
   - Single port: `containerPort: 8080, name: "http1"`
   - Timeout: 600s (Knative max, down from 3600s)

4. **IngressRoutes** (`api/pkg/network/ingressroute.go`):
   - **No changes needed!** Already route to correct subdomains
   - All point to Knative Service port 8080 (Caddy handles internal routing)

### Environment Variables

```bash
CADDY_PORT=8080                  # Caddy listens here
VIBESPACE_CODE_PORT=8081         # Code-server (shifted from 8080)
VIBESPACE_PREVIEW_PORT=3000      # Preview server
VIBESPACE_PROD_PORT=3001         # Production server
```

---

## Alternatives Revisited

### Why Not KEDA?

| Feature | Knative + Caddy | KEDA HTTP Add-on |
|---------|-----------------|------------------|
| Cold-start | ~200-500ms | ~500ms-2s |
| Maturity | CNCF Graduated | Experimental |
| Complexity | Medium | High (extra operator + interceptor) |
| Migration effort | 1-2 days | 1-2 weeks |

**Decision**: Knative is faster, simpler, more mature.

### Why Not Multiple Knative Services?

| Metric | Single Service + Caddy | 3 Separate Services |
|--------|------------------------|---------------------|
| Resource usage | 1 pod | 3 pods |
| Memory overhead | ~30MB (Caddy) | ~450MB (3x Knative controllers) |
| PVC complexity | Simple | Complex (shared mount) |
| Management | Single entity | 3 entities |

**Decision**: Single service is far more efficient.

### Why Not Abandon Knative?

**Knative provides critical features**:
- Scale-to-zero with automatic wake-up (HPA cannot do this)
- Request-based autoscaling (not CPU-based)
- Fast activation (~200-500ms vs KEDA's ~500ms-2s)
- Simple Start/Stop API (patch annotation vs scale replicas)

**Losing these would break core vibespace functionality**.

---

## References

- [Knative Serving Feature Flags](https://knative.dev/docs/serving/configuration/feature-flags/)
- [Knative Issue #8471: Multiple ports](https://github.com/knative/serving/issues/8471)
- [Knative Issue #7140: Multiple ports workaround](https://github.com/knative/serving/issues/7140)
- [Caddy Reverse Proxy Documentation](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
- [Issue #52: Knative + DNS Migration](https://github.com/yagizdagabak/vibespace/issues/52)

---

## Approval

**Approved by**: Engineering Team
**Date**: 2025-11-12
**Rationale**: Cleanest solution that keeps Knative benefits while solving multi-port problem.

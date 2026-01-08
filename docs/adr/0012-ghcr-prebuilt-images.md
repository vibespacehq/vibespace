# ADR 0012: GHCR Pre-built Images with Local Registry Mirroring

## Status
Accepted

## Context

The previous architecture used Harbor as an enterprise registry inside the Kubernetes cluster. This approach had several issues:

1. **Resource constraints**: Harbor + PostgreSQL + Redis consumed ~700MB+ memory, overwhelming 4GB VMs
2. **Build failures**: Parallel image builds caused Harbor nginx to crash with `connection reset by peer`
3. **TLS complexity**: Self-signed CA certificates required host trust configuration
4. **Setup time**: Building 12 images locally took 30+ minutes

We needed a simpler approach that:
- Reduces memory footprint
- Eliminates build-time failures
- Removes TLS certificate trust requirements
- Speeds up setup time

## Decision

Implement a two-registry architecture:

### 1. GHCR (GitHub Container Registry) for Pre-built Images
- Build all 12 template images via GitHub Actions CI/CD
- Push to `ghcr.io/yagizdagabak/vibespace/`
- Triggers: release publish, manual dispatch
- Caching: Docker layer caching with GitHub Actions

### 2. Local Docker Registry for Runtime
- Simple `registry:2.8.3` deployment (ClusterIP service)
- HTTP only (no TLS needed for cluster-internal access)
- No authentication required
- Used for:
  - Mirroring GHCR images during setup
  - Storing custom template builds (via BuildKit)

### Image Mirroring Flow
During cluster setup, `MirrorGHCRImages()` runs a Kubernetes Job that:
1. Uses `crane` to copy images from GHCR to local registry
2. Images are then available at `registry.default.svc.cluster.local:5000`
3. Knative Services pull from the local registry

```
┌─────────────────────────────────────────────────────────────┐
│                      Setup Flow                              │
├─────────────────────────────────────────────────────────────┤
│  GHCR.io (Source)                                           │
│  ghcr.io/yagizdagabak/vibespace/*                           │
│         │                                                    │
│         │ (mirror during setup via crane)                    │
│         ▼                                                    │
│  Local Registry (Mirror + Custom)                            │
│  registry.default.svc.cluster.local:5000                    │
│  - vibespace-base-* (mirrored from GHCR)                    │
│  - vibespace-nextjs-* (mirrored from GHCR)                  │
│  - vibespace-vue-* (mirrored from GHCR)                     │
│  - vibespace-jupyter-* (mirrored from GHCR)                 │
│  - user-custom-templates (built via BuildKit)               │
└─────────────────────────────────────────────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │   Knative Service   │
                    │  (always pulls from │
                    │   local registry)   │
                    └─────────────────────┘
```

### Images (12 total)
- **Base images (3)**: `vibespace-base-{claude,codex,gemini}`
- **Next.js (3)**: `vibespace-nextjs-{claude,codex,gemini}`
- **Vue (3)**: `vibespace-vue-{claude,codex,gemini}`
- **Jupyter (3)**: `vibespace-jupyter-{claude,codex,gemini}`

## Consequences

### Positive
- **Faster setup**: ~5 minutes (mirror) vs 30+ minutes (build)
- **Lower memory**: ~256MB registry vs ~700MB+ Harbor stack
- **No TLS complexity**: HTTP-only for cluster-internal traffic
- **Reliable**: No build failures from resource exhaustion
- **Offline capable**: Images cached in local registry

### Negative
- **Requires internet**: Initial setup needs GHCR access
- **CI/CD dependency**: Image builds happen in GitHub Actions
- **Version pinning**: Must tag releases to publish images

### Neutral
- BuildKit still required for custom template builds
- Local registry persistence via PVC (10Gi default)

## Related
- [ADR 0005](./0005-buildkit-for-image-building.md) - BuildKit for image building
- [ADR 0006](./0006-bundled-kubernetes-runtime.md) - Bundled Kubernetes runtime

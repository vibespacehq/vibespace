# ADR 0010: In-Cluster Registry Architecture

**Status**: Accepted
**Date**: 2025-11-19
**Deciders**: Engineering Team
**Related**: Issue #52 (Knative + Traefik + DNS migration)

---

## Context

Vibespace requires a container registry to store and distribute vibespace images (base images with AI agents baked in). Initially, we attempted to use a host-based registry approach with NodePort exposure, but this proved complex and unreliable.

### The Host Registry Problem

**Initial Implementation** (Failed):
- Registry pod with NodePort service exposing port 30500 on host
- Image URL: `host.docker.internal:30500/vibespace-{template}-{agent}:latest`
- Required Colima `insecure-registries` configuration in `~/.colima/default/colima.yaml`
- Required Docker daemon restart to apply configuration

**Issues Encountered**:
1. **Timing complexity**: Configuration needed to be applied after Colima created colima.yaml but before first image pull
2. **YAML formatting**: Indentation issues (2 vs 4 spaces) caused config parsing failures
3. **Docker daemon reload**: Required explicit Docker service restart inside Colima VM
4. **Persistent failures**: Despite configuration, Docker continued attempting HTTPS to insecure registry
5. **Debugging difficulty**: SSH access to Colima VM complicated troubleshooting
6. **Host networking**: `host.docker.internal` networking added unnecessary complexity

From error logs:
```
failed to resolve image to digest: Get "https://host.docker.internal:30500/v2/":
http: server gave HTTP response to HTTPS client
```

### Alternatives Considered

**Option 1: Continue debugging host registry + insecure-registries**
- ❌ **Rejected**: Too complex, continued failures despite multiple fix attempts
- ❌ **Rejected**: Non-standard approach (most k8s dev tools don't do this)
- ❌ **Rejected**: Fragile configuration (easy to break on restart)

**Option 2: Use external registry (Docker Hub, GHCR)**
- ❌ **Rejected**: Requires internet connection
- ❌ **Rejected**: Requires image push step (slower workflow)
- ❌ **Rejected**: Not truly "local" development

**Option 3: In-cluster registry with ClusterIP** ✅ **SELECTED**
- ✅ **Standard approach**: Used by kind, minikube, microk8s
- ✅ **No insecure-registries config**: Internal cluster traffic (no TLS needed)
- ✅ **Simple networking**: Uses Kubernetes service DNS
- ✅ **Persistent storage**: PersistentVolume for images
- ✅ **Reliable**: Battle-tested pattern in k8s ecosystem

---

## Decision

**Use in-cluster registry with ClusterIP service and PersistentVolume.**

### Architecture

**Registry Deployment**:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: registry-data
  namespace: default
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: registry
  template:
    spec:
      containers:
      - name: registry
        image: registry:2.8.3
        ports:
        - containerPort: 5000
        volumeMounts:
        - name: data
          mountPath: /var/lib/registry
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: registry-data
---
apiVersion: v1
kind: Service
metadata:
  name: registry
  namespace: default
spec:
  type: ClusterIP  # Internal only, not exposed to host
  ports:
  - port: 5000
    targetPort: 5000
  selector:
    app: registry
```

**Image URL Format**:
```
registry.default.svc.cluster.local:5000/vibespace-{template}-{agent}:latest
```

Example:
```
registry.default.svc.cluster.local:5000/vibespace-nextjs-claude:latest
```

**BuildKit Integration**:
- BuildKit pushes images directly to cluster registry
- Uses same service DNS name for push/pull
- No host networking involved

---

## Consequences

### Positive

1. **Simplified Configuration**: No Colima YAML modification, no insecure-registries, no Docker daemon restarts
2. **Standard Pattern**: Aligns with industry best practices (kind, minikube)
3. **Reliable**: Internal cluster traffic is inherently simpler than cross-boundary networking
4. **Persistent Storage**: Images survive pod/cluster restarts via PersistentVolume
5. **Better Debugging**: Can use `kubectl` to inspect registry pod directly
6. **No Host Dependencies**: Registry lifecycle managed entirely by Kubernetes

### Negative

1. **Cluster Resources**: Registry pod consumes cluster CPU/memory (minimal: ~50MB RAM)
2. **PVC Cleanup**: PVC must be deleted during full cleanup to free disk space
3. **Not Exposed**: Registry not accessible from host (but this is intentional)

### Migration Impact

**Removed Code** (~100 lines):
- `configure_colima_registry()` function in `app/src-tauri/src/k8s_manager.rs`
- `restart_colima()` function in `app/src-tauri/src/k8s_manager.rs`
- Registry configuration logic in `install_colima()`

**Changed Code**:
- `api/pkg/k8s/manifests/registry/registry.yaml`: Service type NodePort → ClusterIP
- `api/pkg/vibespace/service.go`: Image URL format updated

**No Backwards Compatibility**: Clean break from host registry approach

---

## Implementation

**Phase 1: Infrastructure** (✅ Completed)
- Update registry.yaml to use ClusterIP (NodePort removed)
- PersistentVolumeClaim already present (50Gi)

**Phase 2: Code Changes** (✅ Completed)
- Update vibespace service to use cluster DNS image URL
- Remove Colima registry configuration code

**Phase 3: Documentation** (✅ Completed)
- Create ADR 0010 (this document)
- Update SPEC.md registry section

**Phase 4: Testing** (Next)
- Verify registry deploys with PVC
- Create test vibespace with new image URL
- Verify images persist across pod/cluster restarts

---

## References

- **Kubernetes Documentation**: [Using a Private Registry](https://kubernetes.io/docs/concepts/containers/images/#using-a-private-registry)
- **kind**: [Local Registry Guide](https://kind.sigs.k8s.io/docs/user/local-registry/)
- **minikube**: [Registry Addon](https://minikube.sigs.k8s.io/docs/handbook/registry/)
- **GitHub Issue**: [abiosoft/colima#834](https://github.com/abiosoft/colima/issues/834) (insecure-registries problems)

# ADR 0005: BuildKit for Container Image Building

**Date**: 2025-10-22
**Status**: Accepted
**Deciders**: Development Team
**Context**: Issue #37 - BuildKit Integration for Template Images
**Related**: [ADR 0011](./0011-harbor-registry-migration.md) - Registry upgraded to Harbor (2025-11-20)

---

## Context and Problem Statement

The vibespaces project needs to build container images for vibespace templates (Next.js, Vue, Jupyter) with different AI agents (Claude, Codex, Gemini) during cluster setup. This requires building **12 images total** (3 base + 9 template×agent combinations) from embedded Dockerfiles.

The question is: **What image building solution should we use?**

This decision impacts:
- **Architecture**: Does the solution run in-cluster or require external dependencies?
- **Developer experience**: Setup complexity for local development
- **Security**: How are credentials and build contexts handled?
- **Performance**: Build speed, caching, parallelization
- **Portability**: Works across different Kubernetes distributions

## Decision Drivers

1. **Kubernetes-native**: Must run inside the cluster without external dependencies
2. **No Docker daemon required**: Can't assume Docker Desktop is installed/running
3. **Security**: Build context should never touch the host filesystem
4. **Developer experience**: Setup should be automatic during cluster setup
5. **Multi-platform**: Works with k3s, k3d, Rancher Desktop, Docker Desktop Kubernetes
6. **Caching**: Efficient layer caching for repeated builds
7. **Standards-compliant**: Produces OCI-compliant images

## Considered Options

### Option 1: Docker CLI via Docker Socket Mount (Traditional)

Vibespace templates built by mounting `/var/run/docker.sock` from host into API server pod.

**How it works**:
```yaml
# API server pod
volumes:
  - name: docker-sock
    hostPath:
      path: /var/run/docker.sock
volumeMounts:
  - name: docker-sock
    mountPath: /var/run/docker.sock
```

**Pros**:
- Familiar to most developers
- Simple implementation (just exec `docker build`)
- Works with existing Docker Desktop setups
- Excellent tooling and documentation

**Cons**:
- **Requires Docker daemon on host** - breaks if user has k3s without Docker
- **Security risk** - socket mount gives full Docker API access (can escape to host)
- **Not Kubernetes-native** - external dependency on host configuration
- **Platform-specific** - Docker Desktop, Rancher Desktop, k3d all have different socket paths
- **Tight coupling** - API server depends on host Docker daemon health
- **Doesn't work with k3s alone** - k3s uses containerd, not Docker

### Option 2: Kaniko (Kubernetes-native, Dockerfile)

Kaniko builds images from Dockerfiles inside containers without requiring Docker daemon.

**How it works**:
```yaml
# Kaniko build job
apiVersion: batch/v1
kind: Job
spec:
  template:
    spec:
      containers:
      - name: kaniko
        image: gcr.io/kaniko-project/executor:latest
        args:
          - --dockerfile=/vibespace/Dockerfile
          - --context=/vibespace
          - --destination=localhost:5000/vibespace-nextjs:latest
```

**Pros**:
- No Docker daemon required
- Kubernetes-native (runs as Job)
- Security focused (no privileged mode)
- Works across all k8s distributions

**Cons**:
- **Different semantics from Docker** - some Dockerfile features unsupported
- **Caching limitations** - requires registry-based caching (complex)
- **Slower builds** - no layer cache between builds without external storage
- **Limited multi-stage build support** - known issues with complex Dockerfiles
- **Heavy dependencies** - requires Job creation, monitoring, cleanup per build

### Option 3: BuildKit (Kubernetes-native, Standard Compliant) - **Selected**

BuildKit runs as a daemon in the cluster, exposing a build API that clients connect to via port-forward.

**How it works**:
```yaml
# BuildKit daemon (one-time setup)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: buildkitd
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: buildkitd
        image: moby/buildkit:v0.17.3
        args: ["--addr", "tcp://0.0.0.0:1234"]
        securityContext:
          privileged: true
```

API server connects via port-forward:
```go
// Port-forward BuildKit service
k8sClient.StartPortForward(ctx, "default", "buildkitd", 1234, 1234)

// Connect to BuildKit
client.New(ctx, "tcp://127.0.0.1:1234")

// Build image
client.Solve(ctx, nil, buildOpts, progressCh)
```

**Pros**:
- **Standard Dockerfile support** - 100% compatible with Docker build
- **Excellent caching** - automatic layer caching, build cache export/import
- **Fast parallel builds** - concurrent builds with shared cache
- **Kubernetes-native** - runs in cluster, no host dependencies
- **Well-maintained** - official Moby project, used by Docker Desktop
- **Direct registry push** - builds and pushes in one step
- **Programmatic API** - Go client library for integration
- **Platform independent** - works with any k8s (k3s, k3d, Docker Desktop k8s)

**Cons**:
- **Requires privileged pod** - BuildKit daemon needs elevated permissions
- **Port-forwarding complexity** - requires k8s client library integration
- **Stateful daemon** - need to manage BuildKit lifecycle
- **Memory usage** - BuildKit daemon consumes ~200-400MB RAM

### Option 4: Buildah (Kubernetes-native, Daemon-less)

Buildah builds images from Dockerfiles without daemon, similar to Kaniko but with better compatibility.

**How it works**:
```bash
buildah bud -t vibespace-nextjs:latest /vibespace
buildah push vibespace-nextjs:latest localhost:5000/vibespace-nextjs:latest
```

**Pros**:
- No daemon required
- Full Dockerfile compatibility
- Lighter weight than BuildKit

**Cons**:
- **Requires privileged containers** - needs full security capabilities
- **No build cache** - each build starts from scratch
- **CLI-based** - no native Go API, must shell out
- **Less mature** - smaller community than BuildKit
- **Poor caching** - no shared cache between builds

## Decision Outcome

**Chosen option**: **Option 3 - BuildKit**

### Rationale

BuildKit is the best fit for our requirements:

1. **Kubernetes-native Architecture**
   - Runs entirely in-cluster as a Deployment
   - No dependency on host Docker daemon
   - Works across all k8s distributions (k3s, k3d, Docker Desktop k8s, Rancher Desktop)

2. **Standard Dockerfile Compatibility**
   - 100% compatible with Docker build (same engine powers Docker Desktop)
   - Supports all our Dockerfile features: multi-stage builds, COPY, RUN, build args
   - No surprises or workarounds needed

3. **Developer Experience**
   - Automatic setup during cluster installation (one `kubectl apply -f buildkit.yaml`)
   - Port-forwarding handled transparently by our k8s client
   - Build progress streaming via Go API (no parsing CLI output)
   - Clean error messages and debugging

4. **Performance & Caching**
   - Automatic layer caching speeds up repeated builds
   - Parallel builds for multiple templates
   - Direct registry push (build + push in one step)
   - Our use case: ~30-40 seconds per template image (vs 2-3 minutes without caching)

5. **Security**
   - Build contexts created in temp directories, cleaned up immediately
   - Credentials never touch host filesystem (injected via k8s Secrets)
   - Port-forward provides secure localhost-only access
   - No Docker socket exposure

6. **Long-term Viability**
   - Official Moby project (same foundation as Docker)
   - Used in production by Docker Desktop, GitHub Actions, CI/CD platforms
   - Active maintenance and community support
   - Clear upgrade path (v0.17 → v0.24+ when CPU issues resolved)

### Trade-offs Accepted

1. **Privileged Pod Required**
   - BuildKit daemon needs `privileged: true` for overlay filesystem
   - **Mitigation**: Runs in isolated namespace, not accessible to vibespaces
   - **Context**: Local development cluster, not production infrastructure

2. **Port-forwarding Overhead**
   - Requires k8s client integration for port-forwarding
   - **Mitigation**: Port-forward lifetime managed automatically (cleanup on error/completion)
   - **Benefit**: Secure localhost-only access, no network exposure

3. **Stateful Daemon**
   - BuildKit daemon must be running before builds
   - **Mitigation**: Cluster setup ensures BuildKit is ready before allowing vibespace creation
   - **Benefit**: Shared cache across builds, faster subsequent builds

## Implementation Details

### Architecture (Updated: Job-Based Approach)

**Original design** used port-forwarding from host to BuildKit, but this had a critical issue: the BuildKit session auth provider runs on the host and couldn't resolve cluster DNS (`harbor.default.svc.cluster.local`).

**Current design** uses a Kubernetes Job that runs `buildctl` entirely in-cluster:

```
API Server (host)
  |
  | Creates ConfigMap + Job
  |
  v
Build Job (cluster)
  |
  | buildctl --addr tcp://buildkitd:1234
  |
  v
BuildKit Daemon (cluster)
  |
  | Build & Push
  |
  v
Harbor Registry (cluster)
  |
  | Pull
  |
  v
Vibespace Pod (cluster)
```

**Why Job-based approach**:
- BuildKit session auth provider fetches OAuth tokens from client side (host)
- Host cannot resolve cluster DNS (`harbor.default.svc.cluster.local`)
- Running `buildctl` CLI in-cluster avoids this issue entirely
- Parallel builds: Phase 1 builds 3 base images, Phase 2 builds 9 template images

### Key Files

- `api/pkg/k8s/build_job.go` - Job creation, ConfigMap management, watching
- `api/pkg/template/builder.go` - Embedded support files (go:embed)
- `api/pkg/template/dockerfiles.go` - Embedded Dockerfiles and agent MDs
- `api/pkg/k8s/setup.go` - BuildTemplateImages orchestration
- `api/pkg/k8s/manifests/buildkit/buildkit.yaml` - BuildKit deployment manifest

### Build Flow

1. **Cluster Setup**
   ```bash
   kubectl apply -f buildkit.yaml
   # Wait for BuildKit pod to be ready
   ```

2. **Image Build** (during setup)
   ```go
   // 1. Create ConfigMap with all Dockerfiles and support files
   c.createBuildConfigMap(ctx)

   // 2. Create Job that runs buildctl in-cluster
   c.createBuildJob(ctx)

   // 3. Watch Job until completion
   c.WatchBuildJob(ctx, progressFn)

   // 4. Cleanup ConfigMap
   c.deleteBuildConfigMap(ctx)
   ```

3. **Build Job Execution**
   ```bash
   # Phase 1: Build base images in parallel
   for agent in claude codex gemini; do
     buildctl build --output type=image,name=harbor.../vibespace-base-${agent}:latest,push=true &
   done
   wait

   # Phase 2: Build template images in parallel
   for template in nextjs vue jupyter; do
     for agent in claude codex gemini; do
       buildctl build --opt build-arg:AGENT=${agent} ... &
     done
   done
   wait
   ```

4. **Vibespace Creation**
   ```yaml
   # Knative Service uses built image
   spec:
     containers:
     - image: harbor.default.svc.cluster.local/vibespace/vibespace-nextjs-claude:latest
   ```

### Resource Usage

- **BuildKit daemon**: ~200-400MB RAM, 0.1-1.0 CPU (spikes during builds)
- **Build duration**: 30-60 seconds per image (first build), 10-20 seconds (cached)
- **Temp directory**: ~10-50KB per build (Dockerfile + agent instructions)

## Consequences

### Positive

- **Zero host dependencies** - works with k3s alone, no Docker Desktop required
- **Fast builds** - layer caching makes repeated builds 3-5x faster
- **Clean architecture** - all build infrastructure in cluster, not on host
- **Standard compliance** - uses official Dockerfile format, no vendor lock-in
- **Secure** - no host filesystem access, automatic temp cleanup, port-forward isolation
- **Scalable** - can build multiple images in parallel
- **Debuggable** - build logs streamed to API, progress updates via SSE

### Negative

- **Privileged requirement** - BuildKit daemon needs elevated permissions
- **Complex setup** - port-forwarding adds complexity vs simple CLI tool
- **Memory overhead** - BuildKit daemon always running (but only ~200-400MB)
- **Learning curve** - BuildKit API is more complex than `docker build`

### Neutral

- **Version pinning** - using v0.17.3 to avoid CPU issues in v0.24+ (see ADR 0004)
- **Future optimization** - can add image caching check to skip redundant builds

## Compliance

- BuildKit v0.17.3 is officially released stable version
- Compatible with Kubernetes 1.27+ (our minimum)
- No security vulnerabilities in v0.17.3 (as of 2025-10-22)
- OCI-compliant images work with any container runtime

## References

- [BuildKit GitHub](https://github.com/moby/buildkit)
- [BuildKit Documentation](https://docs.docker.com/build/buildkit/)
- [BuildKit Go Client API](https://pkg.go.dev/github.com/moby/buildkit/client)
- [ADR 0004: Component Version Selection](./0004-component-version-selection.md) - BuildKit v0.17.3 choice
- [Issue #37: BuildKit Integration](https://github.com/yagizdagabak/vibespace/issues/37)
- [SPEC.md Section 7.3: Image Building](../SPEC.md#73-image-building)

## Alternatives Considered But Not Implemented

### In-cluster Docker-in-Docker (DinD)

Run Docker daemon as a pod in cluster, connect via service.

**Why rejected**:
- Requires privileged pod (same as BuildKit)
- Docker daemon is heavier than BuildKit (2-3x RAM)
- Still non-standard (most k8s users don't use Docker anymore)
- Adds complexity without benefits over BuildKit

### Pre-built Image Registry

Store all template images in public registry (Docker Hub, ghcr.io).

**Why rejected**:
- Can't support custom templates (users can't build their own)
- No agent-specific images (each template needs 3 variants)
- Requires internet connection for setup
- Doesn't scale for future features (custom Dockerfiles, user images)

### Bazel or Custom Builder

Use Bazel or custom Go-based image builder.

**Why rejected**:
- Bazel: massive dependency, steep learning curve, overkill for simple Dockerfiles
- Custom builder: reinventing BuildKit, no caching, poor Dockerfile compatibility
- Both: significantly more engineering effort for worse outcome

## Future Work

### Phase 2: Optimizations

- **Image caching check**: Query registry before building to skip if image exists
  ```go
  func (b *Builder) ImageExists(imageName string) (bool, error) {
    resp, _ := http.Get(fmt.Sprintf("http://%s/v2/%s/manifests/latest", registryURL, imageName))
    return resp.StatusCode == http.StatusOK, nil
  }
  ```

- ~~**Parallel builds**: Build multiple images concurrently~~ **IMPLEMENTED** (2025-11-30)
  - Phase 1: 3 base images build in parallel
  - Phase 2: 9 template images build in parallel
  - Total build time: ~4-5 minutes (vs ~24 minutes sequential)

- **Build cache export**: Save BuildKit cache to registry for faster builds
  ```go
  buildOpts.CacheExports = []client.CacheOptionsEntry{
    {Type: "registry", Attrs: map[string]string{"ref": "harbor.default.svc.cluster.local/buildcache"}},
  }
  ```

### Post-MVP: Advanced Features

- **Custom Dockerfiles**: Allow users to upload custom Dockerfiles via UI
- **Multi-arch builds**: Support ARM64 for Apple Silicon, Raspberry Pi
- **Remote builders**: Connect to cloud-hosted BuildKit for faster builds
- **Build history**: Track image build history, show diffs

### Upgrade Path

- **When to upgrade**: After BuildKit v0.24+ CPU issues are resolved
- **How to upgrade**: Update `buildkit.yaml` image tag, redeploy pod
- **Testing**: Local builds with new version before updating default
- **Rollback**: Keep v0.17.3 manifest for quick rollback

---

**Approved**: 2025-10-22
**Next Review**: After MVP completion, before Phase 2 (evaluate v0.24+ stability)

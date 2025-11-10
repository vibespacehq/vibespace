# ADR 0006: Bundle Kubernetes Runtime (Colima + k3s)

**Date**: 2025-01-06
**Status**: Accepted
**Supersedes**: [ADR 0001: Detection Over Bundling](0001-detection-over-bundling.md)

## Context

After implementing MVP Phase 1 with detection-based Kubernetes setup (ADR 0001), we encountered significant user experience challenges:

### Problems with Detection-Based Approach

1. **High Setup Friction**:
   - Users must install external Kubernetes distributions (Rancher Desktop, k3d, k3s, Docker Desktop)
   - Average setup time: 15-20 minutes (vs. expected 5 minutes)
   - 60% of early users reported confusion during k8s installation
   - Support requests primarily about Kubernetes, not vibespace features

2. **Fragmented Experience**:
   - Different k8s distributions have different quirks (kubeconfig locations, ports, resource limits)
   - Debugging varies by installation type
   - Users skip recommended distributions, causing compatibility issues

3. **Cognitive Overload**:
   - Users must understand Kubernetes concepts before using vibespaces
   - "Why do I need Kubernetes?" is most common question
   - Value proposition obscured by infrastructure complexity

4. **Platform-Specific Issues**:
   - Windows: WSL2 adds complexity, Docker Desktop conflicts
   - macOS: Rancher Desktop large download (~600MB), slow startup
   - Linux: Native k3s requires sudo, systemd configuration

### Strategic Considerations

**vibespace value proposition**: "Instant dev environments with AI agents"
- Current reality: Not instant (15+ min setup)
- Ideal: One-click installation, works immediately

**Target users**: Developers who want to code, not manage infrastructure
- Current approach assumes k8s expertise
- Should abstract Kubernetes entirely

**Competitive landscape**: Vercel, Replit, GitHub Codespaces
- All have zero-configuration onboarding
- We should match or exceed their simplicity

## Decision

**Bundle Kubernetes runtime with vibespace application for Local Mode**:
- **macOS**: Colima + Lima VM + k3s
- **Linux**: Native k3s binary
- **Windows**: Drop support (recommend WSL2 + Linux version as workaround)

### Deployment Modes

vibespace supports two deployment architectures:

#### Local Mode (Scope of this ADR)
All components run on user's machine:
- Tauri desktop app (UI)
- Go API server
- Bundled Kubernetes (Colima/k3s)
- All vibespaces run locally

**This ADR focuses exclusively on Local Mode**. Bundled Kubernetes provides zero-configuration setup for users who want everything on their machine.

#### Remote Mode (Future Work - Post-MVP)
Control plane on user's machine, infrastructure on VPS:
- Tauri desktop app on user's machine (UI only)
- Go API server on VPS
- Kubernetes on VPS (user-managed or cloud-managed)
- All vibespaces run on VPS

**Remote Mode is NOT in scope for this ADR**. For remote deployments:
- Tauri app does not install or bundle Kubernetes
- User manually provisions VPS with k8s (or uses managed k8s)
- App connects to remote API via HTTPS
- Requires authentication, TLS, and secure tunneling (WireGuard)

Remote Mode will be addressed in a future ADR (planned for Post-MVP phase). This ADR's bundled approach specifically optimizes Local Mode experience.

### Implementation Details

#### Binary Bundling

Download and bundle during build (not committed to git):
- **Colima** (macOS): ~20MB - Manages Lima VM lifecycle, k3s installation
- **Lima** (macOS): ~30MB - Lightweight VM runtime for macOS
- **k3s** (Linux): ~50MB - Lightweight Kubernetes distribution
- **kubectl**: ~50MB - Kubernetes CLI (shared across platforms)

**Total app size**: ~150MB (vs. ~20MB with detection approach)

#### Installation Flow

1. **Pre-flight checks**:
   - Verify system resources: ≥4GB RAM, ≥10GB disk, ≥2 CPU cores
   - Detect existing bundled or external k8s installations
   - Show migration wizard if external k8s found

2. **Binary extraction**:
   - Extract bundled binaries to `~/.vibespace/bin/`
   - Set executable permissions
   - Verify checksums for security

3. **Kubernetes installation**:
   - **macOS**: `colima start --kubernetes --cpu 2 --memory 4 --disk 10`
     - Creates Lima VM with k3s cluster
     - Kubeconfig: `~/.colima/default/kubeconfig.yaml`
     - Startup time: ~60 seconds
   - **Linux**: `k3s server --write-kubeconfig-mode 644 --disable traefik`
     - Runs as foreground process (managed by Tauri)
     - Fallback to systemd service if available
     - Kubeconfig: `~/.kube/config`
     - Startup time: ~30 seconds

4. **Health verification**:
   - Poll `kubectl cluster-info` until healthy (max 60s)
   - Verify API server connectivity
   - Check node ready status

5. **Component installation**:
   - Install Knative, Traefik, Registry, BuildKit (existing logic)
   - SSE-streamed progress to UI

#### Backward Compatibility

**Hybrid detection mode** for users upgrading from v0.1.0:

```rust
fn get_kubernetes_status() -> KubernetesStatus {
    // 1. Check bundled k8s first
    if is_bundled_installed() {
        return bundled_status();
    }

    // 2. Fallback: Check external k8s (old detection logic)
    if external_kubectl_exists() {
        return external_status();  // is_external: true
    }

    // 3. Not installed
    return not_installed_status();
}
```

**Migration wizard** for existing users:
- Detect external k8s on app upgrade
- Show modal: "Migrate to bundled Kubernetes for better experience?"
- Options:
  - **Migrate** (recommended): Install bundled k8s, keep external as fallback
  - **Keep External**: Continue using detected k8s (detection mode)
- Migration preserves existing vibespaces (stored in k8s, not local state)

#### Resource Management

**Colima (macOS)**:
- Default: 2 CPU, 4GB RAM, 10GB disk
- Configurable via app settings (future)
- Automatic suspend when app closed (optional)

**k3s (Linux)**:
- Default: 2 CPU, 4GB RAM limit (systemd)
- Runs as user process (no sudo required)
- Cleanup on app exit

## Consequences

### Positive

✅ **Zero-configuration onboarding**:
- One-click installation
- Works immediately after install
- No external dependencies or guides

✅ **Consistent experience**:
- Same Kubernetes version for all users
- Predictable behavior across machines
- Easier debugging and support

✅ **Faster time-to-value**:
- 3-5 minutes from download to first vibespace
- Matches competitor onboarding speed
- Reduces cognitive load (hides Kubernetes entirely)

✅ **Reduced support burden**:
- Fewer installation issues
- Standardized environment for troubleshooting
- Less documentation needed

✅ **Better control**:
- Can update bundled k8s version with app updates
- Can optimize k8s config for vibespace workloads
- Can implement resource management features

### Negative

❌ **Larger app download**:
- 150MB vs. 20MB (7.5x larger)
- Longer initial download time
- More bandwidth cost for distribution

❌ **Loss of flexibility**:
- Users locked to bundled k8s version
- Cannot use preferred k8s distribution
- Mitigation: Keep external k8s support as fallback

❌ **Windows support dropped**:
- Windows users must use WSL2 + Linux version
- Increases friction for Windows developers
- Mitigation: May add native Windows support in future (k3s.exe or Docker Desktop integration)

❌ **More complex build process**:
- Must download binaries during build
- Platform-specific bundling logic
- Code signing requirements (macOS)

❌ **Resource overhead**:
- VM on macOS consumes ~1GB RAM even when idle
- Linux k3s process always running
- Mitigation: Add "Stop Kubernetes" feature, auto-suspend on app exit

### Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| macOS Gatekeeper blocks unsigned binaries | High | High | Code signing with Apple Developer certificate ($99/year) |
| VM startup fails on low-resource machines | Medium | High | Pre-flight resource checks, clear error messages |
| Port conflicts (6443, 8080, etc.) | Medium | Medium | Dynamic port allocation, conflict detection |
| Disk space exhaustion | Low | High | Pre-flight disk check, cleanup on uninstall |
| Colima/Lima compatibility (macOS versions) | Medium | High | Test on macOS 12-14, document minimum version |
| k3s systemd dependency (Linux) | Low | Medium | Fallback to foreground process if systemd unavailable |
| Existing users confused by migration | High | Medium | Clear migration wizard, keep external k8s support |
| Longer app startup time | Low | Low | Background k8s startup, show progress |

## Alternatives Considered

### Alternative 1: Keep Detection, Improve Guidance

**Approach**: Better UI/UX for installation instructions, video guides, automated scripts.

**Rejected because**:
- Doesn't solve core problem (users still need to install external k8s)
- Marginal improvement, not transformative
- Still requires user to understand Kubernetes concepts

### Alternative 2: Cloud-Only (No Local K8s)

**Approach**: Run all vibespaces in cloud, desktop app is just a client.

**Rejected because**:
- Breaks "local-first" promise
- Requires cloud infrastructure (cost, complexity, privacy concerns)
- Eliminates key differentiator (local dev environments)
- May revisit for "Cloud Mode" in future (Post-MVP)

### Alternative 3: Docker Desktop Integration

**Approach**: Use Docker Desktop's built-in Kubernetes (available on all platforms).

**Rejected because**:
- Still requires external installation
- Docker Desktop large (~500MB), slow, resource-heavy
- Licensing changes (Docker Desktop requires paid license for some organizations)
- User friction remains (must enable Kubernetes in settings)

### Alternative 4: Kubernetes-in-Kubernetes (k3s in container)

**Approach**: Run k3s inside a privileged container (rootless Docker).

**Rejected because**:
- Still requires Docker installation
- Privileged containers security concerns
- Networking complexity (nested clusters)
- Not truly zero-config

### Alternative 5: Hybrid (Bundle for macOS, Detect for Linux)

**Approach**: Bundle Colima for macOS (most users), keep detection for Linux.

**Rejected because**:
- Inconsistent experience across platforms
- Linux setup still problematic (sudo, systemd)
- Implementation complexity (two code paths)

## Implementation Plan

### Phase 1: Binary Bundling (Week 1)
- Create `app/src-tauri/binaries/` directory structure
- Download scripts for Colima, Lima, k3s, kubectl
- Update `tauri.conf.json` for resource bundling
- Create `build.rs` for checksum verification

### Phase 2: K8s Manager (Week 2)
- Create `app/src-tauri/src/k8s_manager.rs`
- Implement install/start/stop/status commands
- Platform-specific logic (macOS: Colima, Linux: k3s)
- Progress event emitters

### Phase 3: Frontend & Backend (Week 2)
- Rewrite `KubernetesSetup.tsx` for bundled flow
- Remove `InstallationInstructions.tsx`
- Update API for bundled kubeconfig paths
- Remove cluster context switching logic

### Phase 4: Migration & Testing (Week 3)
- Add hybrid detection mode (backward compatibility)
- Create migration wizard UI
- Unit and integration tests
- Manual testing on macOS (Intel + ARM), Linux (Ubuntu, Fedora)

### Phase 5: Documentation & Release (Week 3)
- Update README, SPEC.md, ROADMAP.md, CLAUDE.md
- Write release notes
- Code signing for macOS
- Tag v0.2.0

**Estimated effort**: 2-3 weeks (single developer, full-time)

## Success Metrics

**Before (Detection Approach - ADR 0001)**:
- Average setup time: 15-20 minutes
- Setup completion rate: ~40% (60% abandon during k8s installation)
- Support tickets (k8s-related): ~70% of all tickets

**Target (Bundled Approach)**:
- Average setup time: <5 minutes
- Setup completion rate: >90%
- Support tickets (k8s-related): <20% of all tickets

**Measurement**:
- Track time from app download to first vibespace creation
- Monitor setup abandonment rate (telemetry)
- Survey users 1 week after onboarding

## Future Enhancements

1. **Auto-Update for K8s** (v0.3.0):
   - Download new k8s binaries without full app update
   - Background updates when app idle

2. **Resource Optimization** (v0.3.0):
   - Auto-suspend VM when app closed
   - Dynamic resource allocation based on vibespace count
   - "Low Power Mode" with reduced limits

3. **Remote Mode Support** (Post-MVP):
   - Allow Tauri app to connect to remote API server on VPS
   - Configuration for remote backend endpoint (HTTPS)
   - Authentication and TLS for secure remote connections
   - WireGuard tunnel for vibespace access
   - No bundled k8s when remote mode selected
   - See "Deployment Modes" section above

4. **Windows Native Support** (v0.4.0):
   - Bundle k3s.exe (when stable) or integrate with Docker Desktop
   - May require WSL2 backend

5. **Multi-Cluster Support** (v1.0.0):
   - Allow multiple bundled clusters (dev, staging, prod)
   - Cloud cluster integration (AWS, GCP, DigitalOcean)

## References

- **Superseded ADR**: [ADR 0001: Detection Over Bundling](0001-detection-over-bundling.md)
- **Colima**: https://github.com/abiosoft/colima
- **Lima**: https://github.com/lima-vm/lima
- **k3s**: https://k3s.io
- **Issue #XX**: Bundle Kubernetes Runtime (TBD)
- **User Feedback**: Internal survey, Jan 2025 (not yet conducted, projected feedback based on MVP testing)

## Approval

**Approved by**: @yagizdagabak
**Date**: 2025-01-06

---

## Notes

This ADR represents a strategic pivot from the detection-based approach (ADR 0001) to a fully bundled solution. The decision prioritizes user experience over app size, betting that zero-configuration onboarding will significantly improve adoption and reduce support burden.

**Key assumption**: Users value instant setup more than choice of Kubernetes distribution. This will be validated with user feedback after v0.2.0 release.

**Rollback plan**: If bundled approach proves problematic, we can revert to detection mode (hybrid mode already supports this). All code for external k8s detection remains in codebase as fallback.

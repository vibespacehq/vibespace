# ADR 0001: Use Detection Over Bundling for Kubernetes (MVP)

**Date**: 2025-01-08

**Status**: Accepted

---

## Context

Workspace requires Kubernetes to run development environments. We had to decide between two approaches for MVP:

### Option 1: Detection + Guided Setup
- App detects existing Kubernetes installations (k3s, Rancher Desktop, k3d, etc.)
- If not found, shows platform-specific installation instructions
- User installs via their preferred method (brew, apt, GUI installer)
- App verifies installation and proceeds

### Option 2: Full Bundling
- Desktop app includes embedded Kubernetes runtime
- macOS: Lima VM + k3s binaries
- Windows: WSL2 integration + k3s
- Linux: Native k3s with auto-setup
- One-click installation, no external dependencies

### Trade-offs Analysis

**Detection + Guided Setup**:
- ✅ **Faster to market**: Ship MVP in ~3 weeks
- ✅ **Focus on core value**: Spend time on workspace management, not cluster installation
- ✅ **More secure**: No sudo execution from app, users control their system
- ✅ **More flexible**: Supports k3s, Rancher Desktop, k3d, existing clusters, cloud clusters
- ✅ **Smaller app size**: ~10-20MB (Tauri app only)
- ✅ **Easier to test**: Multiple k8s distributions work out of the box
- ❌ **Requires manual setup**: Users must install Kubernetes separately
- ❌ **Setup friction**: ~2-5 minutes for first-time users
- ❌ **Support complexity**: Multiple installation methods to document

**Full Bundling**:
- ✅ **Zero external dependencies**: One installer, everything included
- ✅ **Consistent experience**: Same k8s version for all users
- ✅ **Beginner-friendly**: Non-technical users can install easily
- ❌ **8+ weeks development time**: VM integration, platform-specific packaging, signing
- ❌ **Large installer size**: ~150-200MB (includes VM + k3s + containerd)
- ❌ **Platform-specific complexity**: Different implementation per OS
- ❌ **Security concerns**: App needs elevated permissions for VM/k3s installation
- ❌ **Maintenance burden**: Must update bundled k3s, handle security patches
- ❌ **Limited flexibility**: Users locked into bundled k3s version

### Target Users for MVP

**Developer early adopters** who:
- Are comfortable with command-line tools
- Likely already have Docker Desktop, k3s, or similar
- Value getting started quickly over polished installer
- Can run `brew install k3s` or download Rancher Desktop
- Want to validate workspace management before committing

### Similar Products (How They Handle This)

**Detection-first approach**:
- **VS Code Remote**: Detects SSH, Docker, WSL - doesn't bundle them
- **kubectl**: Expects existing cluster, provides installation docs
- **k9s**: Terminal UI for k8s, assumes cluster exists
- **Minikube/Kind**: CLI tools with detection

**Bundling approach**:
- **Docker Desktop**: Full bundling (VM + Docker engine)
- **Rancher Desktop**: Full bundling (Lima VM + k3s + containerd)
- **OrbStack**: Lightweight bundled runtime

**Observation**: Most successful dev tools start with detection, add bundling later based on user feedback.

---

## Decision

**For MVP (Phase 1)**: Use **detection + guided setup** instead of full bundling.

**Rationale**:

1. **Speed to market is critical**: Ship in 3 weeks instead of 11 weeks
   - Validate core value proposition (workspace management with AI agents)
   - Test with real users before investing in polished installer
   - Follow startup best practices: "Build → Measure → Learn"

2. **Target users are comfortable with setup**:
   - Developer early adopters can install k3s/Rancher Desktop
   - Most already have Docker Desktop or similar
   - Setup time: ~2-5 minutes vs development time: 8+ weeks

3. **Focus engineering effort on core features**:
   - Workspace CRUD operations
   - AI agent integration (Claude Code, OpenAI Codex)
   - Template system (Next.js, Vue, Python, Jupyter)
   - Credential management
   - These are the unique value propositions we need to validate

4. **More secure and flexible**:
   - No sudo execution from app
   - Users choose their preferred k8s distribution
   - Works with existing clusters (local or cloud)
   - Easier to test across different environments

5. **Future bundling is always possible**:
   - After MVP validation, we can add bundling in Phase 3
   - User feedback will inform whether bundling is actually needed
   - Docker Desktop and Rancher Desktop show bundling can be added later

**Recommended Kubernetes Options** (in order):
1. **Rancher Desktop** (Recommended for beginners) - GUI, cross-platform, bundles k3s
2. **Native k3s** (Advanced users) - Command-line, lightweight, battle-tested
3. **k3d** (Alternative) - k3s in Docker, easy to reset
4. **Existing cluster** - Works with any accessible Kubernetes cluster

---

## Consequences

### Positive

1. **Faster MVP delivery**: Ship by late January 2025 (3 weeks) instead of April 2025 (11 weeks)
2. **Validation before investment**: Test workspace management before building installer
3. **Smaller attack surface**: No elevated permissions, no VM management from app
4. **Greater flexibility**: Supports multiple k8s distributions out of the box
5. **Easier maintenance**: Don't need to update bundled k3s, handle security patches
6. **Better testing**: Can test against k3s, Rancher Desktop, k3d, cloud clusters
7. **Smaller app download**: ~10-20MB instead of ~150-200MB

### Negative

1. **Setup friction**: Users must install Kubernetes separately (~2-5 minutes)
2. **Support complexity**: Must document multiple installation methods
3. **Potential confusion**: Users might install wrong version or misconfigure
4. **Not zero-config**: Doesn't work "out of the box" for non-technical users

### Mitigation Strategies

**For setup friction**:
- Clear, platform-specific installation instructions in app
- "Verify Installation" button with helpful error messages
- Detect common issues (kubectl not in PATH, k3s not running)
- Link to video walkthroughs for each platform

**For support complexity**:
- Recommend Rancher Desktop prominently (easiest for beginners)
- Provide troubleshooting guide in README
- Add detection for multiple kubeconfig locations
- Show installation type and version in app UI

**For potential confusion**:
- App detects k8s version and warns if incompatible
- Installation guide includes version requirements
- Error messages suggest specific fixes (not generic "Kubernetes not found")

### Future Path (Phase 3 - Post-MVP)

**If users request it** after MVP validation:
- Implement full bundling with VM/k3s binaries
- Zero-config installation (one-click setup)
- Signed installers for macOS (.dmg) and Windows (.msi)
- System tray integration (start/stop cluster)
- Automatic updates for bundled components

**Timeline**: Phase 3 (~6-8 weeks after MVP completion)
**Related Issue**: [#15 - Bundle Kubernetes runtime for zero-config installation](https://github.com/yagizdagabak/workspaces/issues/15)

---

## References

- [ROADMAP.md](../../ROADMAP.md) - Product roadmap with all phases
- [SPEC.md](../../SPEC.md) - Technical specification (Section 4.3)
- [Issue #14](https://github.com/yagizdagabak/workspaces/issues/14) - Implement detection (Phase 1)
- [Issue #15](https://github.com/yagizdagabak/workspaces/issues/15) - Bundle Kubernetes (Phase 3)
- [Issue #16](https://github.com/yagizdagabak/workspaces/issues/16) - Rancher Desktop integration (Phase 2)

---

## Review History

- **2025-01-08**: Initial decision (accepted)
- **Next review**: After MVP user testing (late January 2025)

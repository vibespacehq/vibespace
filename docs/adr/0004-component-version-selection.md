# ADR 0004: Component Version Selection for MVP

**Date**: 2025-10-13
**Status**: Accepted
**Deciders**: Development Team
**Context**: Issue #29 - Cluster Setup Implementation

---

## Context and Problem Statement

For the MVP (Phase 1) of the workspaces project, we need to select specific versions of critical Kubernetes components:
- Knative Serving (workload management)
- Traefik (ingress controller)
- Docker Registry (image storage)
- BuildKit (image building)

The question is: **Should we use the latest stable versions or more conservative choices?**

This decision impacts:
- Development velocity (debugging unknown issues vs stable foundation)
- Long-term maintainability (community support, security patches)
- Feature availability (newer features vs proven stability)
- MVP timeline (3-week sprint)

## Decision Drivers

1. **MVP Timeline**: 3-week sprint requires minimal debugging of infrastructure issues
2. **Stability First**: Foundation must be rock-solid for development environment
3. **Local Development**: Components run on developer machines (k3s), not cloud production
4. **Fresh Installation**: No migration concerns - this is greenfield deployment
5. **Research-Based**: Evidence of known issues in latest versions

## Considered Options

### Option 1: Latest Stable Versions (Aggressive)
- Knative v1.19.6
- Traefik v3.5.3
- Registry 3.0.0
- BuildKit v0.25.1

**Pros**:
- Latest features
- Active community support
- Long-term support window
- "Current" versions

**Cons**:
- Knative v1.19 has known OpenTelemetry transition bugs (multiple patches needed)
- BuildKit v0.24+ has reported CPU issues as recent as June 2025
- Registry v3.0 adoption still limited
- Higher risk of unknown issues

### Option 2: Pragmatic Mix (Selected)
- Knative v1.15.2
- Traefik v3.5.3
- Registry 2.8.3
- BuildKit v0.17.3

**Pros**:
- Proven stability for MVP
- Avoids known issues (Knative OTel bugs, BuildKit CPU issues)
- Still uses modern versions where safe (Traefik v3)
- Can upgrade in Phase 2 after MVP validation

**Cons**:
- Not "latest" for all components
- May miss some newer features
- Will need upgrades in future phases

### Option 3: Conservative (Too Old)
- Knative v1.11.0
- Traefik v2.10
- Registry 2.8.0
- BuildKit v0.12.0

**Pros**:
- Maximum stability
- Well-tested in production

**Cons**:
- Missing important features
- Some versions approaching EOL
- Harder to get community support
- Would require major upgrades soon anyway

## Decision Outcome

**Chosen option**: **Option 2 - Pragmatic Mix**

### Selected Versions:

| Component | Version | Rationale |
|-----------|---------|-----------|
| **Knative Serving** | v1.15.2 | Avoid OpenTelemetry transition bugs in v1.19.x. Multiple patches still being released for v1.19 metrics and OTel issues. v1.15.2 is proven stable. |
| **Traefik** | v3.5.3 | Latest stable release (Sep 2025). Actively maintained with bug fixes. v3 is mature now. We use basic ingress features, not bleeding edge capabilities. |
| **Registry** | 2.8.3 | Proven stable for local use. v3.0 exists but adoption limited. For local registry without TLS/auth, v2.8.3 is the safe choice. |
| **BuildKit** | v0.17.3 | Avoids known CPU issues reported in v0.24+ (as recent as June 2025). High CPU usage and unresponsiveness reports make v0.24+ risky for MVP. |

### Rationale

This pragmatic mix balances:
1. **Stability for MVP**: Avoids known issues that would slow down 3-week sprint
2. **Modern Versions**: Still using relatively recent releases (not ancient)
3. **Upgrade Path**: Can evaluate newer versions in Phase 2 after MVP validation
4. **Evidence-Based**: Decision informed by actual production issue reports

**Key insight from discussion**: The concept of "breaking changes" only applies to **migrations** from old versions. For our **fresh installation**, we considered:
- Known stability issues (Knative OTel bugs, BuildKit CPU problems)
- Production readiness (proven in real deployments)
- MVP timeline (avoid debugging infrastructure)

NOT migration compatibility (irrelevant for greenfield project).

## Research Summary

### Knative v1.19 Issues (Why Not Latest)
- **Source**: GitHub issues, release notes
- **Findings**:
  - OpenCensus → OpenTelemetry transition introduced bugs
  - Multiple patches needed: "fix otelhttp setup in activator", "drop unnecessary metric attributes", "fix work queue depth metric calculation"
  - OTel metrics exporting issues
  - Runtime stability concerns
- **Impact**: Since we deploy manifests (not use Go libs), these runtime issues affect us
- **Decision**: Use v1.15.2 (last stable before transition)

### Traefik v3.5 Status (Why Latest)
- **Source**: GitHub releases, issue tracker
- **Findings**:
  - v3.5.3 (Sep 2025) is latest stable with bug fixes
  - One HTTPS edge case reported (not affecting basic usage)
  - Actively maintained, receiving patches
  - v3 is mature (not bleeding edge anymore)
- **Impact**: We use basic ingress routing, not advanced features
- **Decision**: Use v3.5.3 (latest stable)

### Registry v3.0 Status (Why Not Latest)
- **Source**: Docker Hub, CNCF Distribution docs
- **Findings**:
  - v3.0 exists and documented
  - v2.8.3 still default in many production deployments
  - For local use without TLS/auth, v2.8.3 proven safe
- **Impact**: Local registry for image storage, no external access
- **Decision**: Use v2.8.3 (proven for local use)

### BuildKit v0.24+ Issues (Why Not Latest)
- **Source**: GitHub issues (moby/buildkit, moby/moby)
- **Findings**:
  - High CPU and memory issues reported after v0.24.0 upgrade (June 2025)
  - 100% CPU usage during builds
  - Unresponsiveness and build failures
  - Some issues persist in v0.25.x
- **Impact**: Directly affects our use case (building workspace images)
- **Decision**: Use v0.17.3 (last known stable without CPU issues)

## Consequences

### Positive
- **Faster MVP delivery**: Less time debugging infrastructure issues
- **Stable foundation**: Developers can focus on workspace features, not cluster problems
- **Proven reliability**: All versions have track record in production
- **Clear upgrade path**: Can evaluate newer versions after MVP validation

### Negative
- **Not "cutting edge"**: Missing some newest features in Knative and BuildKit
- **Future upgrades needed**: Will need to update in Phase 2 or Phase 3
- **Potential FOMO**: Community focus shifting to newer versions

### Neutral
- **Upgrade strategy needed**: Phase 2 should evaluate:
  - Knative v1.19+ (when OTel issues resolved)
  - BuildKit v0.24+ (when CPU issues resolved)
  - Registry v3.0 (when adoption increases)

## Compliance

- All selected versions are officially released stable versions
- All versions are compatible with Kubernetes 1.27+ (our minimum)
- All versions are currently supported by their maintainers
- No security vulnerabilities in selected versions (as of 2025-10-13)

## References

- [Knative Serving Releases](https://github.com/knative/serving/releases)
- [Knative v1.19 Issues](https://github.com/knative/serving/issues)
- [Traefik Releases](https://github.com/traefik/traefik/releases)
- [BuildKit Issues (CPU)](https://github.com/moby/buildkit/issues/4942)
- [BuildKit Issues (Docker 28.2.0)](https://github.com/moby/moby/issues/50132)
- [CNCF Distribution Docs](https://distribution.github.io/distribution/)

## Future Work

### Phase 2: Evaluate Upgrades
- Monitor Knative v1.19.x patch releases for stability
- Track BuildKit v0.24+ CPU issue resolution
- Assess Registry v3.0 production adoption
- Consider Traefik v3.6+ when available

### Testing Before Upgrade
- Local k3s testing with each new version
- Load testing (especially BuildKit image builds)
- Knative scale-to-zero testing (Phase 2 feature)
- Compatibility testing with existing workspaces

### Upgrade Strategy
- **When to upgrade**: After MVP validation, before Phase 2 features
- **How to upgrade**: One component at a time, test thoroughly
- **Rollback plan**: Keep old manifests for quick rollback
- **Communication**: Update SPEC.md, CLAUDE.md, and this ADR

---

**Approved**: 2025-10-13
**Next Review**: After MVP completion (Week 3), before Phase 2

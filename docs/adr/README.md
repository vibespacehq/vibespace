# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for vibespace.

## What is an ADR?

An ADR is a document that captures an important architectural decision made along with its context and consequences.

## ADR Index

### Active ADRs

| ADR | Title | Status |
|-----|-------|--------|
| [0002](../../adr/0002-jsdoc-for-future-exports.md) | JSDoc for Future Exports | Accepted |
| [0003](../../adr/0003-frontend-organization.md) | Frontend Organization | Accepted |
| [0004](../../adr/0004-component-version-selection.md) | Component Version Selection | Accepted |
| [0005](../../adr/0005-buildkit-for-image-building.md) | BuildKit for Image Building | Accepted (needs update) |
| [0006](../../adr/0006-bundled-kubernetes-runtime.md) | Bundled Kubernetes Runtime | Accepted |
| [0008](../../adr/0008-https-certificate-management.md) | HTTPS Certificate Management | Proposed (Post-MVP) |
| [0012](../../adr/0012-ghcr-prebuilt-images.md) | GHCR Pre-built Images | Accepted (needs update) |
| [0013](0013-multi-claude-architecture.md) | Multi-Claude Architecture | Accepted |
| [0014](0014-nats-messaging.md) | NATS for Real-Time Messaging | Accepted |
| [0015](0015-dynamic-port-exposure.md) | Dynamic Port Exposure | Accepted |

### Superseded ADRs

| ADR | Title | Superseded By |
|-----|-------|---------------|
| [0001](../../adr/0001-detection-over-bundling.md) | Detection Over Bundling | 0006 (Bundled Kubernetes) |
| [0007](../../adr/0007-dns-resolution-for-local-vibespaces.md) | DNS Resolution for Local Vibespaces | 0015 (Dynamic Port Exposure) |
| [0009](../../adr/0009-single-port-caddy-routing.md) | Single-Port Caddy Routing | 0015 (Dynamic Port Exposure) |

### Notes on Updates Needed

- **0005 (BuildKit)**: Update to reflect single image build instead of 12 template images
- **0012 (GHCR)**: Update to reflect single image workflow instead of 12 images

## Creating a New ADR

Use this template:

```markdown
# ADR XXXX: Title

**Status**: Proposed | Accepted | Deprecated | Superseded
**Date**: YYYY-MM-DD

## Context

What is the issue that we're seeing that is motivating this decision?

## Decision

What is the change that we're proposing?

## Consequences

What becomes easier or more difficult after this change?
```

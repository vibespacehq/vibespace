# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) for the Workspace project.

## What are ADRs?

ADRs document significant architectural decisions made in the project. They capture:
- **Context**: Why we needed to make a decision
- **Decision**: What we decided to do
- **Consequences**: What happens as a result (good and bad)

ADRs are immutable once accepted. If a decision changes, create a new ADR superseding the old one.

## Format

Each ADR follows this structure:

```markdown
# ADR XXXX: [Short Title]

**Date**: YYYY-MM-DD
**Status**: [Proposed | Accepted | Superseded by ADR-YYYY | Deprecated]

## Context
[Why we need to make a decision]

## Decision
[What we decided to do]

## Consequences
[What happens as a result - positive and negative]

## References
[Links to related docs, issues, PRs]
```

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [0001](0001-detection-over-bundling.md) | Use Detection Over Bundling for Kubernetes (MVP) | Accepted | 2025-01-08 |

## When to Write an ADR

Write an ADR when making decisions about:
- ✅ **Architecture changes**: Database choice, deployment model, tech stack
- ✅ **Infrastructure decisions**: Kubernetes vs Docker, bundling vs detection
- ✅ **Security approaches**: Authentication, encryption, credential storage
- ✅ **API design**: REST vs GraphQL, versioning strategy
- ✅ **Major UX flows**: Onboarding, workspace creation, error handling

Don't write an ADR for:
- ❌ **Implementation details**: Variable names, file structure, code style
- ❌ **Minor changes**: Bug fixes, small refactors, dependency updates
- ❌ **Obvious choices**: Using TypeScript (already in stack), standard patterns

## Contributing

To propose a new ADR:

1. Create a new file: `docs/adr/XXXX-short-title.md`
2. Use next available number (current: 0002)
3. Start with `**Status**: Proposed`
4. Open PR for discussion
5. Update status to `Accepted` when merged
6. Add entry to index above

## Further Reading

- [Architecture Decision Records (ADRs) by Michael Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
- [ADR GitHub Organization](https://adr.github.io/)
- [Example ADRs from other projects](https://github.com/joelparkerhenderson/architecture-decision-record)

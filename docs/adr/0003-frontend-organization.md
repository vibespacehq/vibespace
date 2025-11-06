# ADR 0003: Frontend Component and Style Organization

**Date**: 2025-10-11

**Status**: Accepted

---

## Context

The initial frontend structure co-located component files (.tsx) with their styles (.css) in the same directory:

```
src/components/
├── setup/
│   ├── AuthenticationSetup.tsx
│   ├── AuthenticationSetup.css
│   ├── KubernetesSetup.tsx
│   ├── KubernetesSetup.css
│   └── ...
└── vibespace/
    ├── VibespaceList.tsx
    └── VibespaceList.css
```

### The Problem

While co-location is a common pattern in modern React development, this structure had several issues for our specific project:

1. **Visual Clutter**: Mixed file types made it harder to scan for specific components or styles
2. **Cognitive Load**: Developers had to mentally filter between code files and style files
3. **Unclear Separation**: Hard to distinguish feature-level shared styles from component-specific styles
4. **Naming Ambiguity**: Generic filenames like `setup-wizard.css` didn't clearly indicate their purpose

### Options Considered

#### Option 1: Keep Co-location (Industry Standard)
```
src/components/setup/
├── AuthenticationSetup.tsx
├── AuthenticationSetup.css
├── KubernetesSetup.tsx
├── KubernetesSetup.css
```

**Benefits**:
- ✅ Industry standard pattern
- ✅ Related files close together
- ✅ Easy to find component's styles

**Problems**:
- ❌ Visual clutter with mixed file types
- ❌ Hard to distinguish shared vs component-specific styles
- ❌ Doesn't scale well with many components

#### Option 2: Separate Top-Level Directories
```
src/
├── components/
│   └── setup/
│       ├── AuthenticationSetup.tsx
│       └── KubernetesSetup.tsx
└── styles/
    └── setup/
        ├── AuthenticationSetup.css
        └── KubernetesSetup.css
```

**Problems**:
- ❌ Related files far apart (different top-level directories)
- ❌ Feature boundaries unclear
- ❌ Hard to move/refactor features as units

#### Option 3: Subdirectories Within Features (SELECTED)
```
src/components/
├── setup/
│   ├── components/
│   │   ├── AuthenticationSetup.tsx
│   │   └── KubernetesSetup.tsx
│   └── styles/
│       ├── setup.css              (shared feature styles)
│       ├── AuthenticationSetup.css
│       └── KubernetesSetup.css
└── vibespace/
    ├── components/
    │   └── VibespaceList.tsx
    └── styles/
        └── VibespaceList.css
```

**Benefits**:
- ✅ Clear visual separation between code and styles
- ✅ Easy to distinguish shared vs component-specific styles
- ✅ Features remain co-located and movable
- ✅ Scalable as component count grows

---

## Decision

Use **feature-based directories with separate `components/` and `styles/` subdirectories**.

### Directory Structure

```
src/
├── components/
│   ├── shared/                    # Cross-feature shared components
│   │   ├── TitleBar.tsx
│   │   └── TitleBar.css
│   ├── setup/                     # Feature: Setup wizard
│   │   ├── components/            # Setup components
│   │   │   ├── AuthenticationSetup.tsx
│   │   │   ├── ConfigurationSetup.tsx
│   │   │   ├── InstallationInstructions.tsx
│   │   │   ├── KubernetesSetup.tsx
│   │   │   ├── ProgressSidebar.tsx
│   │   │   └── ReadySetup.tsx
│   │   └── styles/                # Setup styles
│   │       ├── setup.css          # Shared feature styles (layout, containers, states)
│   │       ├── AuthenticationSetup.css
│   │       ├── ConfigurationSetup.css
│   │       ├── InstallationInstructions.css
│   │       ├── ProgressSidebar.css
│   │       └── ReadySetup.css
│   └── vibespace/                 # Feature: Vibespace management
│       ├── components/
│       │   └── VibespaceList.tsx
│       └── styles/
│           └── VibespaceList.css
└── styles/                        # Global design system
    ├── tokens.css                 # CSS variables
    ├── animations.css             # Keyframe animations
    ├── base.css                   # Base styles & resets
    └── utilities.css              # Reusable UI utilities
```

### Naming Conventions

**Directories**:
- ✅ `lowercase` or `kebab-case` (e.g., `shared`, `setup`, `components`, `styles`)
- ❌ `PascalCase` (can cause issues on case-insensitive filesystems)

**Component Files**:
- ✅ `PascalCase.tsx` (e.g., `AuthenticationSetup.tsx`, `TitleBar.tsx`)
- Matches component name exactly

**Style Files**:
- ✅ `PascalCase.css` for component-specific styles (e.g., `AuthenticationSetup.css`)
- ✅ `kebab-case.css` for feature-level shared styles (e.g., `setup.css`)
- Named to match the component or feature they style

**Import Paths**:
```typescript
// Component importing its own styles
import '../styles/AuthenticationSetup.css';

// Component importing feature-level shared styles
import '../styles/setup.css';

// Component importing global utilities
import '../../../styles/utilities.css';  // (if needed, usually in index.css)
```

### Style Organization Hierarchy

1. **Global** (`src/styles/`): Design tokens, animations, base resets, reusable utilities
2. **Feature-level** (`src/components/[feature]/styles/[feature].css`): Shared layout, containers, states within a feature
3. **Component-specific** (`src/components/[feature]/styles/[Component].css`): Styles unique to a single component

Example: Setup wizard styles
- `src/styles/utilities.css` - `.btn-primary`, `.spinner`, `.error-message` (used across all features)
- `src/components/setup/styles/setup.css` - `.setup-container`, `.setup-sidebar`, `.progress-bar-*` (shared across setup screens)
- `src/components/setup/styles/AuthenticationSetup.css` - `.auth-container`, `.auth-methods` (unique to AuthenticationSetup)

---

## Consequences

### Positive

1. **Clear Separation of Concerns**: Code and styles visually separated, easier to scan
2. **Scalability**: Structure scales well as component and style count grows
3. **Feature Cohesion**: Features remain co-located and easy to move/refactor as units
4. **Shared Styles Explicit**: Feature-level shared styles (e.g., `setup.css`) clearly distinguished from component-specific styles
5. **Better Naming**: Feature-level styles can use descriptive names matching the feature (e.g., `setup.css` instead of `setup-wizard.css`)
6. **Reduced Cognitive Load**: Developers can focus on either components or styles at a time

### Negative

1. **Additional Directory Nesting**: Imports require `../` navigation (e.g., `../styles/Component.css`)
2. **More Directories**: Adds `components/` and `styles/` subdirectories to each feature
3. **Differs from Industry Standard**: Most React projects use co-location, may surprise new contributors
4. **Refactor Cost**: Required moving and updating imports for all existing components

### Neutral

1. **File Count Unchanged**: Same number of files, just reorganized
2. **Bundle Size Unchanged**: CSS imports work the same regardless of file location
3. **IDE Navigation**: Modern IDEs handle both patterns equally well

### Mitigation Strategies

**For import path complexity**:
- Use TypeScript path aliases if imports become unwieldy (e.g., `@styles/setup.css`)
- Keep nesting shallow (max 2-3 levels)

**For contributor onboarding**:
- Document structure clearly in `.claude/CLAUDE.md`
- Add comments to index.css explaining import order
- Include structure diagram in contribution guide

**For refactor costs**:
- One-time cost, completed in this ADR
- TypeScript compiler and Knip verify all imports correct

---

## References

- [Issue #14](https://github.com/yourusername/vibespace/issues/14) - Kubernetes detection & setup refactor
- [SPEC.md](../../SPEC.md) - Design system and component structure
- [.claude/CLAUDE.md](../../.claude/CLAUDE.md) - AI assistant guidelines (to be updated)
- Commit: `refactor(#14): reorganize components into components/ and styles/ subdirectories`
- Commit: `refactor(#14): extract generic UI utilities from setup-wizard.css`
- Commit: `refactor(#14): rename setup-wizard.css to setup.css`

---

## Review History

- **2025-10-11**: Initial decision (accepted)
- **Next review**: After Phase 2 implementation (assess scalability)

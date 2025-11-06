# ADR 0002: Use JSDoc Tags for Future Exports

**Date**: 2025-10-10

**Status**: Accepted

---

## Context

During MVP development (Phase 1), we're implementing infrastructure and types that won't be fully utilized until later phases (Phase 2+). Examples include:

- Kubernetes detection functions (`checkKubectl`, `findKubeconfig`, `checkClusterHealth`, `detectInstallType`, `getClusterVersion`) - planned for Phase 2 vibespace health monitoring
- Type definitions (`KubernetesInstallType`, `Vibespace`, `CreateVibespaceRequest`, `Template`, `Credential`, `CredentialData`, `SshKeyPair`) - planned for Phase 1-2 CRUD operations
- Utility functions that support future features defined in SPEC.md and ROADMAP.md

### The Problem

Our dead code detection tool (Knip v5) flags these as unused exports, creating a dilemma:

1. **Remove them**: Lose the planned API surface, have to recreate later
2. **Ignore Knip warnings**: Defeats the purpose of dead code detection
3. **Add to `ignoreExports` config**: Doesn't scale, loses context

Additionally, **Knip v5 deprecated the `ignoreExports` configuration option**, forcing us to find an alternative approach.

### Options Considered

#### Option 1: Use `ignoreExports` in Knip config (DEPRECATED)
```typescript
export default {
  ignoreExports: [
    'checkKubectl',
    'findKubeconfig',
    // ... 20+ more exports
  ],
}
```

**Problems**:
- ❌ Deprecated in Knip v5
- ❌ Loses context (why is this ignored?)
- ❌ Config file becomes unwieldy
- ❌ Easy to forget to remove when export is used

#### Option 2: Use JSDoc `@public` tags
```typescript
/**
 * Checks if kubectl is available in the system PATH.
 *
 * @public
 * @returns Promise resolving to true if kubectl is found
 * @see SPEC.md Section 4.3.1
 */
export async function checkKubectl(): Promise<boolean> {
  return await invoke<boolean>('check_kubectl');
}
```

**Benefits**:
- ✅ Works with Knip v5 recommended approach
- ✅ Self-documenting code
- ✅ IDE support (hover tooltips, autocomplete)
- ✅ AI-friendly (clear intent)
- ✅ No config bloat

#### Option 3: Use inline comments
```typescript
// @public - Will be used in Phase 2
export function checkKubectl() { ... }
```

**Problems**:
- ❌ No IDE support
- ❌ No structured documentation
- ❌ Less AI-friendly

#### Option 4: Only implement when needed (YAGNI)

**Problems**:
- ❌ This is **planned API surface**, not speculation
- ❌ Defined in SPEC.md and ROADMAP.md
- ❌ Type definitions needed for TypeScript type safety now
- ❌ Detection functions already referenced in UI components

---

## Decision

Use **JSDoc comments with `@public` tags** to document and preserve future exports.

### Implementation

**For functions**:
```typescript
/**
 * Brief one-line description of what this does.
 * Additional context about when/how it will be used.
 *
 * @public
 * @param paramName - Description of parameter
 * @returns Description of return value
 * @see Issue #X or SPEC.md Section Y
 * @example
 * ```ts
 * const result = await myFunction();
 * console.log(result);
 * ```
 */
export async function myFunction(): Promise<Result> {
  // implementation
}
```

**For types**:
```typescript
/**
 * Brief description of what this type represents.
 * Context about its purpose in the system architecture.
 *
 * @public
 * @see SPEC.md Section X for details
 */
export interface MyType {
  id: string;
  name: string;
}
```

### Configuration

Knip v5 respects JSDoc tags by default. No additional configuration needed beyond:

```typescript
// knip.config.ts
export default {
  ignoreExportsUsedInFile: {
    interface: true,
    type: true,
  },
}
```

### Guidelines

**When to use `@public` JSDoc**:
- ✅ Exports planned for upcoming phases (documented in SPEC.md/ROADMAP.md)
- ✅ Public API functions/types intended for external use
- ✅ Library code meant to be imported by future modules
- ❌ Speculative code ("might be useful someday")
- ❌ Actual dead code that should be removed

**Required JSDoc fields**:
1. Brief description (first line)
2. `@public` tag (tells Knip to ignore)
3. `@see` reference (link to SPEC.md or GitHub issue)
4. `@example` for functions (shows intended usage)
5. `@param` and `@returns` for functions

**When to remove `@public` tag**:
- When the export is actually used in the codebase
- When the planned feature is fully implemented
- Update the JSDoc to reflect current usage

---

## Consequences

### Positive

1. **Knip v5 compliant**: Uses recommended approach instead of deprecated config
2. **Self-documenting**: Code explains its own purpose and future usage
3. **Better IDE support**: Hover tooltips, autocomplete, go-to-definition all work
4. **AI-friendly**: Future AI coding agents understand intent without guessing
5. **Maintainability**: Clear context prevents accidental deletion
6. **Searchability**: `@public` tag is greppable (`git grep '@public'`)
7. **No config bloat**: Doesn't require maintaining long lists in knip.config.ts
8. **Standards-based**: JSDoc is a widely-used JavaScript/TypeScript convention

### Negative

1. **Upfront effort**: Requires writing documentation before implementation
2. **Maintenance overhead**: JSDoc must be updated when signatures change
3. **Discipline required**: Team must remember to add JSDoc to future exports
4. **Not enforced**: No automated check that future exports have `@public` tag (could be added to pre-commit hook later)

### Neutral

1. **Learning curve**: Team needs to learn JSDoc syntax (minimal, widely used)
2. **Review burden**: PRs adding future exports must have JSDoc reviewed

### Mitigation Strategies

**For upfront effort**:
- Create JSDoc templates in `.claude/CLAUDE.md` for copy-paste
- IDE snippets for common patterns
- Clear examples in existing code

**For maintenance overhead**:
- Pre-commit hook to validate JSDoc matches function signature (future enhancement)
- Code review checklist includes JSDoc verification

**For discipline**:
- Updated `.claude/CLAUDE.md` with AI assistant guidelines
- CI/CD check for exports without JSDoc (future enhancement)

### Implementation Examples

Added JSDoc to:
- `app/src/hooks/useKubernetesStatus.ts` - 5 detection functions
- `app/src/lib/types.ts` - 7 type definitions

Total: 12 exports documented for future use.

---

## References

- [Knip v5 Documentation - Handling Issues](https://knip.dev/guides/handling-issues)
- [JSDoc Tags Reference](https://jsdoc.app/)
- [SPEC.md Section 4.3.1](../../SPEC.md) - Kubernetes detection logic
- [ROADMAP.md Phase 2](../../ROADMAP.md) - Feature timeline
- [.claude/CLAUDE.md](../../.claude/CLAUDE.md) - AI assistant guidelines (updated)

---

## Review History

- **2025-10-10**: Initial decision (accepted)
- **Next review**: After Phase 2 implementation (when exports are actually used)

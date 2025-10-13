# workspaces - AI Assistant Context

**Project**: workspaces - containerized dev environments with AI coding agent support
**Status**: MVP Development
**Stack**: Tauri + React + Go + k3s + Knative

---

## What This Project Does

workspaces is a Tauri desktop app that manages isolated dev environments running as containers in k3s. Each workspace includes code-server (VS Code in browser) and supports AI coding agents (Claude Code, OpenAI Codex).

**Deployment Modes**:
- **Local**: Everything runs on your machine (default)
- **Cloud**: Desktop app local, workspaces run in cloud (AWS/GCP/DigitalOcean)
- **Hybrid**: Mix of local and cloud workspaces

Think: Docker Desktop meets VS Code Remote meets Vercel, optimized for AI-assisted development.

**Key Features**:
- 🚀 Local or cloud workspaces
- 🤖 AI agent integration (Claude Code, OpenAI Codex)
- 🔒 TLS certificates via Let's Encrypt (cloud mode)
- 🌐 Custom domain support (`myproject.example.com`)
- 📦 Template-based (Next.js, Vue, Jupyter, custom)
- ⚡ Scale-to-zero with Knative
- 🔐 Secure credential management

---

## Repository Structure

```
workspace/
├── app/                    # Tauri desktop application
│   ├── src-tauri/         # Rust backend (Tauri)
│   └── src/               # React frontend (TypeScript)
│       ├── components/    # UI components
│       ├── hooks/         # React hooks
│       └── lib/           # Utilities
├── api/                   # Go API server
│   ├── cmd/server/        # Entry point
│   └── pkg/               # Business logic
│       ├── handler/       # HTTP handlers
│       ├── workspace/     # Workspace management
│       ├── template/      # Template building
│       ├── credential/    # Credential management
│       ├── k3s/          # Kubernetes client
│       └── model/         # Data models
├── images/                # Container image Dockerfiles
│   ├── base/             # Base image (code-server)
│   └── templates/        # Template images (Next.js, Vue, etc.)
├── k8s/                   # Kubernetes manifests
│   ├── registry.yaml     # Local registry
│   ├── buildkit.yaml     # BuildKit daemon
│   └── traefik.yaml      # Ingress controller
├── script/                # Utility scripts
│   └── install_k3s.sh    # k3s setup script
└── docs/                  # Documentation
```

---

## Key Concepts

### 1. Workspace
A containerized development environment running in k3s. Contains:
- code-server (VS Code in browser)
- Project files (persistent volume)
- AI agent credentials (from app-managed secrets)
- Template-specific tools (Node.js, Python, etc.)

**Lifecycle**: Creating → Starting → Running → Stopping → Stopped → Deleted

### 2. Template
A Docker image definition for a workspace type. Examples:
- `nextjs`: Node.js + Next.js + pnpm
- `vue`: Node.js + Vue 3 + Vite
- `jupyter`: Python + Jupyter Lab

Users can create custom templates via BuildKit.

### 3. Credential
Encrypted secrets managed by the app (stored in `~/.workspaces/credential/`):
- AI agent API keys (Claude, OpenAI)
- Git config (name, email)
- SSH keys (generated or imported)

Injected into workspaces as Kubernetes Secrets.

### 4. Knative Service
Workspaces run as Knative Services for auto-scaling (scale-to-zero when idle).

---

## Naming Conventions

### Code Style
- **Go**: Standard Go conventions (singular package names: `workspace`, `template`)
- **TypeScript**: camelCase for variables, PascalCase for components
- **Files**: kebab-case for non-component files, PascalCase for React components

### Kubernetes
- **Namespace**: `workspace` (singular)
- **Labels**: `workspace.dev/id`, `workspace.dev/template`
- **Resources**: `workspace-{id}`, `workspace-{id}-pvc`, `workspace-{id}-secrets`

### Domains
**Local Mode**:
- Code server: `workspace-{id}.local`
- App ports: `workspace-{id}-3000.local`, `workspace-{id}-8000.local`

**Cloud Mode**:
- Default: `workspace-{id}.yourdomain.com`
- Custom: `myproject.example.com` (with TLS)
- Automatic DNS configuration via Cloudflare/Route53/etc.

### API
- **Endpoints**: `/api/v1/workspaces`, `/api/v1/templates`, `/api/v1/credentials`
- **Methods**: Standard REST (GET, POST, PUT, DELETE)

---

## Design System

**Philosophy**: Nerdy but smooth - terminal-inspired with vibrant accents and modern geometric typography.

**Colors**: Pure black (#000000) + 4 vibrant accents
- **Teal** (#00ABAB) - Primary actions, links, completed states
- **Orange** (#FF7D4B) - Recommended badges
- **Pink** (#F102F3) - Active states, buttons, focus
- **Yellow** (#F5F50A) - Button hover, highlights

**Fonts**:
- **Space Grotesk** (UI, display) - Geometric sans-serif, unique, modern
- **JetBrains Mono** (code blocks, technical content)

**Icons**: Lucide

**Component Patterns**:
- Cards for workspaces
- Modals for creation flows
- Toast notifications for feedback
- Status badges (🟢 Running, ⚪ Stopped, etc.)
- Gradient accents for visual interest (teal→pink, orange→yellow)

See `SPEC.md` section 4.1.3 for complete design tokens.

---

## Development Workflow

### Prerequisites
- Node.js 20+
- Go 1.21+
- Rust 1.70+
- Docker (for building images)
- kubectl

### Running Locally

**Desktop App**:
```bash
cd app
npm install
npm run dev              # Starts Tauri dev server
```

**API Server**:
```bash
cd api
go run cmd/server/main.go
```

**Build Images**:
```bash
cd images/base
docker build -t workspace-base:latest .
```

### Project Commands
```bash
# Install k3s cluster
./script/install_k3s.sh

# Apply manifests
kubectl apply -f k8s/

# Build all templates
cd images
for dir in templates/*; do
  docker build -t workspace-$(basename $dir):latest $dir
done
```

---

## Common Tasks

### Adding a New Workspace Template

1. Create directory: `images/templates/mytemplate/`
2. Write `Dockerfile` based on `images/base/`
3. Add template metadata in API: `api/pkg/model/template.go`
4. Build image: `docker build -t workspace-mytemplate:latest .`
5. Push to local registry: `docker push localhost:5000/workspace-mytemplate:latest`

### Adding a New API Endpoint

1. Define handler: `api/pkg/handler/myresource.go`
2. Add service logic: `api/pkg/myresource/service.go`
3. Register route: `api/cmd/server/main.go`
4. Add TypeScript types: `app/src/lib/types.ts`
5. Create React hook: `app/src/hooks/useMyResource.ts`

### Adding a New UI Component

**Frontend Organization**: Feature-based with separate `components/` and `styles/` subdirectories.

**Structure**:
```
src/components/
├── shared/           # Cross-feature components (TitleBar, etc.)
│   ├── Component.tsx
│   └── Component.css
├── myfeature/        # New feature
│   ├── components/   # Feature components
│   │   └── MyComponent.tsx
│   └── styles/       # Feature styles
│       ├── myfeature.css    # Shared feature styles
│       └── MyComponent.css  # Component-specific styles
└── ...
```

**Steps to add a component**:

1. **Create component directory structure**:
   ```bash
   mkdir -p src/components/myfeature/components
   mkdir -p src/components/myfeature/styles
   ```

2. **Create component file**: `src/components/myfeature/components/MyComponent.tsx`
   ```typescript
   import '../styles/myfeature.css';      // Feature-level styles
   import '../styles/MyComponent.css';     // Component-specific styles

   export function MyComponent() {
     return <div className="my-component">...</div>;
   }
   ```

3. **Create styles**:
   - Component-specific: `src/components/myfeature/styles/MyComponent.css`
   - Feature-level shared: `src/components/myfeature/styles/myfeature.css`
   - Use design tokens from `SPEC.md` section 4.1.3

4. **Naming conventions**:
   - ✅ Directories: `lowercase` (e.g., `myfeature/`, `components/`, `styles/`)
   - ✅ Component files: `PascalCase.tsx` (e.g., `MyComponent.tsx`)
   - ✅ Component styles: `PascalCase.css` (e.g., `MyComponent.css`)
   - ✅ Feature styles: `kebab-case.css` (e.g., `my-feature.css`)

5. **Import paths**:
   ```typescript
   // Component importing its own styles
   import '../styles/MyComponent.css';

   // Component importing feature-level styles
   import '../styles/myfeature.css';

   // Component importing from another feature
   import '../../shared/TitleBar';
   ```

6. **Style hierarchy**:
   - **Global** (`src/styles/`): Design tokens, utilities, base resets
   - **Feature-level** (`src/components/myfeature/styles/myfeature.css`): Shared layouts/containers
   - **Component-specific** (`src/components/myfeature/styles/MyComponent.css`): Unique to one component

7. **Follow accessibility guidelines**

8. **Add to storybook if applicable**

**Example**:
```typescript
// src/components/workspace/components/WorkspaceCard.tsx
import '../styles/workspace.css';        // Feature-level
import '../styles/WorkspaceCard.css';    // Component-specific

export function WorkspaceCard({ workspace }) {
  return (
    <div className="workspace-card">
      <h3>{workspace.name}</h3>
      <span className="workspace-status">{workspace.status}</span>
    </div>
  );
}
```

See `docs/adr/0003-frontend-organization.md` for rationale and `SPEC.md` section 4.1.1 for complete structure.

---

## Architecture Decisions

### Why Knative?
Scale-to-zero saves resources. Workspaces can auto-stop when idle.

### Why BuildKit?
Kubernetes-native, no Docker daemon dependency, faster builds, better caching.

### Why Tauri?
Smaller binaries than Electron (~10MB vs ~100MB), better performance, native feel.

### Why Go for API?
Excellent k8s client library (`client-go`), fast, single binary deployment.

### Why k3s?
Lightweight k8s (<512MB RAM), perfect for local development, easy to install.

### Kubernetes Installation Strategy (MVP Decision)

**Decision**: Use **detection + guided setup** instead of bundling Kubernetes runtime.

**Why Detection Instead of Bundling?**

For **MVP** (Phase 1), we're prioritizing:
1. **Speed to market** - Ship in 3 weeks, not 11 weeks
2. **Focus on core value** - Workspace management, not cluster installation
3. **Security** - No sudo execution from app, users control their system
4. **Flexibility** - Supports k3s, Rancher Desktop, k3d, etc.
5. **Validation first** - Prove the concept before building polished installer

**How It Works (MVP)**:
```
1. App checks for kubectl/k3s on startup
2. If missing → Show platform-specific installation guide
   - macOS: "Install Rancher Desktop OR brew install k3s"
   - Linux: "curl -sfL https://get.k3s.io | sh -"
   - Windows: "Install Rancher Desktop OR use WSL2"
3. User installs via their preferred method
4. Click "Verify" → App detects cluster and enables features
```

**Target Users (MVP)**: Developer early adopters who can run `brew install k3s`

**Roadmap**:
- **Phase 1 (MVP)**: Detection + guided setup (Issue #14)
- **Phase 2**: Rancher Desktop deep integration (Issue #16)
- **Phase 3**: Full bundling with VM/k3s binaries (Issue #15)

**Why This Is the Right Approach**:
- Similar to how VS Code, Docker initially shipped, most dev tools work
- Follows startup best practices: "Ship → Learn → Polish"
- Bundling is always possible later if users demand it
- Most developers already have k3s/Docker Desktop/Rancher Desktop

**Future Enhancement**: Full zero-config bundling (VM + k3s + auto-setup) planned for v2.0 after MVP validation.

---

## Important Files

- `SPEC.md` - Complete technical specification
- `ROADMAP.md` - Product roadmap (5 phases, MVP through enterprise)
- `docs/adr/` - Architecture Decision Records (key design decisions)
- `app/src-tauri/tauri.conf.json` - Tauri configuration
- `api/config/config.yaml` - API server configuration
- `k8s/*.yaml` - Kubernetes manifests
- `images/base/Dockerfile` - Base image for all workspaces

---

## Testing Strategy

### Overview

- **Unit**: Go packages, React components, hooks
- **Integration**: API + k3s interaction
- **E2E**: Full workspace lifecycle (create → open → delete)

### Frontend Testing (Vitest + React Testing Library)

**Test Framework**: Vitest (Vite-native, fast, modern)
**Testing Library**: @testing-library/react (user-centric testing)
**Environment**: jsdom (DOM emulation in Node.js)

**Running Tests**:
```bash
cd app

# Run all tests
npm run test:frontend

# Watch mode (re-run on changes)
npm run test:frontend:watch

# UI mode (visual test runner)
npm run test:frontend:ui

# Coverage report
npm run test:frontend:coverage
```

**Configuration Files**:
- `vitest.config.ts` - Test configuration
- `src/test/setup.ts` - Global test setup (mocks, matchers)
- `knip.config.ts` - Excludes test files from dead code detection

### Writing Component Tests

**Test File Location**: Co-located with component (same directory)

**Naming Convention**: `ComponentName.test.tsx`

**Example Structure**:
```typescript
// src/components/workspace/components/WorkspaceCard.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkspaceCard } from './WorkspaceCard';

describe('WorkspaceCard', () => {
  it('renders workspace name', () => {
    render(<WorkspaceCard workspace={mockWorkspace} />);
    expect(screen.getByText('my-workspace')).toBeInTheDocument();
  });

  it('calls onOpen when Open button is clicked', async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(<WorkspaceCard workspace={mockWorkspace} onOpen={onOpen} />);

    await user.click(screen.getByText('Open'));
    expect(onOpen).toHaveBeenCalledWith('ws-1');
  });
});
```

**What to Test**:
1. **Rendering**: Component displays correct content
2. **User Interactions**: Buttons, forms, clicks work as expected
3. **State Changes**: Loading states, error states, success states
4. **Accessibility**: ARIA attributes, keyboard navigation
5. **Conditional Rendering**: Empty states, populated states

**What NOT to Test**:
- Implementation details (CSS classes, internal state)
- Third-party libraries (React, Tauri API)
- Visual appearance (use visual regression tests separately)

### Test Patterns

**User Events**:
```typescript
import userEvent from '@testing-library/user-event';

it('handles user interaction', async () => {
  const user = userEvent.setup();
  render(<MyComponent />);

  await user.click(screen.getByRole('button'));
  await user.type(screen.getByRole('textbox'), 'Hello');
});
```

**Async Operations**:
```typescript
import { waitFor } from '@testing-library/react';

it('loads data asynchronously', async () => {
  render(<MyComponent />);

  await waitFor(() => {
    expect(screen.getByText('Loaded')).toBeInTheDocument();
  });
});
```

**Mocking Functions**:
```typescript
import { vi } from 'vitest';

it('calls callback with correct arguments', () => {
  const onSubmit = vi.fn();
  render(<Form onSubmit={onSubmit} />);

  // ... trigger submit

  expect(onSubmit).toHaveBeenCalledWith({ name: 'John' });
  expect(onSubmit).toHaveBeenCalledTimes(1);
});
```

**Mocking Tauri API**:
```typescript
// Already mocked globally in src/test/setup.ts
import { getCurrentWindow } from '@tauri-apps/api/window';

it('minimizes window', async () => {
  const mockWindow = getCurrentWindow();
  render(<TitleBar />);

  await user.click(screen.getByLabelText('Minimize'));
  expect(mockWindow.minimize).toHaveBeenCalled();
});
```

### Accessibility Testing

**Check ARIA attributes**:
```typescript
it('has proper accessibility attributes', () => {
  render(<Button disabled loading />);

  const button = screen.getByRole('button');
  expect(button).toHaveAttribute('aria-busy', 'true');
  expect(button).toBeDisabled();
});
```

**Check semantic HTML**:
```typescript
it('uses semantic roles', () => {
  render(<Navigation />);

  expect(screen.getByRole('navigation')).toBeInTheDocument();
  expect(screen.getByRole('list')).toBeInTheDocument();
});
```

### Test Organization

**Group related tests**:
```typescript
describe('WorkspaceList', () => {
  describe('Empty State', () => {
    it('shows empty message', () => { /* ... */ });
    it('shows create button', () => { /* ... */ });
  });

  describe('Populated State', () => {
    it('renders workspace cards', () => { /* ... */ });
    it('shows workspace count', () => { /* ... */ });
  });
});
```

**Setup and teardown**:
```typescript
import { beforeEach, afterEach } from 'vitest';

describe('MyComponent', () => {
  let mockData: Workspace[];

  beforeEach(() => {
    mockData = [/* ... */];
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('uses mock data', () => {
    render(<MyComponent workspaces={mockData} />);
    // ...
  });
});
```

### Coverage Guidelines

**Target Coverage**: 80%+ for components
**Priority**:
1. Critical user flows (authentication, workspace creation)
2. Shared components (TitleBar, buttons, forms)
3. Complex state management
4. Error handling

**Coverage Reports**:
```bash
npm run test:frontend:coverage
open coverage/index.html
```

### CI/CD Integration

Tests run automatically on:
- Every push to main
- Every pull request
- See `.github/workflows/test.yml`

**Required**: All tests must pass before merging PR.

### Examples from Codebase

- `app/src/components/shared/TitleBar.test.tsx` - Window controls
- `app/src/components/workspace/components/WorkspaceList.test.tsx` - Empty/populated states
- `app/src/components/setup/components/AuthenticationSetup.test.tsx` - Loading states

### Backend Testing

**Go Tests**:
```bash
cd api
go test ./...
go test -v ./pkg/workspace  # Specific package
go test -cover ./...        # With coverage
```

**Rust Tests** (Tauri):
```bash
cd app/src-tauri
cargo test
cargo test --verbose
```

---

## Security Considerations

1. **Credential Encryption**: AES-256 at rest, OS keychain integration
2. **Network Isolation**: Workspaces isolated by default (NetworkPolicy)
3. **No Host Access**: Credentials injected via Kubernetes Secrets, not volume mounts
4. **Read-only Mounts**: SSH keys mounted read-only when needed
5. **Non-root Containers**: Workspaces run as UID 1000

---

## Troubleshooting

### k3s won't start
```bash
sudo systemctl status k3s
sudo journalctl -u k3s -f
```

### Workspace stuck in "Creating"
```bash
kubectl get pods -n workspace
kubectl describe pod workspace-{id} -n workspace
kubectl logs workspace-{id} -n workspace
```

### BuildKit build fails
```bash
kubectl logs -n default deployment/buildkitd
```

### Local registry not accessible
```bash
curl http://localhost:5000/v2/_catalog
kubectl get svc -n default registry
```

---

## Contributing Guidelines

1. Follow existing code structure
2. Use defined naming conventions
3. Update `SPEC.md` for architectural changes
4. Add tests for new features
5. Ensure dark theme compatibility for UI changes

---

## External Dependencies

**Core** (all modes):
- **k3s**: Lightweight Kubernetes (1.27+)
- **Knative Serving**: v1.15.2 (serverless workload management)
- **Traefik**: v3.5.3 (ingress controller)
- **Registry**: 2.8.3 (local image storage)
- **BuildKit**: v0.17.3 (container image builder)
- **code-server**: VS Code in browser

**Component Version Strategy (MVP)**:
The pragmatic mix approach balances stability for MVP delivery with modern versions. Versions selected to avoid known issues (Knative v1.19 OTel bugs, BuildKit v0.24+ CPU issues) while maintaining active support. See [ADR 0004](../docs/adr/0004-component-version-selection.md) for detailed rationale.

**Cloud Mode Only**:
- **cert-manager**: TLS certificate management (Let's Encrypt)
- **WireGuard**: Secure tunnel for remote access
- **Cloudflare/Route53**: DNS provider integration (optional)
- **Terraform/Pulumi**: Infrastructure provisioning (optional)

---

## Current Phase

**Phase 1**: Foundation (Weeks 1-2) - IN PROGRESS
- ✅ Project structure
- ✅ Specification complete
- ✅ Tauri app scaffold (PR #5, #7 merged)
- ✅ Functional tests (PR #9 merged)
- ✅ Go API server (PR #13 merged)
- ⏳ Kubernetes detection & setup guide (Issue #14)
- ⏳ Base Docker image

See `ROADMAP.md` for detailed product roadmap and `SPEC.md` section 10 for technical phases.

---

## Claude's Workflow (AI Assistant Guidelines)

This project is **open source** and all work must be **git-tracked**. Every feature, bug fix, and improvement goes through proper issue → branch → commits → PR workflow.

### Core Principles

1. **All work is scoped**: Never work without a clear scope
2. **Git is the source of truth**: Issues, PRs, commits document everything
3. **One concern per PR**: Don't mix multiple features/fixes
4. **Communicate via git**: Issues for discussion, PRs for code review
5. **Never push to main**: Always use feature branches

---

### Workflow Steps

#### 1. Before Starting Any Work

**First, check if there's an issue**:
```bash
# Search existing issues
gh issue list --search "keyword"
```

**If no issue exists, create one**:
```bash
gh issue create --title "Add custom domain support" --body "
## Description
Implement custom domain mapping for workspaces in cloud mode.

## Scope
- DNS provider integration (Cloudflare, Route53)
- Domain validation (TXT record challenge)
- Automatic DNS record creation
- cert-manager certificate provisioning
- UI for domain management

## Technical Details
- Backend: api/pkg/network/dns.go
- Frontend: app/src/components/workspace/DomainSettings.tsx
- API endpoints: POST /api/v1/workspaces/:id/domains
- See SPEC.md Section 8.4

## Acceptance Criteria
- [ ] User can add custom domain in UI
- [ ] DNS records created automatically
- [ ] TLS certificate provisioned
- [ ] Domain accessible over HTTPS
- [ ] Tests written
- [ ] Documentation updated

## Estimated Effort
~2-3 days
" --label "feature" --assignee "@me"
```

**Issue created → Note the issue number** (e.g., #42)

---

#### 2. Create Feature Branch

**Branch naming convention**:
```
feature/#<issue>-<short-description>
fix/#<issue>-<short-description>
docs/#<issue>-<short-description>
refactor/#<issue>-<short-description>
```

**Examples**:
```bash
# For feature
git checkout -b feature/#42-custom-domains

# For bug fix
git checkout -b fix/#15-workspace-creation-error

# For docs
git checkout -b docs/#7-api-documentation

# For refactor
git checkout -b refactor/#23-simplify-credential-manager
```

**Always reference the issue number in branch name**.

---

#### 3. Work in Small, Logical Commits

**Commit message format**:
```
<type>(#<issue>): <description>

[optional body]

[optional footer]
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring (no behavior change)
- `test`: Adding tests
- `chore`: Build, dependencies, tooling

**Examples**:
```bash
# Good commits
git commit -m "feat(#42): add DNS provider interface"
git commit -m "feat(#42): implement Cloudflare DNS integration"
git commit -m "feat(#42): add domain validation endpoint"
git commit -m "test(#42): add DNS provider tests"
git commit -m "docs(#42): update SPEC.md with domain management"

# Bad commits (don't do this)
git commit -m "wip"
git commit -m "fix stuff"
git commit -m "update"
```

**Commit frequently** (every logical change):
- Added a new function? Commit.
- Fixed a bug? Commit.
- Updated documentation? Commit.
- Added tests? Commit.

**Each commit should**:
- Be atomic (one logical change)
- Have a clear message
- Reference the issue number
- Be buildable (don't break CI)

---

#### 4. Push and Create Pull Request

**Push branch**:
```bash
git push origin feature/#42-custom-domains
```

**Create PR**:
```bash
gh pr create --title "feat: Add custom domain support for workspaces (#42)" --body "
## Summary
Implements custom domain mapping for workspaces in cloud mode.

## Changes
- Added DNS provider interface and implementations (Cloudflare, Route53)
- Domain validation using TXT record challenge
- Automatic DNS record creation via provider APIs
- cert-manager integration for TLS certificates
- UI for domain management in workspace settings
- API endpoints for domain CRUD operations

## Testing
- [x] Unit tests for DNS providers
- [x] Integration tests for domain validation
- [x] Manual testing with Cloudflare account
- [x] TLS certificate provisioning verified

## Documentation
- [x] Updated SPEC.md Section 8.4
- [x] Added inline code comments
- [x] Updated API documentation

## Screenshots
![Domain Management UI](./docs/images/domain-ui.png)

## Closes #42

## Checklist
- [x] Code follows style guide
- [x] Tests pass locally
- [x] Documentation updated
- [x] No breaking changes
- [x] PR title includes issue number
" --draft
```

**PR title format**: `<type>: <description> (#<issue>)`

**Start as draft PR** if still working. Mark ready when done.

---

#### 5. Respond to Review Feedback

**When changes are requested**:
```bash
# Make requested changes
# ... edit files ...

# Commit changes
git commit -m "fix(#42): address PR review comments"

# Push
git push origin feature/#42-custom-domains
```

**Don't force push** unless absolutely necessary (preserves review context).

---

#### 6. Merge PR

Once approved:
```bash
# Merge via GitHub CLI
gh pr merge 123 --squash --delete-branch

# Or via GitHub UI (preferred)
# Click "Squash and merge"
```

**Squash merge** for cleaner history (unless preserving commit history is important).

---

### Scoping Guidelines

#### What Defines Good Scope?

✅ **Good Scope** (clear, bounded, testable):
```
Issue: "Add Cloudflare DNS integration"
- Single DNS provider
- Clear acceptance criteria
- Estimatable effort (~4-6 hours)
- Testable in isolation
```

❌ **Bad Scope** (vague, unbounded):
```
Issue: "Improve networking"
- Too broad
- No acceptance criteria
- Can't estimate
- Hard to test
```

#### Breaking Down Large Features

**Before**:
```
Issue: "Add cloud deployment mode"  ❌ Too big
```

**After** (break into smaller issues):
```
Issue #50: "Add deployment mode configuration"
Issue #51: "Implement cloud provider interface"
Issue #52: "Add DigitalOcean provider"
Issue #53: "Add WireGuard tunnel setup"
Issue #54: "Add cloud mode UI toggle"
Issue #55: "Update docs for cloud deployment"
```

Each issue is **independently deliverable**.

---

### What Gets Tracked

#### ✅ Always Create Issues/PRs For:

- **New features** (no matter how small)
- **Bug fixes**
- **Refactoring** (if significant)
- **Documentation** (major updates)
- **Breaking changes**
- **Architecture changes**
- **Performance improvements**
- **Security fixes**

#### ⚠️ Optional (Use Judgment):

- **Typo fixes** (can be direct PR)
- **Minor doc updates** (can be direct PR)
- **Dependency updates** (automated)
- **CI/CD config tweaks** (can be direct PR)

#### ❌ Never Create Issues/PRs For:

- **Exploratory work** (use drafts or comments)
- **Local experiments** (keep in local branches)
- **WIP code** (wait until ready)

---

### Git Conventions

#### Branch Lifetime

```
main branch (protected)
└── feature/#42-custom-domains (your branch)
    ├── commit: "feat(#42): add DNS interface"
    ├── commit: "feat(#42): implement Cloudflare"
    ├── commit: "test(#42): add DNS tests"
    └── PR created → reviewed → merged → branch deleted
```

**Branch lifecycle**: Create → Work → PR → Review → Merge → Delete

#### Commit Hygiene

**Before pushing**, review your commits:
```bash
# Check commits
git log --oneline

# If needed, squash WIP commits
git rebase -i HEAD~5

# Clean commit history before pushing
```

#### Pull Request Size

**Target**: 200-400 lines changed per PR

**If larger**:
- Break into multiple PRs
- Stack PRs (PR #2 depends on PR #1)
- Use feature flags for partial delivery

---

### Issue Labels

Use GitHub labels to categorize:

- `feature`: New functionality
- `bug`: Something isn't working
- `docs`: Documentation changes
- `refactor`: Code cleanup
- `test`: Testing improvements
- `enhancement`: Improvement to existing feature
- `priority-high`: Critical, blocking
- `priority-medium`: Important, not blocking
- `priority-low`: Nice to have
- `good-first-issue`: Easy for newcomers
- `help-wanted`: Community input needed
- `wontfix`: Will not be addressed

---

### Example Full Workflow

```bash
# 1. Create issue
gh issue create --title "Add Jupyter template" --body "..." --label "feature"
# Issue #67 created

# 2. Create branch
git checkout -b feature/#67-jupyter-template

# 3. Work in commits
git commit -m "feat(#67): add base Jupyter Dockerfile"
git commit -m "feat(#67): configure Jupyter Lab settings"
git commit -m "feat(#67): add Python data science libraries"
git commit -m "test(#67): verify Jupyter template builds"
git commit -m "docs(#67): add Jupyter template to README"

# 4. Push and create PR
git push origin feature/#67-jupyter-template
gh pr create --title "feat: Add Jupyter template (#67)" --body "Closes #67" --draft

# 5. Mark PR ready when done
gh pr ready 67

# 6. Address review feedback (if any)
git commit -m "fix(#67): update Jupyter version per review"
git push

# 7. Merge (after approval)
gh pr merge 67 --squash --delete-branch

# 8. Clean up local
git checkout main
git pull
git branch -d feature/#67-jupyter-template
```

---

### Testing Before PR

**Always run tests before creating PR**:
```bash
# Backend tests
cd api
go test ./...

# Frontend tests
cd app
npm test

# Build check
npm run build

# Lint check
npm run lint

# Dead code check (REQUIRED)
make deadcode
```

**If tests fail**: Fix them before creating PR.

**If dead code is detected**: Remove unused code, exports, or dependencies before creating PR.

---

### Dead Code Detection

This project uses automated dead code detection to prevent code bloat and maintain quality.

**Before every commit**, run:
```bash
make deadcode
```

**What it checks**:
- Go: Unused functions, types, variables (via `deadcode` and `staticcheck`)
- TypeScript: Unused exports, components, hooks (via `knip`)
- Rust: Compiler warnings and clippy lints
- Dependencies: Unused npm/go/cargo packages

**Common issues and fixes**:
```bash
# Unused export in TypeScript
# ❌ export const unused = 'value'
# ✅ Remove the export or use it

# Unused import
# ❌ import { unused } from './module'
# ✅ Remove the import

# Unused function in Go
# ❌ func unusedFunction() {}
# ✅ Remove the function or use it

# If intentionally unused (for future use), add JSDoc @public tag:
/**
 * Checks if kubectl is available in the system PATH.
 * Will be used in Phase 2 for workspace health monitoring.
 *
 * @public
 * @see Issue #14
 */
export async function checkKubectl(): Promise<boolean> {
  // implementation
}
```

**CI/CD**: Dead code checks run automatically on all PRs. PRs failing this check cannot be merged.

**Tools configuration**:
- TypeScript: `app/knip.config.ts`
- Go: `api/.staticcheck.conf`
- Commands: `Makefile` (root)

---

### JSDoc Documentation Standards

This project uses **JSDoc comments with `@public` tags** to document exports that are intentionally unused but planned for future phases.

**Why JSDoc instead of inline comments?**
- Knip v5 recognizes `@public`, `@internal`, and other JSDoc tags
- Provides better IDE support (hover tooltips, autocomplete)
- Self-documenting code that explains purpose and usage
- Prevents accidental deletion of planned exports
- Makes it clear to future developers and AI assistants what's intentional

**When to use JSDoc `@public` tag**:
1. **Future features**: Exports planned for upcoming phases
2. **Public APIs**: Functions/types intended for external use
3. **Library code**: Utilities meant to be imported elsewhere

**JSDoc template for future exports**:
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

**JSDoc template for future types**:
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

**Required JSDoc fields**:
- Brief description (first line)
- `@public` tag (so Knip ignores it)
- `@see` reference to issue or SPEC.md section
- `@example` for functions (shows intended usage)

**Best practices**:
- ✅ Write JSDoc **before** implementing the feature
- ✅ Update JSDoc when implementation changes
- ✅ Remove JSDoc `@public` tag once code is actually used
- ✅ Reference specific SPEC.md sections or GitHub issues
- ✅ Include usage examples for non-trivial functions
- ❌ Don't add JSDoc to everything (only future/public exports)
- ❌ Don't write vague descriptions ("Does stuff")
- ❌ Don't forget to update JSDoc when refactoring

**AI Assistant Guidelines**:
- When you see `@public` JSDoc, **respect it** and don't suggest removing the export
- When adding new future exports, **always add JSDoc** with `@public` tag
- When implementing a feature, **update or remove** the `@public` tag as appropriate
- When refactoring, **check JSDoc** and update if the signature or purpose changed

**Examples from this codebase**:
- `app/src/hooks/useKubernetesStatus.ts` - Detection functions for Phase 2
- `app/src/lib/types.ts` - Workspace, Template, Credential types for Phase 1-2

See `docs/adr/003-jsdoc-for-future-exports.md` for the architectural decision record.

---

### Communication Best Practices

#### In Issues
- Be specific about the problem/feature
- Include reproduction steps (for bugs)
- Reference related issues/PRs
- Add screenshots/videos if helpful
- Tag relevant people with @mention

#### In PRs
- Explain **why** not just **what**
- Call out areas needing special attention
- Add testing instructions
- Link to issue with "Closes #X"
- Respond to review comments promptly

#### In Commits
- Write clear, descriptive messages
- Explain **why** in commit body (if not obvious)
- Reference issue number
- Keep subject line under 72 characters

---

### When to Ask for Help

**Create a discussion** (not issue) if:
- You're unsure about approach
- You need architectural guidance
- You want community input
- You're blocked on external dependency

```bash
gh discussion create --title "Best approach for multi-cloud support?" --body "..."
```

---

### Anti-Patterns (Don't Do This)

❌ **Pushing directly to main**
```bash
git push origin main  # NEVER DO THIS
```

❌ **Large monolithic PRs**
```
PR: "Implement entire cloud deployment mode"
Files changed: 3,847 lines  # Too big
```

❌ **Vague commit messages**
```bash
git commit -m "fix"
git commit -m "wip"
git commit -m "updates"
```

❌ **Working without issue**
```
# Starting work without creating/referencing issue
git checkout -b add-some-feature  # Missing issue reference
```

❌ **Mixing concerns**
```
PR: "Add custom domains and fix bug and update docs"
# Should be 3 separate PRs
```

---

### Summary Checklist

Before starting work:
- [ ] Issue exists (create if needed)
- [ ] Issue is scoped and clear
- [ ] Branch created with proper name
- [ ] Reference issue in branch name

While working:
- [ ] Commit frequently
- [ ] Clear commit messages
- [ ] Reference issue in commits
- [ ] Tests pass locally

Before creating PR:
- [ ] All tests pass
- [ ] Code follows style guide
- [ ] Documentation updated
- [ ] Commits are clean
- [ ] PR description is complete

After PR created:
- [ ] Responds to review feedback
- [ ] Keeps PR updated
- [ ] Merges when approved
- [ ] Deletes branch after merge

---

## Getting Help

- Read `SPEC.md` for detailed architecture
- Check `docs/` for guides
- Review existing code patterns before implementing new features
- Ask questions about design decisions, not just code

---

**Last Updated**: 2025-10-07

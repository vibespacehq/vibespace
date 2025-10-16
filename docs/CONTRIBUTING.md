# Contributing to Workspace

Thank you for your interest in contributing! This document provides guidelines for contributing to the project.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Getting Help](#getting-help)

---

## Getting Started

### Prerequisites

- **Node.js** 20+ and npm/pnpm
- **Go** 1.21+
- **Rust** 1.70+ (for Tauri)
- **Docker** (for building images)
- **kubectl** (for k8s interaction)
- **Git** and **GitHub CLI** (`gh`) recommended

### Setup

1. **Fork and clone the repository**:
   ```bash
   gh repo fork yagizdagabak/workspaces --clone
   cd workspaces
   ```

2. **Install dependencies**:
   ```bash
   # Frontend
   cd app
   npm install

   # Backend
   cd ../api
   go mod download
   ```

3. **Install k3s** (optional for local testing):
   ```bash
   ./script/install_k3s.sh
   ```

4. **Run the app**:
   ```bash
   # Frontend (in app/)
   npm run dev

   # Backend (in api/)
   go run cmd/server/main.go
   ```

---

## Development Workflow

We follow a **git-tracked workflow** where all work is documented through issues and pull requests.

### 1. Find or Create an Issue

**Search existing issues first**:
```bash
gh issue list --search "keyword"
```

**If no issue exists, create one**:
```bash
gh issue create --title "Your feature/fix" --label "feature"
```

**Good issue includes**:
- Clear description of the problem/feature
- Scope and acceptance criteria
- Reference to SPEC.md sections if applicable
- Estimated effort

### 2. Create a Feature Branch

**Branch naming**:
```
feature/#<issue>-<short-description>
fix/#<issue>-<bug-description>
docs/#<issue>-<doc-type>
refactor/#<issue>-<refactor-area>
```

**Example**:
```bash
git checkout -b feature/#42-custom-domains
```

### 3. Make Your Changes

- **Work in small, logical commits**
- **Reference the issue number** in each commit
- **Keep commits atomic** (one change per commit)
- **Test locally** before pushing

### 4. Push and Create Pull Request

```bash
git push origin feature/#42-custom-domains
gh pr create --title "feat: Add custom domain support (#42)"
```

**PR should include**:
- Summary of changes
- Link to issue (`Closes #42`)
- Testing done
- Screenshots (if UI changes)
- Documentation updates

### 5. Address Review Feedback

- Respond to comments promptly
- Make requested changes
- Push updates to the same branch
- Request re-review when ready

### 6. Merge

Once approved:
- Maintainer will merge using **squash merge**
- Branch will be automatically deleted
- Update your local repository:
  ```bash
  git checkout main
  git pull
  ```

---

## Code Standards

### Go (Backend)

**Style**:
- Follow standard Go conventions
- Use `gofmt` and `golint`
- Package names: singular (`workspace`, not `workspaces`)
- Exported functions: documented with comments

**Structure**:
```go
// pkg/workspace/service.go
package workspace

// CreateWorkspace creates a new workspace with the given configuration.
// It returns the workspace ID or an error if creation fails.
func CreateWorkspace(config Config) (string, error) {
    // Implementation
}
```

### TypeScript/React (Frontend)

**Style**:
- camelCase for variables/functions
- PascalCase for components
- Functional components with hooks
- Use TypeScript strict mode

**Component structure**:
```tsx
// components/workspace/WorkspaceCard.tsx
interface WorkspaceCardProps {
  workspace: Workspace
  onOpen: (id: string) => void
}

export function WorkspaceCard({ workspace, onOpen }: WorkspaceCardProps) {
  // Implementation
}
```

### Design System

- Use design tokens from `SPEC.md` Section 4.1.3
- Follow dark theme colors
- Use Tailwind utility classes
- Lucide icons only

---

## Dead Code Detection

We use automated tools to detect unused code, exports, and dependencies across the codebase. This keeps the project maintainable and prevents code bloat.

### Tools Used

- **Go**: `deadcode` and `staticcheck`
- **TypeScript**: `knip` (unused exports, files)
- **Rust**: `cargo clippy`
- **Dependencies**: `depcheck` (npm), `go mod tidy`, `cargo udeps`

### Running Checks

**Quick check (all languages)**:
```bash
make deadcode
```

**Individual checks**:
```bash
# TypeScript
cd app
npm run deadcode

# Go
cd api
deadcode ./...

# Rust
cd app/src-tauri
cargo clippy -- -D warnings
```

**Check unused dependencies**:
```bash
make check-unused
```

### Before Submitting PR

Always run dead code detection before creating a PR:

```bash
make deadcode
```

Fix any issues reported. Common issues:
- **Unused exports**: Remove or use the export
- **Unused imports**: Remove the import
- **Unused functions**: Remove or mark as intentionally unused with comments
- **Unused dependencies**: Remove from package.json/go.mod/Cargo.toml

### CI/CD Integration

Dead code checks run automatically on all PRs via GitHub Actions. PRs with dead code issues will fail the check.

### Installing Tools

```bash
# Install all tools at once
make install-tools

# Or install individually:
go install golang.org/x/tools/cmd/deadcode@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
cd app && npm install  # Installs knip, depcheck, ts-prune
```

### Configuration

- **TypeScript**: `app/knip.config.ts`
- **Go**: `api/.staticcheck.conf`
- **Makefile**: `Makefile` (root)

---

## Commit Guidelines

### Format

```
<type>(#<issue>): <description>

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `refactor`: Code refactoring
- `test`: Adding/updating tests
- `chore`: Build, deps, tooling

### Examples

**Good commits**:
```bash
feat(#42): add Cloudflare DNS provider
fix(#15): resolve workspace creation timeout
docs(#7): update API endpoint documentation
test(#42): add DNS provider unit tests
```

**Bad commits** (avoid):
```bash
fix stuff
wip
updates
```

### Rules

- **Reference issue number** in every commit
- **Be descriptive** but concise
- **Subject line**: 50 chars or less
- **Body**: Explain *why*, not *what*
- **One logical change** per commit

---

## Pull Request Process

### Before Creating PR

**Checklist**:
- [ ] Code follows style guide
- [ ] Tests pass locally
- [ ] New tests added (if applicable)
- [ ] Documentation updated
- [ ] No console.log or debug code
- [ ] Dead code checks pass (`make deadcode`)
- [ ] Branch is up to date with main

**Run checks**:
```bash
# Backend
cd api
go test ./...
go vet ./...

# Frontend
cd app
npm test
npm run build
npm run lint
```

### PR Template

Your PR should include:

```markdown
## Summary
Brief description of changes

## Changes
- Bullet point list of changes
- Be specific

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Documentation
- [ ] Updated SPEC.md (if architecture changed)
- [ ] Updated README (if setup changed)
- [ ] Added inline comments

## Screenshots
(if UI changes)

## Closes #<issue-number>
```

### PR Size

**Target**: 200-400 lines changed

**If larger**:
- Consider breaking into multiple PRs
- Use stacked PRs (PR #2 depends on PR #1)
- Discuss with maintainers first

### Review Process

1. **Automated checks** run (lint, test, build)
2. **Maintainer review** (1-2 days typically)
3. **Address feedback** if requested
4. **Approval** -> Squash merge
5. **Branch deleted** automatically

---

## Testing

### Backend (Go)

**Unit tests**:
```go
// pkg/workspace/service_test.go
func TestCreateWorkspace(t *testing.T) {
    // Test implementation
}
```

**Run tests**:
```bash
cd api
go test ./...
go test -v ./pkg/workspace  # Verbose for specific package
```

### Frontend (React)

**Component tests**:
```tsx
// components/workspace/WorkspaceCard.test.tsx
import { render, screen } from '@testing-library/react'
import { WorkspaceCard } from './WorkspaceCard'

test('renders workspace name', () => {
  // Test implementation
})
```

**Run tests**:
```bash
cd app
npm test
npm test -- --coverage  # With coverage
```

### Integration Tests

- Test API + k3s interaction
- Use test k3s cluster
- Clean up resources after tests

---

## Project Structure

Understanding the codebase:

```
workspace/
 app/              # Tauri desktop app
    src/         # React frontend
    src-tauri/   # Rust backend
 api/              # Go API server
    cmd/         # Entry points
    pkg/         # Business logic
 images/           # Docker images
    base/        # Base workspace image
    templates/   # Template images
 k8s/              # Kubernetes manifests
 script/           # Utility scripts
 docs/             # Documentation
```

**Key files**:
- `docs/SPEC.md` - Technical specification (read this first!)
- `.claude/CLAUDE.md` - AI assistant context
- `api/config/config.yaml` - API configuration
- `app/tailwind.config.js` - Design tokens

---

## Naming Conventions

Follow the project standards:

**Go**:
- Package names: singular (`workspace`, `template`)
- Files: lowercase with underscores (`workspace_service.go`)

**TypeScript**:
- Components: PascalCase (`WorkspaceCard.tsx`)
- Utilities: camelCase (`api.ts`, `types.ts`)
- Hooks: `use` prefix (`useWorkspaces.ts`)

**Git**:
- Branches: `feature/#42-description`
- Commits: `feat(#42): description`
- PRs: `feat: Description (#42)`

See `docs/SPEC.md` Section 1.1 for complete conventions.

---

## Documentation

### When to Update Docs

**docs/SPEC.md**: Architecture or design changes
**README.md**: Setup, installation, usage changes
**Inline comments**: Complex logic, non-obvious decisions
**API docs**: New endpoints, changed responses

### Documentation Style

- **Be concise** but complete
- **Use examples** liberally
- **Keep up to date** with code
- **Link related sections**

---

## Getting Help

### Resources

- **docs/SPEC.md**: Complete technical specification
- **.claude/CLAUDE.md**: Development context
- **Issues**: Search existing issues for similar problems
- **Discussions**: For questions, ideas, architecture discussions

### Communication

**For questions**:
1. Check docs/SPEC.md first
2. Search closed issues
3. Create a discussion (not issue)
4. Tag with `question` label

**For bugs**:
1. Search existing issues
2. Create new issue with reproduction steps
3. Include environment details (OS, versions)
4. Add logs/screenshots if helpful

**For features**:
1. Check docs/ROADMAP.md for product phases and current priorities
2. Check docs/SPEC.md Section 10 for implementation milestones
3. Create discussion first (for large features)
4. Create issue with clear scope
5. Reference relevant documentation sections

---

## Code of Conduct

### Our Standards

- **Be respectful** and inclusive
- **Constructive feedback** only
- **Focus on the code**, not the person
- **Assume good intentions**
- **Help newcomers** learn

### Unacceptable Behavior

- Harassment or discrimination
- Trolling or insulting comments
- Personal or political attacks
- Publishing others' private information

**Violations**: Report to project maintainers

---

## Recognition

Contributors are recognized in:
- Git commit history (permanent record)
- GitHub contributors page
- Release notes (for significant contributions)

Thank you for contributing to Workspace!

---

## Quick Reference

### Common Commands

```bash
# Start development
npm run dev                           # Frontend
go run cmd/server/main.go            # Backend

# Testing
go test ./...                        # Backend tests
npm test                             # Frontend tests

# Build
npm run build                        # Frontend build
go build -o bin/server cmd/server/main.go  # Backend build

# Linting
npm run lint                         # Frontend lint
go vet ./...                         # Backend vet

# Dead code detection
make deadcode                        # Check all (Go, TS, Rust)
make check-unused                    # Check unused dependencies
make install-tools                   # Install detection tools

# Git workflow
gh issue create --title "..."        # Create issue
git checkout -b feature/#42-name     # Create branch
git commit -m "feat(#42): ..."       # Commit
gh pr create --title "..."           # Create PR
gh pr merge 42 --squash             # Merge PR
```

### Links

- **Repository**: https://github.com/yagizdagabak/workspaces
- **Specification**: [SPEC.md](SPEC.md)
- **Issues**: https://github.com/yagizdagabak/workspaces/issues
- **Discussions**: https://github.com/yagizdagabak/workspaces/discussions

---

**Last Updated**: 2025-10-16

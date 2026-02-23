# Testing Plan

## Philosophy

No mocks. Every test runs against real infrastructure.

Vibespace is infrastructure-heavy software — subprocess calls, Kubernetes orchestration, WireGuard tunnels, daemon IPC, DNS servers. The bugs live in the integration, not the logic. Mocking the I/O gives green tests and false confidence. A mock can confirm you called `client.Create()` with the right args, but it can't tell you the Deployment spec is wrong, the label selector doesn't match, or the kubeconfig path breaks on one platform.

Instead, we test at two levels:
1. **Pure logic tests** — call Go functions directly with real inputs, assert real outputs. No I/O, no infra needed.
2. **Real infrastructure tests** — run the actual `vibespace` binary against real k3s clusters, real WireGuard tunnels, real VMs. Same as a user would.

### When to reconsider

Add interfaces and mocks if/when:
- A second contributor can't run integration tests locally
- CI times exceed 15 minutes and need a fast-path
- CLI handlers grow complex enough to warrant isolated testing

None of these apply today.

---

## Test Tiers

### Tier 1: Pure logic (no I/O, no infra)

77 tests across 17 files. Packages: `pkg/errors`, `pkg/agent`, `pkg/vibespace` (validation), `pkg/ui`, `pkg/session`, `pkg/remote`, `pkg/dns`, `pkg/daemon/protocol`, `internal/platform`, `internal/cli` (output). All run with `-race` in ~3s.

### Tier 2: K8s service layer

10 tests across 2 files (`pkg/vibespace/service_test.go`, `pkg/deployment/manager_test.go`). CRUD against real k3s installed directly on the CI runner. Pods stay `Pending` (busybox image) but Deployment specs, labels, Services, PVCs, and Secrets are validated.

### Tier 3: Full binary lifecycle (E2E)

~37 subtests per platform, testing the entire CLI stack as a subprocess.

| Platform | Runner | Build tag |
|----------|--------|-----------|
| Bare metal (k3s) | GitHub ubuntu-latest | `e2e && linux` |
| Colima (macOS) | Self-hosted macos-m2 | `e2e && darwin` |
| Lima + QEMU | Self-hosted linux-hub | `e2e && lima` |

Subtest flow:
```
init → status → create → list → agents →
  // JSON mode
  info → config-show-all → config-show → config-set → config-verify →
  session-list → wait-for-ready →
  agent-create → agent-list-two → agent-delete → agent-list-one →
  exec → forward-list-default → forward-add → forward-list-active →
  forward-remove → multi-list-sessions → multi-list-agents →
  multi-message → stop → start →
  // Plain mode (re-run read-only commands with --plain)
  plain/list → plain/info → plain/agents → plain/config-show-all →
  plain/config-show → plain/session-list → plain/forward-list →
  plain/multi-list-sessions → plain/multi-list-agents →
delete → verify-deleted
```

---

## CI Pipeline

### Every push — `ci.yml` (3 parallel jobs)

```
ci.yml (every push)
  ├── lint          (~30s)  — go vet, staticcheck, deadcode, gofmt
  ├── unit          (~30s)  — go test -short -race (pure logic only)
  └── integration   (~2min) — k3s install + go test -race (all tests)
```

### Push to main + PRs to main — `ci-e2e.yml` (3 parallel jobs)

```
ci-e2e.yml
  ├── baremetal     (~5min)  — ubuntu-latest, init --bare-metal
  ├── colima        (~8min)  — macos-m2 (self-hosted)
  └── lima          (~10min) — linux-hub (self-hosted, KVM)
```

---

## Coverage

Standard `go test -cover` only measures lines inside the test process. E2E tests run the binary as a subprocess, so CLI handlers show 0% without binary coverage.

**Solution:** Build with `go build -cover`, set `GOCOVERDIR`, merge unit + E2E profiles.

5 Codecov flags with carryforward:
- `unit` — `go test -short -coverprofile` (every push)
- `integration` — `go test -coverprofile` with k3s (every push)
- `e2e-baremetal` — binary coverage on ubuntu-latest
- `e2e-colima` — binary coverage on macos-m2
- `e2e-lima` — binary coverage on linux-hub

---

## Self-Hosted Runners

| Label | Machine | Use |
|-------|---------|-----|
| `macos-m2` | Mac Mini M2 | Colima E2E tests |
| `linux-hub` | Home Linux PC (i5-3470S, 16GB, KVM) | Lima E2E tests |

---

## Increasing Coverage

Current `internal/cli/` coverage is ~26%. Every command has 3 output modes (JSON, plain, default) and E2E only exercises JSON + plain. The main uncovered areas:

### Default (human-readable) output mode

Every command has a default output path with tables, colors, and spinners that E2E never hits (E2E always passes `--json` or `--plain`). This is the largest coverage gap.

**How to cover:** Add `runDefaultModeSubtests(t, vsName)` — same pattern as plain mode, run commands without `--json`/`--plain`, assert exit code 0 and non-empty stdout. Won't validate formatting but will exercise the code paths.

### Error paths

Commands like `delete nonexistent`, `exec` on a stopped vibespace, `config set` with invalid flags, `agent delete` on primary agent — none of these are tested.

**How to cover:** Add `runErrorSubtests(t, vsName)` — run commands expected to fail, assert non-zero exit code and that JSON error output has the right error code.

### Flag parsing variants

Commands accept both `--flag value` and `--flag=value`. E2E only uses the space-separated form. The `=` form has separate parsing branches.

**How to cover:** Re-run a few commands with `=` style flags (e.g., `config set claude-1 --model=opus`).

### `--help` blocks

Every command has 10-30 lines of help text that are never executed.

**How to cover:** Run each command with `--help`, assert exit code 0. Low ROI but easy.

### Connect (interactive SSH)

`connect.go` is at 0% because it opens an interactive SSH session. But it can be tested non-interactively by piping commands through stdin using the existing `runWithStdin` helper:

```go
r := runWithStdin(t, "whoami && exit\n", vsName, "connect")
// verify exit code 0 and stdout contains username
```

### 0% files

| File | Why 0% | How to cover |
|------|--------|--------------|
| `connect.go` | Interactive SSH session | Pipe commands via stdin (see above) |
| `serve.go` | Starts WireGuard server, needs real network | Remote mode E2E |
| `remote.go` | Connects via WireGuard tunnel | Remote mode E2E |
| `daemon_cmd.go` | Internal daemon entry point | Covered indirectly by forward tests |
| `info_tui.go` | Bubbletea TUI, requires TTY | Manual only |
| `suggestions.go` | "Did you mean?" for typos | Error path E2E |

### Priority order

1. Default output mode subtests — biggest coverage gain (~15-20% increase), easy to add
2. Error path subtests — covers error handling + `suggestions.go`
3. Remote mode E2E — covers `serve.go`, `remote.go`, needs VPS runner
4. Help flag subtests — low ROI, do last

---

## What's Next

1. **Default output mode E2E** — exercise table/color/spinner code paths
2. **Error path E2E** — invalid args, not found, conflict scenarios
3. **Remote mode E2E** — WireGuard tunnel tests: `vibespace serve` on VPS, `remote connect` from runners

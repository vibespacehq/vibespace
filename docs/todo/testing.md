# Testing Plan

## Philosophy

No mocks. Every test runs against real infrastructure.

Vibespace is infrastructure-heavy software — subprocess calls, Kubernetes orchestration, WireGuard tunnels, daemon IPC, DNS servers. The bugs live in the integration, not the logic. Mocking the I/O gives green tests and false confidence. A mock can confirm you called `client.Create()` with the right args, but it can't tell you the Deployment spec is wrong, the label selector doesn't match, or the kubeconfig path breaks on one platform.

Instead, we test at two levels:
1. **Pure logic tests** — call Go functions directly with real inputs, assert real outputs. No I/O, no infra needed.
2. **Real infrastructure tests** — run the actual `vibespace` binary against real k3s clusters, real WireGuard tunnels, real VMs. Same as a user would.

This means:
- No mock frameworks, no `//go:generate mockgen`, no fake implementations
- No interface extraction purely for testability (existing interfaces like `ClusterManager` exist for real polymorphism, not testing)
- No test infrastructure to maintain that drifts from reality
- Tests that pass mean the software works. Tests that fail mean it's broken.

### When to reconsider

Add interfaces and mocks if/when:
- A second contributor can't run integration tests locally
- CI times exceed 15 minutes and need a fast-path
- A new platform manager is added and the interface pattern needs expansion
- CLI handlers grow complex enough to warrant isolated testing

None of these apply today.

---

## Codebase Testability

~26,500 lines of Go across 74+ files. Zero test files currently.

### Tier 1: Pure logic (no I/O, no infra)

| Package | What's testable | Lines |
|---------|----------------|-------|
| `pkg/errors` | `ErrorCode()` mapping, sentinel error identity | ~130 |
| `pkg/agent` | Type parsing, config clone/merge/validate, registry, tool defaults, `ParseStreamLine()` | ~380 |
| `pkg/vibespace` (partial) | Name validation (`ValidateName`) | ~30 |
| `pkg/ui` | Table rendering, plain vs colored output | ~230 |
| `pkg/session` | Session store with `t.TempDir()` | ~315 |
| `pkg/remote` (partial) | Token encode/decode/sign/verify, key generation, `AllocateClientIP()`, state serialization | ~500 |
| `pkg/dns` (partial) | Record add/remove (in-memory map) | ~50 |
| `pkg/daemon/protocol` | JSON request/response marshaling | ~100 |
| `internal/platform` (partial) | `verifySHA256()`, `parseSHA256SUMS()`, `extractTarGz()`, `Platform.String()` | ~200 |
| `internal/cli` (partial) | JSON output structure, error hints | ~150 |

### Tier 2: Needs real k3s (installed on CI runner, not via vibespace)

| Package | What's testable | Infrastructure |
|---------|----------------|---------------|
| `pkg/vibespace` | Full CRUD — create/list/get/delete vibespaces and agents | k3s on ubuntu runner |
| `pkg/deployment` | Deployment creation, scaling, label selectors | k3s on ubuntu runner |

These tests call service-layer Go functions directly against a real k3s. They verify that the correct Kubernetes objects are created with the right specs, labels, and env vars. Pods will be `Pending` (no real container images on the runner) but the Deployment specs are what we're validating.

**Kubeconfig resolution:** `pkg/k8s/client.go` reads only from `~/.vibespace/kubeconfig` (or `~/.vibespace/remote_kubeconfig` in remote mode) — it does NOT honor the `KUBECONFIG` env var. CI must copy the k3s kubeconfig to that path:

```bash
mkdir -p ~/.vibespace
sudo cp /etc/rancher/k3s/k3s.yaml ~/.vibespace/kubeconfig
sudo chown $(id -u):$(id -g) ~/.vibespace/kubeconfig
```

Alternatively, `pkg/k8s/client.go` could be updated to check `KUBECONFIG` env var first, falling back to `~/.vibespace/kubeconfig`. This is a small change that makes testing easier without compromising production behavior. Decision: TBD during implementation.

### Tier 3: Full binary lifecycle (real `vibespace` commands)

| Platform | Runner | What's tested |
|----------|--------|---------------|
| Bare metal (k3s) | GitHub ubuntu-latest | `vibespace init --bare-metal` through full lifecycle |
| Colima (macOS) | GitHub macos-14 | `vibespace init` through full lifecycle |
| Lima + QEMU | Self-hosted linux-vps | `vibespace init` through full lifecycle |
| Remote mode | GitHub runners + VPS | `vibespace serve` on VPS, `remote connect` from runners |

These tests build the binary, run it as a subprocess (like `exec.Command("vibespace", "create", ...)`), and verify results via `vibespace list --json`. They test the entire stack: flag parsing, output formatting, kubeconfig resolution, service layer, and Kubernetes — in one shot.

---

## Test Distribution

### Every push — `ci.yml` (3 parallel jobs)

Single workflow, three jobs that run in parallel (unit and integration both wait for lint):

```
ci.yml (every push)
  ├── lint          (~30s)  — go vet, staticcheck, deadcode, gofmt
  ├── unit          (~30s)  — go test -short -race (pure logic only)
  └── integration   (~2min) — k3s install + go test -race (all tests)
```

Integration tests use `testing.Short()` to skip under `-short` flag, so the unit job runs only pure logic tests without k3s. The integration job runs everything including k8s service layer tests.

```
Unit tests:
  pkg/errors/errors_test.go
    TestErrorCode                — every sentinel error -> correct (exitCode, code)
    TestErrorCodeWrapped         — wrapped errors still match
    TestErrorCodeUnknown         — unknown error -> (1, "internal")

  pkg/agent/agent_test.go
    TestParseType                — "claude-code" -> TypeClaudeCode, "" -> default
    TestConfigClone              — deep copy doesn't share references
    TestConfigMerge              — non-zero fields override, zero fields don't
    TestConfigValidate           — invalid combinations caught
    TestDefaultAllowedTools      — returns expected tool list
    TestConfigIsEmpty            — zero value vs non-zero

  pkg/agent/registry_test.go
    TestRegistryGetKnown         — TypeClaudeCode -> non-nil
    TestRegistryGetUnknown       — unknown type -> error

  pkg/agent/claude/claude_test.go
    TestParseStreamLine          — real JSON samples -> correct StreamMessage
    TestBuildPrintModeCommand    — config -> expected command string

  pkg/vibespace/validate_test.go
    TestValidateName             — valid names, empty, too long, special chars

  pkg/ui/table_test.go
    TestNewTable                 — headers + rows -> formatted string
    TestPlainTableRows           — data rows only, tab-separated
    TestPlainTableWithHeader     — tab-separated with optional header

  pkg/remote/token_test.go
    TestGenerateKeyPair          — produces valid Curve25519 pair
    TestEncodeDecodeToken        — round-trip preserves all fields
    TestSignVerifyToken          — valid signature passes, tampered fails
    TestExpiredToken             — expired token rejected
    TestTokenPrefix              — always starts with "vs-"

  pkg/remote/state_test.go
    TestRemoteStateSaveLoad      — write to t.TempDir(), read back, compare
    TestServerStateSaveLoad      — same pattern
    TestAllocateClientIP         — sequential: 10.100.0.2, .3, .4...
    TestAddClient                — append to Clients slice, verify fields

  pkg/remote/server_test.go
    TestAddClientIdempotent      — same pubkey -> same IP (Server.AddClient checks FindClientByPublicKey)
    TestRemoveClient             — by name, by key, by hostname (Server.RemoveClient with ambiguity detection)

  pkg/remote/tls_test.go
    TestGenerateSelfSignedCert   — produces valid PEM, correct CN
    TestPinningTLSConfig         — matching fingerprint passes, wrong rejects

  pkg/session/store_test.go
    TestSessionSaveLoad          — write to t.TempDir(), read back
    TestSessionList              — multiple sessions, filter by vibespace

  pkg/dns/server_test.go
    TestAddRemoveRecord          — in-memory record tracking
    TestDNSServerStartStop       — real UDP on random port, real DNS query
    TestDNSResolution            — add record, query it, get correct IP
    TestDefaultFallback          — unknown name -> 127.0.0.1

  pkg/daemon/protocol_test.go
    TestRequestMarshal           — JSON round-trip for each request type
    TestResponseMarshal          — success and error responses

  internal/platform/detect_test.go
    TestPlatformString           — "darwin" -> "macOS (arm64)"
    TestIsMacOS                  — platform method correctness

  internal/platform/download_test.go
    TestVerifySHA256             — correct hash passes, wrong fails
    TestParseSHA256SUMS          — real checksum file format -> correct hash
    TestExtractTarGz             — real tiny tar.gz -> correct files extracted
    TestExtractTarGzTraversal    — path with ".." -> rejected (security)
    TestExtractTarGzAbsolutePath — absolute path in tar entry -> rejected
    TestExtractTarGzSymlinkEscape — symlink pointing outside destDir -> rejected

  internal/platform/manager_test.go
    TestSaveLoadClusterState     — JSON round-trip in t.TempDir()
    TestNewClusterManager        — platform + opts -> correct manager type

  internal/cli/output_test.go
    TestJSONOutput               — NewJSONOutput -> correct structure
    TestErrorHints               — each error -> appropriate hint string

K8s service layer (k3s installed directly on the runner):
  pkg/vibespace/service_test.go
    TestCreateVibespace          — real namespace + deployment created
    TestListVibespaces           — create 2, list returns both
    TestGetVibespace             — by name and by ID
    TestDeleteVibespace          — resources cleaned up
    TestCreateAgent              — deployment with correct labels/env
    TestListAgents               — filters by vibespace correctly

  pkg/deployment/manager_test.go
    TestCreateDeployment         — real k8s deployment created
    TestScaleDeployment          — replicas updated
    TestDeleteDeployment         — cleaned up
    TestListByLabel              — label selector works
```

### PRs to main — `ci-e2e.yml` (future)

### PRs to main — ubuntu-latest (~5 min)

Full binary lifecycle on Linux bare metal + remote mode E2E.

```
vibespace init --bare-metal
vibespace status --json                     -> assert cluster running
vibespace create testproject --agent-type claude-code
vibespace list --json                       -> assert testproject exists
vibespace testproject agents --json         -> assert agent exists
vibespace delete testproject -f
vibespace uninstall

Remote mode (linux client -> VPS server):
  SSH to VPS: vibespace init --bare-metal
  SSH to VPS: vibespace serve &
  SSH to VPS: vibespace serve --generate-token -> TOKEN
  vibespace remote connect $TOKEN             -> real WireGuard tunnel
  vibespace remote status --json              -> assert connected
  kubectl over tunnel: get nodes              -> assert k8s reachable
  vibespace remote disconnect
  SSH to VPS: vibespace uninstall
```

### PRs to main — macos-14 (~8 min)

Full binary lifecycle on macOS with Colima + remote mode E2E.

```
vibespace init                              -> colima + k3s in VM
vibespace status --json                     -> assert cluster running
vibespace create testproject --agent-type claude-code
vibespace list --json                       -> assert testproject exists
vibespace testproject agents --json         -> assert agent exists
vibespace delete testproject -f
vibespace uninstall

Remote mode (macOS client -> VPS server):
  SSH to VPS: vibespace init --bare-metal
  SSH to VPS: vibespace serve &
  SSH to VPS: vibespace serve --generate-token -> TOKEN
  sudo vibespace remote connect $TOKEN        -> real WireGuard tunnel
  vibespace remote status --json              -> assert connected
  kubectl over tunnel: get nodes              -> assert k8s reachable
  vibespace remote disconnect
  SSH to VPS: vibespace uninstall
```

### PRs to main — self-hosted linux-vps (~10 min)

Lima + QEMU lifecycle on the VPS (the only environment with custom QEMU binaries, no /dev/kvm).

```
vibespace init                              -> lima + QEMU + k3s in VM
vibespace status --json                     -> assert cluster running
vibespace create testproject --agent-type claude-code
vibespace list --json                       -> assert testproject exists
vibespace delete testproject -f
vibespace uninstall
```

---

## Remote Mode Testing

Remote mode is inherently a two-machine problem: server on one host, client on another. GitHub-hosted runners cannot network with each other directly (NAT isolation, UDP blocked between runners).

**Solution:** Use the Hetzner VPS as the server. GitHub-hosted runners (ubuntu, macOS) SSH into the VPS to start `vibespace serve`, then connect as clients from the runner. This tests the real production path — exactly how users deploy remote mode.

The client uses a different `HOME` directory implicitly (it's a different machine), so `stopLocalClusterIfRunning()` doesn't interfere with the server's cluster.

WireGuard UDP from GitHub runners to external servers with stable IPs works fine (outbound traffic is allowed). The VPS has a public IP (49.13.120.186) and WireGuard port exposed.

---

## Self-Hosted Runners

Two self-hosted runners for tests that need real hardware (VM creation, hardware virtualization):

| Label | Machine | Use |
|-------|---------|-----|
| `macos-m2` | Mac Mini M2 | Colima E2E tests |
| `linux-hub` | Home Linux PC (i5-3470S, 16GB, KVM) | Lima E2E tests |

### linux-hub setup

- **User:** `github-runner` (dedicated, `NOPASSWD: ALL` sudo, `kvm` group)
- **Runner:** GitHub Actions Runner v2.331.0, systemd service
- **Go:** 1.24.0 system-wide (`/usr/local/go/bin/go`)
- **KVM:** Hardware virtualization for fast QEMU (no TCG)
- **Power:** Sleep/suspend disabled, wifi power save disabled

### Test isolation

CI tests use `/home/github-runner/.vibespace/` — completely separate from any user state. The `vibespace uninstall --force` cleanup in tests tears down everything, so the runner machine cannot simultaneously run a user's vibespace instance.

---

## CI Pipeline Design

### Linting (part of every-push job)

- `go vet ./...`
- `staticcheck ./...` (config already exists in `.staticcheck.conf`)
- `gofmt` check (verify no unformatted files)

Not using `golangci-lint` — `staticcheck` is already configured and sufficient. Add `golangci-lint` later if more linters are needed.

### Coverage

**Important:** Standard `go test -cover` only measures lines executed inside the test process. Our E2E tests run the binary as a subprocess (`exec.Command("./vibespace", ...)`), so the bulk of the codebase — CLI handlers, platform managers, k8s orchestration — shows 0% despite being fully exercised.

To get real coverage numbers, E2E jobs must build with `go build -cover` and set `GOCOVERDIR` so the binary writes coverage data on exit. Then merge unit + E2E profiles before uploading.

```bash
# Unit tests — standard coverage
go test -coverprofile=unit.out ./...

# E2E tests — binary coverage
mkdir -p /tmp/covdata
go build -cover -o vibespace ./cmd/vibespace
GOCOVERDIR=/tmp/covdata go test -tags e2e -v -timeout 15m -count=1 ./test/e2e/
go tool covdata textfmt -i=/tmp/covdata -o=e2e.out

# Merge profiles
go tool cover -merge unit.out e2e.out -o merged.out
```

Without this, Codecov would report ~15-25% (unit tests only) which is misleading — the real coverage including E2E is much higher.

- Upload merged profile to Codecov on every push
- No minimum threshold initially — start with visibility, ratchet up over time
- PR comments showing coverage diff

### Build verification

- `go build ./...` runs as part of every-push (implicit in test compilation)
- Binary built and used in PR-to-main lifecycle tests

---

## Cost

All costs for GitHub Actions hosted runners (private repo):

| Job | Trigger | Runner | Runs/month | Minutes | Rate | Cost |
|-----|---------|--------|-----------|---------|------|------|
| Unit + k3s | Every push | ubuntu-latest | 200 | 700 | $0.008/min | $5.60 |
| Bare metal + remote | PR to main | ubuntu-latest | 10 | 50 | $0.008/min | $0.40 |
| Colima + remote | PR to main | self-hosted (mac-m2) | 10 | 80 | $0 | $0 |
| Lima + QEMU | PR to main | self-hosted (linux-vps) | 10 | 100 | $0 | $0 |
| **Total** | | | | | | **~$6/month** |

Free if public repo. Fits within the 2,000 min/month free tier for private repos (750 Linux equivalent minutes).

GitHub's macos-14 runners use M1 VMs which don't support nested virtualization (Colima/Lima need to create VMs). Apple added nested virtualization support starting with M3 chips on macOS 15, but GitHub hasn't updated their runners yet. Self-hosted mac-m2 runner on real hardware avoids this limitation.

---

## What's Tested vs Not

| Component | Tested | How |
|-----------|--------|-----|
| Error codes + mapping | Yes | Unit test |
| Agent config, registry, parsing | Yes | Unit test |
| Token encode/decode/sign/verify | Yes | Unit test |
| WireGuard key generation | Yes | Unit test |
| TLS cert generation + pinning | Yes | Unit test |
| SHA256 verification | Yes | Unit test |
| Tar.gz extraction + path traversal guard | Yes | Unit test |
| DNS record management | Yes | Unit test (real UDP server on loopback) |
| Daemon IPC protocol | Yes | Unit test |
| Table/UI rendering | Yes | Unit test |
| Session persistence | Yes | Unit test with t.TempDir() |
| State file serialization | Yes | Unit test with t.TempDir() |
| Cluster state persistence | Yes | Unit test with t.TempDir() |
| K8s vibespace CRUD | Yes | Service layer against real k3s |
| K8s deployment management | Yes | Service layer against real k3s |
| Bare metal k3s lifecycle | Yes | Full binary on ubuntu runner |
| Colima lifecycle | Yes | Full binary on macos-14 runner |
| Lima + QEMU lifecycle | Yes | Full binary on VPS (self-hosted) |
| Remote mode (linux client) | Yes | WireGuard tunnel: runner -> VPS |
| Remote mode (macOS client) | Yes | WireGuard tunnel: runner -> VPS |
| CLI flag parsing + output | Yes | Covered by binary lifecycle tests |
| Daemon auto-start | Yes | Covered by binary lifecycle tests |
| TUI (bubbletea interactive) | No | Requires terminal simulation, low ROI |

Coverage target: ~50-60 test functions across ~20 test files. Pure logic tests run in milliseconds. Infrastructure tests run in minutes on real hardware.

---

## Implementation Order

1. ~~CI workflow file — lint + build + unit tests~~ **Done**
2. ~~Pure logic test files (Tier 1) — fastest to write, immediate value~~ **Done**
3. ~~K8s service layer tests — add k3s to CI, test against real cluster~~ **Done**
4. ~~Binary lifecycle tests (bare metal) — ubuntu runner~~ **Done**
5. ~~Binary lifecycle tests (colima) — macos-14 runner~~ **Done**
6. ~~VPS runner setup — self-hosted GitHub Actions runner on VPS~~ **Done**
7. ~~Lima lifecycle — self-hosted runner on VPS~~ **Done**
8. Remote mode E2E — WireGuard tunnel tests on linux-hub runner
9. ~~Codecov integration — coverage tracking and PR comments~~ **Done**

---

## Completed Work

### Step 1: CI workflow + Tier 1 pure logic tests

**CI workflow** (`.github/workflows/ci.yml`): runs on every push to any branch. Three parallel jobs: lint, unit tests (`-short`), and integration tests (k3s). Codecov upload from both test jobs with separate flags.

Workflows directory convention — `ci-` for continuous integration, `release-` for artifact publishing:
- `ci.yml` — lint + unit + integration (every push, 3 parallel jobs)
- `ci-e2e.yml` — E2E binary lifecycle tests (PRs to main, bare metal on ubuntu-latest)
- `release-agent-images.yml` — agent container images (tag/manual trigger)
- `release-qemu-binaries.yml` — QEMU binaries (`qemu-v*` tags + manual only)

**17 test files, 77 tests** across 12 packages:

| File | Tests | What's covered |
|------|-------|----------------|
| `pkg/errors/errors_test.go` | 3 | ErrorCode mapping, wrapped errors, unknown fallback |
| `pkg/agent/agent_test.go` | 11 | ParseType, IsValid, Clone, Merge, IsEmpty, DefaultAllowedTools, AllowedToolsString |
| `pkg/agent/registry_test.go` | 2 | Get known type, get unknown type |
| `pkg/agent/claude/claude_test.go` | 10 | ParseStreamLine (6 message types + invalid + empty), BuildPrintModeCommand (4 configs) |
| `pkg/vibespace/validate_test.go` | 2 | Valid names, invalid names with specific error messages |
| `pkg/ui/table_test.go` | 3 | Plain table, rows only, with/without header |
| `pkg/remote/token_test.go` | 7 | Key generation, encode/decode round-trip, sign/verify, expired, prefix, missing fields, unsigned |
| `pkg/remote/state_test.go` | 4 | AllocateClientIP, AddClient, FindClientByPublicKey, JSON save/load |
| `pkg/remote/tls_test.go` | 3 | Cert generation (IP + DNS), fingerprint verification, pinning TLS config |
| `pkg/session/store_test.go` | 5 | Save/load, list sorted, SaveNew duplicate, delete, get not found |
| `pkg/dns/server_test.go` | 4 | Add/remove record, start/stop, real DNS resolution, default fallback |
| `pkg/daemon/protocol_test.go` | 3 | Request marshal, response marshal (success + error + nil), StatusResponse |
| `internal/platform/detect_test.go` | 4 | String, IsMacOS, IsLinux, IsARM |
| `internal/platform/download_test.go` | 5 | SHA256 verify, parseSHA256SUMS, extractTarGz, path traversal rejection, symlink |
| `internal/platform/manager_test.go` | 6 | Save/load cluster state, darwin/linux/baremetal/unsupported/persisted state |
| `internal/cli/output_test.go` | 2 | Error hints for each sentinel error, unknown error empty hint |

All tests pass with `-race`. No mocks — pure logic with `t.TempDir()` for file I/O and real UDP for DNS.

### Step 3: K8s service layer tests

Added k8s integration tests and restructured CI from a single-job `ci-unit.yml` into a three-job `ci.yml`:

- **lint** — go vet, staticcheck, deadcode, gofmt
- **unit** (needs lint) — `go test -short -race` (pure logic only, integration tests skip via `testing.Short()`)
- **integration** (needs lint) — k3s install + `go test -race` (all tests including k8s service layer)

Unit and integration jobs run in parallel after lint passes. Integration tests use `testing.Short()` to skip in the unit job and `t.Skip("k8s not available")` to skip on machines without a cluster.

**Bug fix:** `ScaleAllDeploymentsForVibespace` and `ScaleAgentDeployment` now use `retry.RetryOnConflict` — the k8s controller modifies deployments between List and Update, causing stale resourceVersion conflicts.

**2 test files, 10 tests:**

| File | Tests | What's covered |
|------|-------|----------------|
| `pkg/deployment/manager_test.go` | 4 | CreateDeployment (labels, container, ports, resources, Service), ScaleDeployment (0 then 1), DeleteDeployment (Deployment + Service gone), ListByLabel (ListDeployments + ListAgentsForVibespace filtering) |
| `pkg/vibespace/service_test.go` | 6 | CreateVibespace (ID, status, resources, k8s Deployment), ListVibespaces (create 2, both appear), GetVibespace (by name, by ID, nonexistent), DeleteVibespace (resources cleaned up: Secret, PVC), CreateAgent (labels: is-agent, agent-type, agent-num), ListAgents (2 agents, primary flag, names) |

All tests run against real k3s. Pods stay `Pending` (busybox image, no real agents) but Deployment specs, labels, Services, PVCs, and Secrets are validated. Each test uses unique UUIDs and cleans up via `t.Cleanup()`.

### Step 4: E2E binary lifecycle tests (bare metal)

Added end-to-end tests that build the actual `vibespace` binary, run it as a subprocess, and verify the entire stack on a Linux bare metal environment.

**CI workflow** (`.github/workflows/ci-e2e.yml`): triggered on PRs to main only (not every push). Builds the binary, then runs E2E tests with `-tags e2e` on ubuntu-latest. Separate from `ci.yml` because `vibespace init --bare-metal` installs its own k3s (conflicts with integration job's direct k3s install). 15-minute job timeout, 10-minute test timeout.

**CLI change:** Added `--force`/`-f` flag to `vibespace uninstall` to skip the confirmation prompt, matching the existing pattern from `vibespace delete -f`.

**2 test files, 1 test function with 7 subtests:**

| File | What's covered |
|------|----------------|
| `test/e2e/helpers_test.go` | Binary runner (`run`, `runJSON`, `mustSucceed`), JSON parsing (`parseData[T]`), mirrored JSON types (`JSONOutput`, `StatusData`, `CreateData`, `ListData`, `AgentsData`, `DeleteData`) |
| `test/e2e/baremetal_test.go` | `TestBareMetalLifecycle`: init (bare metal) → status (cluster running) → create (vibespace + agent) → list (vibespace exists) → agents (claude-code agent exists) → delete (force) → verify (vibespace gone) |

Test files use `//go:build e2e` so `go test ./...` never picks them up. The lifecycle test uses `t.Cleanup` to always run `vibespace uninstall --force` even on failure.

### Step 5: E2E binary lifecycle tests (Colima)

Added Colima E2E lifecycle test for macOS, testing the default `vibespace init` path (no `--bare-metal` flag). This validates the full Colima flow: binary downloads (Lima, Colima, kubectl, Docker), VM creation, k3s-in-VM, and vibespace CRUD.

**OS build constraints** split the E2E tests by platform:
- `baremetal_test.go`: `//go:build e2e && linux` — only compiles on Linux
- `colima_test.go`: `//go:build e2e && darwin` — only compiles on macOS
- `helpers_test.go`: `//go:build e2e` — shared by both platforms

No `-run` flag needed in CI — the build constraints handle test selection automatically.

**CI workflow** (`.github/workflows/ci-e2e.yml`): added `colima` job on `macos-14` alongside existing `baremetal` job. 20-minute job timeout, 15-minute test timeout. Conservative VM resources (`--cpu 3 --memory 6 --disk 30`) for macos-14 runners (Apple Silicon M1, 3 cores, 7GB RAM, 14GB SSD).

**1 new test file, 1 test function with 7 subtests:**

| File | What's covered |
|------|----------------|
| `test/e2e/colima_test.go` | `TestColimaLifecycle`: init (Colima default) → status (platform=darwin) → create (vibespace + agent) → list (vibespace exists) → agents (claude-code agent exists) → delete (force) → verify (vibespace gone) |

### Step 6: Self-hosted runner setup

Originally planned as VPS runner (Hetzner CPX, TCG software emulation). Moved to a home Linux PC (`linux-hub`) with KVM hardware acceleration after discovering TCG was too slow (~11min for VM boot) and the VPS couldn't host both a runner and a user's vibespace instance simultaneously.

**linux-hub (home iMac, i5-3470S, 16GB RAM, KVM):**

- **`github-runner` user** with `NOPASSWD: ALL` sudo and `kvm` group
- **GitHub Actions Runner v2.331.0**, systemd service, label `linux-hub`
- **Go 1.24.0** system-wide
- **Sleep/suspend disabled**, wifi power save disabled for always-on operation

### Step 7: Lima lifecycle E2E tests

Tests the Lima+QEMU path on Linux — `vibespace init` without `--bare-metal`, which creates a VM with k3s inside. Runs on `linux-hub` (home Linux PC with KVM hardware acceleration). Build tag `lima` keeps this from compiling on ubuntu-latest where baremetal tests live.

**CI workflow** (`.github/workflows/ci-e2e.yml`): added `lima` job on `linux-hub` alongside `baremetal` and `colima`. 15-minute job timeout, 10-minute test timeout. VM resources: `--cpu 4 --memory 4 --disk 20`.

**1 new test file, 1 test function with 7 subtests:**

| File | What's covered |
|------|----------------|
| `test/e2e/lima_test.go` | `TestLimaLifecycle`: init (Lima, no --bare-metal) → status (platform=linux) → create (vibespace + agent) → list (vibespace exists) → agents (claude-code agent exists) → delete (force) → verify (vibespace gone) |

### Step 9: Codecov integration

Binary coverage for E2E tests using `go build -cover` + `GOCOVERDIR`. Standard `go test -cover` only tracks lines inside the test process — E2E tests run vibespace as a subprocess, so CLI handlers, platform managers, and k8s orchestration would show 0% without this. The coverage-instrumented binary writes `.covcounters` files to `GOCOVERDIR` on each exit, then `go tool covdata textfmt` converts them to standard Go coverage format for upload.

**5 Codecov flags** with carryforward (partial uploads don't tank reported coverage):
- `unit` — `go test -short -coverprofile` (ci.yml, every push)
- `integration` — `go test -coverprofile` with k3s (ci.yml, every push)
- `e2e-baremetal` — binary coverage on ubuntu-latest (ci-e2e.yml, PRs to main)
- `e2e-colima` — binary coverage on macos-m2 (ci-e2e.yml, PRs to main)
- `e2e-lima` — binary coverage on linux-hub (ci-e2e.yml, PRs to main)

**Configuration:**
- `codecov.yml` at repo root — flag definitions, carryforward, PR comment layout
- `CODECOV_TOKEN` repo secret for private repo auth
- `codecov-action@v5` for uploads
- `VIBESPACE_DEBUG=1` on all E2E jobs with debug log artifact upload on failure
- Cluster readiness timeout increased to 10min for slow environments

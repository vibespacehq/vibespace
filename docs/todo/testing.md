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

### Every push — ubuntu-latest (~3-4 min)

Fast feedback on every commit. Pure logic + k8s service layer. Includes k3s install (~30s), staticcheck install, and `go test -race` across all packages.

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

CI setup for this job:
```yaml
steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with: { go-version: '1.24' }
  - name: Install k3s
    run: |
      curl -sfL https://get.k3s.io | sudo sh -
      sudo chmod 644 /etc/rancher/k3s/k3s.yaml
      # vibespace reads kubeconfig from ~/.vibespace/kubeconfig, not KUBECONFIG env
      mkdir -p ~/.vibespace
      sudo cp /etc/rancher/k3s/k3s.yaml ~/.vibespace/kubeconfig
      sudo chown $(id -u):$(id -g) ~/.vibespace/kubeconfig
  - name: Lint
    run: |
      go vet ./...
      go install honnef.co/go/tools/cmd/staticcheck@latest
      staticcheck ./...
  - name: Test
    run: go test ./... -race -coverprofile=coverage.out
  - name: Coverage
    uses: codecov/codecov-action@v4
    with: { files: coverage.out }
```

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

## VPS Security for CI

The VPS is used as both a test target (SSH from runners) and a self-hosted runner (Lima tests). Security measures:

### Dedicated CI user

```bash
sudo useradd -m -s /bin/bash ci-test
# Limited sudo — only vibespace and k3s commands
echo 'ci-test ALL=(ALL) NOPASSWD: /usr/local/bin/vibespace, /usr/local/bin/k3s, \
  /usr/bin/systemctl stop k3s, /usr/bin/systemctl start k3s, \
  /usr/local/bin/k3s-uninstall.sh' | sudo tee /etc/sudoers.d/ci-test
```

### SSH key restrictions

Separate SSH key for CI (not the personal key), stored in GitHub Secrets. Optionally restrict to a command whitelist in `authorized_keys`:

```
command="/home/ci-test/ci-runner.sh",no-port-forwarding,no-X11-forwarding ssh-ed25519 AAAA... github-ci
```

### Test isolation

CI tests use `/home/ci-test/.vibespace/` — completely separate from any production state under `/home/vibeuser/.vibespace/`.

### Workflow guards

```yaml
# Only run VPS-touching jobs on PRs from this repo, never forks
if: github.event.pull_request.head.repo.full_name == github.repository
```

GitHub Actions does not expose secrets to fork PRs by default. This guard makes it explicit.

---

## CI Pipeline Design

### Linting (part of every-push job)

- `go vet ./...`
- `staticcheck ./...` (config already exists in `.staticcheck.conf`)
- `gofmt` check (verify no unformatted files)

Not using `golangci-lint` — `staticcheck` is already configured and sufficient. Add `golangci-lint` later if more linters are needed.

### Coverage

- Upload to Codecov on every push
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
| Colima + remote | PR to main | macos-14 | 10 | 80 | $0.08/min | $6.40 |
| Lima + QEMU | PR to main | self-hosted | 10 | 100 | $0 | $0 |
| **Total** | | | | | | **~$12/month** |

Free if public repo. Fits within the 2,000 min/month free tier for private repos (850 Linux + 80×10 macOS multiplier = 1,650 equivalent minutes).

CircleCI was evaluated and rejected: $29/month (Performance plan) for the same workload, with macOS costing 200 credits/min (M4 Pro, since M1/M2 reach EOL Feb 16, 2026). No advantage for cross-runner networking.

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

1. CI workflow file (`.github/workflows/ci.yml`) — lint + build + unit tests
2. Pure logic test files (Tier 1) — fastest to write, immediate value
3. K8s service layer tests — add k3s to CI, test against real cluster
4. Binary lifecycle tests (bare metal) — ubuntu runner
5. Binary lifecycle tests (colima) — macos-14 runner
6. VPS security setup — ci-test user, SSH keys, sudoers
7. Remote mode E2E — SSH to VPS, WireGuard tunnel
8. Lima lifecycle — self-hosted runner on VPS
9. Codecov integration — coverage tracking and PR comments

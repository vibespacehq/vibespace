# Code Quality Issues

Identified during objective project assessment (2026-02-06).

---

## 1. Race Conditions in Server Registration

**Severity:** Low (single-user tool), but pattern risk at scale

- `pkg/remote/server.go`: `FindClientByPublicKey()` and `AddClient()` aren't atomic — concurrent registrations with the same pubkey could both succeed
- `pkg/remote/state.go`: Global `stateMu` protects individual operations, but read-modify-write sequences across `LoadRemoteState()` -> modify -> `Save()` aren't locked

**Fix:** Lock the entire find + add sequence with a mutex. Consider file-level locking for cross-process safety.

---

## 2. Partial State on Failure in Connect()

**Severity:** Medium

- `pkg/remote/client.go`: State is saved to disk *before* `QuickUp()` brings up the tunnel
- If WireGuard fails to start, you have state on disk but no working tunnel

**Fix:** Reorder to: bring up tunnel -> verify connectivity -> then persist state.

---

## 3. Temp File Vulnerabilities in WireGuard Setup

**Severity:** Medium (requires local access)

- `pkg/remote/wireguard.go`: Uses predictable paths in `/tmp/` (`wg-tun-name`, `wg-config-filtered.conf`)
- A local attacker could symlink these to sensitive files

**Fix:** Use `os.CreateTemp()` with `O_EXCL` or write to `~/.vibespace/`.

---

## 4. No Binary Integrity Verification

**Severity:** Medium (supply chain risk)

- WireGuard binaries downloaded from Homebrew bottles (`pkg/remote/wireguard.go`) aren't checksum-verified
- Compromised mirror -> malicious wireguard-go binary
- The bearer token `QQ==` (decodes to `@`) looks like a placeholder or undocumented Homebrew quirk

**Fix:** Verify SHA256 hashes of downloaded binaries against known-good values. Clarify or remove the bearer token.

---

## 5. Kube API Proxy Has No Connection Limiting

**Severity:** Low-Medium

- `pkg/remote/server.go`: Bidirectional `io.Copy` with no connection cap in `proxyToKubeAPI()`
- Every connected client can open unlimited proxy connections
- Easy DoS vector in multi-client scenarios

**Fix:** Add a `semaphore` or `golang.org/x/sync/semaphore` to cap concurrent proxy connections per client.

---

## 6. Single-Platform Assumption for Linux

**Severity:** Medium

- `pkg/remote/wireguard.go`: Linux WireGuard install uses `apt-get` — breaks on Fedora, Arch, Alpine

**Fix:** Detect package manager (`apt-get`, `dnf`, `pacman`, `apk`) or provide static binaries.

---

## 7. Large Files — Decomposition Opportunities

**Severity:** Low (maintainability)

| File | LOC | Concern |
|------|-----|---------|
| `pkg/tui/update.go` | 1,057 | Event handling in one file |
| `pkg/vibespace/service.go` | 1,029 | Core service logic |
| `pkg/deployment/manager.go` | 883 | K8s deployment |
| `internal/cli/multi.go` | 789 | Headless mode |

Not urgent, but splitting along responsibility boundaries would improve readability and reduce merge conflict surface.

---

## 8. No CI Pipeline

**Severity:** High

- Even `go build ./... && go vet ./...` isn't automated
- `.staticcheck.conf` exists but isn't wired into CI
- Single developer with no code review process

**Fix:** Add GitHub Actions workflow:
- `go build ./...`
- `go vet ./...`
- `staticcheck ./...`
- `go test ./...` (once tests exist)

---

## 9. No README at Repo Root

**Severity:** Medium (first impression)

- First thing anyone sees when visiting the repo — nothing
- 100k+ bytes of docs exist under `docs/` but no entry point

**Fix:** Add a root `README.md` with project overview, quick start, and links to detailed docs.

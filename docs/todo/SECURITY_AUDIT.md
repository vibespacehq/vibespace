# Security Audit

Findings from a comprehensive security audit of the vibespace codebase.

## Core Codebase

Issues outside of the remote mode feature. These apply to the stable parts of the codebase.

### Medium

#### ~~1. SSH host key verification disabled~~ ✅ Fixed

**File:** `pkg/tui/agent.go:185-186`

~~SSH connections use `StrictHostKeyChecking=no` and `UserKnownHostsFile=/dev/null`, making them vulnerable to MITM on first connect.~~

**Fixed:** Changed to `StrictHostKeyChecking=accept-new` with `~/.vibespace/known_hosts`.

#### ~~2. Debug logs world-readable in /tmp~~ ✅ Fixed

**File:** `pkg/tui/agent.go:255`

~~Debug logs are written to `/tmp/agents_debug.log` with `0644` permissions. On multi-user systems, other users can read agent interaction data.~~

**Fixed:** Moved to `~/.vibespace/agents_debug.log` with `0600` permissions.

#### ~~3. Session and history files world-readable~~ ✅ Fixed

**Files:**
- `pkg/session/store.go:119` — sessions saved with `0644`
- `pkg/tui/history_store.go:64` — chat history with `0644`

**Fixed:** Both changed to `0600`.

#### ~~4. Debug mode logs sensitive request bodies~~ ✅ Fixed

**File:** `pkg/permission/server.go:148`

~~When `VIBESPACE_DEBUG=1` is set, the permission server logs the full JSON body of incoming requests, potentially exposing tool inputs, file paths, and command outputs.~~

**Fixed:** Debug log now logs `body_len` only, not the raw content.

#### ~~5. Unbounded request body on permission server~~ ✅ Fixed

**File:** `pkg/permission/server.go:140`

~~`io.ReadAll(r.Body)` is used without size limits. Large payloads can exhaust memory.~~

**Fixed:** Wrapped with `http.MaxBytesReader(w, r.Body, 1<<20)` (1 MB limit).

#### 6. Docker container runs as root with passwordless sudo

**Files:** `build/base/Dockerfile:127`, `build/base/Dockerfile:92`

The entrypoint runs as root via supervisord. The `user` account has `NOPASSWD:ALL` sudo access. Any container compromise leads to full root.

**Fix:** Run supervisord as a non-root user where possible. Restrict sudo to only the commands that actually need it (e.g., SSH service management).

### Low

#### ~~7. Log directory permissions too permissive~~ ✅ Fixed

**File:** `internal/cli/logging.go:119`

~~Log directories are created with `0755` (world-readable).~~

**Fixed:** Changed to `0700`.

### Priority

| Priority | Issue | Status |
|----------|-------|--------|
| P1 | ~~Use `StrictHostKeyChecking=accept-new` for SSH (#1)~~ | ✅ Fixed |
| P1 | ~~Fix file permissions to `0600` for session/history (#3)~~ | ✅ Fixed |
| P2 | ~~Move debug logs out of `/tmp`, restrict permissions (#2)~~ | ✅ Fixed |
| P2 | ~~Add `http.MaxBytesReader()` to permission server (#5)~~ | ✅ Fixed |
| P2 | ~~Redact sensitive data from debug logs (#4)~~ | ✅ Fixed |
| P3 | Run containers as non-root (#6) | Open |
| P3 | ~~Restrict log directory permissions (#7)~~ | ✅ Fixed |

---

## Remote Mode (WIP)

Issues specific to `pkg/remote/`. To be addressed as the remote feature matures.

### Critical

#### ~~R1. Management API served over plain HTTP~~ ✅ Fixed

**Files:** `pkg/remote/server.go:263-290`, `pkg/remote/server.go:688`

~~The management API (kubeconfig, disconnect endpoints) listens on unencrypted HTTP over the WireGuard interface. The kubeconfig fetch at line 688 also uses `http://`.~~

**Fixed:** Management API now uses TLS with the same self-signed cert as registration. Client uses cert fingerprint pinning (saved in `RemoteState.CertFingerprint` during connect) for all mgmt API calls.

#### ~~R2. No security headers on HTTP servers~~ ✅ Fixed (remote)

**Files:** `pkg/remote/server.go`

~~None of the HTTP servers set `X-Frame-Options`, `Content-Security-Policy`, `X-Content-Type-Options`, or `Strict-Transport-Security`.~~

**Fixed:** Added `securityHeaders()` middleware to both management and registration APIs (X-Content-Type-Options, X-Frame-Options, CSP, Cache-Control). Permission server and daemon server are localhost-only, lower priority.

### High

#### ~~R3. No rate limiting on public registration endpoint~~ ✅ Fixed

**File:** `pkg/remote/server.go:491-566`

~~The registration endpoint listens on `0.0.0.0:7781` with token-only authentication. No rate limiting, IP whitelisting, or throttling — enabling brute-force and DoS attacks.~~

**Fixed:** Per-IP rate limiting (5 req/min, burst 3) using `golang.org/x/time/rate` with `sync.Map` storage.

#### ~~R4. Unbounded JSON deserialization on registration endpoint~~ ✅ Fixed

**File:** `pkg/remote/server.go:497`

~~`json.NewDecoder(r.Body).Decode()` used without size limits.~~

**Fixed:** Wrapped with `http.MaxBytesReader(w, r.Body, 1<<20)` (1 MB limit).

### Medium

#### R5. InsecureSkipVerify in TLS certificate pinning

**File:** `pkg/remote/tls.go:75`

`PinningTLSConfig()` sets `InsecureSkipVerify: true` and relies solely on a custom `VerifyPeerCertificate` callback. If the callback is ever accidentally removed, all certificates are silently accepted.

**Fix:** Document the intent clearly. Consider adding a wrapper that validates the callback is non-nil.

#### ~~R6. Verbose error messages leak certificate info~~ ✅ Fixed

**File:** `pkg/remote/tls.go:83`

~~Fingerprint mismatch errors return both the actual and expected fingerprints.~~

**Fixed:** Error now only shows the actual fingerprint: `"certificate fingerprint mismatch (got %s)"`.

#### ~~R7. World-readable remote state files~~ ✅ Fixed

**Files:**
- `pkg/remote/server.go:664` — PID file with `0644`
- `pkg/remote/wireguard.go:370` — tunnel name file with `0644`

**Fixed:** Both changed to `0600`.

#### ~~R8. TLS 1.2 as minimum version~~ ✅ Fixed

**File:** `pkg/remote/server.go:475`

~~TLS 1.3 provides stronger cipher negotiation and removes legacy algorithms.~~

**Fixed:** Bumped to `tls.VersionTLS13` on both management and registration APIs.

### Low

#### R9. Hardcoded Bearer token for Homebrew downloads

**File:** `pkg/remote/wireguard.go:714`

`Bearer QQ==` (decodes to `@`) is hardcoded. Downloaded binaries also lack SHA256 hash verification. (Also noted in `CODE_QUALITY_ISSUES.md`.)

**Fix:** Clarify or remove the bearer token. Add SHA256 verification for downloaded binaries.

### Priority

| Priority | Issue | Status |
|----------|-------|--------|
| P0 | ~~Add `http.MaxBytesReader()` to registration endpoint (R4)~~ | ✅ Fixed |
| P0 | ~~Implement rate limiting on registration endpoint (R3)~~ | ✅ Fixed |
| P1 | ~~Encrypt management API with TLS (R1)~~ | ✅ Fixed |
| P1 | ~~Fix remote state file permissions (R7)~~ | ✅ Fixed |
| P2 | ~~Add security headers (R2)~~ | ✅ Fixed |
| P2 | ~~Bump minimum TLS to 1.3 (R8)~~ | ✅ Fixed |
| P3 | ~~Reduce error verbosity (R6)~~ | ✅ Fixed |
| P3 | Verify SHA256 of downloaded binaries (R9) | Open |

---

## What's Done Well

- No hardcoded secrets — no API keys, database URLs, or credentials in the repo
- Strong cryptography — Curve25519 for WireGuard, Ed25519 for invite tokens, `crypto/rand`
- No shell injection — all `exec.Command()` calls use array arguments
- Proper SSH key permissions — private keys stored with `0600`
- Token validation — Ed25519 signatures with TTL expiration
- Clean `.gitignore` — `.env`, kubeconfig, `~/.vibespace/` properly excluded
- Permission system — explicit user approval with request timeouts

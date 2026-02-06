# Remote Mode Implementation Context

This document explains what was implemented across 10 commits, how each relates to `docs/todo/REMOTE_MODE_IMPROVEMENTS.md`, design decisions made, and likely failure points during E2E testing.

## How the Old Flow Worked (Before These Changes)

The old flow required 7 steps with 3 manual exchanges:

1. Server admin: `vibespace serve` (start server)
2. Server admin: `vibespace serve --generate-token` → gives token to client
3. Client: `vibespace remote connect <token>` → outputs public key
4. Client gives public key back to server admin (manual exchange)
5. Server admin: `vibespace serve --add-client <pubkey>` → outputs assigned IP
6. Server admin gives assigned IP back to client (manual exchange)
7. Client: `vibespace remote activate <assigned-ip>` → tunnel comes up

## How the New Flow Works (After These Changes)

Reduced to 2 steps, zero manual exchanges after token:

1. Server admin: `vibespace serve --generate-token` → gives token to client
2. Client: `vibespace remote connect <token>` → DONE (registered, tunnel up, kubeconfig fetched)

The token now carries a TLS cert fingerprint and the server host IP. The client uses these to hit `https://<host>:7781/register` with cert pinning, sends its public key, gets back an IP assignment, then configures everything automatically.

---

## Commit-by-Commit Breakdown

### Commit 1: Fix macOS QuickUp() server bug
**Improvements.md ref:** P0 - Critical Bug
**Problem:** `quickUpMacOS()` at `wireguard.go:260` hardcoded reading from `RemoteState` (client state file `remote.json`) to get the interface IP. When the server called `QuickUp()`, it read the wrong file (client state instead of server state) and failed with "no local IP configured".
**Fix:** Changed `QuickUp()` signature to `QuickUp(address ...string)`. If an address is passed, it's used directly. If not, falls back to reading `RemoteState` (backward compat for client mode). Server now calls `QuickUp(s.state.ServerIP)`.
**Files:** `pkg/remote/wireguard.go`, `pkg/remote/server.go`
**Debug tip:** If server fails to start WireGuard on macOS with "no local IP", the address param isn't being passed. Check `server.go` Start() and reloadWireGuard() calls.

### Commit 2: Fix TLS SAN for WireGuard IP
**Improvements.md ref:** P0 - TLS SAN Bug
**Problem:** k3s generates its API server cert with SANs for 127.0.0.1 and the VM's IP, but NOT for 10.100.0.1 (the WireGuard IP). So when a remote client tries to use kubeconfig pointing to `https://10.100.0.1:6443`, TLS verification fails.
**Fix:** Added `ConfigureK3sTLSSAN()` and `RestartK3s()` to `LimaManager`. During `Start()`, after `limactl start` but before `waitAndCopyKubeconfig`, it writes `/etc/rancher/k3s/config.yaml` with `tls-san: ["10.100.0.1"]` inside the VM, then restarts k3s so it regenerates certs.
**Files:** `internal/platform/lima.go`
**Debug tip:** This only applies to Lima-managed clusters (macOS development). On the VPS, k3s is likely installed directly. If k3s is on the VPS, you need to manually add `--tls-san 10.100.0.1` to `/etc/rancher/k3s/config.yaml` or the k3s systemd service args and restart k3s. The kube API proxy in `server.go` (`startKubeAPIProxy`) proxies 10.100.0.1:6443 → 127.0.0.1:6443 as a workaround, but TLS certs still need the SAN for proper validation.
**Important for VPS testing:** The VPS likely runs k3s natively (not via Lima). The proxy in server.go handles the routing, but if kubectl complains about cert SANs, you may need:
```bash
ssh vibespace-remote 'mkdir -p /etc/rancher/k3s && echo "tls-san:\n  - \"10.100.0.1\"" > /etc/rancher/k3s/config.yaml && systemctl restart k3s'
```

### Commit 3: Self-signed TLS cert generation
**Improvements.md ref:** P1 - Security for Registration
**What:** New file `pkg/remote/tls.go` with two functions:
- `GenerateSelfSignedCert(host)` → ECDSA P-256 cert, 1 year validity, returns PEM cert+key and SHA256 fingerprint
- `PinningTLSConfig(fingerprint)` → returns `*tls.Config` with custom `VerifyPeerCertificate` that checks SHA256 of the DER cert against expected fingerprint. Sets `InsecureSkipVerify: true` because we verify via fingerprint, not CA chain.
**Files:** `pkg/remote/tls.go`
**Debug tip:** If cert fingerprint mismatch errors occur, the server may have regenerated its cert (e.g., after cleanup) but the token still has the old fingerprint. Solution: generate a new token after any cert regeneration. Certs are stored at `~/.vibespace/reg-cert.pem` and `reg-key.pem`.

### Commit 4: One-shot registration (server side)
**Improvements.md ref:** P1 - Registration API
**What:** Added to server.go:
- `RegisterRequest` / `RegisterResponse` types
- `ensureRegistrationCert()` - generates self-signed cert on first run, loads from disk on subsequent runs, stores fingerprint on Server struct
- `startRegistrationAPI()` - HTTPS server on `0.0.0.0:7781` with the self-signed cert
- `handleRegister()` - validates token, validates pubkey format (base64, decodes to 32 bytes), allocates IP, adds peer to WireGuard, returns assignment
- Registration is idempotent: if the same pubkey is already registered, returns existing assignment

Added to state.go:
- `CertFingerprint` (`cf`) and `Host` (`h`) fields on `InviteToken` and `inviteTokenPayload`
- `Hostname` field on `ClientRegistration`
- `DefaultRegistrationPort = 7781` constant

**Files:** `pkg/remote/server.go`, `pkg/remote/state.go`
**Debug tip:** Registration API binds to 0.0.0.0:7781 (public). If connection refused, check: (1) firewall allows TCP 7781, (2) no other service on 7781, (3) serve process is actually running. The `handleRegister` validates the token by calling `DecodeInviteToken` which checks signature + expiration. The invite token is re-encoded by the client and sent in the POST body (not as a header).

### Commit 5: One-shot registration (client side)
**Improvements.md ref:** P1 - Simplified Client Flow
**What:** Rewrote `Connect()` in client.go. Old signature returned `(clientPubKey string, err error)`, new signature returns just `error` since no manual key exchange is needed.

New flow inside `Connect()`:
1. Decode token → extract host, cert fingerprint
2. Install WireGuard if needed
3. Generate keypair
4. `registerWithServer()` → POST to `https://<host>:7781/register` with cert pinning
5. Save state (including assigned IP from response)
6. Write WireGuard client config
7. Install config to /etc/wireguard
8. Save state again (macOS QuickUp reads LocalIP from disk)
9. `QuickUp()`
10. `waitForConnectivity()` → polls /health
11. `FetchKubeconfigFromServer()`
12. Mark connected

`Activate()` has been removed entirely (was deprecated, now deleted).

CLI `remote.go` updated: `runRemoteConnect` no longer outputs pubkey instructions. `remoteActivateCmd` removed.

**Files:** `pkg/remote/client.go`, `internal/cli/remote.go`
**Debug tip:** The `registerWithServer` function extracts the host from `invite.Host` (new field) or falls back to parsing `invite.Endpoint`. If the host in the token is wrong (e.g., a private IP), registration will fail with connection refused. The cert fingerprint in the token MUST match the server's current cert - if they diverge, you get "certificate fingerprint mismatch". `mustEncodeInviteToken` re-encodes the decoded token to send in the POST body - this works because the signature is preserved from the original encoding.

### Commit 6: Auto-reconnect
**Improvements.md ref:** P2 - Reliability
**What:** Added `ConnectionWatcher` struct to client.go:
- Pings `/health` every 15 seconds
- After 3 consecutive failures, triggers reconnect: `QuickDown()` → sleep 2s → `QuickUp()` → `waitForConnectivity()`
- Has `OnDisconnect` / `OnReconnect` callbacks
- `Start()` runs in goroutine, `Stop()` blocks until goroutine exits

**Files:** `pkg/remote/client.go`
**Note:** The watcher is now wired into the CLI as `vibespace remote watch`. It blocks on SIGINT/SIGTERM and prints status messages on disconnect/reconnect. For E2E testing, run `vibespace remote watch` in a terminal, then kill the WireGuard tunnel to verify auto-reconnect.

### Commit 7: Firewall detection + diagnostics
**Improvements.md ref:** P2 - Error Messages
**What:** New file `pkg/remote/diagnostics.go` with:

Client diagnostics (`RunDiagnostics(state)`):
1. WireGuard Interface - `IsInterfaceUp()`
2. UDP Connectivity - `net.DialTimeout("udp", endpoint, 3s)` (note: UDP "connect" doesn't prove reachability, just that local socket works)
3. WireGuard Handshake - parses `wg show <iface> latest-handshakes` output
4. Management API - HTTP GET to `/health`
5. Kubeconfig - file exists and non-empty

Server firewall check (`CheckFirewall()`):
- Tries to bind UDP 51820 and TCP 7781
- If bind fails, outputs actionable messages with ufw/iptables/firewalld/cloud commands

Server.Start() now calls `CheckFirewall()` and logs warnings.

**Files:** `pkg/remote/diagnostics.go`, `pkg/remote/server.go`
**Debug tip:** The handshake check runs `sudo wg show <iface> latest-handshakes` - this requires sudo. If the test user can't sudo, the check will fail with a misleading message. The UDP connectivity check with `net.DialTimeout("udp", ...)` will almost always succeed (UDP connect doesn't require a response) - it mainly catches DNS resolution failures or completely unreachable hosts.

### Commit 8: Enhanced remote status
**Improvements.md ref:** P2 - Better UX
**What:** Rewrote `runRemoteStatus()` in CLI to:
- Run `RunDiagnostics()` when connected
- Display results with `[ok]` / `[!!]` prefixes
- Show overall health summary
- JSON output includes `tunnel_up` bool and `diagnostics` array

Added JSON types: `DiagnosticOutput`, `ClientListOutput`, `ClientOutput`

**Files:** `internal/cli/remote.go`, `internal/cli/json_types.go`

### Commit 9: Client management + graceful disconnect
**Improvements.md ref:** P3 - Management
**What:**

Server (`server.go`):
- `ListClients()` returns `[]ClientRegistration`
- `RemoveClient(nameOrKey)` - finds by name, pubkey, or hostname; removes from state, rewrites WireGuard config, reloads
- `handleDisconnect()` - POST /disconnect on management API, logs client disconnect

Client (`client.go`):
- `Disconnect()` now calls `notifyServerDisconnect()` as fire-and-forget goroutine before tearing down tunnel
- `notifyServerDisconnect()` POSTs pubkey to `/disconnect` with 2s timeout

CLI (`serve.go`):
- `--list-clients` flag → table of clients with name, IP, key prefix, hostname, registration time
- `--remove-client <name>` flag → removes client by name/hostname/pubkey

**Files:** `pkg/remote/server.go`, `pkg/remote/client.go`, `internal/cli/serve.go`
**Debug tip:** `RemoveClient` matches on name, public key, OR hostname. If the client registered with hostname "MacBook-Pro" but you try to remove by "client", it won't match. Use `--list-clients` first to see exact names. The disconnect notification is fire-and-forget - if the tunnel is already down when Disconnect() runs, the notification silently fails (by design).

### Commit 10: DNS resolution
**Improvements.md ref:** P4 - Service Discovery
**What:** New package `pkg/dns/`:

`server.go`:
- `DNSServer` with thread-safe record map
- Handles A record queries for `*.vibespace.internal`
- Default: anything under the domain resolves to 127.0.0.1
- Custom records via `AddRecord(name, ip)` / `RemoveRecord(name)`
- Listens on UDP 127.0.0.1:5553

`resolver.go`:
- macOS: writes `/etc/resolver/vibespace.internal` pointing to 127.0.0.1:5553
- Linux: writes systemd-resolved drop-in at `/etc/systemd/resolved.conf.d/vibespace.conf`
- Both require sudo

Integration with daemon (`pkg/daemon/server.go`):
- DNS server starts on daemon Start()
- DNS server stops on daemon Stop()

**Files:** `pkg/dns/server.go`, `pkg/dns/resolver.go`, `pkg/daemon/server.go`, `go.mod`
**Debug tip:** DNS is integrated with the DAEMON (port-forward manager), not the SERVE command. The daemon manages port forwards for vibespaces. DNS maps e.g., `myagent.vibespace.internal` → 127.0.0.1 so you can access a port-forwarded service by hostname. DNS A records are now automatically added/removed when port-forwards are added/removed. The system resolver is now configured on daemon startup (`ConfigureSystemResolver(5553)`) and removed on shutdown. For testing: `dig @127.0.0.1 -p 5553 anything.vibespace.internal`.

---

## State Files Reference

| File | Machine | Purpose |
|------|---------|---------|
| `~/.vibespace/remote.json` | Client (Mac) | Client connection state |
| `~/.vibespace/serve.json` | Server (VPS) | Server state, registered clients |
| `~/.vibespace/wg-client.key` | Client | WireGuard private key |
| `~/.vibespace/wg-server.key` | Server | WireGuard private key |
| `~/.vibespace/remote-signing.key` | Server | ED25519 signing key for tokens |
| `~/.vibespace/reg-cert.pem` | Server | Registration API TLS cert |
| `~/.vibespace/reg-key.pem` | Server | Registration API TLS key |
| `~/.vibespace/remote_kubeconfig` | Client | Kubeconfig fetched from server |
| `~/.vibespace/serve.pid` | Server | PID of daemonized serve process |
| `~/.vibespace/serve.log` | Server | Stdout/stderr of daemon process |
| `~/.vibespace/utun-name` | Mac only | Saves macOS utun interface name for QuickDown |
| `/etc/wireguard/wg-vibespace.conf` | Both | WireGuard configuration |

## Network Ports

| Port | Proto | Binding | Purpose |
|------|-------|---------|---------|
| 51820 | UDP | 0.0.0.0 (server) | WireGuard tunnel |
| 7780 | TCP | 10.100.0.1 (WG IP only) | Management API (HTTPS, health, kubeconfig, disconnect) |
| 7781 | TCP | 0.0.0.0 (server) | Registration API (HTTPS with self-signed cert) |
| 6443 | TCP | 10.100.0.1 (WG IP only) | Kube API proxy (→ 127.0.0.1:6443) |
| 5553 | UDP | 127.0.0.1 (daemon) | DNS server for *.vibespace.internal |

## Likely E2E Failure Points

1. **VPS doesn't have vibespace binary** - Need to cross-compile and scp it over
2. **VPS firewall blocks 51820/UDP or 7781/TCP** - Check ufw/iptables/cloud SGs
3. **WireGuard not installed on VPS** - Need `apt install wireguard-tools` + `modprobe wireguard`
4. **Server not running when client tries to connect** - Must start `vibespace serve` first
5. **Token expired** - Default 30min TTL, generate fresh token before connecting
6. **Cert fingerprint mismatch** - Happens if server cert was regenerated after token was issued
7. **macOS sudo prompts** - WireGuard operations require sudo (InstallConfig, QuickUp, QuickDown)
8. **k3s not running on VPS** - The kubeconfig fetch will work (server sends its local kubeconfig) but kubectl commands against the cluster will fail if k3s isn't actually running
9. **DNS resolver needs sudo** - `ConfigureSystemResolver()` is called on daemon startup but requires sudo for `/etc/resolver/` (macOS) or systemd-resolved (Linux)
10. **`vibespace serve` needs cluster initialized** - The serve command checks `checkClusterInitialized()` when starting the daemon (not when doing --generate-token or --add-client). If the VPS doesn't have a vibespace cluster initialized, you may need to bypass this or init one first. Token generation and client management don't need the cluster.

# Architecture Reference

## State Files

| File | Machine | Purpose |
|------|---------|---------|
| `~/.vibespace/remote.json` | Client | Client connection state |
| `~/.vibespace/serve.json` | Server | Server state, registered clients |
| `~/.vibespace/wg-client.key` | Client | WireGuard private key |
| `~/.vibespace/wg-server.key` | Server | WireGuard private key |
| `~/.vibespace/remote-signing.key` | Server | ED25519 signing key for tokens |
| `~/.vibespace/reg-cert.pem` | Server | Registration API TLS cert |
| `~/.vibespace/reg-key.pem` | Server | Registration API TLS key |
| `~/.vibespace/remote_kubeconfig` | Client | Kubeconfig fetched from server |
| `~/.vibespace/kubeconfig` | Both | Local cluster kubeconfig |
| `~/.vibespace/serve.pid` | Server | PID of daemonized serve process |
| `~/.vibespace/serve.log` | Server | Stdout/stderr of daemon process |
| `~/.vibespace/utun-name` | macOS | Saves utun interface name for QuickDown |
| `/etc/wireguard/wg-vibespace.conf` | Both | WireGuard configuration |

## Network Ports

| Port | Proto | Binding | Purpose |
|------|-------|---------|---------|
| 51820 | UDP | 0.0.0.0 (server) | WireGuard tunnel |
| 7780 | TCP | 10.100.0.1 (WG only) | Management API (HTTPS) |
| 7781 | TCP | 0.0.0.0 (server) | Registration API (HTTPS, self-signed) |
| 6443 | TCP | 10.100.0.1 (WG only) | Kube API proxy (→ 127.0.0.1:6443) |
| 5553 | UDP | 127.0.0.1 (daemon) | DNS server for *.vibespace.internal |

## WireGuard Subnet

- Network: `10.100.0.0/24`
- Server: `10.100.0.1`
- Clients: `10.100.0.2` and up (sequential allocation)

## Key Packages

| Package | Purpose |
|---------|---------|
| `internal/cli` | Cobra commands, TUI, output handling |
| `internal/platform` | ClusterManager (Lima, Colima, future bare metal) |
| `pkg/remote` | WireGuard, server, client, state, TLS, diagnostics |
| `pkg/dns` | Embedded DNS server + system resolver config |
| `pkg/daemon` | Port forwarding daemon, DNS integration |
| `pkg/agent` | Agent config interface, types |
| `pkg/vibespace` | Vibespace CRUD via k8s client |
| `pkg/ui` | Colors, styles, tables |
| `pkg/errors` | Error types, exit codes |

## Token Format

- Prefix: `vs-`
- Payload: base64url-encoded JSON (endpoint, server pubkey, cert fingerprint)
- Signature: ED25519 (appended to payload)
- Default TTL: 30 minutes

# Remote Mode

<!-- TODO: add demo GIF of serve → connect flow -->

Remote mode runs your vibespace cluster on a VPS instead of your local machine. You connect to it over a WireGuard VPN tunnel. All CLI commands work the same — the CLI detects the tunnel and routes through it automatically.

## Why remote mode

- More CPU, RAM, and disk than your laptop
- Agents keep running when your laptop is closed
- Access from multiple machines
- Better for heavy workloads or many agents

## Requirements

**Server (VPS):**
- Linux (Ubuntu 20.04+ recommended)
- Root access
- Public IP address
- Port 51820/UDP open (WireGuard)
- Port 7781/TCP open (registration API)
- vibespace binary installed

**Client (your machine):**
- macOS or Linux
- sudo access (for WireGuard tunnel setup)

## Server setup

SSH into your VPS and install vibespace. Then initialize and start serving:

```bash
# On the VPS
vibespace init --bare-metal
vibespace serve
```

This starts a background daemon that runs WireGuard and the management API. The server auto-detects its public IP.

Generate an invite token for your client:

```bash
vibespace serve --generate-token
```

The token looks like `vs-<base64>...` and expires in 30 minutes by default. Change the TTL with `--token-ttl`:

```bash
vibespace serve --generate-token --token-ttl 1h
```

## Client connection

On your local machine, connect using the token:

```bash
sudo vibespace remote connect <token>
```

Sudo is required because WireGuard needs to create a network interface. This is a one-time setup — the tunnel persists across reboots until you disconnect.

Once connected, every vibespace command targets the remote cluster:

```bash
vibespace create my-project -t claude-code -s
vibespace list
vibespace connect --vibespace my-project
```

## Managing the connection

```bash
# Check connection status and run diagnostics
vibespace remote status

# Disconnect and tear down the tunnel
vibespace remote disconnect

# Auto-reconnect on drops (blocks until interrupted)
vibespace remote watch
```

## Managing clients

On the server:

```bash
# List all registered clients
vibespace serve --list-clients

# Remove a client
vibespace serve --remove-client <name-or-hostname>
```

## How it works

1. `vibespace serve` starts WireGuard on `10.100.0.1` and listens for registrations on port 7781 (self-signed TLS with cert pinning)
2. The invite token contains the server endpoint, WireGuard public key, and TLS certificate fingerprint
3. `vibespace remote connect` verifies the token signature, registers with the server, and sets up a WireGuard tunnel
4. The client gets an IP in `10.100.0.0/24` (starting at `10.100.0.2`)
5. The management API on port 7780 is only accessible over the WireGuard tunnel
6. A kubeconfig is fetched from the server and stored at `~/.vibespace/remote_kubeconfig`

## Network layout

| Port | Protocol | Binding | Purpose |
|---|---|---|---|
| 51820 | UDP | Public | WireGuard tunnel |
| 7780 | TCP | WireGuard only (10.100.0.1) | Management API |
| 7781 | TCP | Public | Registration API (one-time use per client) |
| 6443 | TCP | WireGuard only (10.100.0.1) | Kubernetes API proxy |

## Troubleshooting

**Connection times out:** Check that ports 51820/UDP and 7781/TCP are open in your VPS firewall.

**Token rejected:** Tokens expire (default 30 minutes). Generate a new one.

**Tunnel up but commands fail:** Run `vibespace remote status` to check diagnostics. The status command tests ping, DNS, and API reachability.

**WireGuard not found:** On macOS, WireGuard tools are bundled. On Linux, install with `sudo apt-get install wireguard-tools`.

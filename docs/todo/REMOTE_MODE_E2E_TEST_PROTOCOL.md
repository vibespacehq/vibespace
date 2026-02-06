# Remote Mode E2E Test Protocol

## Environment

| Machine | Role | Access |
|---------|------|--------|
| Local Mac | Client | Direct terminal |
| Remote VPS | Server | `ssh vibeuser` (user) / `ssh vibespace-remote` (root) |

## Prerequisites

- vibespace binary built from current `main` branch
- The VPS has a public IP reachable from the Mac
- VPS has port 51820/UDP and 7781/TCP open (or we verify firewall detection catches it)

---

## Phase 0: Build & Deploy

### 0.1 Build vibespace binary for both platforms

```bash
# On Mac - build for local use
cd ~/Desktop/repos/vibespace
go build -o ~/.vibespace/bin/vibespace ./cmd/vibespace

# Cross-compile for Linux (VPS)
GOOS=linux GOARCH=amd64 go build -o /tmp/vibespace-linux ./cmd/vibespace
```

### 0.2 Deploy binary to VPS

```bash
scp /tmp/vibespace-linux vibespace-remote:/usr/local/bin/vibespace
ssh vibespace-remote 'chmod +x /usr/local/bin/vibespace'
ssh vibespace-remote 'vibespace version'
```

---

## Phase 1: Full Cleanup (Both Machines)

### 1.1 Clean up LOCAL (Mac)

```bash
# Stop any running WireGuard
sudo wg show 2>/dev/null && {
  # Read utun name if exists
  UTUN=$(cat ~/.vibespace/utun-name 2>/dev/null)
  if [ -n "$UTUN" ]; then
    sudo rm -f /var/run/wireguard/${UTUN}.sock
  fi
}

# Remove WireGuard config
sudo rm -f /etc/wireguard/wg-vibespace.conf

# Remove DNS resolver config
sudo rm -f /etc/resolver/vibespace.internal

# Remove all vibespace remote state (keep bin/ and other non-remote files)
rm -f ~/.vibespace/remote.json
rm -f ~/.vibespace/remote_kubeconfig
rm -f ~/.vibespace/wg-client.key
rm -f ~/.vibespace/utun-name
rm -f ~/.vibespace/wg-vibespace.conf

# Verify clean state
echo "--- Local cleanup verification ---"
cat ~/.vibespace/remote.json 2>/dev/null || echo "remote.json: CLEAN"
ls /etc/wireguard/wg-vibespace.conf 2>/dev/null || echo "wg config: CLEAN"
sudo wg show 2>/dev/null || echo "wg interfaces: CLEAN"
ls /etc/resolver/vibespace.internal 2>/dev/null || echo "DNS resolver: CLEAN"
```

### 1.2 Clean up REMOTE (VPS)

```bash
ssh vibespace-remote bash <<'REMOTE_CLEANUP'
set -e

# Stop any running serve process
SERVE_PID=$(cat ~/.vibespace/serve.pid 2>/dev/null)
if [ -n "$SERVE_PID" ] && kill -0 "$SERVE_PID" 2>/dev/null; then
  echo "Killing serve process $SERVE_PID"
  kill "$SERVE_PID" 2>/dev/null || true
  sleep 2
fi

# Bring down WireGuard
wg-quick down wg-vibespace 2>/dev/null || true

# Remove WireGuard config
rm -f /etc/wireguard/wg-vibespace.conf

# Remove ALL vibespace state
rm -f ~/.vibespace/serve.json
rm -f ~/.vibespace/serve.pid
rm -f ~/.vibespace/serve.log
rm -f ~/.vibespace/wg-server.key
rm -f ~/.vibespace/remote-signing.key
rm -f ~/.vibespace/reg-cert.pem
rm -f ~/.vibespace/reg-key.pem
rm -f ~/.vibespace/wg-vibespace.conf
rm -f ~/.vibespace/utun-name

# Verify clean state
echo "--- Remote cleanup verification ---"
cat ~/.vibespace/serve.json 2>/dev/null || echo "serve.json: CLEAN"
ls /etc/wireguard/wg-vibespace.conf 2>/dev/null || echo "wg config: CLEAN"
wg show 2>/dev/null || echo "wg interfaces: CLEAN"
ss -tulnp | grep -E '51820|7780|7781' || echo "ports: CLEAN"
REMOTE_CLEANUP
```

---

## Phase 2: Test Firewall Detection (Commit 7)

### 2.1 Verify firewall warnings on server start attempt

```bash
# If the VPS has a firewall, temporarily block port 51820 to test detection
# (Skip this if ports are already open - the test is that warnings appear)

ssh vibespace-remote 'vibespace serve --foreground' &
SERVE_PID=$!
sleep 5
kill $SERVE_PID 2>/dev/null

# CHECK: Look at output for firewall check warnings like:
#   "firewall check failed" for ports 51820/UDP or 7781/TCP
# If ports are open, checks should pass silently
```

### 2.2 Ensure required ports are open on VPS

```bash
# Verify/open ports (adjust for your firewall)
ssh vibespace-remote bash <<'FIREWALL'
# For ufw:
which ufw >/dev/null 2>&1 && {
  ufw allow 51820/udp
  ufw allow 7781/tcp
  ufw status | grep -E '51820|7781'
}

# For iptables (if no ufw):
# iptables -A INPUT -p udp --dport 51820 -j ACCEPT
# iptables -A INPUT -p tcp --dport 7781 -j ACCEPT

# For firewalld:
# firewall-cmd --add-port=51820/udp --permanent
# firewall-cmd --add-port=7781/tcp --permanent
# firewall-cmd --reload

# Verify ports are bindable
echo "Port check:"
ss -tulnp | grep -E '51820|7781' && echo "PORTS IN USE" || echo "PORTS FREE"
FIREWALL
```

---

## Phase 3: Server Startup (Commits 1, 3, 4)

### 3.1 Install WireGuard on VPS (if needed)

```bash
ssh vibespace-remote bash <<'INSTALL_WG'
which wg >/dev/null 2>&1 && echo "WireGuard already installed" || {
  apt-get update && apt-get install -y wireguard-tools
  modprobe wireguard
}
wg --version
INSTALL_WG
```

### 3.2 Initialize and start the server

```bash
# Start server in foreground (in a separate terminal or backgrounded)
ssh vibespace-remote 'vibespace serve --foreground' &
VPS_SERVE_PID=$!
sleep 5

# CHECK: Server should output:
#   - "starting management API" on 10.100.0.1:7780 with tls=true
#   - "starting registration API" on 0.0.0.0:7781 with fingerprint
```

### 3.3 Verify server state

```bash
ssh vibespace-remote bash <<'VERIFY_SERVER'
echo "=== WireGuard Interface ==="
wg show

echo ""
echo "=== Server State ==="
cat ~/.vibespace/serve.json | python3 -m json.tool 2>/dev/null || cat ~/.vibespace/serve.json

echo ""
echo "=== Listening Ports ==="
ss -tulnp | grep -E '51820|7780|7781'

echo ""
echo "=== Registration Cert ==="
ls -la ~/.vibespace/reg-cert.pem ~/.vibespace/reg-key.pem

echo ""
echo "=== WireGuard Config ==="
cat /etc/wireguard/wg-vibespace.conf
VERIFY_SERVER
```

### 3.4 Verify management API is HTTPS

```bash
# Should fail without -k (self-signed cert)
ssh vibespace-remote 'curl https://10.100.0.1:7780/health 2>&1 | head -3'
# CHECK: TLS error (self-signed cert rejected)

# Should succeed with -k
ssh vibespace-remote 'curl -k https://10.100.0.1:7780/health 2>&1'
# CHECK: {"status":"ok"}

# Verify security headers
ssh vibespace-remote 'curl -k -I https://10.100.0.1:7780/health 2>&1 | grep -i "x-content-type\|x-frame\|cache-control\|content-security"'
# CHECK: X-Content-Type-Options: nosniff, X-Frame-Options: DENY, etc.
```

**Expected:**
- WireGuard interface is up with server IP 10.100.0.1/24
- serve.json has `running: true`, public key, signing key
- Ports 51820/UDP, 7780/TCP, 7781/TCP are listening
- reg-cert.pem and reg-key.pem exist
- WireGuard config has the server private key and address
- Management API responds over HTTPS with security headers

---

## Phase 4: Token Generation (Commit 4)

### 4.1 Generate invite token

```bash
# Get the VPS public IP first
VPS_IP=$(ssh vibespace-remote 'curl -s https://api.ipify.org')
echo "VPS IP: $VPS_IP"

# Generate token with explicit endpoint
TOKEN=$(ssh vibespace-remote "vibespace serve --generate-token --endpoint $VPS_IP --json" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")
echo "Token: $TOKEN"
```

### 4.2 Verify token contents

```bash
# Decode and inspect the token (strip vs- prefix, base64 decode)
echo "$TOKEN" | sed 's/^vs-//' | base64 -d 2>/dev/null | python3 -m json.tool

# CHECK: Token JSON should contain:
#   "k"   - server WireGuard public key
#   "e"   - endpoint (VPS_IP:51820)
#   "s"   - server WireGuard IP (10.100.0.1)
#   "exp" - expiration timestamp
#   "n"   - nonce
#   "spk" - signing public key
#   "sig" - signature
#   "cf"  - cert fingerprint (sha256:...)  <-- NEW
#   "h"   - host IP                        <-- NEW
```

### 4.3 Test token with JSON output

```bash
ssh vibespace-remote "vibespace serve --generate-token --endpoint $VPS_IP --json"

# CHECK: JSON output has token and expires_in fields
```

---

## Phase 5: One-Shot Client Connect (Commit 5)

### 5.1 Connect from Mac using the token

```bash
# This is the big one - should do everything in one step
vibespace remote connect "$TOKEN"

# CHECK: Output should show:
#   "Connecting to remote server..."
#   "Connected to remote server"
#   Server: <VPS_IP>:51820
#   Local IP: 10.100.0.2/32
#   Server IP: 10.100.0.1
```

### 5.2 Verify client state

```bash
echo "=== Remote State ==="
cat ~/.vibespace/remote.json | python3 -m json.tool

echo ""
echo "=== WireGuard Interface ==="
sudo wg show

echo ""
echo "=== Kubeconfig ==="
ls -la ~/.vibespace/remote_kubeconfig
head -5 ~/.vibespace/remote_kubeconfig

echo ""
echo "=== Ping Server ==="
ping -c 3 10.100.0.1
```

**Expected:**
- remote.json shows `connected: true`, local_ip, server_ip, public keys, `cert_fingerprint`
- WireGuard interface is up on Mac (utun device)
- remote_kubeconfig exists with server address pointing to 10.100.0.1
- Ping to 10.100.0.1 succeeds

### 5.3 Verify server saw the registration

```bash
ssh vibespace-remote bash <<'CHECK_REG'
echo "=== Server State (clients) ==="
cat ~/.vibespace/serve.json | python3 -c "
import json, sys
data = json.load(sys.stdin)
for c in data.get('clients', []):
    print(f\"  Name: {c['name']}\")
    print(f\"  IP: {c['assigned_ip']}\")
    print(f\"  Hostname: {c.get('hostname', 'N/A')}\")
    print(f\"  Key: {c['public_key'][:16]}...\")
    print()
"

echo "=== WireGuard Peers ==="
wg show wg-vibespace peers
CHECK_REG
```

### 5.4 Test idempotent re-connect (should fail with already-connected)

```bash
vibespace remote connect "$TOKEN" 2>&1

# CHECK: Should fail with "already connected" error
```

---

## Phase 6: Enhanced Status with Diagnostics (Commit 8)

### 6.1 Human-readable status

```bash
vibespace remote status

# CHECK: Should show:
#   Remote: connected (in teal)
#   Server: <endpoint>
#   Local IP: 10.100.0.2/32
#   Server IP: 10.100.0.1
#   Connected at: <timestamp>
#   Tunnel: active (in green)
#
#   Diagnostics:
#     [ok] WireGuard Interface: Interface is up
#     [ok] UDP Connectivity: Can reach <endpoint>
#     [ok] WireGuard Handshake: Last handshake X ago
#     [ok] Management API: Server API is reachable
#     [ok] Kubeconfig: Kubeconfig exists (N bytes)
#
#   Health: all checks passed
```

### 6.2 JSON status

```bash
vibespace remote status --json | python3 -m json.tool

# CHECK: JSON should include:
#   connected: true
#   tunnel_up: true
#   diagnostics: array of {check, status, message}
```

---

## Phase 7: TLS SAN Verification (Commit 2)

### 7.1 Test kubeconfig connectivity

```bash
# The kubeconfig should point to 10.100.0.1:6443
# If k3s is running on the VPS with our TLS SAN, this should work:
KUBECONFIG=~/.vibespace/remote_kubeconfig kubectl cluster-info 2>&1

# If k3s is not running, at least verify the kubeconfig has the right address:
grep 'server:' ~/.vibespace/remote_kubeconfig

# CHECK: server should be https://10.100.0.1:6443
```

### 7.2 Verify TLS SAN on VPS (if k3s is running)

```bash
ssh vibespace-remote bash <<'TLS_CHECK'
if [ -f /etc/rancher/k3s/config.yaml ]; then
  echo "=== k3s config ==="
  cat /etc/rancher/k3s/config.yaml
  # Should contain tls-san: ["10.100.0.1"]
fi

# Check cert SANs directly
if [ -f /var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt ]; then
  echo "=== API Server Cert SANs ==="
  openssl x509 -in /var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt -noout -text | grep -A5 "Subject Alternative"
fi
TLS_CHECK
```

---

## Phase 8: Client Management (Commit 9)

### 8.1 List clients

```bash
ssh vibespace-remote 'vibespace serve --list-clients'

# CHECK: Should show our Mac client with:
#   Name, IP (10.100.0.2/32), Key prefix, Registration time
```

### 8.2 List clients (JSON)

```bash
ssh vibespace-remote 'vibespace serve --list-clients --json' | python3 -m json.tool

# CHECK: JSON has clients array with count
```

---

## Phase 9: Auto-Reconnect via `remote watch` (Commit 6)

### 9.1 Start the watcher

```bash
# In a separate terminal:
vibespace remote watch

# CHECK: Output should show:
#   "Watching tunnel to <server>..."
#   "Press Ctrl-C to stop."
```

### 9.2 Simulate tunnel drop and verify auto-reconnect

```bash
# In another terminal, kill the tunnel:
UTUN=$(cat ~/.vibespace/utun-name)
echo "Interface: $UTUN"
sudo rm -f /var/run/wireguard/${UTUN}.sock
sleep 2

# Verify it's down
ping -c 1 -W 2 10.100.0.1  # Should fail

# CHECK: The watch terminal should show:
#   [!!] Tunnel lost, attempting reconnect...
#   ... (after ~45 seconds of health check failures)
#   [ok] Tunnel restored

# After reconnect, verify:
ping -c 3 10.100.0.1
# CHECK: Ping succeeds again
```

### 9.3 Stop the watcher

```bash
# In the watch terminal, press Ctrl-C
# CHECK: "Stopping watcher..." and clean exit
```

### 9.4 Manual reconnect fallback (if watcher not used)

```bash
# Alternative: manual disconnect/reconnect
vibespace remote disconnect
vibespace remote connect "$TOKEN"
vibespace remote status
# CHECK: Should reconnect successfully
```

---

## Phase 10: Graceful Disconnect (Commit 9)

### 10.1 Disconnect and verify cleanup

```bash
vibespace remote disconnect

# CHECK output:
#   "Disconnecting from remote server..."
#   "Disconnected from remote server"
```

### 10.2 Verify local cleanup

```bash
echo "=== Remote State ==="
cat ~/.vibespace/remote.json | python3 -m json.tool
# CHECK: connected: false, all fields cleared

echo ""
echo "=== WireGuard ==="
sudo wg show 2>&1
# CHECK: No interfaces

echo ""
echo "=== Kubeconfig ==="
ls ~/.vibespace/remote_kubeconfig 2>&1
# CHECK: File should be gone

echo ""
echo "=== Client Key ==="
ls ~/.vibespace/wg-client.key 2>&1
# CHECK: File should be gone
```

### 10.3 Verify server received disconnect notification

```bash
# Check server logs for disconnect message
ssh vibespace-remote 'tail -20 ~/.vibespace/serve.log 2>/dev/null | grep -i disconnect'

# CHECK: Should see "client disconnected" log entry
```

---

## Phase 11: Client Removal (Commit 9)

### 11.1 Reconnect first (so we have a client to remove)

```bash
# Generate fresh token (old one may have expired)
TOKEN2=$(ssh vibespace-remote "vibespace serve --generate-token --endpoint $VPS_IP --json" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")
vibespace remote connect "$TOKEN2"
vibespace remote status  # Verify connected
```

### 11.2 Remove the client from server

```bash
# List clients to get the name
ssh vibespace-remote 'vibespace serve --list-clients'

# Remove by hostname (or name)
HOSTNAME=$(hostname)
ssh vibespace-remote "vibespace serve --remove-client $HOSTNAME"

# CHECK: "Client removed" success message
```

### 11.3 Verify client is gone

```bash
ssh vibespace-remote 'vibespace serve --list-clients'
# CHECK: No clients listed (or our client is gone)

# The Mac-side tunnel should still be "up" but non-functional
# since the server no longer has our peer
ping -c 1 -W 2 10.100.0.1
# CHECK: Should fail (server dropped our peer)

# Clean up client side
vibespace remote disconnect
```

---

## Phase 12: DNS Resolution (Commit 10)

Note: DNS is integrated with the **daemon** (port-forward manager), not the serve command.
The daemon now auto-configures the system resolver on startup and removes it on shutdown.
DNS A records are automatically added/removed when port-forwards are added/removed.

### 12.1 Verify DNS server starts with daemon

```bash
# Start the daemon (requires a vibespace to be initialized locally)
vibespace daemon start

# CHECK: Daemon starts, DNS server listens on 127.0.0.1:5553
# The system resolver should be auto-configured:
ls -la /etc/resolver/vibespace.internal
# CHECK: File exists with nameserver 127.0.0.1 and port 5553

# Quick test with dig:
dig @127.0.0.1 -p 5553 test.vibespace.internal A +short
# CHECK: Should return 127.0.0.1 (default for any *.vibespace.internal)
```

### 12.2 Verify DNS records track port-forwards

```bash
# Add a port-forward (if a vibespace is running)
# DNS record should be auto-added for the agent name
vibespace daemon add-forward <agent-name>

dig @127.0.0.1 -p 5553 <agent-name>.vibespace.internal A +short
# CHECK: Returns 127.0.0.1

# Remove the port-forward
vibespace daemon remove-forward <agent-name>

dig @127.0.0.1 -p 5553 <agent-name>.vibespace.internal A +short
# CHECK: Still returns 127.0.0.1 (default catch-all)
```

### 12.3 Verify resolver cleanup on daemon stop

```bash
vibespace daemon stop

ls /etc/resolver/vibespace.internal 2>&1
# CHECK: File should be gone (RemoveSystemResolver called on shutdown)
```

---

## Phase 13: Token Expiration Test

### 13.1 Generate short-lived token and let it expire

```bash
# Generate token with 10-second TTL
SHORT_TOKEN=$(ssh vibespace-remote "vibespace serve --generate-token --endpoint $VPS_IP --token-ttl 10s --json" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")
echo "Short token: $SHORT_TOKEN"

# Wait for expiration
echo "Waiting 15 seconds for token to expire..."
sleep 15

# Try to connect with expired token
vibespace remote connect "$SHORT_TOKEN" 2>&1

# CHECK: Should fail with "invite token expired" error
```

---

## Phase 14: Rate Limiting on Registration (Commit 4 security)

### 14.1 Verify rate limiting kicks in

```bash
# Fire rapid requests at the registration endpoint
# Rate limit is 5 req/min with burst of 3
for i in $(seq 1 8); do
  STATUS=$(ssh vibespace-remote "curl -k -s -o /dev/null -w '%{http_code}' -X POST https://localhost:7781/register -d '{}' 2>/dev/null")
  echo "Request $i: HTTP $STATUS"
done

# CHECK: First few requests return 400 (bad token) or 200
# After burst is exceeded, should return 429 (Too Many Requests)
```

---

## Phase 15: Full Cleanup (Post-Test)

### 15.1 Stop server on VPS

```bash
# If running in foreground, Ctrl+C the terminal
# If running as daemon:
ssh vibespace-remote bash <<'STOP'
PID=$(cat ~/.vibespace/serve.pid 2>/dev/null)
if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
  kill "$PID"
  echo "Server stopped (PID $PID)"
else
  echo "No running server found"
fi
STOP

# Or if serve is still in foreground from Phase 3:
kill $VPS_SERVE_PID 2>/dev/null
```

### 15.2 Clean up both machines (same as Phase 1)

```bash
# Run Phase 1.1 (local cleanup)
# Run Phase 1.2 (remote cleanup)
```

---

## Results Checklist

| # | Test | Expected | Pass? |
|---|------|----------|-------|
| 3.2 | Server starts with WireGuard + mgmt API + registration API | All 3 listening | |
| 3.4 | Management API uses HTTPS with security headers | TLS + headers present | |
| 4.2 | Token contains `cf` (cert fingerprint) and `h` (host) | Both fields present | |
| 5.1 | One-shot connect (token → connected in 1 command) | Connected, tunnel up | |
| 5.2 | Client state includes `cert_fingerprint`, ping works | All green | |
| 5.3 | Server shows registered client with hostname | Client in list | |
| 5.4 | Re-connect fails with already-connected | Error returned | |
| 6.1 | Status shows diagnostics with all checks passing | 5/5 checks pass | |
| 6.2 | JSON status includes diagnostics array | Array present | |
| 7.1 | Kubeconfig points to 10.100.0.1:6443 | Correct address | |
| 8.1 | List clients shows Mac client | Client displayed | |
| 9.1 | `remote watch` starts and monitors tunnel | Watching output | |
| 9.2 | Tunnel drop → auto-reconnect via watcher | Re-established | |
| 10.1 | Graceful disconnect succeeds | Clean output | |
| 10.2 | All local state cleaned up | Files removed | |
| 11.2 | Remove client from server | Client gone from list | |
| 11.3 | Removed client can't reach server | Ping fails | |
| 12.1 | DNS auto-configures on daemon start | Resolver file created | |
| 12.3 | DNS resolver cleaned up on daemon stop | Resolver file removed | |
| 13.1 | Expired token rejected | Token expired error | |
| 14.1 | Rate limiting returns 429 after burst exceeded | 429 status code | |

---

## Troubleshooting

### WireGuard won't come up on Mac
```bash
# Check if wireguard-go and wg are installed
ls -la ~/.vibespace/bin/wg ~/.vibespace/bin/wireguard-go
# If missing, install:
vibespace serve  # triggers install, or manually:
# vibespace remote connect <token> also triggers install
```

### Registration fails with "connection refused"
```bash
# Verify registration API is running on VPS
ssh vibespace-remote 'ss -tlnp | grep 7781'
# Verify firewall allows TCP 7781
ssh vibespace-remote 'curl -k https://localhost:7781/register 2>&1'
```

### "certificate fingerprint mismatch"
```bash
# The cert on server may have been regenerated
# Delete old cert and restart server:
ssh vibespace-remote 'rm ~/.vibespace/reg-cert.pem ~/.vibespace/reg-key.pem'
# Then restart serve and generate a new token
```

### Tunnel up but no connectivity
```bash
# Check handshake status
sudo ~/.vibespace/bin/wg show
# If "latest handshake" is missing, UDP 51820 may be blocked
# Check from Mac:
nc -u -z -w 3 <VPS_IP> 51820 && echo "UDP open" || echo "UDP blocked"
```

### macOS QuickUp fails with "no local IP configured"
```bash
# This was the P0 bug fixed in Commit 1
# If still happening, check that remote.json has local_ip set:
cat ~/.vibespace/remote.json | grep local_ip
```

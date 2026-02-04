# Remote Mode Improvements

This document outlines planned improvements to Vibespace's remote mode implementation based on architectural analysis and comparison with similar tools (Netclode, Tailscale, Nebula).

## Current State

### What We Have

| Component | Status | Implementation |
|-----------|--------|----------------|
| WireGuard key exchange | ✅ Done | Curve25519 keypairs |
| Signed invite tokens | ✅ Done | ED25519 signatures with TTL |
| IP allocation | ✅ Done | Sequential 10.100.0.x pool |
| Kubeconfig proxy | ✅ Done | Rewrites server URL to VPN IP |
| macOS support | ✅ Done | Bundles wireguard-go + wg |
| Linux support | ✅ Done | System WireGuard tools |
| State persistence | ✅ Done | JSON files in ~/.vibespace/ |
| Daemonization | ✅ Done | Background server process |

### Current Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Server (VPS)                                                │
│                                                             │
│  vibespace serve                                            │
│  ├── WireGuard interface (10.100.0.1/24)                   │
│  ├── Management API (10.100.0.1:7780) - private            │
│  ├── KubeAPI Proxy (10.100.0.1:6443) - private             │
│  └── WireGuard listener (0.0.0.0:51820) - public           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ WireGuard tunnel
                           │
┌─────────────────────────────────────────────────────────────┐
│ Client (Laptop)                                             │
│                                                             │
│  vibespace remote connect                                   │
│  ├── WireGuard interface (10.100.0.x/32)                   │
│  ├── Kubeconfig pointing to 10.100.0.1:6443                │
│  └── State in ~/.vibespace/remote.json                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Current Connection Flow (Too Many Steps)

```
1. Server: vibespace serve --generate-token    → outputs token
2. Admin shares token with client              → manual
3. Client: vibespace remote connect <token>    → outputs client pubkey
4. Client shares pubkey with admin             → manual
5. Server: vibespace serve --add-client <key>  → outputs assigned IP
6. Admin shares IP with client                 → manual
7. Client: vibespace remote activate <ip>      → tunnel established
```

**Problem:** 7 steps with 3 manual exchanges. Too much friction.

---

## Planned Improvements

### Priority 0: Fix TLS SAN for WireGuard IP

**Problem:** Remote clients cannot access the Kubernetes API because the k3s API server certificate doesn't include the WireGuard IP (`10.100.0.1`) in its Subject Alternative Names (SANs). When the kubeconfig is rewritten to point to `10.100.0.1:6443`, TLS verification fails.

**Solution:** During cluster setup/start, configure k3s to include the WireGuard IP in its certificate SANs.

```go
// internal/platform/lima.go

const wireguardServerIP = "10.100.0.1"

// configureK3sTLSSAN adds the WireGuard IP to k3s TLS SANs
func (m *LimaManager) configureK3sTLSSAN(ctx context.Context) error {
    currentPath := os.Getenv("PATH")
    newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

    // k3s config file inside the VM
    k3sConfigPath := "/etc/rancher/k3s/config.yaml"

    // Create k3s config with tls-san
    configContent := fmt.Sprintf(`tls-san:
  - %s
`, wireguardServerIP)

    // Write config inside Lima VM
    writeCmd := exec.CommandContext(ctx, m.limactlBin(), "shell", limaInstanceName, "--",
        "sudo", "mkdir", "-p", "/etc/rancher/k3s")
    writeCmd.Env = append(os.Environ(), "PATH="+newPath)
    if err := writeCmd.Run(); err != nil {
        return fmt.Errorf("failed to create k3s config dir: %w", err)
    }

    // Write the config file
    writeConfigCmd := exec.CommandContext(ctx, m.limactlBin(), "shell", limaInstanceName, "--",
        "sudo", "tee", k3sConfigPath)
    writeConfigCmd.Env = append(os.Environ(), "PATH="+newPath)
    writeConfigCmd.Stdin = strings.NewReader(configContent)
    if err := writeConfigCmd.Run(); err != nil {
        return fmt.Errorf("failed to write k3s config: %w", err)
    }

    slog.Debug("k3s TLS SAN configured", "ip", wireguardServerIP)
    return nil
}

// restartK3s restarts k3s to regenerate certificates with new SANs
func (m *LimaManager) restartK3s(ctx context.Context) error {
    currentPath := os.Getenv("PATH")
    newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

    // Restart k3s service
    restartCmd := exec.CommandContext(ctx, m.limactlBin(), "shell", limaInstanceName, "--",
        "sudo", "systemctl", "restart", "k3s")
    restartCmd.Env = append(os.Environ(), "PATH="+newPath)
    if err := restartCmd.Run(); err != nil {
        return fmt.Errorf("failed to restart k3s: %w", err)
    }

    slog.Debug("k3s restarted for certificate regeneration")
    return nil
}
```

**Integration with Start:**

```go
// In LimaManager.Start(), after VM is created but before copying kubeconfig:

func (m *LimaManager) Start(ctx context.Context, config ClusterConfig) error {
    // ... existing create and start code ...

    // Configure TLS SAN for remote mode before k3s generates its cert
    if err := m.configureK3sTLSSAN(ctx); err != nil {
        slog.Warn("failed to configure k3s TLS SAN", "error", err)
        // Don't fail - remote mode just won't work
    }

    // Restart k3s to pick up new config and regenerate cert
    if err := m.restartK3s(ctx); err != nil {
        slog.Warn("failed to restart k3s", "error", err)
    }

    // Wait for and copy kubeconfig
    if err := m.waitAndCopyKubeconfig(ctx); err != nil {
        return fmt.Errorf("failed to copy kubeconfig: %w", err)
    }

    // ...
}
```

**Alternative approach (for existing clusters):**

If the cluster already exists, we need a way to add the TLS SAN without recreating:

```go
// pkg/remote/server.go

func (s *Server) ensureK3sTLSSAN() error {
    // Check if we're on the server (Lima host)
    mgr := platform.GetClusterManager()
    limaMgr, ok := mgr.(*platform.LimaManager)
    if !ok {
        return nil // Not Lima, skip
    }

    // Configure and restart
    ctx := context.Background()
    if err := limaMgr.configureK3sTLSSAN(ctx); err != nil {
        return err
    }
    if err := limaMgr.restartK3s(ctx); err != nil {
        return err
    }

    // Re-copy kubeconfig since cert changed
    return limaMgr.RefreshKubeconfig(ctx)
}
```

---

### Priority 1: One-Shot Registration

**Goal:** Reduce connection flow to 2 steps.

```
1. Server: vibespace serve --generate-token    → outputs token
2. Client: vibespace remote connect <token>    → done!
```

**Implementation:** Add a public registration endpoint.

```go
// pkg/remote/server.go

// Public registration API (accessible before VPN is established)
// Listens on public IP, validates token, auto-registers client
func (s *Server) startRegistrationAPI() {
    mux := http.NewServeMux()
    mux.HandleFunc("/register", s.handleRegister)

    // HTTPS with self-signed cert or ACME
    go http.ListenAndServeTLS(":7781", certFile, keyFile, mux)
}

type RegisterRequest struct {
    Token     string `json:"token"`
    PublicKey string `json:"public_key"`
    Hostname  string `json:"hostname,omitempty"`
}

type RegisterResponse struct {
    AssignedIP      string `json:"assigned_ip"`
    ServerPublicKey string `json:"server_public_key"`
    ServerEndpoint  string `json:"server_endpoint"`
    ServerIP        string `json:"server_ip"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
    var req RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // 1. Verify token signature and expiration
    invite, err := ParseAndVerifyToken(req.Token, s.signingKey)
    if err != nil {
        http.Error(w, "invalid or expired token", http.StatusUnauthorized)
        return
    }

    // 2. Validate public key format
    if !isValidWireGuardKey(req.PublicKey) {
        http.Error(w, "invalid public key format", http.StatusBadRequest)
        return
    }

    // 3. Check if already registered
    if s.isKeyRegistered(req.PublicKey) {
        // Return existing assignment
        existing := s.getClientByKey(req.PublicKey)
        json.NewEncoder(w).Encode(RegisterResponse{
            AssignedIP:      existing.IP,
            ServerPublicKey: s.publicKey,
            ServerEndpoint:  s.endpoint,
            ServerIP:        s.serverIP,
        })
        return
    }

    // 4. Allocate IP
    ip := s.allocateNextIP()

    // 5. Add to WireGuard config
    s.addPeer(req.PublicKey, ip)

    // 6. Reload WireGuard
    if err := s.reloadWireGuard(); err != nil {
        http.Error(w, "failed to update wireguard", http.StatusInternalServerError)
        return
    }

    // 7. Persist state
    s.state.Clients = append(s.state.Clients, ClientInfo{
        PublicKey: req.PublicKey,
        IP:        ip,
        Hostname:  req.Hostname,
        AddedAt:   time.Now(),
    })
    s.saveState()

    // 8. Return connection details
    json.NewEncoder(w).Encode(RegisterResponse{
        AssignedIP:      ip,
        ServerPublicKey: s.publicKey,
        ServerEndpoint:  s.endpoint,
        ServerIP:        s.serverIP,
    })
}
```

**Updated client flow:**

```go
// pkg/remote/client.go

func (c *Client) Connect(token string) error {
    // 1. Parse token (no verification yet, just extract endpoint)
    invite, err := ParseToken(token)
    if err != nil {
        return fmt.Errorf("invalid token format: %w", err)
    }

    // 2. Generate local keypair
    privateKey, publicKey, err := generateWireGuardKeypair()
    if err != nil {
        return fmt.Errorf("failed to generate keypair: %w", err)
    }

    // 3. Register with server
    regURL := fmt.Sprintf("https://%s:7781/register", invite.Host)
    resp, err := c.register(regURL, token, publicKey)
    if err != nil {
        return fmt.Errorf("registration failed: %w", err)
    }

    // 4. Save state
    c.state = &RemoteState{
        Connected:       false,
        ServerHost:      invite.Host,
        ServerEndpoint:  resp.ServerEndpoint,
        ServerPublicKey: resp.ServerPublicKey,
        ServerIP:        resp.ServerIP,
        LocalIP:         resp.AssignedIP,
        PrivateKey:      privateKey,
        PublicKey:       publicKey,
    }
    c.saveState()

    // 5. Write WireGuard config
    if err := c.writeConfig(); err != nil {
        return fmt.Errorf("failed to write config: %w", err)
    }

    // 6. Bring up tunnel
    if err := c.QuickUp(); err != nil {
        return fmt.Errorf("failed to start tunnel: %w", err)
    }

    // 7. Wait for connectivity
    if err := c.waitForConnectivity(30 * time.Second); err != nil {
        c.QuickDown()
        return fmt.Errorf("tunnel established but no connectivity: %w", err)
    }

    // 8. Fetch kubeconfig
    if err := c.fetchKubeconfig(); err != nil {
        return fmt.Errorf("failed to fetch kubeconfig: %w", err)
    }

    // 9. Mark connected
    c.state.Connected = true
    c.saveState()

    return nil
}
```

---

### Priority 2: Auto-Reconnect

**Goal:** Automatically recover from network changes, laptop sleep, etc.

```go
// pkg/remote/client.go

type ConnectionWatcher struct {
    client       *Client
    checkInterval time.Duration
    maxRetries   int
    onDisconnect func()
    onReconnect  func()
    stopCh       chan struct{}
}

func NewConnectionWatcher(client *Client) *ConnectionWatcher {
    return &ConnectionWatcher{
        client:        client,
        checkInterval: 10 * time.Second,
        maxRetries:    3,
        stopCh:        make(chan struct{}),
    }
}

func (w *ConnectionWatcher) Start(ctx context.Context) {
    ticker := time.NewTicker(w.checkInterval)
    defer ticker.Stop()

    consecutiveFailures := 0

    for {
        select {
        case <-ctx.Done():
            return
        case <-w.stopCh:
            return
        case <-ticker.C:
            if w.client.ping() {
                consecutiveFailures = 0
                continue
            }

            consecutiveFailures++
            log.Warnf("Health check failed (%d/%d)", consecutiveFailures, w.maxRetries)

            if consecutiveFailures >= w.maxRetries {
                log.Info("Connection lost, attempting reconnect...")
                if w.onDisconnect != nil {
                    w.onDisconnect()
                }

                if err := w.reconnect(); err != nil {
                    log.Errorf("Reconnect failed: %v", err)
                    continue
                }

                consecutiveFailures = 0
                if w.onReconnect != nil {
                    w.onReconnect()
                }
            }
        }
    }
}

func (w *ConnectionWatcher) reconnect() error {
    // 1. Bring down existing tunnel
    w.client.QuickDown()

    // 2. Wait a moment
    time.Sleep(2 * time.Second)

    // 3. Bring tunnel back up
    if err := w.client.QuickUp(); err != nil {
        return fmt.Errorf("failed to restart tunnel: %w", err)
    }

    // 4. Wait for connectivity
    if err := w.client.waitForConnectivity(30 * time.Second); err != nil {
        return fmt.Errorf("tunnel up but no connectivity: %w", err)
    }

    log.Info("Reconnected successfully")
    return nil
}

func (w *ConnectionWatcher) Stop() {
    close(w.stopCh)
}
```

**Integration with CLI:**

```go
// internal/cli/remote.go

func runRemoteConnect(cmd *cobra.Command, args []string) error {
    client := remote.NewClient()

    if err := client.Connect(args[0]); err != nil {
        return err
    }

    fmt.Println("✓ Connected to remote cluster")

    // Start connection watcher in background
    if !foreground {
        watcher := remote.NewConnectionWatcher(client)
        watcher.onDisconnect = func() {
            fmt.Println("⚠ Connection lost, reconnecting...")
        }
        watcher.onReconnect = func() {
            fmt.Println("✓ Reconnected")
        }

        go watcher.Start(context.Background())
    }

    return nil
}
```

---

### Priority 3: Better Error Messages & Diagnostics

```go
// pkg/remote/diagnostics.go

type DiagnosticResult struct {
    Check   string
    Status  bool
    Message string
}

func (c *Client) RunDiagnostics() []DiagnosticResult {
    var results []DiagnosticResult

    // 1. Check WireGuard interface exists
    results = append(results, DiagnosticResult{
        Check:   "WireGuard Interface",
        Status:  c.interfaceExists(),
        Message: c.interfaceMessage(),
    })

    // 2. Check UDP connectivity to server
    udpOK := c.checkUDPConnectivity()
    results = append(results, DiagnosticResult{
        Check:   "UDP Connectivity",
        Status:  udpOK,
        Message: c.udpMessage(udpOK),
    })

    // 3. Check WireGuard handshake
    handshake := c.getLastHandshake()
    handshakeOK := handshake != nil && time.Since(*handshake) < 3*time.Minute
    results = append(results, DiagnosticResult{
        Check:   "WireGuard Handshake",
        Status:  handshakeOK,
        Message: c.handshakeMessage(handshake),
    })

    // 4. Check management API reachable
    apiOK := c.ping()
    results = append(results, DiagnosticResult{
        Check:   "Management API",
        Status:  apiOK,
        Message: c.apiMessage(apiOK),
    })

    // 5. Check kubeconfig valid
    kubeconfigOK := c.validateKubeconfig()
    results = append(results, DiagnosticResult{
        Check:   "Kubeconfig",
        Status:  kubeconfigOK,
        Message: c.kubeconfigMessage(kubeconfigOK),
    })

    return results
}

func (c *Client) udpMessage(ok bool) string {
    if ok {
        return fmt.Sprintf("Can reach %s on UDP 51820", c.state.ServerEndpoint)
    }
    return fmt.Sprintf(`Cannot reach %s on UDP 51820.

Possible causes:
  - Server is not running (check: vibespace serve status)
  - Firewall blocking UDP 51820 (check: ufw status, iptables -L)
  - Your network blocks outbound UDP (try a different network)
  - Server IP changed (regenerate invite token)`, c.state.ServerEndpoint)
}

func (c *Client) handshakeMessage(t *time.Time) string {
    if t == nil {
        return `No WireGuard handshake established.

Possible causes:
  - Server hasn't added your client yet
  - Public key mismatch
  - Token was used on a different device`
    }

    age := time.Since(*t)
    if age > 3*time.Minute {
        return fmt.Sprintf("Last handshake was %v ago (stale)", age.Round(time.Second))
    }
    return fmt.Sprintf("Last handshake %v ago", age.Round(time.Second))
}
```

**CLI command:**

```go
// internal/cli/remote.go

var remoteStatusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show remote connection status and diagnostics",
    RunE:  runRemoteStatus,
}

func runRemoteStatus(cmd *cobra.Command, args []string) error {
    client := remote.NewClient()
    state, err := client.LoadState()
    if err != nil || !state.Connected {
        fmt.Println("Not connected to any remote cluster")
        fmt.Println("\nTo connect: vibespace remote connect <token>")
        return nil
    }

    // Show connection info
    fmt.Println("Remote Connection Status")
    fmt.Println("========================")
    fmt.Printf("Server:      %s\n", state.ServerEndpoint)
    fmt.Printf("Server IP:   %s\n", state.ServerIP)
    fmt.Printf("Local IP:    %s\n", state.LocalIP)
    fmt.Println()

    // Run diagnostics
    fmt.Println("Diagnostics")
    fmt.Println("-----------")

    results := client.RunDiagnostics()
    allOK := true

    for _, r := range results {
        status := "✓"
        if !r.Status {
            status = "✗"
            allOK = false
        }
        fmt.Printf("%s %s\n", status, r.Check)
        if !r.Status {
            // Indent the message
            for _, line := range strings.Split(r.Message, "\n") {
                fmt.Printf("    %s\n", line)
            }
        }
    }

    fmt.Println()
    if allOK {
        fmt.Println("Status: ✓ Healthy")
    } else {
        fmt.Println("Status: ✗ Issues detected")
    }

    return nil
}
```

---

### Priority 4: DNS Resolution (Optional Enhancement)

**Goal:** Access services via hostnames like `api.myworkspace.vibespace.local`

See separate section below for full implementation.

---

### Priority 5: Graceful Disconnect

```go
// pkg/remote/client.go

func (c *Client) Disconnect() error {
    if !c.state.Connected {
        return nil
    }

    // 1. Stop connection watcher if running
    if c.watcher != nil {
        c.watcher.Stop()
    }

    // 2. Optionally notify server (fire and forget)
    go c.notifyServerDisconnect()

    // 3. Bring down WireGuard interface
    if err := c.QuickDown(); err != nil {
        log.Warnf("Failed to bring down WireGuard: %v", err)
    }

    // 4. Remove kubeconfig
    kubeconfigPath := filepath.Join(c.vibespaceHome, "remote_kubeconfig")
    os.Remove(kubeconfigPath)

    // 5. Clear state
    c.state.Connected = false
    c.state.PrivateKey = "" // Clear sensitive data
    c.saveState()

    return nil
}

func (c *Client) notifyServerDisconnect() {
    // Best effort notification to server
    // Server can clean up client state if desired
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, "POST",
        fmt.Sprintf("http://%s:7780/disconnect", c.state.ServerIP),
        bytes.NewBufferString(c.state.PublicKey))

    http.DefaultClient.Do(req)
}
```

---

## DNS Resolution Feature

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Vibespace Daemon                                            │
│                                                             │
│  ┌─────────────────┐      ┌──────────────────────────────┐ │
│  │ Port Forward    │      │ DNS Server (:5353)           │ │
│  │ Manager         │─────►│                              │ │
│  │                 │ add/ │ api.myapp.vibespace.local    │ │
│  │                 │ del  │ db.myapp.vibespace.local     │ │
│  └─────────────────┘      └──────────────────────────────┘ │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                               │
                               │ DNS queries for *.vibespace.local
                               │
┌──────────────────────────────┴──────────────────────────────┐
│ System Resolver                                             │
│                                                             │
│ macOS: /etc/resolver/vibespace.local → 127.0.0.1:5353      │
│ Linux: systemd-resolved → 127.0.0.1:5353 for ~vibespace    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Implementation

```go
// pkg/dns/server.go

package dns

import (
    "fmt"
    "net"
    "strings"
    "sync"

    "github.com/miekg/dns"
)

type DNSServer struct {
    records map[string]string
    mu      sync.RWMutex
    port    int
    server  *dns.Server
}

func NewDNSServer(port int) *DNSServer {
    return &DNSServer{
        records: make(map[string]string),
        port:    port,
    }
}

func (s *DNSServer) AddRecord(name, ip string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Ensure FQDN format
    if !strings.HasSuffix(name, ".") {
        name = name + "."
    }
    s.records[strings.ToLower(name)] = ip
}

func (s *DNSServer) RemoveRecord(name string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if !strings.HasSuffix(name, ".") {
        name = name + "."
    }
    delete(s.records, strings.ToLower(name))
}

func (s *DNSServer) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
    msg := dns.Msg{}
    msg.SetReply(r)
    msg.Authoritative = true

    for _, q := range r.Question {
        switch q.Qtype {
        case dns.TypeA:
            s.mu.RLock()
            ip, ok := s.records[strings.ToLower(q.Name)]
            s.mu.RUnlock()

            if ok {
                msg.Answer = append(msg.Answer, &dns.A{
                    Hdr: dns.RR_Header{
                        Name:   q.Name,
                        Rrtype: dns.TypeA,
                        Class:  dns.ClassINET,
                        Ttl:    60,
                    },
                    A: net.ParseIP(ip),
                })
            }
        }
    }

    w.WriteMsg(&msg)
}

func (s *DNSServer) Start() error {
    dns.HandleFunc("vibespace.local.", s.handleQuery)

    s.server = &dns.Server{
        Addr: fmt.Sprintf("127.0.0.1:%d", s.port),
        Net:  "udp",
    }

    return s.server.ListenAndServe()
}

func (s *DNSServer) Stop() error {
    if s.server != nil {
        return s.server.Shutdown()
    }
    return nil
}
```

### System Resolver Configuration

```go
// pkg/dns/resolver.go

package dns

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
)

func ConfigureSystemResolver(port int) error {
    switch runtime.GOOS {
    case "darwin":
        return configureMacOS(port)
    case "linux":
        return configureLinux(port)
    default:
        return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
    }
}

func RemoveSystemResolver() error {
    switch runtime.GOOS {
    case "darwin":
        return removeMacOS()
    case "linux":
        return removeLinux()
    default:
        return nil
    }
}

// macOS: Uses /etc/resolver/ directory
func configureMacOS(port int) error {
    resolverDir := "/etc/resolver"

    // Create directory if needed (requires sudo)
    if err := os.MkdirAll(resolverDir, 0755); err != nil {
        return fmt.Errorf("failed to create resolver dir (try with sudo): %w", err)
    }

    content := fmt.Sprintf(`# Vibespace DNS resolver
nameserver 127.0.0.1
port %d
`, port)

    resolverFile := filepath.Join(resolverDir, "vibespace.local")
    if err := os.WriteFile(resolverFile, []byte(content), 0644); err != nil {
        return fmt.Errorf("failed to write resolver file (try with sudo): %w", err)
    }

    return nil
}

func removeMacOS() error {
    return os.Remove("/etc/resolver/vibespace.local")
}

// Linux: Uses systemd-resolved
func configureLinux(port int) error {
    confDir := "/etc/systemd/resolved.conf.d"

    if err := os.MkdirAll(confDir, 0755); err != nil {
        return fmt.Errorf("failed to create resolved.conf.d (try with sudo): %w", err)
    }

    content := fmt.Sprintf(`# Vibespace DNS resolver
[Resolve]
DNS=127.0.0.1:%d
Domains=~vibespace.local
`, port)

    confFile := filepath.Join(confDir, "vibespace.conf")
    if err := os.WriteFile(confFile, []byte(content), 0644); err != nil {
        return fmt.Errorf("failed to write resolved config (try with sudo): %w", err)
    }

    // Restart systemd-resolved to pick up changes
    cmd := exec.Command("systemctl", "restart", "systemd-resolved")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to restart systemd-resolved: %w", err)
    }

    return nil
}

func removeLinux() error {
    os.Remove("/etc/systemd/resolved.conf.d/vibespace.conf")
    exec.Command("systemctl", "restart", "systemd-resolved").Run()
    return nil
}
```

### Integration with Port Forwarding

```go
// pkg/daemon/forward.go

func (d *Daemon) AddForward(req ForwardRequest) (*Forward, error) {
    // ... existing port forward logic

    // Register DNS if hostname provided
    if req.Hostname != "" {
        fqdn := fmt.Sprintf("%s.%s.vibespace.local", req.Hostname, req.Workspace)
        d.dnsServer.AddRecord(fqdn, "127.0.0.1")

        forward.Hostname = fqdn
    }

    return forward, nil
}

func (d *Daemon) RemoveForward(id string) error {
    forward := d.forwards[id]

    // Remove DNS record
    if forward.Hostname != "" {
        d.dnsServer.RemoveRecord(forward.Hostname)
    }

    // ... existing removal logic
}
```

---

## What We're NOT Implementing

Based on architectural analysis, these features are **not needed** for the current use case:

| Feature | Reason to Skip |
|---------|----------------|
| **NAT Traversal (STUN/TURN)** | Server has public IP; client→server works with basic NAT |
| **Nebula/Headscale** | Overkill for client-server model |
| **MagicDNS (full)** | Simple embedded DNS is sufficient |
| **Warm Pool** | Sessions are long-lived, cold start is once per workspace |
| **Redis Streams** | JSON files work for single-client architecture |
| **Connect RPC/Protobuf** | SSH + HTTP is sufficient for CLI |

---

## Implementation Priority

| Priority | Task | Effort | Impact |
|----------|------|--------|--------|
| P0 | One-shot registration endpoint | Medium | High (UX) |
| P1 | Auto-reconnect loop | Low | High (reliability) |
| P2 | Better error messages + diagnostics | Low | Medium (UX) |
| P2 | `vibespace remote status` command | Low | Medium (UX) |
| P3 | Graceful disconnect | Low | Low (cleanup) |
| P3 | DNS resolution for local services | Medium | Medium (UX) |

---

## Target UX After Improvements

### Server Setup

```bash
$ vibespace serve --public-ip 203.0.113.50
✓ WireGuard listening on :51820
✓ Registration API ready on :7781
✓ Management API ready on 10.100.0.1:7780

Share this token with clients (expires in 24h):
  vs-eyJrIjoiYWJjMTIzLi4uIiwiZSI6IjIwMy4wLjExMy41MDo1MTgyMCIsInMiOiIxMC4xMDAuMC4xIn0...
```

### Client Connection (Single Command)

```bash
$ vibespace remote connect vs-eyJrIjoiYWJjMTIz...
✓ Registered with server
✓ Assigned IP: 10.100.0.2
✓ WireGuard tunnel established
✓ Kubeconfig saved

Connected to remote cluster at 203.0.113.50
```

### Status Check

```bash
$ vibespace remote status
Remote Connection Status
========================
Server:      203.0.113.50:51820
Server IP:   10.100.0.1
Local IP:    10.100.0.2

Diagnostics
-----------
✓ WireGuard Interface
✓ UDP Connectivity
✓ WireGuard Handshake (12s ago)
✓ Management API
✓ Kubeconfig

Status: ✓ Healthy
```

### Disconnect

```bash
$ vibespace remote disconnect
✓ Disconnected from remote cluster
```

### With DNS (Optional)

```bash
$ vibespace forward myapp --port 3000 --name api
✓ Forwarding myapp:3000 → localhost:3000
✓ DNS: api.myapp.vibespace.local

$ curl http://api.myapp.vibespace.local:3000
{"status": "ok"}
```

---

## Security Considerations

### Token Security

- Tokens are signed with ED25519 (unforgeable)
- Tokens have TTL (default 24h, configurable)
- Tokens are single-use or limited-use (configurable)
- Tokens don't contain sensitive data (only public keys and endpoints)

### Registration Endpoint

- HTTPS with TLS (self-signed or ACME)
- Rate limiting to prevent abuse
- Token validation before any action
- No sensitive data exposed on failure

### WireGuard Security

- Curve25519 key exchange (standard WireGuard)
- Keys never leave the device
- Perfect forward secrecy via WireGuard protocol

---

## Future Considerations

If requirements change, these could be revisited:

1. **Home server deployments** → Add Nebula for NAT traversal
2. **Multi-client sync** → Add Redis Streams for state
3. **Mobile app** → Add Connect RPC for protocol
4. **P2P between workspaces** → Add mesh networking

For now, the simple client-server WireGuard model is sufficient and maintainable.

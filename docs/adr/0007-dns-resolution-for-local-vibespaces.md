# ADR 0007: DNS Resolution Strategy for Local Vibespaces

## Status
Accepted

## Context

MVP Phase 1 uses `kubectl port-forward` for vibespace access, resulting in URLs like `http://127.0.0.1:8142`. This works but:

- **Not user-friendly**: Numeric IPs and random ports are hard to remember
- **No semantic meaning**: Can't tell which port is code-server vs preview vs prod
- **Fragile with Knative**: Pod names change with Knative revisions, port-forward breaks on scale-from-zero
- **Not production-ready**: Real deployments need domain-based routing

MVP Phase 2 migrates to Knative Services with Traefik IngressRoutes, requiring DNS resolution for human-friendly URLs like `code.my-app.vibe.space`.

## Decision

Implement **custom lightweight DNS server** using `miekg/dns` Go library, bundled within the vibespace application.

### Key Choices

#### 1. Custom Go DNS vs Alternatives

**Selected: Custom Go DNS Server**

Alternatives considered:
- ❌ **CoreDNS**: Too large (~20+ MB uncompressed), overkill for simple wildcard DNS
- ❌ **dnsmasq**: C-based external dependency, harder to bundle and manage cross-platform
- ❌ **nip.io/sslip.io**: Requires internet for DNS lookups, adds latency, external dependency
- ❌ **/etc/hosts**: Doesn't support wildcards, requires updating for every new vibespace (clunky at scale)

**Rationale**:
- **Small footprint**: ~5-10 MB (stripped binary) vs 20+ MB for CoreDNS
- **Full control**: No external dependencies, implement exactly what we need
- **Same toolchain**: Go (like API server), easy to maintain and extend
- **Bundleable**: Single binary, cross-compile for all platforms
- **Extensible**: Can add service discovery features in future

#### 2. Port 5353 vs Port 53

**Selected: Port 5353** (unprivileged)

**Rationale**:
- **No root required**: Port 53 needs privileged process (root/sudo), port 5353 is unprivileged
- **No conflicts**: Port 53 often occupied by systemd-resolved on Linux
- **Standard alternative**: Port 5353 is RFC 6763 multicast DNS port, commonly used for local DNS
- **OS forwards queries**: Configure OS to forward `*.vibe.space` queries to 127.0.0.1:5353

#### 3. Automatic + Fallback vs Manual Only

**Selected: Automatic Setup with Graceful Fallback**

**Flow**:
1. **Try automatic**: Use osascript (macOS) / pkexec (Linux) for graphical sudo prompt
2. **If declined/failed**: Gracefully fall back to port-forward mode
3. **No manual instructions**: Reduces friction, simplifies UX

**Rationale**:
- **Best UX**: Most users succeed with one-click setup
- **No friction**: Users who decline still get working vibespaces (port-forward)
- **No complexity**: Manual instructions add support burden

#### 4. Platform Abstraction

**Selected: DnsProvider Trait**

Rust trait for platform-specific implementations:
- **MacOsDnsProvider**: `/etc/resolver/vibe.space` configuration
- **LinuxDnsProvider**: systemd-resolved or resolvconf detection and configuration

**Rationale**:
- **Follows existing pattern**: Similar to K8sProvider trait (ADR 0006)
- **Testable**: Mock providers for unit tests
- **Extensible**: Easy to add Windows support (future)

### Architecture

```
User clicks "Setup DNS" in onboarding
           ↓
Tauri: DnsManager.setup()
           ↓
1. Start DNS server binary (dnsd on port 5353)
           ↓
2. OS configuration:
   macOS: osascript (sudo prompt)
     → Create /etc/resolver/vibe.space
     → nameserver 127.0.0.1
     → port 5353

   Linux: pkexec (PolicyKit prompt)
     → Create /etc/systemd/resolved.conf.d/vibespace.conf
     → [Resolve]
     → DNS=127.0.0.1:5353
     → Domains=~vibe.space
     → Restart systemd-resolved
           ↓
3. Health check: dig code.test.vibe.space
           ↓
DNS configured! All *.vibe.space → 127.0.0.1
```

### DNS Server Implementation

**File**: `api/pkg/dns/server.go`

```go
// Minimal wildcard DNS server
func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
    for _, question := range r.Question {
        // Match *.vibe.space
        if strings.HasSuffix(question.Name, "vibe.space.") {
            switch question.Qtype {
            case dns.TypeA:
                // Return A record: 127.0.0.1
            case dns.TypeAAAA:
                // Return AAAA record: ::1
            }
        } else {
            // Not our domain, return NXDOMAIN
        }
    }
}
```

**Features**:
- Wildcard resolution (`*.vibe.space` → `127.0.0.1`)
- IPv4 (A) and IPv6 (AAAA) support
- Zero TTL (no caching)
- UDP and TCP protocols
- Health check endpoint

### Subdomain Pattern

Each vibespace gets **3 subdomains**:

```
code.{project-name}.vibe.space     → port 8080 (code-server / VS Code)
preview.{project-name}.vibe.space  → port 3000 (dev server: npm run dev, vite, etc.)
prod.{project-name}.vibe.space     → port 8000 (production build)
```

**Example**:
- Project name: `my-app`
- URLs:
  - `http://code.my-app.vibe.space` (VS Code in browser)
  - `http://preview.my-app.vibe.space` (live development preview)
  - `http://prod.my-app.vibe.space` (production build preview)

### Fallback Strategy

If DNS setup fails or user declines:

```
Vibespace access falls back to port-forward:
  code:    http://127.0.0.1:8142
  preview: http://127.0.0.1:8242
  prod:    http://127.0.0.1:8342
```

**Detection logic**:
```rust
fn is_dns_configured() -> bool {
    #[cfg(target_os = "macos")]
    return Path::new("/etc/resolver/vibe.space").exists();

    #[cfg(target_os = "linux")]
    return Path::new("/etc/systemd/resolved.conf.d/vibespace.conf").exists();
}
```

**Access endpoint**:
```go
func (s *Service) Access(ctx context.Context, id string) (map[string]string, error) {
    if s.isDNSConfigured() && s.config.EnableKnativeRouting {
        // Primary path: subdomain URLs
        return map[string]string{
            "code":    fmt.Sprintf("http://code.%s.vibe.space", vs.ProjectName),
            "preview": fmt.Sprintf("http://preview.%s.vibe.space", vs.ProjectName),
            "prod":    fmt.Sprintf("http://prod.%s.vibe.space", vs.ProjectName),
        }, nil
    } else {
        // Fallback path: port-forward
        return s.portForwardURLs(ctx, vs)
    }
}
```

## Consequences

### Positive

✅ **Human-friendly URLs**: `code.my-app.vibe.space` is memorable and semantic

✅ **Knative-compatible**: Domain-based routing works with scale-from-zero and revision updates

✅ **Future-ready**: Supports HTTPS with wildcard certificates (Post-MVP)

✅ **Zero external dependencies**: No dnsmasq, CoreDNS, or cloud DNS services required

✅ **Graceful degradation**: Port-forward fallback ensures vibespaces always work

✅ **Small footprint**: ~5-10 MB DNS binary, minimal resource usage

✅ **Cross-platform**: Works on macOS and Linux with platform abstraction

✅ **No config bloat**: Single domain (`.vibe.space`), doesn't interfere with other DNS

### Negative

⚠️ **One-time sudo prompt**: Unavoidable for DNS configuration (industry standard, see Docker Desktop, Colima)

⚠️ **Platform-specific code**: macOS and Linux have different DNS config approaches

⚠️ **Maintenance overhead**: Custom DNS server needs testing and updates (but very simple, <300 LOC)

⚠️ **Not zero-config**: Requires user to approve sudo prompt (though automatic, not manual steps)

### Neutral

⚙️ **Binary size**: Adds ~5-10 MB to Tauri bundle (acceptable tradeoff)

⚙️ **DNS server process**: One additional background process (negligible resource usage)

⚙️ **Cleanup required**: Uninstall must remove `/etc/resolver` files (handled in Tauri cleanup)

## Alternatives Considered

### Alternative 1: Use System dnsmasq

**Approach**: Install dnsmasq via Homebrew/apt, configure it

**Rejected because**:
- External dependency (dnsmasq package)
- C-based, different toolchain
- Harder to bundle and version
- Colima already uses dnsmasq internally (potential conflicts)

### Alternative 2: Use nip.io or sslip.io

**Approach**: Use public DNS services that resolve `*.127.0.0.1.nip.io` to `127.0.0.1`

**Rejected because**:
- Requires internet for DNS lookups (adds latency)
- External dependency (if service goes down, vibespaces break)
- Privacy concern (DNS queries leak vibespace names)
- Ugly URLs: `code.my-app.127.0.0.1.nip.io`

### Alternative 3: Modify /etc/hosts for each vibespace

**Approach**: Add 3 lines to `/etc/hosts` per vibespace

**Rejected because**:
- Doesn't support wildcards (need to update for every new vibespace)
- Clunky at scale (100 vibespaces = 300 `/etc/hosts` entries)
- Still requires sudo for every vibespace creation

### Alternative 4: No DNS (port-forward only)

**Approach**: Keep port-forward indefinitely

**Rejected because**:
- Not production-ready architecture
- Breaks with Knative scale-from-zero
- Poor UX (random ports)
- Future HTTPS support requires domains

## Implementation Notes

### Binary Bundling

Similar to kubectl bundling (ADR 0006):

```
app/src-tauri/tauri.conf.json:
  "bundle": {
    "resources": {
      "binaries/macos/dnsd-darwin-amd64": "dnsd",
      "binaries/macos/dnsd-darwin-arm64": "dnsd",
      "binaries/linux/dnsd-linux-amd64": "dnsd",
      "binaries/linux/dnsd-linux-arm64": "dnsd"
    }
  }
```

Extract to `~/.vibespace/bin/dnsd` on install.

### CI/CD

`.github/workflows/build-dns.yml`: Cross-compile DNS binaries for all platforms.

### Testing

- Unit tests: DNS server resolution logic
- Integration tests: OS DNS configuration, health checks
- E2E tests: Full vibespace access via subdomains

## Future Considerations

### HTTPS Support (Post-MVP)

See ADR 0008. Wildcard certificate for `*.vibe.space` can be:
- Self-signed (local mode, browser warnings)
- Let's Encrypt (cloud mode via cert-manager)

### Remote Mode (Post-MVP)

In remote/cloud mode:
- No local DNS server needed
- Use real DNS provider (Cloudflare, Route53)
- Vibespaces on VPS use real domains: `code.my-app.yourdomain.com`

### Windows Support (Future)

Windows DNS configuration requires different approach:
- Option 1: Modify Windows Registry (HKEY_LOCAL_MACHINE)
- Option 2: Use Windows DNS Client API
- Option 3: Ship with Acrylic DNS Proxy (third-party)

Currently out of scope (Local Mode targets macOS/Linux per ADR 0006).

## References

- [miekg/dns](https://github.com/miekg/dns) - Go DNS library
- [RFC 6763](https://datatracker.ietf.org/doc/html/rfc6763) - Multicast DNS (port 5353)
- [macOS resolver man page](https://www.manpagez.com/man/5/resolver/)
- [systemd-resolved.service](https://www.freedesktop.org/software/systemd/man/systemd-resolved.service.html)
- ADR 0006: Bundled Kubernetes Runtime (similar bundling pattern)
- ADR 0008: HTTPS and Certificate Management (future HTTPS support)

## Related Issues

- Issue #52: Migrate to Knative Services with Traefik routing and DNS

## Date

2025-01-11

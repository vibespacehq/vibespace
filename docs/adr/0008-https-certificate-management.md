# ADR 0008: HTTPS and Certificate Management

## Status
Proposed (Post-MVP)

## Context

MVP Phase 2 implements Knative Services with Traefik routing and DNS resolution, but uses **HTTP only** for local development. Production deployments (cloud/remote mode) require **HTTPS with valid TLS certificates** for:

- **Security**: Encrypt traffic between browser and vibespace
- **Browser features**: Many modern APIs (Service Workers, WebRTC, etc.) require HTTPS
- **Production parity**: Match real-world deployment environments
- **Trust**: Avoid browser security warnings

This ADR defines the HTTPS strategy for future cloud mode while keeping local mode simple (HTTP only).

## Decision

Use **cert-manager** for automated TLS certificate management in cloud mode, with different approaches for local vs remote deployments.

### Architecture: Two Modes

#### Local Mode (MVP Phase 2) - HTTP Only

**Current implementation** (this PR):
- DNS: `*.vibe.space` resolves to `127.0.0.1`
- Protocol: HTTP
- URLs: `http://code.my-app.vibe.space`
- Certificates: None

**Rationale**:
- ✅ **Browsers trust localhost**: Chrome/Firefox don't show warnings for `http://127.0.0.1` or local domains resolving to localhost
- ✅ **Simpler onboarding**: No certificate complexity for MVP
- ✅ **Faster development**: Skip certificate provisioning delays
- ✅ **Sufficient for local dev**: Most development doesn't require HTTPS locally

**Future enhancement** (optional):
- Self-signed wildcard certificate for `*.vibe.space`
- User-installable root CA (similar to mkcert)
- Eliminates "Not Secure" indicators
- Lower priority (local HTTPS is cosmetic, not security-critical)

#### Cloud Mode (Post-MVP) - HTTPS with Let's Encrypt

**Future implementation**:
- DNS: Real domain (e.g., `*.yourdomain.com` via Cloudflare, Route53)
- Protocol: HTTPS
- URLs: `https://code.my-app.yourdomain.com`
- Certificates: Let's Encrypt wildcard certificate via cert-manager

**Architecture**:

```
User's domain: yourdomain.com
           ↓
DNS provider: Cloudflare / Route53 / etc.
           ↓
Wildcard DNS: *.yourdomain.com → VPS IP
           ↓
cert-manager:
  - ACME DNS-01 challenge
  - Provisions wildcard cert: *.yourdomain.com
  - Stores in Kubernetes Secret: vibespace-tls
           ↓
Traefik IngressRoute:
  - entryPoints: [websecure]  # HTTPS
  - tls:
      secretName: vibespace-tls
           ↓
Knative Service: vibespace-ws123
           ↓
HTTPS: https://code.my-app.yourdomain.com
```

### cert-manager Configuration

**Installation** (cloud mode only):

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

**ClusterIssuer** (Let's Encrypt production):

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: user@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - dns01:
        cloudflare:
          apiTokenSecretRef:
            name: cloudflare-api-token
            key: api-token
```

**Certificate** (wildcard for all vibespaces):

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: vibespace-wildcard
  namespace: vibespace
spec:
  secretName: vibespace-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - '*.yourdomain.com'
    - 'yourdomain.com'
  acme:
    config:
      - dns01:
          provider: cloudflare
```

**Certificate lifecycle**:
1. User configures DNS provider API credentials (Cloudflare token, Route53 keys, etc.)
2. cert-manager creates DNS TXT record for ACME challenge
3. Let's Encrypt verifies domain ownership
4. Certificate issued and stored in `vibespace-tls` Secret
5. Auto-renewal before expiry (90 days → renew at 60 days)

### Traefik Integration

**HTTP → HTTPS redirect** (Traefik Middleware):

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: https-redirect
  namespace: vibespace
spec:
  redirectScheme:
    scheme: https
    permanent: true
```

**IngressRoute with TLS** (cloud mode):

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-ws123-code
  namespace: vibespace
spec:
  entryPoints:
    - websecure  # HTTPS (port 443)
  routes:
    - match: Host(`code.my-app.yourdomain.com`)
      kind: Rule
      services:
        - name: vibespace-ws123
          port: 8080
      middlewares:
        - name: https-redirect  # Redirect HTTP → HTTPS
  tls:
    secretName: vibespace-tls  # Wildcard certificate
```

**IngressRoute without TLS** (local mode, current implementation):

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-ws123-code
  namespace: vibespace
spec:
  entryPoints:
    - web  # HTTP (port 80)
  routes:
    - match: Host(`code.my-app.vibe.space`)
      kind: Rule
      services:
        - name: vibespace-ws123
          port: 8080
  # No tls section
```

### Implementation Strategy

**Phase 1: MVP Phase 2** (this PR) - HTTP foundation
- ✅ DNS server for `*.vibe.space`
- ✅ Traefik IngressRoutes (HTTP only)
- ✅ Knative Services
- ✅ Architecture ready for HTTPS (entrypoint switch)

**Phase 2: Post-MVP** (future) - HTTPS for cloud mode
- Install cert-manager in cloud k8s clusters
- Add DNS provider configuration UI (Cloudflare, Route53 API keys)
- Certificate creation and management
- Update IngressRoutes to use `websecure` entrypoint
- HTTP → HTTPS redirect middleware
- Certificate renewal monitoring

**Phased rollout**:
1. Local mode stays HTTP (no change)
2. Cloud mode: Optional HTTPS toggle
3. Cloud mode: HTTPS default (after validation period)

## Consequences

### Positive

✅ **Security in production**: HTTPS encrypts traffic, protects credentials

✅ **Browser API access**: Enables Service Workers, WebRTC, etc.

✅ **Wildcard certificate**: Single cert for all vibespaces (no per-vibespace certs)

✅ **Automated renewal**: cert-manager handles Let's Encrypt renewal (no manual intervention)

✅ **Standard tooling**: cert-manager is industry-standard (used by Kubernetes ecosystem)

✅ **Extensible**: Supports multiple DNS providers (Cloudflare, Route53, Google Cloud DNS, etc.)

✅ **Local simplicity**: HTTP-only local mode avoids certificate complexity for MVP

### Negative

⚠️ **Complexity in cloud mode**: DNS provider API setup, cert-manager configuration

⚠️ **Let's Encrypt rate limits**: 50 certificates/week/domain (wildcard mitigates this)

⚠️ **DNS provider dependency**: Requires API access (Cloudflare, Route53, etc.)

⚠️ **cert-manager overhead**: Additional Kubernetes component (~100 MB memory)

⚠️ **Debugging difficulty**: ACME challenges can fail (DNS propagation, API issues)

### Neutral

⚙️ **Local HTTP acceptable**: Browsers don't warn for localhost, modern development often uses HTTP locally

⚙️ **Certificate size**: ~4 KB per certificate (negligible)

⚙️ **Renewal frequency**: 90-day Let's Encrypt certs renew every 60 days (automated)

## Alternatives Considered

### Alternative 1: Self-Signed Certificates for Local Mode

**Approach**: Generate self-signed wildcard cert for `*.vibe.space`, install root CA

**Tools**: mkcert, truststore

**Rejected because**:
- Adds complexity to onboarding (install root CA)
- Browser trust prompts confuse users
- Not necessary for localhost development
- HTTP on localhost is widely accepted

**Keep as optional enhancement**: User preference in Settings

### Alternative 2: Cloudflare Tunnel (Cloudflared)

**Approach**: Use Cloudflare Tunnel for HTTPS, no cert-manager

**Rejected because**:
- Cloudflare-specific (vendor lock-in)
- Requires Cloudflare account (not self-hosted)
- Limited to Cloudflare DNS
- Less control over certificate management

### Alternative 3: Traefik ACME (without cert-manager)

**Approach**: Use Traefik's built-in ACME support

**Rejected because**:
- Traefik ACME is per-IngressRoute (need wildcard)
- cert-manager is Kubernetes-native (better integration)
- cert-manager supports more DNS providers
- Industry best practice: dedicated certificate manager

### Alternative 4: Manually Provisioned Certificates

**Approach**: User uploads certificates via UI

**Rejected because**:
- Manual renewal every 90 days (poor UX)
- Error-prone (certificate format issues, expiry)
- Not scalable
- Defeats purpose of automation

## Implementation Notes

### Code Changes (Post-MVP)

**File**: `api/pkg/network/ingress.go`

```go
func (m *IngressManager) buildIngressRouteWithTLS(
    vibespaceID, projectName, name, subdomain string, port int,
    tlsEnabled bool, certSecretName string,
) *traefikv1alpha1.IngressRoute {
    ingress := m.buildIngressRoute(vibespaceID, projectName, name, subdomain, port)

    if tlsEnabled {
        ingress.Spec.EntryPoints = []string{"websecure"} // HTTPS
        ingress.Spec.TLS = &traefikv1alpha1.TLS{
            SecretName: certSecretName, // vibespace-tls
        }
    } else {
        ingress.Spec.EntryPoints = []string{"web"} // HTTP
        // No TLS section
    }

    return ingress
}
```

**Feature flag**:
```yaml
# api/config/config.yaml
features:
  enable_https: false  # Local mode: false, Cloud mode: true

tls:
  cert_secret_name: vibespace-tls
  enable_auto_redirect: true  # HTTP → HTTPS redirect
```

**DNS Provider Configuration** (UI):
```go
type DNSProviderConfig struct {
    Provider string // "cloudflare", "route53", "google"
    APIToken string // API credentials (encrypted at rest)
}
```

### DNS Providers Supported

cert-manager supports **15+ DNS providers** via webhook:

- **Cloudflare** (most popular)
- **AWS Route53**
- **Google Cloud DNS**
- **Azure DNS**
- **DigitalOcean**
- **Linode**
- **OVH**
- **Namecheap**
- Many more via [cert-manager webhooks](https://cert-manager.io/docs/configuration/acme/dns01/)

**Recommended**: Cloudflare (easiest API, free tier, fast propagation)

### Certificate Monitoring

**cert-manager metrics**:
- `certmanager_certificate_expiration_timestamp_seconds` (Prometheus)
- Alert before expiry (30 days)
- UI indicator: "Certificate expires in X days"

**Frontend display**:
```typescript
interface VibespaceURLs {
  code: string;       // http:// or https://
  preview: string;
  prod: string;
  tlsEnabled: boolean;
  certExpiry: Date | null;
}
```

### Testing Strategy

**Local mode** (MVP Phase 2):
- ✅ HTTP access to `code.my-app.vibe.space`
- ✅ No certificate warnings
- ✅ Traefik routing works

**Cloud mode** (Post-MVP):
- ⏳ cert-manager provisions wildcard certificate
- ⏳ HTTPS access to `code.my-app.yourdomain.com`
- ⏳ HTTP → HTTPS redirect works
- ⏳ Certificate auto-renewal (test at 60-day mark)

## Security Considerations

### Certificate Private Keys

- Stored in Kubernetes Secrets (encrypted at rest via etcd encryption)
- Never exposed via API or UI
- Access restricted to cert-manager and Traefik

### DNS Provider API Credentials

- Stored encrypted in `~/.vibespace/credentials/` (user's machine)
- Injected into Kubernetes Secrets (cert-manager namespace)
- Read-only API tokens (no write/delete permissions)
- User can revoke and rotate tokens

### ACME Account Key

- cert-manager generates ACME account key
- Stored in `letsencrypt-prod` Secret
- Never exposed, used only for ACME protocol

### Certificate Transparency Logs

- Let's Encrypt publishes all certificates to CT logs (public)
- Vibespace domains will be visible in CT logs
- This is standard practice, not a security issue

## Future Enhancements

### Local HTTPS (Optional)

**Phase 3** (optional, low priority):

```go
// Generate self-signed wildcard cert
func GenerateLocalCert() (*tls.Certificate, error) {
    // Create CA
    ca := &x509.Certificate{
        Subject: pkix.Name{CommonName: "vibespace Local CA"},
        NotBefore: time.Now(),
        NotAfter: time.Now().AddDate(10, 0, 0), // 10 years
        IsCA: true,
    }

    // Create wildcard cert for *.vibe.space
    cert := &x509.Certificate{
        DNSNames: []string{"*.vibe.space", "vibe.space"},
        NotBefore: time.Now(),
        NotAfter: time.Now().AddDate(1, 0, 0), // 1 year
    }

    // Install root CA in system truststore
    // (requires sudo, similar to mkcert)
}
```

### Custom Domains

**Phase 4** (Post-MVP):

Allow users to map custom domains to vibespaces:

```
User domain: myapp.example.com
            ↓
DNS CNAME: myapp.example.com → vibespace-ws123.yourdomain.com
            ↓
cert-manager: Issue certificate for myapp.example.com
            ↓
Traefik: Route myapp.example.com → vibespace-ws123
```

**UI**:
- "Add custom domain" button in vibespace settings
- DNS validation (TXT record check)
- Automatic certificate provisioning
- SSL/TLS status indicator

### mTLS (Mutual TLS)

**Phase 5** (future):

Client certificate authentication for enhanced security:

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
spec:
  tls:
    secretName: vibespace-tls
    options:
      name: mtls-options
```

## References

- [cert-manager Documentation](https://cert-manager.io/docs/)
- [Let's Encrypt Rate Limits](https://letsencrypt.org/docs/rate-limits/)
- [Traefik TLS Configuration](https://doc.traefik.io/traefik/routing/routers/#tls)
- [ACME DNS-01 Challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)
- [Kubernetes Secret Encryption](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)
- [Certificate Transparency](https://certificate.transparency.dev/)
- [mkcert](https://github.com/FiloSottile/mkcert) - Local CA for development

## Related Issues

- Issue #52: Migrate to Knative Services with Traefik routing and DNS
- Future: Issue for cloud mode HTTPS implementation

## Related ADRs

- ADR 0007: DNS Resolution Strategy for Local Vibespaces
- ADR 0006: Bundled Kubernetes Runtime (Local vs Remote mode)

## Date

2025-01-11

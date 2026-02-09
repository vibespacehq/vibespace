# Notes (to verify)

## From Security Audit (unverified)

- ~~Docker container runs as root with passwordless sudo (`build/base/Dockerfile`)~~ — mitigated via container SecurityContext (drop ALL caps, add only what's needed) in `pkg/deployment/manager.go`
- `InsecureSkipVerify: true` in TLS cert pinning (`pkg/remote/tls.go:75`) — by design for self-signed certs, but callback removal would silently accept all certs
- ~~Hardcoded `Bearer QQ==` for Homebrew binary downloads (`pkg/remote/wireguard.go`) + no SHA256 verification of downloaded binaries~~ — `Bearer QQ==` is a public Homebrew CDN token (not a secret). SHA256 verification now implemented for: Homebrew bottles, kubectl, Lima, Colima. Docker has no checksums available. QEMU pending (needs checksums published to vibespace-binaries repo).

## Untested

- DNS resolution (`pkg/dns/`) — never tested end-to-end. Needs running cluster + vibespace + active port forward to verify `*.vibespace.internal` resolves correctly

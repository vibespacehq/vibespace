# Notes (to verify)

## From Security Audit (unverified)

- Docker container runs as root with passwordless sudo (`build/base/Dockerfile`) — consider restricting sudo scope
- `InsecureSkipVerify: true` in TLS cert pinning (`pkg/remote/tls.go:75`) — by design for self-signed certs, but callback removal would silently accept all certs
- Hardcoded `Bearer QQ==` for Homebrew binary downloads (`pkg/remote/wireguard.go`) + no SHA256 verification of downloaded binaries

## Untested

- DNS resolution (`pkg/dns/`) — never tested end-to-end. Needs running cluster + vibespace + active port forward to verify `*.vibespace.internal` resolves correctly

# Notes (to verify)

## From Security Audit (unverified)

- ~~Docker container runs as root with passwordless sudo (`build/base/Dockerfile`)~~ — mitigated via container SecurityContext (drop ALL caps, add only what's needed) in `pkg/deployment/manager.go`
- ~~Hardcoded `Bearer QQ==` for Homebrew binary downloads (`pkg/remote/wireguard.go`) + no SHA256 verification of downloaded binaries~~ — `Bearer QQ==` is a public Homebrew CDN token (not a secret). SHA256 verification now implemented for: Homebrew bottles, kubectl, Lima, Colima. Docker has no checksums available. QEMU pending (needs checksums published to vibespace-binaries repo).

## To Test

### SHA256 download verification (integration)

Deploy `develop` branch to VPS and test binary downloads with verification:

1. `vibespace init` on VPS — triggers Lima, kubectl, QEMU downloads (Linux path). Verify SHA256 checks pass for Lima (SHA256SUMS) and kubectl (.sha256 sidecar). QEMU has no checksum yet (TODO).
2. `vibespace init` on macOS — triggers Colima, Lima, kubectl, Docker downloads. Verify SHA256 checks pass for Colima (.sha256sum), Lima (SHA256SUMS), and kubectl (.sha256). Docker has no checksum (expected).
3. `vibespace remote connect` on macOS (when WireGuard not installed) — triggers Homebrew bottle downloads. Verify SHA256 from Homebrew API is checked against downloaded bottle.
4. Negative test: tamper with a downloaded file or expected hash to confirm verification fails with clear error message.

### SecurityContext on running pod

Deploy `develop` branch to VPS, create a vibespace, and inspect the pod:

1. `kubectl get pod <pod> -o jsonpath='{.spec.containers[0].securityContext}'` — verify capabilities drop ALL + add the 8 needed ones.
2. SSH into the container, run `capsh --print` or `grep Cap /proc/1/status` — verify effective capabilities match expected set.
3. Verify agents can still: `sudo apt-get install`, `sudo pip install`, run supervisord, bind sshd on port 22.
4. Verify agents cannot: `mount`, `mknod`, raw sockets, `ptrace` attach.

## Untested

- DNS resolution (`pkg/dns/`) — never tested end-to-end. Needs running cluster + vibespace + active port forward to verify `*.vibespace.internal` resolves correctly

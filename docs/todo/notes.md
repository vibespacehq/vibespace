# Notes

## Untested

- QEMU SHA256 verification — pending checksums published to vibespace-binaries repo
- VPS integration test for SHA256 verification (Linux path: Lima, kubectl, QEMU)
- SecurityContext in-container verification — `capsh --print` to confirm effective caps, test sudo/apt/pip still work, test mount/mknod/ptrace are blocked

## Known Bugs

- DNS resolver file (`/etc/resolver/vibespace.internal`) not created on macOS — the daemon runs detached without a TTY so `sudo` in `ConfigureSystemResolver` silently fails. DNS server itself works fine on port 5553, but macOS doesn't know to query it. Workaround: `sudo bash -c 'echo -e "nameserver 127.0.0.1\nport 5553" > /etc/resolver/vibespace.internal'`
- Chrome ignores `/etc/resolver/` on macOS — Chrome uses its own async DNS resolver and bypasses the macOS resolver directory. `*.vibespace.internal` DNS works with CLI tools (`curl`, etc.) but not in Chrome. No clean fix without hijacking port 53 (too invasive). `localhost:PORT` always works as fallback.
- `InterfaceStatus()` in `pkg/remote/wireguard.go` reports `tunnel_up: false` even when the tunnel is working (ping to 10.100.0.1 succeeds). Detection strategies are unreliable on macOS without sudo — needs a better approach (e.g. ping-based check)

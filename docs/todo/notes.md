# Notes

## Untested

- DNS resolution (`pkg/dns/`) — never tested end-to-end. Needs running cluster + vibespace + active port forward to verify `*.vibespace.internal` resolves correctly
- QEMU SHA256 verification — pending checksums published to vibespace-binaries repo
- VPS integration test for SHA256 verification (Linux path: Lima, kubectl, QEMU)
- SecurityContext in-container verification — `capsh --print` to confirm effective caps, test sudo/apt/pip still work, test mount/mknod/ptrace are blocked

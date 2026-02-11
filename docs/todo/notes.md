# Notes

## Untested

- QEMU SHA256 verification — pending checksums published to vibespace-binaries repo
- VPS integration test for SHA256 verification (Linux path: Lima, kubectl, QEMU)
- SecurityContext in-container verification — `capsh --print` to confirm effective caps, test sudo/apt/pip still work, test mount/mknod/ptrace are blocked

## Stubs

- `vibespace <name> ports` — CLI reader exists but no in-container port detector writes `/tmp/vibespace-ports.json`

## Testing Gaps

- **Daemon regression risk** — port forwarding + DNS integration is critical path but has no dedicated test coverage. Should add E2E subtests after `create`: verify port forwards are reachable, DNS records resolve, cleanup happens on `delete`.
- **TUI non-TTY mode** — bubbletea has a non-interactive fallback when there's no TTY. This is testable as a subprocess (same as other E2E tests) and should be covered since CI and scripted usage always hit this path.

## Known Bugs

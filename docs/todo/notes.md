# Notes

## Untested

- QEMU SHA256 verification — pending checksums published to vibespace-binaries repo
- VPS integration test for SHA256 verification (Linux path: Lima, kubectl, QEMU)
- SecurityContext in-container verification — `capsh --print` to confirm effective caps, test sudo/apt/pip still work, test mount/mknod/ptrace are blocked

## Stubs

- `vibespace <name> ports` — CLI reader exists and is E2E tested, but no in-container port detector writes `/tmp/vibespace-ports.json` yet (always returns empty list)

## Known Bugs

# Project Status

## Current State

- **Core CLI**: Stable — create, list, delete, connect, exec, multi all verified
- **Remote Mode**: E2E tested — serve, connect, create vibespace, uninstall. Auto-reconnect untested
- **Daemon**: Port forwarding and session management verified. DNS resolution untested
- **TUI**: Verified locally, untested in remote mode
- **Cluster**: Lima/Colima on macOS verified, Lima/QEMU on Linux verified (slow without KVM)

## Recent Work (2026-02-08)

- Fixed key race condition between CLI and daemon on `vibespace serve`
- Fixed stale WireGuard interface detection on serve start
- Made all CLI commands remote-mode aware (status, stop, create, list, ports)
- Fixed serve daemon killing during uninstall (PID-based instead of in-memory Stop)
- Fixed file ownership under sudo (chownToRealUser)
- Full E2E test: serve on VPS → connect from Mac → create vibespace → uninstall

## Known Issues

- Permission hook fails in interactive mode — use `--skip-permissions` as workaround
- VPS init extremely slow (no KVM, QEMU software emulation) — bare metal mode will fix this
- Linux WireGuard install only uses apt-get (multi-distro detection added but untested beyond apt)

## What's Next

See `todo/roadmap.md` for the full prioritized roadmap. Top priority is **bare metal mode** (P1) to eliminate the QEMU/Lima overhead on Linux servers.

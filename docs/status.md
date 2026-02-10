# Project Status

## Current State

- **Core CLI**: Stable — create, list, delete, connect, exec, multi all verified
- **Remote Mode**: E2E tested — serve, connect, create vibespace, uninstall. Auto-reconnect untested
- **Daemon**: Port forwarding, session management, and embedded DNS server verified
- **DNS**: Working on macOS via `/etc/resolver/`. Chromium browsers bypass it (Safari/curl work)
- **TUI**: Verified locally, untested in remote mode. Info command has interactive tabbed view
- **Security**: Container SecurityContext with AUDIT_WRITE capability — SSH and browser mode verified
- **Cluster**: Lima/Colima on macOS verified, Lima/QEMU on Linux verified, bare metal mode E2E tested on VPS

## Recent Work (2026-02-09)

- Fixed SSH failure caused by missing AUDIT_WRITE capability in container SecurityContext
- Implemented opt-in DNS resolution for port forwards (`--dns` / `--dns-name` flags)
- Fixed `configureMacOSResolver` pipe conflict (Stdin + StdinPipe)
- Added interactive tabbed TUI for `vibespace info` (bubbletea + lipgloss)
- DNS names use `agent.vibespace.vibespace.internal` format, customizable via `--dns-name`
- Resolver file creation moved to CLI level (daemon can't sudo when detached)

## Recent Work (2026-02-08)

- Fixed key race condition between CLI and daemon on `vibespace serve`
- Fixed stale WireGuard interface detection on serve start
- Made all CLI commands remote-mode aware (status, stop, create, list, ports)
- Fixed serve daemon killing during uninstall (PID-based instead of in-memory Stop)
- Fixed file ownership under sudo (chownToRealUser)
- Full E2E test: serve on VPS → connect from Mac → create vibespace → uninstall

## Known Issues

- Chromium browsers bypass macOS `/etc/resolver/` — use Safari or curl for DNS-based access
- `InterfaceStatus()` reports `tunnel_up: false` even when tunnel works — needs ping-based check
- Permission hook fails in interactive mode — use `--skip-permissions` as workaround
- VPS init slow with Lima/QEMU (no KVM) — use `--bare-metal` flag instead
- Linux WireGuard install only uses apt-get (multi-distro detection added but untested beyond apt)

## What's Next

See `todo/roadmap.md` for the full prioritized roadmap. Top priority is **bare metal mode** (P1) to eliminate the QEMU/Lima overhead on Linux servers.

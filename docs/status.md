# Project Status

## Current State

- **Core CLI**: Stable — create, list, delete, connect, exec, multi, config, forward, session all verified
- **TUI**: Five-tab app shell (Vibespaces, Chat, Monitor, Sessions, Remote) with command palette, help overlay, animated tab bar, inline forms, and live resource charts. Verified locally, untested in remote mode
- **Remote Mode**: E2E tested — serve, connect, create vibespace, uninstall. Auto-reconnect untested
- **Daemon**: Port forwarding, session management, and embedded DNS server verified
- **DNS**: Working on macOS via `/etc/resolver/`. Chromium browsers bypass it (Safari/curl work)
- **Security**: Container SecurityContext with AUDIT_WRITE capability — SSH and browser mode verified
- **Cluster**: Lima/Colima on macOS verified, Lima/QEMU on Linux verified, bare metal mode E2E tested on VPS

## Recent Work (2026-02-23)

- Implemented command palette (`pkg/tui/overlay_palette.go`) with fuzzy multi-word filtering
- Fixed k8s conflict error in `config set` by adding `retry.RetryOnConflict` to `UpdateAgentConfig`
- Fixed dead code (`StripAnsi`) and `gofmt` formatting drift across TUI files
- Moved TUI design doc from `ideas/` to `reference/tui.md` and rewrote as factual reference
- Updated docs: ideas.md (marked 15+ implemented features), removed obsolete monitor mockup

## Recent Work (2026-02-11)

- Expanded E2E lifecycle tests from 7 to ~37 subtests per platform
- JSON mode subtests: info, config show/set, session list, agent create/delete, exec, forward add/list/remove, ports, multi list-sessions/list-agents/message, stop, start
- Plain mode subtests: re-run all 10 read-only commands with --plain flag
- Added waitForReady + waitForDaemonReady helpers for CI reliability

## Recent Work (2026-02-09)

- Fixed SSH failure caused by missing AUDIT_WRITE capability in container SecurityContext
- Implemented opt-in DNS resolution for port forwards (`--dns` / `--dns-name` flags)
- Added interactive tabbed TUI for `vibespace info` (bubbletea + lipgloss)

## Recent Work (2026-02-08)

- Fixed key race condition between CLI and daemon on `vibespace serve`
- Made all CLI commands remote-mode aware (status, stop, create, list, ports)
- Full E2E test: serve on VPS → connect from Mac → create vibespace → uninstall

## Known Issues

- Chromium browsers bypass macOS `/etc/resolver/` — use Safari or curl for DNS-based access
- `InterfaceStatus()` reports `tunnel_up: false` even when tunnel works — needs ping-based check
- Permission hook fails in interactive mode — use `--skip-permissions` as workaround
- VPS init slow with Lima/QEMU (no KVM) — use `--bare-metal` flag instead
- Linux WireGuard install only uses apt-get (multi-distro detection added but untested beyond apt)

## What's Next

1. **Testing coverage** — default output mode E2E, error path E2E, remote mode E2E (see `todo/testing.md`)
2. **Help & diagnostics** — `vibespace doctor`, `vibespace wait` (see `todo/roadmap.md` P1)

See `todo/roadmap.md` for the full prioritized roadmap.

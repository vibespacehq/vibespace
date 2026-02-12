# Notes

## Untested

- QEMU SHA256 verification — pending checksums published to vibespace-binaries repo
- VPS integration test for SHA256 verification (Linux path: Lima, kubectl, QEMU)
- SecurityContext in-container verification — `capsh --print` to confirm effective caps, test sudo/apt/pip still work, test mount/mknod/ptrace are blocked

## Stubs

- `vibespace <name> ports` — CLI reader exists and is E2E tested, but no in-container port detector writes `/tmp/vibespace-ports.json` yet (always returns empty list)

## CI Notes

- Lima runner (`linux-hub`) has a pre-test cleanup step to remove stale `.vibespace`/`.lima` state — needed because `t.Cleanup` doesn't run when Go panics on test timeout
- Colima runner (`macos-m2`) doesn't have this yet — add the same cleanup if it starts failing with stale state

## TUI Remaining Phases

All future phases reference `docs/ideas/tui-design-document.md`.

### Phase 3: Vibespaces Tab (`pkg/tui/tab_vibespaces.go`)

**Design doc §4-5 (lines 64-380)**. Most complex tab — implement in sub-phases.

| Sub-phase | Scope | Design doc ref |
|-----------|-------|----------------|
| 3a | Table view: NAME, STATUS, AGENTS, CPU, MEM, STORAGE, AGE. `j/k` nav, status colors, responsive column hiding | §4.1 (lines 64-130) |
| 3b | Inline expansion: `Enter` toggles agent tree below row. Resources, mounts, forwards display | §4.2 (lines 131-200) |
| 3c | Connect actions: `x` SSH via `tea.ExecProcess`, `b` browser via ttyd URL | §4.3 (lines 201-240) |
| 3d | Inline forms: `n` create, `d` delete, `a` add agent, `S` start/stop | §5.1-5.2 (lines 241-320) |
| 3e | Inline editors: `e` config editor, `f` forward manager | §5.3-5.4 (lines 321-380) |

Depends on: `pkg/vibespace/service.go` (List, ListAgents, Create, Delete, Start, Stop), `pkg/daemon/client.go` (ListForwardsForVibespace), k8s cluster access.

### Phase 4a: Monitor Tab (`pkg/tui/tab_monitor.go`)

**Design doc §6 (lines 381-504)**. Three sections: resource table with bar charts, agent activity table, CPU sparkline (`ntcharts/sparkline`). Auto-refresh via `tea.Tick` (1s), `p` pauses, `R` force refresh, `1/2/3` toggle sections, `v` vibespace picker.

Depends on: k8s metrics API, daemon client, session manager.

### Phase 4b: Remote Tab (`pkg/tui/tab_remote.go`)

**Design doc §8 (lines 578-648)**. Three modes: connected (`lipgloss/list` + `lipgloss/table` + sparkline), disconnected (token text input + connect flow), serving (client table + token generation).

Depends on: `pkg/remote/` state files (remote.json, serve.json), WireGuard status.

### Phase 5: Command Palette (`pkg/tui/overlay_palette.go`)

**Design doc §9.2 (lines 691-730)**. Replace stub with fuzzy-filtered action list. `bubbles/textinput` for filter, action list below. Actions: switch tab, chat with vibespace, new session, connect, start/stop, etc.

## Known Bugs

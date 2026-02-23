# Notes

## Untested

- QEMU SHA256 verification — pending checksums published to vibespace-binaries repo
- VPS integration test for SHA256 verification (Linux path: Lima, kubectl, QEMU)
- SecurityContext in-container verification — `capsh --print` to confirm effective caps, test sudo/apt/pip still work, test mount/mknod/ptrace are blocked

## CI Notes

- Lima runner (`linux-hub`) has a pre-test cleanup step to remove stale `.vibespace`/`.lima` state — needed because `t.Cleanup` doesn't run when Go panics on test timeout
- Colima runner (`macos-m2`) doesn't have this yet — add the same cleanup if it starts failing with stale state

## TUI Remaining Phases

TUI reference: `docs/reference/tui.md`.

### Phase 3: Vibespaces Tab (`pkg/tui/tab_vibespaces.go`) — ✅ Complete

3,174 lines. All sub-phases implemented:

| Sub-phase | Status |
|-----------|--------|
| 3a — Table view (responsive columns, gradient selected row, detail panel) | ✅ |
| 3b — Agent view (3-level stack nav, per-agent config/forwards/resources) | ✅ |
| 3c — Sessions (SSH `history.jsonl` parsing, `tea.ExecProcess` resume, `x` SSH, `b` browser) | ✅ |
| 3d — Inline forms (create, delete, add-agent with 8 fields, start/stop) | ✅ |
| 3e — Config editor (`e`) and forward manager (`f`) with add/remove | ✅ |

### Phase 4a: Monitor Tab (`pkg/tui/tab_monitor.go`) — ✅ Mostly Complete

812 lines. Resource tables + ntcharts streaming line charts + Unicode bar rendering + vibespace picker + pause/resume.

**Missing from design doc:**
- Activity table (Uptime, Messages, Tools, Tokens, Errors, State columns) — not implemented
- Viewport scrolling for overflow
- Agent table overflow (top 5 + "and N more...")

### Phase 4b: Remote Tab (`pkg/tui/tab_remote.go`) — ✅ Complete

Connect/disconnect, diagnostics, server management. Three modes: connected, disconnected, serving.

### Phase 5: Command Palette (`pkg/tui/overlay_palette.go`) — ✅ Complete

Fuzzy-filtered action list with `bubbles/textinput`. 9 actions (tab nav, new vibespace, new session, toggle help, quit). Multi-word case-insensitive search, up/down navigation, Enter to execute. Actions emit typed messages handled by App and tabs.

## Hardcoded Values

- Agent container images in `pkg/tui/tab_vibespaces.go` (`agentImage()`) are hardcoded per agent type (`claude-code` → `ghcr.io/vibespacehq/vibespace/claude-code:latest`, `codex` → `ghcr.io/vibespacehq/vibespace/codex:latest`). Should be resolved from the deployment/k8s layer instead — `vibespace.AgentInfo` doesn't carry an image field.

## Simultaneous Local + Remote Mode

Currently the system assumes one active cluster per session — remote OR local, never both. When remote is connected, local is completely ignored. This needs to change so users can see and manage vibespaces from both clusters simultaneously, with clear visual differentiation.

### Current bottleneck: single-cluster assumption

The chain starts in `resolveKubeconfig()` (`internal/cli/errors.go`): if `isRemoteConnected()` returns true, it returns the remote kubeconfig and the local kubeconfig is never consulted. This cascades through every layer:

| Layer | Current | Needed |
|-------|---------|--------|
| `resolveKubeconfig()` | Returns one path (remote > local) | Return map of cluster name → kubeconfig path |
| `k8s.NewClient()` | Reads `KUBECONFIG` env var | Accept kubeconfig path as parameter |
| `vibespace.Service` | One instance, one cluster | Multiple instances keyed by cluster |
| Daemon | One socket (`daemon.sock`), one reconciler | Multi-cluster aware or separate daemons |
| TUI `SharedState` | One `*vibespace.Service`, one `*daemon.Client` | `map[string]*vibespace.Service` |
| CLI commands | `getVibespaceService()` → single cluster | Accept `--cluster` flag or infer from vibespace name |

### What changes

**Core:**
- `resolveKubeconfig()` → detect both local and remote kubeconfigs, return both when available
- `k8s.NewClient(kubeconfigPath)` → pass path explicitly instead of relying on env var
- `SharedState` → hold multiple services, track which cluster each vibespace belongs to
- State files → `remote.json` becomes one of N cluster configs, not a binary toggle

**CLI:**
- Add `--cluster` flag to commands (or infer from vibespace name)
- `getVibespaceService()` → accept cluster context

**TUI:**
- Vibespaces tab → show vibespaces from both clusters with "Local" / "Remote" badge
- Monitor tab → cluster selector or merged view
- All tabs need cluster context awareness

### Effort

- CLI-only dual-mode (`--cluster` flag): M (2-3 days)
- Full TUI dual-mode with visual differentiation: L (1-2 weeks)

## Known Bugs

# CLI Implementation Status

Last verified: 2026-01-28

---

## Current Status

### Command Matrix

| Command | JSON | Plain | Stream | Exit Codes | Notes |
|---------|:----:|:-----:|:------:|:----------:|-------|
| **Cluster** |
| `init` | ✅ | - | - | ✅ | Platform auto-detect |
| `status` | ✅ | - | - | ✅ | Shows cluster + daemon + components |
| `stop` | ✅ | - | - | ✅ | Stops cluster |
| `uninstall` | ✅ | - | - | ✅ | Removes cluster and data |
| **Vibespace** |
| `create` | ✅ | - | - | ✅ | `-t claude-code\|codex` |
| `list` | ✅ | ✅ | - | ✅ | Tab-separated with `--header` |
| `delete` | ✅ | - | - | ✅ | `--keep-data` flag |
| **Agent** |
| `<vs> agent list` | ✅ | ✅ | - | ✅ | Tab-separated with `--header` |
| `<vs> agent create` | ✅ | - | - | ✅ | `-t claude-code\|codex` |
| `<vs> agent delete` | ✅ | - | - | ✅ | |
| `<vs> start` | ✅ | - | - | ✅ | Start all or specific agent |
| `<vs> stop` | ✅ | - | - | ✅ | Stop all or specific agent |
| `<vs> connect` | N/A | N/A | - | ✅ | Requires TTY, `--browser` flag |
| **Config** |
| `<vs> config show` | ✅ | ✅ | - | ✅ | Tab-separated with `--header` |
| `<vs> config set` | ✅ | - | - | ✅ | |
| **Port Forwarding** |
| `<vs> forward list` | ✅ | ✅ | - | ✅ | Tab-separated with `--header` |
| `<vs> forward add` | ✅ | - | - | ✅ | |
| `<vs> forward remove` | ✅ | - | - | ✅ | |
| `<vs> ports` | ✅ | ✅ | - | ✅ | Detected ports, tab-separated |
| **Session** |
| `session list` | ✅ | ✅ | - | ✅ | Tab-separated with `--header` |
| `session show` | ✅ | - | - | ✅ | |
| `session delete` | ✅ | - | - | ✅ | |
| **Multi (Headless)** |
| `multi --list-sessions` | ✅ | ✅ | - | ✅ | Tab-separated with `--header` |
| `multi --list-agents` | ✅ | ✅ | - | ✅ | Tab-separated with `--header` |
| `multi "message"` | ✅ | ✅ | ✅ | ✅ | `--agent` flag for targeting |
| `multi -r <id> "msg"` | ✅ | ✅ | ✅ | ✅ | Resume session |
| `multi --batch` | ✅ | - | - | ✅ | JSONL input from stdin |
| **Utility** |
| `version` | ✅ | - | - | ✅ | |
| `completion` | N/A | N/A | - | ✅ | Shell script output |

### Output Modes

| Mode | Flag | Auto-Enable | Description |
|------|------|-------------|-------------|
| Human | (default) | TTY detected | Styled tables, colors, spinners |
| JSON | `--json` | Non-TTY | Standard envelope: `{success, data, error, meta}` |
| Plain | `--plain` | Never | Tab-separated, `--header` for column names |
| Stream | `--stream` | Never | Plain text, line-by-line (multi only) |

### JSON Envelope

All JSON output uses standard envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": {
    "schema_version": "1",
    "cli_version": "dev",
    "timestamp": "2026-01-28T00:00:00Z"
  }
}
```

Error responses include:

```json
{
  "success": false,
  "data": null,
  "error": {
    "message": "vibespace 'foo' not found",
    "code": "NOT_FOUND",
    "exit_code": 10,
    "hint": "Use 'vibespace list' to see available vibespaces"
  },
  "meta": { ... }
}
```

### Exit Codes

| Code | Constant | Used For |
|------|----------|----------|
| 0 | `ExitSuccess` | Success |
| 1 | `ExitInternal` | Internal errors |
| 2 | `ExitUsage` | Invalid args/flags |
| 10 | `ExitNotFound` | Resource not found |
| 11 | `ExitConflict` | Already exists |
| 12 | `ExitPermission` | Permission denied |
| 13 | `ExitTimeout` | Timeout |
| 14 | `ExitCancelled` | User cancelled |
| 15 | `ExitUnavailable` | Cluster/daemon down |

### Plain Mode Columns

| Command | Columns |
|---------|---------|
| `list` | NAME, STATUS, AGENTS, CPU, MEMORY, STORAGE, CREATED |
| `agent list` | AGENT, TYPE, VIBESPACE, STATUS |
| `forward list` | AGENT, LOCAL, REMOTE, TYPE, STATUS |
| `config show` | AGENT, TYPE, SKIP_PERMISSIONS, MODEL, MAX_TURNS, REASONING_EFFORT |
| `ports` | PORT, PROCESS, DETECTED_AT |
| `session list` | NAME, VIBESPACES, AGENTS, LAST_USED |
| `multi --list-sessions` | SESSION, VIBESPACES, LAST USED |
| `multi --list-agents` | SESSION, AGENT |

### Foundation

| Component | Status | Location |
|-----------|--------|----------|
| Brand colors (Teal, Pink, Orange, Yellow) | ✅ | `pkg/ui/colors.go` |
| Lipgloss styling | ✅ | `pkg/ui/styles.go` |
| Unified tables | ✅ | `pkg/ui/table.go` |
| JSON types | ✅ | `internal/cli/json_types.go` |
| Error handling | ✅ | `pkg/errors/errors.go` |
| Output handler | ✅ | `internal/cli/output.go` |
| Spinners | ✅ | `internal/cli/spinner.go` |
| NO_COLOR support | ✅ | Auto-detected |
| Non-TTY detection | ✅ | Auto-switches to JSON |

---

## Remaining Work

### P1: Linux Support

**Goal:** Run vibespace on Linux using Lima VM (same approach as macOS with Colima).

| Task | Description | Effort |
|------|-------------|--------|
| Create `lima.go` | Lima VM manager for Linux | M |
| Update `manager.go` | Return LimaManager for Linux | S |
| Test on Ubuntu/Debian | Verify full workflow | M |
| Test on Fedora/RHEL | Verify full workflow | S |
| Optional: `--native` flag | Direct k3s without VM | L |

**Implementation notes:**
- Lima releases include Linux binaries
- Use `limactl create template://k3s --name vibespace`
- Paths: `~/.lima/vibespace` instead of `~/.colima/_lima/colima-vibespace`

### P2: Remote Mode

**Goal:** Connect to a vibespace cluster running on another machine via WireGuard.

| Task | Description | Effort |
|------|-------------|--------|
| `vibespace serve` | Start WireGuard server, expose API | L |
| `vibespace remote connect <host>` | Connect to remote cluster | M |
| `vibespace remote disconnect` | Disconnect from remote | S |
| Kubeconfig switching | Manage local vs remote contexts | M |
| Connection status | Show in `vibespace status` | S |

### P3: Help & Diagnostics

| Task | Description | Effort |
|------|-------------|--------|
| `vibespace doctor` | Diagnose common issues (Docker, ports, disk) | M |
| `vibespace wait` | Wait for cluster ready state | S |
| `<vs> wait` | Wait for agent(s) ready | S |
| `vibespace help --json` | Machine-readable command schema | M |
| Improve `--help` text | Consistent examples, better descriptions | S |

### P4: Testing

**Goal:** 100% test coverage for CLI commands.

| Task | Description | Effort |
|------|-------------|--------|
| Unit tests for `internal/cli` | Test each command handler | L |
| Unit tests for `pkg/errors` | Test exit code mapping | S |
| Unit tests for `pkg/ui` | Test table/style rendering | S |
| Integration tests | End-to-end CLI workflows | L |
| Contract tests | Validate JSON/plain output schemas | M |
| CI pipeline | Run tests on PR | M |

### P5: Declarative Config

| Task | Description | Effort |
|------|-------------|--------|
| `vibespace apply -f spec.yaml` | Create/update from YAML | L |
| `vibespace diff -f spec.yaml` | Show what apply would change | M |
| `vibespace export <vs>` | Export vibespace to YAML | M |
| Spec schema | Define YAML schema, validate | M |

### P6: Automation Enhancements

| Task | Description | Effort |
|------|-------------|--------|
| `--plan` / `--apply` for delete | Two-step confirmation | S |
| `--status` filter for list | Filter by running/stopped/error | S |
| `--sort` flag for list | Sort by name/created/status | S |
| `--limit`/`--cursor` pagination | For large result sets | M |
| `--timeout` global flag | Timeout for all operations | S |
| `--non-interactive` global flag | Disable all prompts | S |
| `multi --stream --json` | JSONL streaming output | M |

### P7: Versioning & Releases

| Task | Description | Effort |
|------|-------------|--------|
| Build-time version injection | `-ldflags "-X main.version=..."` | S |
| `version` enhancements | Show commit, build date, Go version | S |
| GitHub Actions release | Auto-release on tag | M |
| Changelog generation | From conventional commits | S |
| Update checker | Notify when new version available | M |

### P8: Distribution

| Task | Description | Effort |
|------|-------------|--------|
| GitHub Releases | Pre-built binaries (darwin/linux, amd64/arm64) | M |
| Homebrew formula | `brew install vibespace` | S |
| Install script | `curl -sSL .../install.sh \| bash` | S |
| Go install | `go install github.com/.../vibespace@latest` | S |
| APT repository | For Debian/Ubuntu | M |
| RPM repository | For RHEL/Fedora | M |
| AUR package | For Arch Linux | S |

### P9: Feature Flags

| Task | Description | Effort |
|------|-------------|--------|
| `--experimental` global flag | Enable experimental features | S |
| `VIBESPACE_EXPERIMENTAL=1` env | Environment variable toggle | S |
| `vibespace features list` | Show available flags | S |
| `vibespace features enable <flag>` | Enable specific flag | S |
| Feature graduation process | Promote stable features | S |

### P10: JSON Enhancements

| Task | Description | Effort |
|------|-------------|--------|
| `request_id` in meta | Correlation ID with `--verbose` | S |
| `duration_ms` in meta | Operation timing with `--verbose` | S |
| `warnings` array in meta | Non-fatal warnings | S |

---

## Effort Key

| Symbol | Meaning |
|--------|---------|
| S | Small (< 1 day) |
| M | Medium (1-3 days) |
| L | Large (> 3 days) |

# CLI v2 Migration Guide

This document maps current CLI commands to the proposed v2 structure and defines output mode support for scriptability.

**Goal:** Every command must be fully scriptable with predictable, parseable output.

---

## Command Mapping

### Root Commands

| Current | v2 | Notes |
|---------|-----|-------|
| `vibespace init` | `vibespace init` | No change |
| `vibespace status` | `vibespace status` | No change |
| `vibespace stop` | `vibespace stop` | No change |
| `vibespace uninstall` | `vibespace uninstall` | Add `--plan` / `--apply` for automation |
| `vibespace create <name>` | `vibespace create <name>` | No change |
| `vibespace list` | `vibespace list` | Add filtering/sorting/pagination |
| `vibespace delete <name>` | `vibespace delete <name>` | Add `--plan` / `--apply` for automation |
| `vibespace version` | `vibespace version` | No change |
| `vibespace completion` | `vibespace completion` | No change |

### New Root Commands

| Command | Description |
|---------|-------------|
| `vibespace apply <file>` | Declarative create/update from YAML spec |
| `vibespace diff <file>` | Show what `apply` would change |
| `vibespace export <name>` | Export vibespace spec to YAML |
| `vibespace wait` | Wait for cluster/agent ready state |
| `vibespace doctor` | Diagnose common issues |
| `vibespace help --json` | Machine-readable command schema |

### Session Commands

| Current | v2 | Notes |
|---------|-----|-------|
| `vibespace session list` | `vibespace session list` | No change |
| `vibespace session show <name>` | `vibespace session show <name>` | No change |
| `vibespace session delete <name>` | `vibespace session delete <name>` | No change |

### Multi Command

| Current | v2 | Notes |
|---------|-----|-------|
| `vibespace multi --vibespaces <vs>` | `vibespace multi --vibespaces <vs>` | No change |
| `vibespace multi --agents <a@vs>` | `vibespace multi --agents <a@vs>` | No change |
| `vibespace <vs> multi` | **REMOVED** | Use `vibespace multi --vibespaces <vs>` |

### Vibespace-Scoped Commands

| Current | v2 | Notes |
|---------|-----|-------|
| `vibespace <vs> agents` | `vibespace <vs> agent list` | Renamed for consistency |
| `vibespace <vs> spawn` | `vibespace <vs> agent create` | Renamed for consistency |
| `vibespace <vs> kill <agent>` | `vibespace <vs> agent delete <agent>` | Renamed for consistency |
| `vibespace <vs> up` | `vibespace <vs> start` | Alias removed, single name |
| `vibespace <vs> down` | `vibespace <vs> stop` | Alias removed, single name |
| `vibespace <vs> start` | `vibespace <vs> start` | No change |
| `vibespace <vs> stop` | `vibespace <vs> stop` | No change |
| `vibespace <vs> connect` | `vibespace <vs> connect` | No change |
| `vibespace <vs> config show` | `vibespace <vs> config show` | No change |
| `vibespace <vs> config set` | `vibespace <vs> config set` | Add JSON output |
| `vibespace <vs> forward list` | `vibespace <vs> forward list` | No change |
| `vibespace <vs> forward add` | `vibespace <vs> forward add` | Add JSON output |
| `vibespace <vs> forward remove` | `vibespace <vs> forward remove` | Add JSON output |
| `vibespace <vs> ports` | `vibespace <vs> ports` | Add JSON output |

### New Vibespace-Scoped Commands

| Command | Description |
|---------|-------------|
| `vibespace <vs> wait` | Wait for agent(s) to be ready |

---

## Output Mode Support Matrix

**Modes:**
- **JSON**: Machine-readable, standard envelope format
- **Plain**: Tab-separated values, no headers, stable columns
- **Human**: Colored, formatted for terminal (default in TTY)

**Non-TTY Behavior:** Defaults to JSON unless `--plain` or `--human` specified.

### Root Commands

| Command | JSON | Plain | Human | Non-TTY Default |
|---------|:----:|:-----:|:-----:|-----------------|
| `version` | ✅ | ❌ | ✅ | JSON |
| `init` | ✅ | ❌ | ✅ | JSON |
| `status` | ✅ | ❌ | ✅ | JSON |
| `stop` | ✅ | ❌ | ✅ | JSON |
| `uninstall` | ✅ | ❌ | ✅ | JSON (requires `--apply`) |
| `create` | ✅ | ❌ | ✅ | JSON |
| `list` | ✅ | ✅ | ✅ | JSON |
| `delete` | ✅ | ❌ | ✅ | JSON |
| `apply` | ✅ | ❌ | ✅ | JSON |
| `diff` | ✅ | ❌ | ✅ | JSON |
| `export` | ✅ | ❌ | ✅ | JSON (YAML output) |
| `wait` | ✅ | ❌ | ✅ | JSON |
| `doctor` | ✅ | ❌ | ✅ | JSON |
| `help` | ✅ | ❌ | ✅ | Human |
| `completion` | N/A | N/A | ✅ | Shell script |

### Session Commands

| Command | JSON | Plain | Human | Non-TTY Default |
|---------|:----:|:-----:|:-----:|-----------------|
| `session list` | ✅ | ✅ | ✅ | JSON |
| `session show` | ✅ | ❌ | ✅ | JSON |
| `session delete` | ✅ | ❌ | ✅ | JSON |

### Multi Command

| Command | JSON | Plain | Human | Non-TTY Default |
|---------|:----:|:-----:|:-----:|-----------------|
| `multi` (interactive) | ❌ | ❌ | ✅ | N/A (requires TTY) |
| `multi` (message) | ✅ | ✅ | ❌ | JSON |
| `multi --stream` | ✅ (JSONL) | ✅ | ❌ | JSONL |
| `multi --batch` | ✅ (JSONL) | ❌ | ❌ | JSONL |
| `multi --list-agents` | ✅ | ✅ | ✅ | JSON |
| `multi --list-sessions` | ✅ | ✅ | ✅ | JSON |

### Vibespace-Scoped Commands

| Command | JSON | Plain | Human | Non-TTY Default |
|---------|:----:|:-----:|:-----:|-----------------|
| `<vs> agent list` | ✅ | ✅ | ✅ | JSON |
| `<vs> agent create` | ✅ | ❌ | ✅ | JSON |
| `<vs> agent delete` | ✅ | ❌ | ✅ | JSON |
| `<vs> start` | ✅ | ❌ | ✅ | JSON |
| `<vs> stop` | ✅ | ❌ | ✅ | JSON |
| `<vs> connect` | ❌ | ❌ | ✅ | N/A (requires TTY) |
| `<vs> config show` | ✅ | ✅ | ✅ | JSON |
| `<vs> config set` | ✅ | ❌ | ✅ | JSON |
| `<vs> forward list` | ✅ | ✅ | ✅ | JSON |
| `<vs> forward add` | ✅ | ❌ | ✅ | JSON |
| `<vs> forward remove` | ✅ | ❌ | ✅ | JSON |
| `<vs> ports` | ✅ | ✅ | ✅ | JSON |
| `<vs> wait` | ✅ | ❌ | ✅ | JSON |

---

## Plain Mode Column Definitions

Plain mode outputs tab-separated values with stable column order. No headers by default; use `--header` to include them.

### `vibespace list --plain`

```
COLUMNS: name, status, agent_count, cpu, memory, storage, created_at
```

Example:
```
myproject	running	2	1000m	1Gi	10Gi	2026-01-27T10:30:00Z
test	stopped	1	500m	512Mi	5Gi	2026-01-26T08:00:00Z
```

### `vibespace <vs> agent list --plain`

```
COLUMNS: name, type, vibespace, status
```

Example:
```
claude-1	claude-code	myproject	running
codex-1	codex	myproject	running
```

### `vibespace <vs> forward list --plain`

```
COLUMNS: agent, local_port, remote_port, type, status
```

Example:
```
claude-1	61119	22	ssh	active
claude-1	17681	7681	ttyd	active
```

### `vibespace <vs> config show --plain`

```
COLUMNS: agent, type, skip_permissions, model, max_turns, reasoning_effort
```

Example:
```
claude-1	claude-code	true	opus	0
codex-1	codex	true	gpt-5.2-codex	0	high
```

### `vibespace <vs> ports --plain`

```
COLUMNS: port, process, detected_at
```

Example:
```
3000	node	2026-01-27T10:35:00Z
8080	python	2026-01-27T10:30:00Z
```

### `vibespace session list --plain`

```
COLUMNS: name, vibespace_count, last_used
```

Example:
```
abc123	2	2026-01-27T10:00:00Z
mywork	1	2026-01-26T15:30:00Z
```

### `vibespace multi --list-agents --plain`

```
COLUMNS: agent
```

Example:
```
claude-1@myproject
codex-1@myproject
claude-1@test
```

### `vibespace multi --list-sessions --plain`

```
COLUMNS: name, vibespaces, created_at, last_used
```

Example:
```
abc123	myproject,test	2026-01-27T09:00:00Z	2026-01-27T10:00:00Z
```

---

## JSON Envelope Format

All JSON output uses a standard envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": {
    "schema_version": "1",
    "cli_version": "1.0.0",
    "timestamp": "2026-01-27T10:30:00Z"
  }
}
```

### Error Format

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "exit_code": 10,
    "message": "vibespace 'foo' not found",
    "hint": "Run 'vibespace list' to see available vibespaces"
  },
  "meta": { ... }
}
```

### Streaming Format (JSONL)

For `multi --stream --json`, each line is a separate JSON event:

```jsonl
{"event":"start","session":"abc123","agents":["claude-1@test"]}
{"event":"token","agent":"claude-1@test","content":"Hello"}
{"event":"tool_use","agent":"claude-1@test","tool":"Bash","input":"ls"}
{"event":"tool_result","agent":"claude-1@test","tool":"Bash","output":"file.txt"}
{"event":"response","agent":"claude-1@test","content":"Done","tool_uses":[...]}
{"event":"end","session":"abc123","duration_ms":1234}
```

---

## Exit Codes

| Code | Meaning | JSON `error.code` |
|------|---------|-------------------|
| 0 | Success | N/A |
| 1 | Unexpected internal error | `INTERNAL` |
| 2 | Usage error (invalid args/flags) | `USAGE` |
| 10 | Resource not found | `NOT_FOUND` |
| 11 | Conflict (already exists, state conflict) | `CONFLICT` |
| 12 | Permission denied | `PERMISSION` |
| 13 | Timeout | `TIMEOUT` |
| 14 | User cancelled | `CANCELLED` |
| 15 | Service unavailable (cluster down) | `UNAVAILABLE` |

---

## Non-Interactive Automation Patterns

### Plan + Apply for Dangerous Actions

```bash
# Generate plan (returns plan_id)
vibespace delete myproject --plan --json
# {"success":true,"data":{"plan_id":"delete-x1y2z3","resources":[...],"expires_at":"..."}}

# Apply plan (non-interactive)
vibespace delete myproject --apply --plan-id="delete-x1y2z3"
# {"success":true,"data":{"name":"myproject","deleted":true}}
```

Same pattern for `uninstall`:

```bash
vibespace uninstall --plan --json
vibespace uninstall --apply --plan-id="uninstall-a1b2c3"
```

### Wait for Ready State

```bash
# Wait for cluster to be ready
vibespace wait --for=cluster-ready --timeout=5m --json

# Wait for specific agent to be ready
vibespace myproject wait --for=agent-ready --agent=claude-1 --timeout=2m --json
```

### Filtering and Pagination

```bash
# Filter by status
vibespace list --status=running --json

# Sort by creation date (descending)
vibespace list --sort=-created_at --json

# Paginate results
vibespace list --limit=10 --cursor=abc123 --json
```

### Batch Operations

```bash
# Send multiple messages to different agents
cat <<EOF | vibespace multi --vibespaces test --batch
{"target":"claude-1@test","message":"task 1"}
{"target":"codex-1@test","message":"task 2"}
EOF

# Process responses as JSONL
vibespace multi --vibespaces test --stream --json "work on this" | while read -r line; do
  echo "$line" | jq -r '.event'
done
```

### Script Example: Create and Wait

```bash
#!/bin/bash
set -e

# Create vibespace
result=$(vibespace create myproject -t claude-code --json)
if ! echo "$result" | jq -e '.success' > /dev/null; then
  echo "Failed to create: $(echo "$result" | jq -r '.error.message')"
  exit 1
fi

# Wait for agent to be ready
vibespace myproject wait --for=agent-ready --timeout=2m --json

# Send initial message
vibespace multi --vibespaces myproject --json "initialize the project"
```

### Script Example: List and Process

```bash
#!/bin/bash

# Get all running vibespaces
vibespace list --status=running --json | jq -r '.data.vibespaces[].name' | while read -r vs; do
  # Get agent count
  count=$(vibespace "$vs" agent list --json | jq '.data.count')
  echo "$vs: $count agents"
done
```

---

## Human Output Design Guidelines

This section defines the visual design language for all human-readable CLI output.

### Design Principles

1. **Use advanced TUI libraries** (lipgloss, bubbletea, huh) for all interactive and formatted output
2. **Consistent color palette** centered around brand colors
3. **Graceful degradation** - always respect `NO_COLOR` and non-TTY environments
4. **Unified table style** - one table implementation for all list commands
5. **Structured help text** - consistent formatting across all commands

### Brand Color Palette

```
┌─────────────────────────────────────────────────────────┐
│  BRAND COLORS (foundation of design)                    │
├─────────────────────────────────────────────────────────┤
│  Teal     #00ABAB   Primary actions, prompts, links     │
│  Pink     #F102F3   Highlights, active states           │
│  Orange   #FF7D4B   Warnings, important notices         │
│  Yellow   #F5F50A   Caution, pending states             │
└─────────────────────────────────────────────────────────┘
```

### Semantic Color Mapping

| Semantic Use | Color | Hex | Usage |
|--------------|-------|-----|-------|
| **Primary** | Teal | `#00ABAB` | Commands, prompts, interactive elements, links |
| **Success** | Teal | `#00ABAB` | Success messages, running status, checkmarks |
| **Accent** | Pink | `#F102F3` | Highlights, focused items, active selections |
| **Warning** | Orange | `#FF7D4B` | Warnings, deprecated notices, attention needed |
| **Caution** | Yellow | `#F5F50A` | Pending states, in-progress, careful actions |
| **Error** | Red | `#FF4D4D` | Errors, failed states, destructive actions |
| **Dim** | Gray | `#666666` | Secondary text, timestamps, hints |
| **Muted** | Dark Gray | `#444444` | Borders, separators, disabled items |

### Agent Color Palette

For multi-agent views, cycle through these colors:

```
1. Teal    #00ABAB
2. Pink    #F102F3
3. Orange  #FF7D4B
4. Yellow  #F5F50A
5. Cyan    #00D9FF
6. Purple  #7B61FF
7. Green   #00FF9F
8. Coral   #FF6B6B
```

### NO_COLOR Support

All color output MUST respect the `NO_COLOR` environment variable and non-TTY detection:

```go
// Check order:
// 1. NO_COLOR env var set → no colors
// 2. TERM=dumb → no colors
// 3. stdout not TTY → no colors
// 4. --no-color flag → no colors
```

When colors are disabled:
- Remove all ANSI escape codes
- Use text indicators instead: `[ok]`, `[error]`, `[!]`, `[->]`
- Tables remain functional with ASCII borders or no borders

### Table Style (lipgloss)

Use lipgloss tables with rounded borders for ALL list commands:

```
╭──────────────┬──────────┬────────┬─────────────╮
│ NAME         │ STATUS   │ AGENTS │ CREATED     │
├──────────────┼──────────┼────────┼─────────────┤
│ myproject    │ running  │ 2      │ 1 hour ago  │
│ test         │ stopped  │ 1      │ yesterday   │
╰──────────────┴──────────┴────────┴─────────────╯
```

Style specification:
```go
table.New().
    Border(lipgloss.RoundedBorder()).
    BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))).
    Headers(...).
    StyleFunc(func(row, col int) lipgloss.Style {
        if row == table.HeaderRow {
            return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ABAB"))
        }
        return lipgloss.NewStyle().Padding(0, 1)
    })
```

**NO_COLOR fallback:** Simple space-padded columns, no borders:
```
NAME          STATUS    AGENTS  CREATED
myproject     running   2       1 hour ago
test          stopped   1       yesterday
```

### Message Prefixes

| Type | Colored | NO_COLOR | Usage |
|------|---------|----------|-------|
| Success | `✓` (teal) | `[ok]` | Operation completed |
| Error | `✗` (red) | `[error]` | Operation failed |
| Warning | `⚠` (orange) | `[!]` | Warning notice |
| Step | `→` (teal) | `[->]` | Progress step |
| Info | `•` (dim) | `[-]` | Informational |

Example:
```
✓ Vibespace 'myproject' created
→ Starting agent claude-1...
✓ Agent ready

# NO_COLOR:
[ok] Vibespace 'myproject' created
[->] Starting agent claude-1...
[ok] Agent ready
```

### Progress Indicators

Use lipgloss spinners for long-running operations:

```go
spinner.New(
    spinner.WithSpinner(spinner.Dot),
    spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#00ABAB"))),
)
```

**NO_COLOR/non-TTY fallback:** Simple text updates:
```
-> Initializing cluster...
-> Downloading binaries...
-> Starting VM...
[ok] Cluster ready
```

### Interactive Prompts (huh)

Use the `huh` library for all interactive prompts with brand theming:

```go
theme := huh.ThemeBase()
theme.Focused.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ABAB")).Bold(true)
theme.Focused.SelectSelector = lipgloss.NewStyle().Foreground(lipgloss.Color("#F102F3"))

huh.NewSelect[string]().
    Title("Select vibespace").
    Options(...).
    WithTheme(theme)
```

### Help Text Format

Standardize all help text using this structure:

```
<Short description>

<Long description if needed, wrapped at 80 chars>

Usage:
  vibespace <command> [flags]

Arguments:
  name    Description of argument

Flags:
  -f, --flag string   Description (default "value")
  -h, --help          Help for command

Examples:
  vibespace command arg              # Comment
  vibespace command --flag value     # Another example

See also:
  vibespace other-command
```

For vibespace-scoped commands, use Cobra's built-in formatting (not manual fmt.Println).

### Config/Detail Views

Use indented key-value format with icons:

```
  ⬡ claude-1

    ◉ skip_permissions     enabled
    ○ share_credentials    disabled
    ⚙ allowed_tools        Bash, Read, Write
    ◈ model                opus
```

Icons:
- `⬡` Section header
- `◉` Enabled boolean
- `○` Disabled boolean
- `⚙` List/array value
- `◈` String value
- `↻` Numeric value

**NO_COLOR fallback:**
```
  claude-1

    skip_permissions:     enabled
    share_credentials:    disabled
    allowed_tools:        Bash, Read, Write
    model:                opus
```

### Time Formats

| Context | Format | Example |
|---------|--------|---------|
| Tables (human) | Relative | `1 hour ago`, `yesterday` |
| Tables (plain) | ISO 8601 | `2026-01-27T10:30:00Z` |
| Detail views | Mixed | `2026-01-27 10:30 (1 hour ago)` |
| JSON | ISO 8601 | `2026-01-27T10:30:00Z` |

### Status Indicators

| Status | Colored | Symbol | NO_COLOR |
|--------|---------|--------|----------|
| Running | Teal | `●` | `[running]` |
| Stopped | Dim | `○` | `[stopped]` |
| Creating | Yellow | `◐` | `[creating]` |
| Error | Red | `✗` | `[error]` |
| Pending | Orange | `◌` | `[pending]` |

### Library Usage Summary

| Component | Library | Fallback |
|-----------|---------|----------|
| Tables | lipgloss/table | printf with padding |
| Spinners | bubbletea/spinner | Text updates |
| Prompts | huh | Simple stdin read |
| Colors | lipgloss | None (text only) |
| Borders | lipgloss | ASCII or none |
| Full TUI | bubbletea | Not applicable |

### Implementation Checklist

- [ ] Create shared `pkg/ui` package with brand colors and styles
- [ ] Migrate CLI output.go to use lipgloss instead of fatih/color
- [ ] Unify all tables to use lipgloss/table
- [ ] Standardize all help text to Cobra format
- [ ] Add huh theming for interactive prompts
- [ ] Implement NO_COLOR fallbacks for all components
- [ ] Update spinners to use lipgloss styling
- [ ] Create style guide examples in docs

---

## Migration Checklist

### Phase 1: JSON Support (All Commands)

- [ ] Add JSON output to `init`
- [ ] Add JSON output to `stop`
- [ ] Add JSON output to `create`
- [ ] Add JSON output to `<vs> agent create` (spawn)
- [ ] Add JSON output to `<vs> agent delete` (kill)
- [ ] Add JSON output to `<vs> start` (up)
- [ ] Add JSON output to `<vs> stop` (down)
- [ ] Add JSON output to `<vs> config set`
- [ ] Add JSON output to `<vs> forward add`
- [ ] Add JSON output to `<vs> forward remove`
- [ ] Add JSON output to `<vs> ports`
- [ ] Add JSON output to `session delete`
- [ ] Add `meta` field to all existing JSON outputs
- [ ] Implement standard error envelope for all commands

### Phase 2: Plain Mode

- [ ] Add plain mode to `session list`
- [ ] Add plain mode to `<vs> config show`
- [ ] Add plain mode to `<vs> ports`
- [ ] Add plain mode to `multi --list-agents`
- [ ] Add plain mode to `multi --list-sessions`
- [ ] Add `--header` flag for plain mode

### Phase 3: Command Renames

- [ ] `agents` → `agent list`
- [ ] `spawn` → `agent create`
- [ ] `kill` → `agent delete`
- [ ] Remove `up`/`down` aliases
- [ ] Remove `<vs> multi`

### Phase 4: New Commands

- [ ] `wait`
- [ ] `doctor`
- [ ] `apply`
- [ ] `diff`
- [ ] `export`
- [ ] `help --json`

### Phase 5: Automation Features

- [ ] `--plan` / `--apply` for `delete`
- [ ] `--plan` / `--apply` for `uninstall`
- [ ] Filtering flags (`--status`, `--sort`, etc.)
- [ ] Pagination (`--limit`, `--cursor`)
- [ ] Exit codes (10+ for domain errors)

### Phase 6: Non-TTY Defaults

- [ ] Default to JSON in non-TTY for all commands
- [ ] Add `--human` flag to force human output
- [ ] Suppress progress output in non-TTY unless `--verbose`

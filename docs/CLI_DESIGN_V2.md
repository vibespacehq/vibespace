# vibespace CLI Design v2

A proposal for making the CLI consistent, predictable, and automation-friendly.

**Status:** Draft
**Created:** 2026-01-26

---

## Current Problems

### 1. Output Mode Inconsistency

| Issue | Example |
|-------|---------|
| Some commands support JSON + plain | `list`, `agents`, `forward list` |
| Some commands support JSON only | `status`, `session list`, `config show` |
| Some commands silently ignore `--json` | `config set` - flag ignored, outputs text |
| Some commands have no machine output | `spawn`, `kill`, `up`, `down`, `ports` |

**Impact:** Scripts break silently when `--json` is ignored.

### 2. Inconsistent Error Schemas

```bash
# Standard commands return:
{"success": false, "error": {"message": "..."}}

# Multi command returns:
{"error": "...", "session": "", "request": {...}, "responses": null}
```

**Impact:** Can't write one `jq` filter for all errors.

### 3. Non-TTY Behavior is Arbitrary

| Command | Non-TTY Behavior | Problem |
|---------|------------------|---------|
| `delete` | Requires `--force` | Fine |
| `uninstall` | Blocked entirely | No escape hatch for automation |
| `connect` | Blocked | Requires SSH terminal - valid |
| `<vs> multi` | Blocked | TUI only, but top-level works - confusing |

### 4. Plain Mode is Underspecified

- No documented column order
- No stability guarantee
- No escaping rules
- Headers included? Unknown.

### 5. Duplicated Command Surface

- `vibespace multi` vs `vibespace <vs> multi` (scoped version is worse)
- Inconsistent: `spawn`/`kill` vs `create`/`delete`

### 6. Exit Codes Undocumented

Everything returns 0 or 1. No way to distinguish error types.

### 7. Naming Inconsistency

- `spawn` vs `create`, `kill` vs `delete`
- `up`/`down` and `start`/`stop` as aliases
- Mixed noun/verb patterns

---

## Design Principles

1. **No silent failures** - If a flag is not supported, error loudly
2. **One schema everywhere** - Same JSON envelope for all commands
3. **Predictable non-TTY** - Default to JSON, no human progress lines
4. **Stable plain output** - Columns are documented
5. **Meaningful exit codes** - Scripts can branch on error type
6. **No duplicate lesser commands** - One way to do things
7. **Self-enforcing** - Contract tests validate every command

---

## Solutions

### 1. Universal Output Contract

**Rule:** Every command supports `--json`. No exceptions. No silent ignores.

**Rule:** In non-TTY, default to JSON output. Forbid human progress lines unless `--verbose`.

```
┌─────────────────────────────────────────────────────────────┐
│  --json       Machine-readable JSON (envelope format)       │
│  --plain      Tab-separated, no headers, stable columns     │
│  --human      Force human-readable (default in TTY only)    │
│  (non-TTY)    Defaults to --json                            │
└─────────────────────────────────────────────────────────────┘
```

---

### 2. Normalized JSON Schema

**Envelope format for ALL commands:**

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "meta": {
    "schema_version": "1",
    "cli_version": "1.2.0",
    "timestamp": "2026-01-26T00:30:00Z"
  }
}
```

**Meta field is always present.** Minimal by default, extended with `--verbose`:

| Field | Default | With `--verbose` |
|-------|---------|------------------|
| `schema_version` | ✓ | ✓ |
| `cli_version` | ✓ | ✓ |
| `timestamp` | ✓ | ✓ |
| `request_id` | | ✓ |
| `duration_ms` | | ✓ |
| `warnings` | ✓ (if any) | ✓ (if any) |

**Warnings array:**

```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "schema_version": "1",
    "cli_version": "1.2.0",
    "timestamp": "2026-01-26T00:30:00Z",
    "warnings": [
      {"code": "DEPRECATED_FLAG", "message": "--old-flag is deprecated, use --new-flag"}
    ]
  }
}
```

Warnings do not affect exit code (success is still true).

**Error format:**

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "message": "vibespace 'foo' not found",
    "hint": "Run 'vibespace list' to see available vibespaces"
  },
  "meta": {
    "schema_version": "1",
    "cli_version": "1.2.0",
    "timestamp": "2026-01-26T00:30:00Z"
  }
}
```

---

### 3. Exit Codes

Reserve `1` for unexpected internal errors. Use `10+` for typed domain errors.

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Unexpected internal error |
| 2 | Usage error (invalid args/flags) |
| 10 | NOT_FOUND - Resource doesn't exist |
| 11 | CONFLICT - Already exists or state conflict |
| 12 | PERMISSION - Auth or permission failure |
| 13 | TIMEOUT - Operation timed out |
| 14 | CANCELLED - User cancelled |
| 15 | UNAVAILABLE - Service unavailable (cluster down, daemon not running) |

Error JSON includes the exit code:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "exit_code": 10,
    "message": "vibespace 'foo' not found",
    "hint": "Run 'vibespace list' to see available vibespaces"
  }
}
```

---

### 4. JSON Streaming (JSONL)

For streaming output (`multi --stream`), use JSONL events, not one big object:

```jsonl
{"event":"start","session":"abc123","agents":["claude-1@test"]}
{"event":"token","agent":"claude-1@test","content":"Hello"}
{"event":"token","agent":"claude-1@test","content":" world"}
{"event":"tool_use","agent":"claude-1@test","tool":"Bash","input":"ls"}
{"event":"tool_result","agent":"claude-1@test","tool":"Bash","output":"file.txt"}
{"event":"response","agent":"claude-1@test","content":"Hello world","tool_uses":[...]}
{"event":"end","session":"abc123","duration_ms":1234}
```

Each line is valid JSON. Parsers can process line-by-line without buffering.

Use `--stream --json` for JSONL, `--stream --plain` for text streaming.

---

### 5. Two-Step Confirmation for Dangerous Actions

Instead of guessable confirmation strings, use plan + apply:

```bash
# Step 1: Generate plan
./vibespace uninstall --plan --json
{
  "success": true,
  "data": {
    "plan_id": "uninstall-a1b2c3",
    "resources": [
      {"type": "vm", "name": "colima"},
      {"type": "directory", "path": "~/.vibespace"}
    ],
    "expires_at": "2026-01-26T01:00:00Z"
  }
}

# Step 2: Apply plan
./vibespace uninstall --apply --plan-id="uninstall-a1b2c3" --non-interactive
```

This makes audits nicer and prevents accidental reuse.

**Same pattern for delete:**

```bash
./vibespace delete myproject --plan --json
./vibespace delete myproject --apply --plan-id="delete-x1y2z3" --non-interactive
```

For quick interactive use, `--force` still works in TTY.

---

### 6. Plain Mode Specification

**Rules:**
1. Tab-separated values
2. No header row by default
3. One record per line
4. Empty values are empty string
5. Values with tabs/newlines are escaped (`\t`, `\n`)

**Column definitions (stable):**

```
vibespace list --plain
COLUMNS: name, status, agent_count, cpu, memory, storage, created_at

vibespace <vs> agent list --plain
COLUMNS: name, vibespace, status, type

vibespace <vs> forward list --plain
COLUMNS: agent, local_port, remote_port, type, status

vibespace session list --plain
COLUMNS: name, vibespace_count, last_used
```

**Add `--header` flag:**
```bash
./vibespace list --plain --header
# name	status	agent_count	cpu	memory	storage	created_at
# test-mixed	running	2	1000m	1Gi	10Gi	2026-01-25T21:10:30Z
```

---

### 7. Filtering, Sorting, Pagination

**Filtering:**

```bash
./vibespace list --status=running
./vibespace list --name-prefix=test
./vibespace list --created-after=2026-01-01

./vibespace myproject agent list --type=claude-code
./vibespace myproject agent list --status=running

./vibespace session list --since=24h
```

**Sorting:**

```bash
./vibespace list --sort=created_at
./vibespace list --sort=-name  # descending
```

**Pagination:**

```bash
./vibespace list --limit=10 --cursor=abc123
```

JSON output includes pagination info:

```json
{
  "success": true,
  "data": {
    "vibespaces": [...],
    "pagination": {
      "total": 50,
      "limit": 10,
      "next_cursor": "xyz789",
      "has_more": true
    }
  },
  "meta": { ... }
}
```

---

### 8. Command Structure

```
vibespace
├── init
├── status
├── stop
├── uninstall [--plan] [--apply --plan-id=ID --non-interactive]
├── create <name>
├── list [--status] [--sort] [--limit] [--cursor]
├── delete <name> [--force | --plan | --apply --plan-id=ID]
├── apply <file>           # Declarative create/update
├── diff <file>            # Show what apply would change
├── export <name>          # Export vibespace spec
├── multi [--vibespaces] [--agents] [--json|--plain] [--stream] [--batch]
├── session
│   ├── list [--since]
│   ├── show <name>
│   └── delete <name>
├── wait [--for=cluster-ready|agent-ready] [--timeout]
├── doctor
├── completion [bash|zsh|fish]
├── help [--json]          # Machine-readable help
└── <vibespace>
    ├── start [agent]
    ├── stop [agent]
    ├── connect [agent] [--browser]
    ├── agent
    │   ├── list [--type] [--status]
    │   ├── create [--name] [--type] [--skip-permissions] [--model]
    │   └── delete <agent>
    ├── config
    │   ├── show [agent]
    │   └── set <agent> key=value [key=value...]
    ├── forward
    │   ├── list
    │   ├── add <port> [--agent] [--local]
    │   └── remove <port> [--agent]
    ├── ports
    └── wait [--for=agent-ready] [--agent] [--timeout]
```

**Multi command agent selection:**

```bash
# All agents from multiple vibespaces
./vibespace multi --vibespaces test1,test2

# Specific agents from different vibespaces
./vibespace multi --agents claude-1@test1,codex-1@test2

# Mix: all from test1 + specific agent from test2
./vibespace multi --vibespaces test1 --agents claude-1@test2
```

**Removed:**
- `spawn` → `agent create`
- `kill` → `agent delete`
- `up` → `start`
- `down` → `stop`
- `<vs> multi` → use top-level `multi --vibespaces <vs>`
- `agents` → `agent list`

---

### 9. Declarative Apply/Diff/Export

**Spec file format (YAML):**

```yaml
# vibespace.yaml
apiVersion: vibespace.dev/v1
kind: Vibespace
metadata:
  name: myproject
spec:
  storage: 20Gi
  agents:
    - name: claude-1
      type: claude-code
      config:
        skip_permissions: true
        model: opus
    - name: codex-1
      type: codex
```

**Commands:**

```bash
# Apply spec (create or update)
./vibespace apply vibespace.yaml --json

# Show what would change
./vibespace diff vibespace.yaml --json

# Export current state
./vibespace export myproject > vibespace.yaml
```

**Apply output:**

```json
{
  "success": true,
  "data": {
    "vibespace": "myproject",
    "action": "updated",
    "changes": [
      {"path": "spec.agents[0].config.model", "old": "sonnet", "new": "opus"},
      {"path": "spec.agents[1]", "action": "created"}
    ]
  },
  "meta": { ... }
}
```

---

### 10. Consistent Timeouts

`--timeout` is available on any network/cluster operation:

```bash
./vibespace init --timeout=10m
./vibespace create myproject --timeout=5m
./vibespace myproject start --timeout=2m
./vibespace multi --vibespaces test --timeout=30s "hello"
./vibespace wait --for=cluster-ready --timeout=5m
```

Default timeout is command-specific but always documented in `--help`.

TIMEOUT errors include hints:

```json
{
  "error": {
    "code": "TIMEOUT",
    "exit_code": 13,
    "message": "timed out waiting for agent to be ready",
    "hint": "Try increasing --timeout or check 'vibespace doctor'"
  }
}
```

---

### 11. Machine-Readable Help

```bash
./vibespace help --json
{
  "success": true,
  "data": {
    "commands": [
      {
        "name": "create",
        "path": ["vibespace", "create"],
        "description": "Create a new vibespace",
        "args": [{"name": "name", "required": true}],
        "flags": [
          {"name": "cpu", "type": "string", "default": "1000m"},
          {"name": "memory", "type": "string", "default": "1Gi"}
        ]
      }
    ]
  }
}

./vibespace myproject agent --help --json
# Returns schema for agent subcommands
```

---

### 12. Help Text Clarity

For nested `create` commands, help text always shows the noun:

```bash
./vibespace create --help
# Create a new vibespace
# Usage: vibespace create <name> [flags]

./vibespace myproject agent create --help
# Create a new agent in vibespace 'myproject'
# Usage: vibespace <vibespace> agent create [flags]
```

The mental model is always clear: `create vibespace` vs `create agent`.

---

### 13. Contract Test Suite

Every command is tested in three modes:

```go
func TestCommandContract(t *testing.T) {
    commands := getAllCommands()

    for _, cmd := range commands {
        t.Run(cmd.Name+"/tty", func(t *testing.T) {
            // Test with TTY simulation
            // Verify human-readable output
            // Verify exit code
        })

        t.Run(cmd.Name+"/json", func(t *testing.T) {
            // Test with --json flag
            // Validate against JSON schema
            // Verify no stdout noise (progress lines)
            // Verify meta field present
            // Verify exit code matches error.exit_code
        })

        t.Run(cmd.Name+"/plain", func(t *testing.T) {
            if !cmd.SupportsPlain {
                // Verify error when --plain used
                return
            }
            // Validate column count
            // Verify no headers (unless --header)
            // Verify escaping
        })
    }
}
```

**Validations:**
- JSON schema validation (envelope, meta, error format)
- No stdout noise in JSON mode (no progress lines, spinners)
- stderr only for errors and warnings
- Exit code matches `error.exit_code` in JSON
- Timestamps are ISO 8601 UTC
- Plain mode column count is stable

---

## Summary of Key Rules

1. **Non-TTY defaults to JSON** - No human output unless `--human` or `--verbose`
2. **Meta always present** - `schema_version`, `cli_version`, `timestamp` minimum
3. **Warnings in meta** - Don't affect exit code
4. **Exit code 1 = internal error** - Domain errors use 10+
5. **JSON streaming = JSONL** - One event per line
6. **Dangerous actions = plan + apply** - Not guessable strings
7. **Timeouts everywhere** - With hints on failure
8. **Contract tests enforce consistency** - No drift

---

## Implementation Checklist

### Output Consistency
- [ ] Add JSON support to all commands
- [ ] Add `meta` field to all JSON responses
- [ ] Default to JSON in non-TTY
- [ ] Add `--human` flag to force human output
- [ ] Fix `multi` error schema to use standard envelope
- [ ] Implement JSONL streaming for `multi --stream --json`

### Exit Codes
- [ ] Reserve 1 for internal errors
- [ ] Implement 10+ for domain errors
- [ ] Include `exit_code` in JSON error responses

### Non-TTY Support
- [ ] Add `--non-interactive` global flag
- [ ] Implement `--plan` and `--apply --plan-id` for uninstall
- [ ] Implement `--plan` and `--apply --plan-id` for delete
- [ ] Remove `<vs> multi`

### Filtering & Pagination
- [ ] Add `--status`, `--sort`, `--limit`, `--cursor` to list commands
- [ ] Add filtering flags to `agent list`, `session list`
- [ ] Include `pagination` in JSON response

### Declarative
- [ ] Implement `apply <file>`
- [ ] Implement `diff <file>`
- [ ] Implement `export <name>`

### Plain Mode
- [ ] Document columns for all list commands
- [ ] Add `--header` flag
- [ ] Ensure consistent escaping

### Command Renames
- [ ] `spawn` → `agent create`
- [ ] `kill` → `agent delete`
- [ ] `agents` → `agent list`
- [ ] Remove `up`/`down` aliases

### New Commands
- [ ] `wait`
- [ ] `doctor`
- [ ] `completion`
- [ ] `help --json`

### Timeouts
- [ ] Add `--timeout` to all network/cluster commands
- [ ] Include hints in TIMEOUT errors

### Testing
- [ ] Implement contract test suite
- [ ] JSON schema validation
- [ ] No-stdout-noise validation in JSON mode

---

## Checklist for New Commands

When adding a new command:

- [ ] Supports `--json` with standard envelope and `meta`
- [ ] Supports `--plain` with documented columns (if list-like)
- [ ] Returns appropriate exit code (10+ for domain errors)
- [ ] Works in non-TTY (defaults to JSON)
- [ ] No stdout noise in JSON mode
- [ ] Has `--help` with examples
- [ ] Help text is clear about what noun is being acted on
- [ ] Timestamps are ISO 8601 UTC
- [ ] Has `--timeout` if network/cluster operation
- [ ] Contract tests cover all three modes

# vibespace CLI Reference

Complete reference for CLI commands, design philosophy, and implementation status.

---

## Design Philosophy

### Core Principles

1. **Scriptability First** - Every command must work in CI/CD pipelines with predictable, parseable output
2. **Graceful Degradation** - Respect `NO_COLOR`, non-TTY environments, and provide text fallbacks
3. **Consistent Interface** - Same flags, same output patterns, same error handling across all commands
4. **Progressive Disclosure** - Simple defaults, advanced options available but not required

### Output Modes

| Mode | Flag | Default When | Format |
|------|------|--------------|--------|
| Human | (default) | TTY | Colored, formatted tables, spinners |
| JSON | `--json` | Non-TTY piping | Standard envelope with metadata |
| Plain | `--plain` | Scripting | Tab-separated, no colors, stable columns |

### Brand Color Palette

```
Teal     #00ABAB   Status (running), success, primary actions
Pink     #F102F3   Metadata values, highlights, active states
Orange   #FF7D4B   Entity names, titles, warnings
Yellow   #F5F50A   Caution, pending states
Red      #FF4D4D   Errors, failed states
Gray     #666666   Dim/secondary text
```

### Color Usage Guidelines

| Element | Color | Example |
|---------|-------|---------|
| Status indicators | Teal | `running`, `ready` |
| Metadata values | Pink | uptime `37m19s`, pid `15918` |
| Entity names | Orange | vibespace names, agent names |
| Section titles | Bold | `Cluster`, `Daemon`, `Vibespaces` |
| Success messages | Teal | `ok Agent created` |
| Warnings | Orange | `warning 'spawn' is deprecated` |
| Errors | Red | `error not found` |

### Message Prefixes

| Type | TTY | NO_COLOR |
|------|-----|----------|
| Success | `ok` (teal) | `[ok]` |
| Error | `error` (red) | `[error]` |
| Warning | `warning` (orange) | `[!]` |
| Step | `->` (teal) | `[->]` |

### Exit Codes

| Code | Meaning | JSON `error.code` |
|------|---------|-------------------|
| 0 | Success | N/A |
| 1 | Internal error | `INTERNAL` |
| 2 | Usage error (bad args/flags) | `USAGE` |
| 10 | Not found | `NOT_FOUND` |
| 11 | Conflict (exists, state) | `CONFLICT` |
| 12 | Permission denied | `PERMISSION` |
| 13 | Timeout | `TIMEOUT` |
| 14 | Cancelled | `CANCELLED` |
| 15 | Unavailable (cluster down) | `UNAVAILABLE` |

### JSON Envelope

All JSON output uses this structure:

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

Error response:
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "NOT_FOUND",
    "message": "vibespace 'foo' not found"
  },
  "meta": { ... }
}
```

---

## Implementation Checklist

### Foundation

- [x] `pkg/ui` package with brand colors and styles
- [x] Lipgloss-based CLI output (replaced fatih/color)
- [x] Unified table component with `ui.NewTable()`
- [x] NO_COLOR and non-TTY detection
- [x] Exit codes in `pkg/errors`
- [x] JSON envelope with metadata

### Output Modes

| Command | Human | JSON | Plain | Notes |
|---------|:-----:|:----:|:-----:|-------|
| `version` | [x] | [x] | - | |
| `init` | [x] | [ ] | - | Spinner degrades |
| `status` | [x] | [x] | - | Uses lipgloss/list |
| `stop` | [x] | [ ] | - | Spinner degrades |
| `create` | [x] | [ ] | - | Spinner degrades |
| `list` | [x] | [x] | [x] | |
| `delete` | [x] | [x] | - | Requires `--force` in non-TTY |
| `session list` | [x] | [x] | - | |
| `session show` | [x] | [x] | - | |
| `session delete` | [x] | - | - | |
| `multi` | [x] | [x] | [x] | Full headless support |
| `<vs> agents` | [x] | [x] | [x] | |
| `<vs> spawn` | [x] | - | - | |
| `<vs> kill` | [x] | - | - | |
| `<vs> up/down` | [x] | - | - | |
| `<vs> connect` | [x] | - | - | Requires TTY |
| `<vs> config show` | [x] | [x] | - | |
| `<vs> config set` | [x] | - | - | |
| `<vs> forward list` | [x] | [x] | [x] | |
| `<vs> forward add` | [x] | - | - | |
| `<vs> forward remove` | [x] | - | - | |
| `<vs> ports` | [x] | - | - | |

### Styling

- [x] Tables use lipgloss with rounded borders
- [x] Table headers teal, borders muted gray
- [x] Status command uses lipgloss/list
- [x] Config show uses indented key-value format
- [x] Interactive forms (huh) use brand colors
- [x] TUI uses shared brand colors from pkg/ui

### Deferred (Future)

- [ ] `vibespace wait` - wait for ready state
- [ ] `vibespace doctor` - diagnose issues
- [ ] `vibespace apply/diff/export` - declarative config
- [ ] `--plan`/`--apply` flags for delete/uninstall
- [ ] Filtering/sorting/pagination flags
- [ ] Command renames: `agents` -> `agent list`, `spawn` -> `agent create`

---

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | JSON output |
| `--plain` | Plain text (tab-separated, no colors) |
| `--header` | Include headers in plain output |
| `-v, --verbose` | Verbose output |
| `-q, --quiet` | Suppress non-essential output |
| `--no-color` | Disable colors |
| `-h, --help` | Help |

**Environment Variables:**

| Variable | Description |
|----------|-------------|
| `NO_COLOR` | Disable colors globally |
| `VIBESPACE_DEBUG=1` | Enable debug logging |
| `VIBESPACE_LOG_LEVEL` | Log level: debug, info, warn, error |

---

## Command Reference

### Root Commands

#### `vibespace version`

```bash
vibespace version           # vibespace dev (unknown)
vibespace version --json    # {"success":true,"data":{"version":"dev","commit":"unknown"},"meta":{...}}
```

#### `vibespace init`

Initialize the cluster.

```bash
vibespace init
vibespace init --cpu 4 --memory 8 --disk 60
vibespace init --external --kubeconfig ~/.kube/config
```

| Flag | Default | Description |
|------|---------|-------------|
| `--external` | false | Use external Kubernetes |
| `--kubeconfig` | | Path to kubeconfig |
| `--cpu` | 4 | CPU cores |
| `--memory` | 8 | Memory (GB) |
| `--disk` | 60 | Disk size (GB) |

#### `vibespace status`

```bash
vibespace status            # Formatted list output
vibespace status --json     # {"success":true,"data":{"cluster":{...},"daemon":{...}},"meta":{...}}
```

#### `vibespace stop`

Stop the cluster.

```bash
vibespace stop
```

#### `vibespace create <name>`

```bash
vibespace create myproject -t claude-code
vibespace create myproject -t codex --repo https://github.com/user/repo
vibespace create myproject -t claude-code --skip-permissions --model opus
```

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --agent-type` | **required** | `claude-code` or `codex` |
| `-n, --name` | auto | Custom agent name |
| `--repo` | | GitHub repo to clone |
| `--cpu` | 1000m | CPU request |
| `--memory` | 1Gi | Memory request |
| `--storage` | 10Gi | Storage size |
| `--skip-permissions` | false | Skip permissions (Claude only) |
| `--allowed-tools` | | Allowed tools (Claude only) |
| `--disallowed-tools` | | Disallowed tools (Claude only) |
| `--model` | | Model to use |
| `--max-turns` | | Max conversation turns |

#### `vibespace list`

```bash
vibespace list              # Table output
vibespace list --json       # {"success":true,"data":{"vibespaces":[...],"count":1},"meta":{...}}
vibespace list --plain      # name<TAB>status<TAB>agents<TAB>...
```

#### `vibespace delete <name>`

```bash
vibespace delete myproject              # Prompts for confirmation
vibespace delete myproject --force      # No prompt
vibespace delete myproject --dry-run    # Show what would be deleted
```

| Flag | Description |
|------|-------------|
| `-f, --force` | Skip confirmation (required for non-TTY) |
| `--keep-data` | Preserve persistent storage |
| `-n, --dry-run` | Show what would be deleted |

---

### Session Commands

#### `vibespace session list`

```bash
vibespace session list
vibespace session list --json
```

#### `vibespace session show <name>`

```bash
vibespace session show abc123
vibespace session show abc123 --json
```

#### `vibespace session delete <name>`

```bash
vibespace session delete mywork
```

---

### Multi-Agent Commands

#### `vibespace multi`

Interactive TUI or headless mode.

**Interactive (TUI):**
```bash
vibespace multi --vibespaces test
vibespace multi --resume                 # Select from previous sessions
vibespace multi -r abc123                # Resume specific session
```

**Non-Interactive:**
```bash
# JSON output
vibespace multi --vibespaces test --json "what is 2+2?"

# Plain text
vibespace multi --vibespaces test --plain "what is 2+2?"

# Streaming
vibespace multi --vibespaces test --plain --stream "count to 5"

# Target specific agent
vibespace multi --vibespaces test --agent claude-1@test --json "hello"

# List agents
vibespace multi --vibespaces test --list-agents --json

# List sessions
vibespace multi --list-sessions --json

# Batch mode (JSONL from stdin)
echo '{"target":"claude-1@test","message":"hello"}' | vibespace multi --vibespaces test --batch
```

| Flag | Description |
|------|-------------|
| `--vibespaces` | Comma-separated vibespaces |
| `--agents` | Specific agents (agent@vibespace) |
| `--name` | Session name |
| `--agent` | Target agent |
| `--batch` | JSONL batch mode |
| `--list-agents` | List agents and exit |
| `--list-sessions` | List sessions and exit |
| `--stream` | Stream responses |
| `--timeout` | Response timeout |
| `-r, --resume` | Resume session |

---

### Vibespace-Scoped Commands

#### `vibespace <vs> agents`

```bash
vibespace myproject agents
vibespace myproject agents --json
vibespace myproject agents --plain
```

#### `vibespace <vs> spawn`

Add an agent.

```bash
vibespace myproject spawn
vibespace myproject spawn --name researcher
vibespace myproject spawn --agent-type codex
vibespace myproject spawn --skip-permissions --model opus
```

| Flag | Description |
|------|-------------|
| `-n, --name` | Custom name |
| `-t, --agent-type` | Agent type (inherits if not specified) |
| `--skip-permissions` | Skip permissions (Claude only) |
| `--model` | Model to use |
| `--max-turns` | Max turns |

#### `vibespace <vs> kill <agent>`

Remove an agent.

```bash
vibespace myproject kill claude-2
```

#### `vibespace <vs> up [agent]` / `vibespace <vs> start`

Scale up agents.

```bash
vibespace myproject up            # All agents
vibespace myproject up claude-2   # Specific agent
```

#### `vibespace <vs> down [agent]` / `vibespace <vs> stop`

Scale down agents.

```bash
vibespace myproject down
vibespace myproject down claude-1
```

#### `vibespace <vs> connect [agent]`

SSH to an agent. **Requires TTY.**

```bash
vibespace myproject connect
vibespace myproject connect claude-2
vibespace myproject connect --browser   # Opens ttyd in browser
```

#### `vibespace <vs> config show [agent]`

```bash
vibespace myproject config show
vibespace myproject config show claude-1 --json
```

#### `vibespace <vs> config set <agent>`

```bash
# Claude Code agents
vibespace myproject config set claude-1 --skip-permissions
vibespace myproject config set claude-1 --model opus
vibespace myproject config set claude-1 --allowed-tools "Bash,Read,Write"

# Codex agents
vibespace myproject config set codex-1 --model gpt-5.2-codex
vibespace myproject config set codex-1 --reasoning-effort high
```

| Flag | Description |
|------|-------------|
| `--skip-permissions` | Enable (Claude only) |
| `--no-skip-permissions` | Disable (Claude only) |
| `--allowed-tools` | Allowed tools (Claude only) |
| `--disallowed-tools` | Disallowed tools (Claude only) |
| `--model` | Model |
| `--max-turns` | Max turns |
| `--reasoning-effort` | low, medium, high, xhigh (Codex only) |

#### `vibespace <vs> forward list`

```bash
vibespace myproject forward list
vibespace myproject forward list --json
vibespace myproject forward list --plain
```

#### `vibespace <vs> forward add <port>`

```bash
vibespace myproject forward add 3000
vibespace myproject forward add 8080 --agent claude-2 --local 9000
```

#### `vibespace <vs> forward remove <port>`

```bash
vibespace myproject forward remove 3000
vibespace myproject forward remove 8080 --agent claude-2
```

#### `vibespace <vs> ports`

List detected ports in the agent.

```bash
vibespace myproject ports
```

---

## Agent Types

### Claude Code (`claude-code`)

| Model | Description |
|-------|-------------|
| `sonnet` | Sonnet 4.5 for daily coding (default) |
| `opus` | Opus 4.5 for complex reasoning |
| `haiku` | Fast for simple tasks |
| `opusplan` | Opus for planning, Sonnet for execution |

Config: `--skip-permissions`, `--allowed-tools`, `--disallowed-tools`, `--model`, `--max-turns`, `--system-prompt`

### Codex (`codex`)

| Model | Description |
|-------|-------------|
| `gpt-5.2-codex` | Most advanced (recommended) |
| `gpt-5.1-codex-mini` | Smaller, cost-effective |
| `gpt-5.1-codex-max` | Long-horizon tasks |
| `gpt-5.2` | General agentic model |

Config: `--model`, `--max-turns`, `--reasoning-effort`

**Note:** Codex always runs in yolo mode - `--skip-permissions` and tool restrictions don't apply.

---

## Shell Completion

```bash
# zsh
source <(vibespace completion zsh)

# bash
source <(vibespace completion bash)

# fish
vibespace completion fish | source
```

---

## Scripting Examples

```bash
# Check cluster status
vibespace status --json | jq -e '.data.cluster.running'

# List vibespace names
vibespace list --json | jq -r '.data.vibespaces[].name'

# Count agents
vibespace myproject agents --json | jq '.data.count'

# Send message to agents
vibespace multi --vibespaces myproject --json "list files"

# Delete without prompt
vibespace delete myproject --force

# Wait for agent ready
while ! vibespace myproject agents --json | jq -e '.data.agents[] | select(.status == "running")' >/dev/null 2>&1; do
  sleep 2
done
```

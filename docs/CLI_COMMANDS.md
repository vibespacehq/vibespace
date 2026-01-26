# vibespace CLI Commands Reference

Complete reference for all CLI commands, their output modes, and non-TTY compatibility.

**Verified on:** 2026-01-26

---

## Quick Reference Table

| Command | Non-TTY | JSON | Plain | Notes |
|---------|:-------:|:----:|:-----:|-------|
| `version` | ✅ | ✅ | ❌ | |
| `init` | ✅ | ❌ | ❌ | Spinner degrades to single line |
| `status` | ✅ | ✅ | ❌ | |
| `stop` | ✅ | ❌ | ❌ | Spinner degrades to single line |
| `uninstall` | ❌ | ❌ | ❌ | Requires interactive confirmation |
| `create` | ✅ | ❌ | ❌ | Spinner degrades to single line |
| `list` | ✅ | ✅ | ✅ | |
| `delete` | ✅ | ✅ | ❌ | Requires `--force` in non-TTY |
| `session list` | ✅ | ✅ | ❌ | |
| `session show` | ✅ | ✅ | ❌ | |
| `session delete` | ✅ | ❌ | ❌ | |
| `multi` | ✅ | ✅ | ✅ | Full headless support |
| `<vs> agents` | ✅ | ✅ | ✅ | |
| `<vs> spawn` | ✅ | ❌ | ❌ | Prints progress messages |
| `<vs> kill` | ✅ | ❌ | ❌ | |
| `<vs> up` | ✅ | ❌ | ❌ | Prints progress messages |
| `<vs> down` | ✅ | ❌ | ❌ | Prints progress messages |
| `<vs> connect` | ❌ | ❌ | ❌ | Requires interactive SSH |
| `<vs> config show` | ✅ | ✅ | ❌ | |
| `<vs> config set` | ✅ | ❌ | ❌ | JSON flag ignored |
| `<vs> forward list` | ✅ | ✅ | ✅ | |
| `<vs> forward add` | ✅ | ❌ | ❌ | Prints success message |
| `<vs> forward remove` | ✅ | ❌ | ❌ | |
| `<vs> ports` | ✅ | ❌ | ❌ | |
| `<vs> multi` | ❌ | ❌ | ❌ | Use top-level `multi` instead |

**Legend:**
- ✅ Supported
- ❌ Not supported
- `<vs>` = vibespace name (e.g., `vibespace myproject agents`)

---

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |
| `--plain` | Plain output for scripting (tab-separated, no colors) |
| `-v, --verbose` | Enable verbose output |
| `-q, --quiet` | Suppress non-essential output |
| `--no-color` | Disable colored output |
| `-h, --help` | Show help information |

**Environment Variables:**
| Variable | Description |
|----------|-------------|
| `VIBESPACE_DEBUG=1` | Enable debug logging |
| `VIBESPACE_LOG_LEVEL` | Log level: debug, info, warn, error |
| `NO_COLOR` | Disable colors globally |

---

## Root Commands

### `vibespace version`

```bash
./vibespace version
# vibespace dev (unknown)

./vibespace version --json
# {"success":true,"data":{"version":"dev","commit":"unknown"}}
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ❌

---

### `vibespace init`

Initialize the cluster.

```bash
./vibespace init
./vibespace init --cpu 4 --memory 8 --disk 60
./vibespace init --external --kubeconfig ~/.kube/config
```

| Flag | Default | Env Var | Description |
|------|---------|---------|-------------|
| `--external` | false | | Use external Kubernetes cluster |
| `--kubeconfig` | | | Path to external kubeconfig |
| `--cpu` | 4 | `VIBESPACE_CLUSTER_CPU` | CPU cores |
| `--memory` | 8 | `VIBESPACE_CLUSTER_MEMORY` | Memory in GB |
| `--disk` | 60 | `VIBESPACE_CLUSTER_DISK` | Disk size in GB |

**Non-TTY:** ✅ (spinner degrades to `-> message`) | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace status`

```bash
./vibespace status
# Cluster: running
# Components:
#   Namespace: ready
# Daemon: running (uptime: 2h35m, pid: 87608)

./vibespace status --json
# {"success":true,"data":{"cluster":{"installed":true,"running":true,...}}}
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ❌

---

### `vibespace stop`

Stop the cluster.

```bash
./vibespace stop
```

**Non-TTY:** ✅ (spinner degrades) | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace uninstall`

Remove vibespace completely. **Requires interactive confirmation.**

```bash
./vibespace uninstall
# This will remove ALL vibespace data...
# Continue? [y/N]
```

**Non-TTY:** ❌ (no `--force` option) | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace create [name]`

```bash
./vibespace create myproject -t claude-code
./vibespace create myproject -t codex --repo https://github.com/user/repo
./vibespace create myproject --agent-type codex --skip-permissions
```

| Flag | Default | Description |
|------|---------|-------------|
| `--repo` | | GitHub repository to clone |
| `--cpu` | 1000m | CPU request/limit |
| `--memory` | 1Gi | Memory request/limit |
| `--storage` | 10Gi | Storage size |
| `-s, --share-credentials` | false | Share credentials across agents |
| `-t, --agent-type` | **required** | Agent type: claude-code, codex |
| `--skip-permissions` | false | Enable --dangerously-skip-permissions |
| `--allowed-tools` | | Comma-separated allowed tools |
| `--disallowed-tools` | | Comma-separated disallowed tools |
| `--model` | | Claude model |
| `--max-turns` | | Maximum conversation turns |

**Non-TTY:** ✅ (spinner degrades) | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace list`

```bash
./vibespace list
# NAME             STATUS     AGENTS   CPU      MEMORY   STORAGE  CREATED
# test-mixed       running    2        1        1Gi      10Gi     2026-01-25T21:10:30Z

./vibespace list --json
# {"success":true,"data":{"vibespaces":[...],"count":1}}

./vibespace list --plain
# test-mixed	running	2	1	1Gi	10Gi	2026-01-25T21:10:30Z
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ✅ (tab-separated)

---

### `vibespace delete <name>`

```bash
./vibespace delete myproject              # Prompts for confirmation
./vibespace delete myproject --force      # No prompt
./vibespace delete myproject --dry-run    # Show what would be deleted
./vibespace delete myproject --keep-data  # Preserve PVC

./vibespace delete myproject --dry-run --json
# {"success":true,"data":{"name":"myproject","keep_data":false,"dry_run":true,"resources":[...]}}
```

| Flag | Description |
|------|-------------|
| `-f, --force` | Skip confirmation (required for non-TTY) |
| `--keep-data` | Preserve persistent storage |
| `-n, --dry-run` | Show what would be deleted |

**Non-TTY:** ✅ (requires `--force`) | **JSON:** ✅ | **Plain:** ❌

**Non-TTY without --force:**
```
error cannot prompt for confirmation (stdin is not a terminal). Use --force to skip confirmation
```

---

## Session Commands

### `vibespace session list`

```bash
./vibespace session list
# ╭──────────┬────────────┬────────┬────────────────╮
# │ NAME     │ VIBESPACES │ AGENTS │ LAST USED      │
# ...

./vibespace session list --json
# {"success":true,"data":{"sessions":[...],"count":17}}
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ❌

---

### `vibespace session show <name>`

```bash
./vibespace session show 2dab7d1e
# Session: 2dab7d1e
# Created: 2026-01-26 00:01
# Last used: 12 minutes ago
# ...

./vibespace session show 2dab7d1e --json
# {"success":true,"data":{"name":"2dab7d1e","created_at":"...","vibespaces":[...]}}
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ❌

---

### `vibespace session delete <name>`

```bash
./vibespace session delete mywork
# ok Deleted session 'mywork'
```

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

## Multi-Agent Commands

### `vibespace multi`

Full headless support via `HeadlessRunner`. Auto-detects non-TTY.

#### Interactive Mode (TUI)

```bash
./vibespace multi --vibespaces test
./vibespace multi --resume
```

#### Non-Interactive Mode

```bash
# JSON output (default in non-TTY)
./vibespace multi --vibespaces test --json "what is 2+2?"
# {"session":"...","request":{...},"responses":[{"agent":"claude-1@test","content":"4"}]}

# Plain text output
./vibespace multi --vibespaces test --plain "what is 2+2?"
# [claude-1@test]
# 4

# Streaming (real-time)
./vibespace multi --vibespaces test --plain --stream "count to 5"
# [claude-1@test] 1
# 2
# 3
# ...

# Target specific agent
./vibespace multi --vibespaces test --agent claude-1@test --json "hello"

# List agents only
./vibespace multi --vibespaces test --list-agents --json
# {"session":"test","agents":["claude-1@test","codex-1@test"]}

# Batch mode (JSONL from stdin)
echo '{"target":"claude-1@test","message":"hello"}' | ./vibespace multi --vibespaces test --batch
```

| Flag | Default | Description |
|------|---------|-------------|
| `--vibespaces` | | Comma-separated vibespaces |
| `--agents` | | Specific agents (agent@vibespace) |
| `--name` | auto-UUID | Session name |
| `--agent` | all | Target agent |
| `--batch` | false | JSONL batch mode from stdin |
| `--list-agents` | false | List agents and exit |
| `--stream` | false | Stream responses (plain mode) |
| `--timeout` | 2m | Response timeout |
| `-r, --resume` | false | Resume existing session |

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ✅ | **Streaming:** ✅ | **Batch:** ✅

---

## Vibespace-Scoped Commands

### `vibespace <name> agents`

```bash
./vibespace test-mixed agents
# AGENT        TYPE         VIBESPACE            STATUS
# claude-1     claude-code  test-mixed           running
# codex-1      codex        test-mixed           running

./vibespace test-mixed agents --json
# {"success":true,"data":{"vibespace":"test-mixed","agents":[{"name":"claude-1","type":"claude-code",...}],"count":2}}

./vibespace test-mixed agents --plain
# claude-1	claude-code	test-mixed	running
# codex-1	codex	test-mixed	running
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ✅ (tab-separated)

---

### `vibespace <name> spawn`

```bash
./vibespace myproject spawn                    # Inherits type from primary agent
./vibespace myproject spawn --name researcher
./vibespace myproject spawn --agent-type codex # Explicit type
./vibespace myproject spawn --skip-permissions
```

| Flag | Default | Description |
|------|---------|-------------|
| `-n, --name` | auto | Custom agent name |
| `-t, --agent-type` | inherit | Agent type (inherits from primary if not specified) |
| `-s, --share-credentials` | false | Share credentials |
| `--skip-permissions` | false | Skip permissions |
| `--allowed-tools` | | Allowed tools |
| `--disallowed-tools` | | Disallowed tools |
| `--model` | | Model to use |
| `--max-turns` | | Max turns |

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> kill <agent>`

```bash
./vibespace myproject kill claude-2
# ok Agent 'claude-2' removed
```

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> up [agent]` / `start`

```bash
./vibespace myproject up           # All agents
./vibespace myproject up claude-2  # Specific agent
```

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> down [agent]` / `stop`

```bash
./vibespace myproject down           # All agents
./vibespace myproject down claude-1  # Specific agent
```

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> connect [agent]`

Interactive SSH connection. **Requires TTY.**

```bash
./vibespace myproject connect
./vibespace myproject connect claude-2
./vibespace myproject connect --browser  # Opens ttyd in browser
```

**Non-TTY:** ❌ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> config show [agent]`

```bash
./vibespace test-mixed config show
#   ⬡ claude-1
#     ◉ skip_permissions     enabled
#     ...

./vibespace test-mixed config show claude-1 --json
# {"success":true,"data":{"agent":"claude-1","config":{"skip_permissions":true},...}}
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ❌

---

### `vibespace <name> config set <agent>`

```bash
./vibespace myproject config set claude-1 --skip-permissions
./vibespace myproject config set claude-1 --no-skip-permissions
./vibespace myproject config set claude-1 --model opus
./vibespace myproject config set claude-1 --max-turns 50
./vibespace myproject config set claude-1 --allowed-tools "Bash,Read,Write"
```

| Flag | Description |
|------|-------------|
| `--skip-permissions` | Enable skip permissions |
| `--no-skip-permissions` | Disable skip permissions |
| `--allowed-tools` | Comma-separated allowed tools |
| `--disallowed-tools` | Comma-separated disallowed tools |
| `--model` | Claude model |
| `--max-turns` | Max turns (0 = unlimited) |
| `--system-prompt` | Custom system prompt |

**Non-TTY:** ✅ | **JSON:** ❌ (flag ignored) | **Plain:** ❌

---

### `vibespace <name> forward list`

```bash
./vibespace test-mixed forward list
# AGENT     LOCAL  REMOTE  TYPE  STATUS
# claude-1  61119  22      ssh   active
# ...

./vibespace test-mixed forward list --json
# {"success":true,"data":{"vibespace":"test-mixed","agents":[...]}}

./vibespace test-mixed forward list --plain
# claude-1	61119	22	ssh	active
```

**Non-TTY:** ✅ | **JSON:** ✅ | **Plain:** ✅ (tab-separated)

---

### `vibespace <name> forward add <port>`

```bash
./vibespace myproject forward add 3000
./vibespace myproject forward add 8080 --agent claude-2 --local 9000
# ok Forward added: localhost:9000 → 8080
```

| Flag | Default | Description |
|------|---------|-------------|
| `-a, --agent` | claude-1 | Agent to forward from |
| `-l, --local` | auto | Local port |

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> forward remove <port>`

```bash
./vibespace myproject forward remove 3000
./vibespace myproject forward remove 8080 --agent claude-2
# ok Forward removed: port 8080
```

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> ports`

```bash
./vibespace myproject ports
# PORT  PROCESS  DETECTED
# 3000  node     2m ago
```

**Non-TTY:** ✅ | **JSON:** ❌ | **Plain:** ❌

---

### `vibespace <name> multi`

Vibespace-scoped multi (TUI only). **Use top-level `multi` for non-interactive.**

```bash
./vibespace myproject multi  # TTY only
# error TUI requires an interactive terminal (stdin is not a TTY); use --json for non-interactive mode
```

**Non-TTY:** ❌ | **JSON:** ❌ | **Plain:** ❌

**Alternative for non-TTY:**
```bash
./vibespace multi --vibespaces myproject --json "message"
```

---

## Scripting Examples

```bash
# Check cluster status
./vibespace status --json | jq -e '.data.cluster.running'

# List vibespace names
./vibespace list --json | jq -r '.data.vibespaces[].name'

# Count agents
./vibespace myproject agents --json | jq '.data.count'

# Send message to agents
./vibespace multi --vibespaces myproject --json "list files"

# Batch process
cat <<EOF | ./vibespace multi --vibespaces test --batch
{"target":"claude-1@test","message":"task 1"}
{"target":"claude-2@test","message":"task 2"}
EOF

# Delete without prompt
./vibespace delete myproject --force

# Wait for agent ready
while ! ./vibespace myproject agents --json | jq -e '.data.agents[] | select(.status == "running")' >/dev/null 2>&1; do
  sleep 2
done
```

---

## Error Handling in JSON Mode

```bash
./vibespace nonexistent agents --json
# {"success":false,"error":{"message":"failed to list agents: vibespace not found..."}}

./vibespace multi --vibespaces nonexistent --json --list-agents
# {"session":"","request":{"target":"","message":""},"responses":null,"error":"vibespace 'nonexistent' not found..."}
```

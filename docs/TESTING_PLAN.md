# Agent Abstraction Testing Plan

**Purpose:** Verify feature parity after the agent abstraction refactor.
**Date:** 2026-01-26
**Branch:** `vibespace-wip-agent-abstraction`

---

## Constraints

- Limited system resources - only 1-2 vibespaces active at a time
- Clean up (delete) after each test group
- Tests are sequential - later tests may depend on earlier ones passing

---

## Part 1: Non-TUI Commands

### 1.1 Vibespace Creation - Claude Code (Default)

```bash
vibespace create test-claude
```

**Expected:**
- Spinner shows "Creating vibespace 'test-claude'..."
- Success message: "Vibespace 'test-claude' created"
- `vibespace list` shows test-claude with status=running, agents=1
- `vibespace test-claude agents` shows `claude-1` with type `claude-code`

**Cleanup:** Keep for next tests

---

### 1.2 Agent Listing Formats

```bash
vibespace test-claude agents
vibespace test-claude agents --json
vibespace test-claude agents --plain
```

**Expected:**
- Default: Table with columns AGENT, TYPE, VIBESPACE, STATUS
- JSON: `{"success":true,"data":{"agents":[{"name":"claude-1","type":"claude-code",...}]}}`
- Plain: Tab-separated `claude-1\tclaude-code\ttest-claude\trunning`

**Cleanup:** None

---

### 1.3 Spawn Codex into Claude Vibespace (Mixed)

```bash
vibespace test-claude spawn --agent-type codex
```

**Expected:**
- Success message mentions new agent name (e.g., "codex-1")
- `vibespace test-claude agents` shows 2 agents: `claude-1` (claude-code) and `codex-1` (codex)

**Cleanup:** None

---

### 1.4 Config Show

```bash
vibespace test-claude config show
vibespace test-claude config show claude-1 --json
vibespace test-claude config show codex-1 --json
```

**Expected:**
- Default: Lists all agents with their config (skip_permissions, model, etc.)
- JSON claude-1: `{"success":true,"data":{"agent":"claude-1","type":"claude-code","config":{...}}}`
- JSON codex-1: `{"success":true,"data":{"agent":"codex-1","type":"codex","config":{...}}}`

**Cleanup:** None

---

### 1.5 Config Set

```bash
vibespace test-claude config set claude-1 --skip-permissions
vibespace test-claude config set codex-1 --skip-permissions
vibespace test-claude config show --json
```

**Expected:**
- Each config set prints success message
- Config show confirms `skip_permissions: true` for both agents
- Pod restarts to apply new config

**Cleanup:** None

---

### 1.6 Agent Lifecycle (down/up)

```bash
vibespace test-claude down codex-1
vibespace test-claude agents
vibespace test-claude up codex-1
vibespace test-claude agents
```

**Expected:**
- After down: codex-1 status changes to "stopped" or disappears from running pods
- After up: codex-1 status returns to "running"

**Cleanup:** None

---

### 1.7 Agent Kill

```bash
vibespace test-claude kill codex-1
vibespace test-claude agents
```

**Expected:**
- Success message: "Agent 'codex-1' removed"
- `agents` shows only claude-1 remaining

**Cleanup:** Delete vibespace

```bash
vibespace delete test-claude --force
```

---

### 1.8 Share Credentials - Same Agent Type Both Sharing

```bash
vibespace create test-shared --share-credentials
vibespace test-shared spawn --share-credentials
vibespace test-shared config show --json
```

**Expected:**
- claude-1: `share_credentials: true`
- claude-2: `share_credentials: true`
- Both agents have `/vibespace/.vibespace/.claude/` with same credentials

**Verification:**
```bash
# Connect to claude-1, check credentials path
vibespace test-shared connect claude-1
# Inside: ls -la /vibespace/.vibespace/.claude/
# Note the content of credentials

# Connect to claude-2, verify same credentials
vibespace test-shared connect claude-2
# Inside: ls -la /vibespace/.vibespace/.claude/
# Should be same as claude-1
```

**Cleanup:** Keep for next test

---

### 1.8b Share Credentials - One With, One Without

```bash
vibespace test-shared spawn
# This spawns claude-3 WITHOUT --share-credentials

vibespace test-shared config show --json
```

**Expected:**
- claude-1: `share_credentials: true`
- claude-2: `share_credentials: true`
- claude-3: `share_credentials: false`

**Verification:**
```bash
# Connect to claude-3
vibespace test-shared connect claude-3
# Inside: should have isolated credentials in ~/.claude/, NOT /vibespace/.vibespace/
```

**Cleanup:** Delete

```bash
vibespace delete test-shared --force
```

---

### 1.8c Share Credentials - Mixed Vibespace (Claude + Codex)

```bash
vibespace create test-mixed-creds --share-credentials
vibespace test-mixed-creds spawn --agent-type codex --share-credentials
vibespace test-mixed-creds config show --json
```

**Expected:**
- claude-1: `share_credentials: true`, type: `claude-code`
- codex-1: `share_credentials: true`, type: `codex`
- Each agent type has its own credential directory:
  - Claude: `/vibespace/.vibespace/.claude/`
  - Codex: `/vibespace/.vibespace/.codex/` (or equivalent)

**Verification:**
```bash
# Check claude-1 credentials
vibespace test-mixed-creds connect claude-1
# Inside: ls -la /vibespace/.vibespace/
# Should see .claude/ directory

# Check codex-1 credentials
vibespace test-mixed-creds connect codex-1
# Inside: ls -la /vibespace/.vibespace/
# Should see .codex/ directory
# Claude and Codex credentials are separate
```

**Cleanup:** Keep for next test

---

### 1.8d Share Credentials - Mixed Vibespace Partial Sharing

```bash
# Spawn another claude WITH share-credentials
vibespace test-mixed-creds spawn --agent-type claude-code --share-credentials

# Spawn another codex WITHOUT share-credentials
vibespace test-mixed-creds spawn --agent-type codex

vibespace test-mixed-creds config show --json
```

**Expected:**
- claude-1: `share_credentials: true`
- codex-1: `share_credentials: true`
- claude-2: `share_credentials: true` (shares with claude-1)
- codex-2: `share_credentials: false` (isolated from codex-1)

**Verification:**
```bash
# claude-2 shares credentials with claude-1
vibespace test-mixed-creds connect claude-2
# Inside: /vibespace/.vibespace/.claude/ should match claude-1

# codex-2 has isolated credentials
vibespace test-mixed-creds connect codex-2
# Inside: credentials in ~/.codex/, NOT /vibespace/.vibespace/
```

**Cleanup:** Delete

```bash
vibespace delete test-mixed-creds --force
```

---

### 1.8e Share Credentials - Enable via Config Set

```bash
vibespace create test-toggle
vibespace test-toggle config show claude-1 --json
# Expected: share_credentials: false (default)

vibespace test-toggle config set claude-1 --share-credentials
vibespace test-toggle config show claude-1 --json
# Expected: share_credentials: true
# Pod should restart

# Spawn another agent with share
vibespace test-toggle spawn --share-credentials
vibespace test-toggle config show --json
# Expected: both claude-1 and claude-2 have share_credentials: true
```

**Verification:**
```bash
# Both agents should now share credentials
vibespace test-toggle connect claude-1
# Check /vibespace/.vibespace/.claude/

vibespace test-toggle connect claude-2
# Should have same credentials as claude-1
```

**Cleanup:** Delete

```bash
vibespace delete test-toggle --force
```

---

### 1.9 Allowed Tools - Create

```bash
vibespace create test-tools --allowed-tools "Bash,Read,Write"
vibespace test-tools config show claude-1 --json
```

**Expected:**
- Config shows `allowed_tools: ["Bash","Read","Write"]`

**Cleanup:** Keep for next test

---

### 1.9b Allowed Tools - Spawn with Different Tools

```bash
vibespace test-tools spawn --agent-type codex --allowed-tools "Read,Grep"
vibespace test-tools config show codex-1 --json
```

**Expected:**
- codex-1 config shows `allowed_tools: ["Read","Grep"]`
- Different from claude-1's allowed tools (per-agent config)

**Cleanup:** Keep for next test

---

### 1.9c Allowed Tools - Config Set

```bash
vibespace test-tools config set claude-1 --allowed-tools "Bash,Read,Write,Edit,Glob,Grep"
vibespace test-tools config show claude-1 --json
```

**Expected:**
- Config updated to new allowed tools list
- Pod restarts to apply

**Cleanup:** Delete

```bash
vibespace delete test-tools --force
```

---

### 1.10 Disallowed Tools - Create

```bash
vibespace create test-disallow --disallowed-tools "Bash,Write"
vibespace test-disallow config show claude-1 --json
```

**Expected:**
- Config shows `disallowed_tools: ["Bash","Write"]`

**Cleanup:** Keep for next test

---

### 1.10b Disallowed Tools - Config Set

```bash
vibespace test-disallow config set claude-1 --disallowed-tools "Bash"
vibespace test-disallow config show claude-1 --json
```

**Expected:**
- Config shows `disallowed_tools: ["Bash"]` (replaces previous list)

**Cleanup:** Delete

```bash
vibespace delete test-disallow --force
```

---

### 1.11 Model - Create

```bash
vibespace create test-model --model opus
vibespace test-model config show claude-1 --json
```

**Expected:**
- Config shows `model: "opus"`

**Cleanup:** Keep for next test

---

### 1.11b Model - Spawn with Different Model

```bash
vibespace test-model spawn --model sonnet
vibespace test-model config show --json
```

**Expected:**
- claude-1: `model: "opus"`
- claude-2: `model: "sonnet"`

**Cleanup:** Keep for next test

---

### 1.11c Model - Config Set

```bash
vibespace test-model config set claude-1 --model haiku
vibespace test-model config show claude-1 --json
```

**Expected:**
- Config shows `model: "haiku"`
- Pod restarts to apply

**Cleanup:** Delete

```bash
vibespace delete test-model --force
```

---

### 1.12 Max Turns - Create

```bash
vibespace create test-turns --max-turns 10
vibespace test-turns config show claude-1 --json
```

**Expected:**
- Config shows `max_turns: 10`

**Cleanup:** Keep for next test

---

### 1.12b Max Turns - Spawn with Different Value

```bash
vibespace test-turns spawn --max-turns 50
vibespace test-turns config show --json
```

**Expected:**
- claude-1: `max_turns: 10`
- claude-2: `max_turns: 50`

**Cleanup:** Keep for next test

---

### 1.12c Max Turns - Config Set (Unlimited)

```bash
vibespace test-turns config set claude-1 --max-turns 0
vibespace test-turns config show claude-1 --json
```

**Expected:**
- Config shows `max_turns: 0` (unlimited)

**Cleanup:** Delete

```bash
vibespace delete test-turns --force
```

---

### 1.13 System Prompt - Create

```bash
vibespace create test-prompt --system-prompt "You are a helpful coding assistant."
vibespace test-prompt config show claude-1 --json
```

**Expected:**
- Config shows `system_prompt: "You are a helpful coding assistant."`

**Cleanup:** Keep for next test

---

### 1.13b System Prompt - Config Set

```bash
vibespace test-prompt config set claude-1 --system-prompt "Be concise."
vibespace test-prompt config show claude-1 --json
```

**Expected:**
- Config shows `system_prompt: "Be concise."`

**Cleanup:** Delete

```bash
vibespace delete test-prompt --force
```

---

### 1.14 Skip Permissions - Create

```bash
vibespace create test-skip --skip-permissions
vibespace test-skip config show claude-1 --json
```

**Expected:**
- Config shows `skip_permissions: true`

**Cleanup:** Keep for next test

---

### 1.14b Skip Permissions - Toggle via Config Set

```bash
vibespace test-skip config set claude-1 --no-skip-permissions
vibespace test-skip config show claude-1 --json
```

**Expected:**
- Config shows `skip_permissions: false`

```bash
vibespace test-skip config set claude-1 --skip-permissions
vibespace test-skip config show claude-1 --json
```

**Expected:**
- Config shows `skip_permissions: true`

**Cleanup:** Delete

```bash
vibespace delete test-skip --force
```

---

### 1.15 Combined Flags - Create

```bash
vibespace create test-combo \
  --agent-type claude-code \
  --share-credentials \
  --skip-permissions \
  --model opus \
  --max-turns 100 \
  --allowed-tools "Bash,Read,Write,Edit"

vibespace test-combo config show claude-1 --json
```

**Expected:**
```json
{
  "success": true,
  "data": {
    "agent": "claude-1",
    "type": "claude-code",
    "config": {
      "share_credentials": true,
      "skip_permissions": true,
      "model": "opus",
      "max_turns": 100,
      "allowed_tools": ["Bash", "Read", "Write", "Edit"]
    }
  }
}
```

**Cleanup:** Keep for next test

---

### 1.15b Combined Flags - Spawn Different Agent Type

```bash
vibespace test-combo spawn \
  --agent-type codex \
  --skip-permissions \
  --model gpt-4o

vibespace test-combo config show codex-1 --json
```

**Expected:**
```json
{
  "success": true,
  "data": {
    "agent": "codex-1",
    "type": "codex",
    "config": {
      "skip_permissions": true,
      "model": "gpt-4o"
    }
  }
}
```

**Cleanup:** Delete

```bash
vibespace delete test-combo --force
```

---

### 1.16 Credential Persistence - Claude Code After Config Change

```bash
vibespace create test-persist
vibespace test-persist connect claude-1
# Inside: verify claude is authenticated (run a simple prompt)
# Exit
```

```bash
# Change allowed-tools to trigger pod restart
vibespace test-persist config set claude-1 --allowed-tools "Bash,Read,Write,Edit"
# Wait for pod to restart

vibespace test-persist connect claude-1
# Inside: verify claude is STILL authenticated (not logged out)
```

**Expected:**
- Claude credentials persist after pod restart
- No re-login required
- Agent responds to prompts without authentication errors

**Cleanup:** Keep for next test

---

### 1.16b Credential Persistence - Multiple Config Changes

```bash
vibespace test-persist config set claude-1 --skip-permissions
# Wait for restart

vibespace test-persist config set claude-1 --model opus
# Wait for restart

vibespace test-persist config set claude-1 --max-turns 50
# Wait for restart

vibespace test-persist connect claude-1
# Verify still authenticated
```

**Expected:**
- Credentials survive multiple consecutive pod restarts
- Each config change triggers restart but credentials persist

**Cleanup:** Delete

```bash
vibespace delete test-persist --force
```

---

### 1.16c Credential Persistence - With Share Credentials

```bash
vibespace create test-persist-shared --share-credentials
vibespace test-persist-shared connect claude-1
# Inside: verify authenticated
# Check: ls -la /vibespace/.vibespace/.claude/
# Exit
```

```bash
vibespace test-persist-shared config set claude-1 --allowed-tools "Bash,Read"
# Wait for restart

vibespace test-persist-shared connect claude-1
# Verify still authenticated
# Verify credentials still in /vibespace/.vibespace/.claude/
```

**Expected:**
- Shared credentials persist in PVC-backed `/vibespace/.vibespace/.claude/`
- Pod restart doesn't lose credentials

**Cleanup:** Delete

```bash
vibespace delete test-persist-shared --force
```

---

### 1.17 Credential Persistence - Codex After Config Change

```bash
vibespace create test-persist-codex --agent-type codex
vibespace test-persist-codex connect codex-1
# Inside: verify codex is authenticated
# Exit
```

```bash
vibespace test-persist-codex config set codex-1 --allowed-tools "Read,Write"
# Wait for restart

vibespace test-persist-codex connect codex-1
# Verify still authenticated
```

**Expected:**
- Codex credentials persist after pod restart
- No re-login required

**Cleanup:** Delete

```bash
vibespace delete test-persist-codex --force
```

---

### 1.18 Credential Persistence - Mixed Vibespace

```bash
vibespace create test-persist-mixed
vibespace test-persist-mixed spawn --agent-type codex

# Verify both authenticated
vibespace test-persist-mixed connect claude-1
# Verify authenticated, exit

vibespace test-persist-mixed connect codex-1
# Verify authenticated, exit
```

```bash
# Trigger restart on claude
vibespace test-persist-mixed config set claude-1 --allowed-tools "Bash,Read"

# Trigger restart on codex
vibespace test-persist-mixed config set codex-1 --allowed-tools "Read,Write"

# Verify both still authenticated
vibespace test-persist-mixed connect claude-1
# Should still be authenticated

vibespace test-persist-mixed connect codex-1
# Should still be authenticated
```

**Expected:**
- Both agent types maintain separate credentials
- Each agent's restart doesn't affect the other
- Both persist credentials independently

**Cleanup:** Delete

```bash
vibespace delete test-persist-mixed --force
```

---

### 1.19 Credential Persistence - down/up Cycle

```bash
vibespace create test-lifecycle
vibespace test-lifecycle connect claude-1
# Verify authenticated, exit

vibespace test-lifecycle down claude-1
vibespace test-lifecycle up claude-1

vibespace test-lifecycle connect claude-1
# Verify STILL authenticated after down/up
```

**Expected:**
- `down` scales pod to 0 but preserves state
- `up` scales back to 1
- Credentials persist through down/up cycle

**Cleanup:** Delete

```bash
vibespace delete test-lifecycle --force
```

---

### 1.20 Codex-Only Vibespace

```bash
vibespace create test-codex --agent-type codex
vibespace test-codex agents --json
```

**Expected:**
- Vibespace created with codex as primary agent
- Agents shows `codex-1` with type `codex`
- No claude agents present

**Cleanup:** Keep for headless tests

---

### 1.21 Headless Multi - JSON Output

```bash
vibespace multi --vibespaces test-codex --json --list-agents
```

**Expected:**
- JSON output: `{"session":"...","agents":["codex-1@test-codex"]}`

```bash
echo "hello" | vibespace multi --vibespaces test-codex --json
```

**Expected:**
- JSON response with codex-1 response
- Structure: `{"session":"...","responses":[{"agent":"codex-1@test-codex","content":"..."}]}`

**Cleanup:** Delete

```bash
vibespace delete test-codex --force
```

---

### 1.22 Headless Multi - Mixed Agents

```bash
vibespace create test-headless
vibespace test-headless spawn --agent-type codex
vibespace test-headless agents --json
```

**Expected:**
- Two agents: claude-1 (claude-code), codex-1 (codex)

```bash
vibespace multi --vibespaces test-headless --json --list-agents
```

**Expected:**
- Both agents listed: `["claude-1@test-headless","codex-1@test-headless"]`

```bash
vibespace multi --vibespaces test-headless --agent claude-1@test-headless --json "say hi"
```

**Expected:**
- Response only from claude-1

```bash
vibespace multi --vibespaces test-headless --agent codex-1@test-headless --json "say hi"
```

**Expected:**
- Response only from codex-1

**Cleanup:** Keep for TUI tests

---

## Part 2: TUI Commands

### 2.1 TUI Launch

```bash
vibespace multi --vibespaces test-headless
```

**Expected:**
- TUI launches with header showing session info
- Agent list shows claude-1@test-headless, codex-1@test-headless
- Input prompt at bottom
- Both agents shown as connected

---

### 2.2 TUI Message to All

Type: `hello everyone`

**Expected:**
- Message sent to both agents
- Both claude-1 and codex-1 respond
- Responses displayed with agent prefix `[claude-1@test-headless]` and `[codex-1@test-headless]`
- Syntax highlighting for any code in responses

---

### 2.3 TUI Targeted Message - Claude

Type: `@claude-1@test-headless what model are you?`

**Expected:**
- Only claude-1 receives and responds
- Response identifies as Claude (mentions Claude, Anthropic, etc.)
- codex-1 does NOT respond

---

### 2.4 TUI Targeted Message - Codex

Type: `@codex-1@test-headless what model are you?`

**Expected:**
- Only codex-1 receives and responds
- Response identifies as Codex/GPT (mentions OpenAI, GPT, etc.)
- claude-1 does NOT respond

---

### 2.5 TUI Tool Use Display - Claude

Type: `@claude-1@test-headless list files in /vibespace using ls`

**Expected:**
- Tool use indicator shows for Bash tool
- Tool result displayed
- Final response includes file listing
- Tool name displayed correctly (e.g., "Bash")

---

### 2.6 TUI Tool Use Display - Codex

Type: `@codex-1@test-headless list files in /vibespace`

**Expected:**
- Tool use indicator shows for shell/exec tool
- Tool result displayed
- Final response includes file listing
- Tool name displayed correctly for Codex's tool format

---

### 2.7 TUI Help Command

Type: `/help`

**Expected:**
- Help text displayed with available commands
- Lists: /help, /agents, /quit, /focus, /session, etc.

---

### 2.8 TUI Agents Command

Type: `/agents`

**Expected:**
- List of connected agents with status
- Shows both claude-1@test-headless and codex-1@test-headless
- Shows connection status (connected/disconnected)

---

### 2.9 TUI Session Info

Type: `/session @claude-1@test-headless info`

**Expected:**
- Current session ID displayed
- Message count shown

Type: `/session @codex-1@test-headless info`

**Expected:**
- Codex session info displayed (may differ in format from Claude)

---

### 2.10 TUI New Session

Type: `/session @claude-1@test-headless new`

**Expected:**
- New session created for claude-1
- Previous session ended
- Fresh conversation context

---

### 2.11 TUI Focus Mode - Claude

Type: `/focus @claude-1@test-headless`

**Expected:**
- Enters full interactive Claude session (via tmux)
- Can type directly to Claude
- Full Claude Code experience
- Ctrl+B D detaches back to TUI

---

### 2.12 TUI Focus Mode - Codex

Type: `/focus @codex-1@test-headless`

**Expected:**
- Enters full interactive Codex session (via tmux)
- Can type directly to Codex
- Full Codex experience
- Ctrl+B D detaches back to TUI

---

### 2.13 TUI Exit

Type: `/quit` or press Ctrl+C

**Expected:**
- Clean exit
- Connections closed gracefully
- Session saved to history file
- Returns to shell prompt

**Final Cleanup:**
```bash
vibespace delete test-headless --force
vibespace stop
```

---

## Test Summary Tables

### Config Flags Coverage

| Flag | Create | Spawn | Config Set |
|------|:------:|:-----:|:----------:|
| `--share-credentials` | 1.8 | 1.8b | 1.8e |
| `--skip-permissions` | 1.14 | 1.15b | 1.14b |
| `--allowed-tools` | 1.9 | 1.9b | 1.9c |
| `--disallowed-tools` | 1.10 | - | 1.10b |
| `--model` | 1.11 | 1.11b | 1.11c |
| `--max-turns` | 1.12 | 1.12b | 1.12c |
| `--system-prompt` | 1.13 | - | 1.13b |
| `--agent-type` | 1.20 | 1.3 | N/A |

### Credential Persistence Coverage

| Scenario | Test | Agent Type |
|----------|:----:|------------|
| Config change (allowed-tools) | 1.16 | claude-code |
| Multiple config changes | 1.16b | claude-code |
| With share-credentials | 1.16c | claude-code |
| Codex config change | 1.17 | codex |
| Mixed vibespace | 1.18 | both |
| down/up cycle | 1.19 | claude-code |

### Share Credentials Coverage

| Scenario | Test |
|----------|:----:|
| Same type, both share | 1.8 |
| Same type, one shares | 1.8b |
| Mixed types, both share | 1.8c |
| Mixed types, partial share | 1.8d |
| Enable via config set | 1.8e |

### TUI Coverage

| Feature | Test |
|---------|:----:|
| Launch with mixed agents | 2.1 |
| Broadcast message | 2.2 |
| Target Claude | 2.3 |
| Target Codex | 2.4 |
| Claude tool use | 2.5 |
| Codex tool use | 2.6 |
| Help command | 2.7 |
| Agents command | 2.8 |
| Session info | 2.9 |
| New session | 2.10 |
| Focus Claude | 2.11 |
| Focus Codex | 2.12 |
| Clean exit | 2.13 |

---

## Execution Checklist

### Part 1: Non-TUI
- [ ] 1.1 Create Claude vibespace
- [ ] 1.2 Agent listing formats
- [ ] 1.3 Spawn Codex (mixed)
- [ ] 1.4 Config show
- [ ] 1.5 Config set
- [ ] 1.6 Agent lifecycle (down/up)
- [ ] 1.7 Agent kill
- [ ] 1.8 Share credentials - both share
- [ ] 1.8b Share credentials - partial
- [ ] 1.8c Share credentials - mixed types
- [ ] 1.8d Share credentials - mixed partial
- [ ] 1.8e Share credentials - config set
- [ ] 1.9 Allowed tools - create
- [ ] 1.9b Allowed tools - spawn
- [ ] 1.9c Allowed tools - config set
- [ ] 1.10 Disallowed tools - create
- [ ] 1.10b Disallowed tools - config set
- [ ] 1.11 Model - create
- [ ] 1.11b Model - spawn
- [ ] 1.11c Model - config set
- [ ] 1.12 Max turns - create
- [ ] 1.12b Max turns - spawn
- [ ] 1.12c Max turns - config set
- [ ] 1.13 System prompt - create
- [ ] 1.13b System prompt - config set
- [ ] 1.14 Skip permissions - create
- [ ] 1.14b Skip permissions - toggle
- [ ] 1.15 Combined flags - create
- [ ] 1.15b Combined flags - spawn codex
- [ ] 1.16 Credential persistence - config change
- [ ] 1.16b Credential persistence - multiple changes
- [ ] 1.16c Credential persistence - shared
- [ ] 1.17 Credential persistence - codex
- [ ] 1.18 Credential persistence - mixed
- [ ] 1.19 Credential persistence - down/up
- [ ] 1.20 Codex-only vibespace
- [ ] 1.21 Headless multi - JSON
- [ ] 1.22 Headless multi - mixed agents

### Part 2: TUI
- [ ] 2.1 TUI launch
- [ ] 2.2 Message to all
- [ ] 2.3 Target Claude
- [ ] 2.4 Target Codex
- [ ] 2.5 Claude tool use
- [ ] 2.6 Codex tool use
- [ ] 2.7 Help command
- [ ] 2.8 Agents command
- [ ] 2.9 Session info
- [ ] 2.10 New session
- [ ] 2.11 Focus Claude
- [ ] 2.12 Focus Codex
- [ ] 2.13 Clean exit

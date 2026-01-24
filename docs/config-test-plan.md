# Claude Agent Configuration - Test Plan

## Prerequisites
- Clean environment: `rm -rf ~/.vibespace/daemons ~/.vibespace/sessions ~/.vibespace/history ~/.vibespace/claude_sessions.json`
- Build: `go build -o vibespace ./cmd/vibespace`

---

## 1. Create Command Tests

### 1.1 Create with --skip-permissions
```bash
./vibespace create test1 --skip-permissions
./vibespace test1 config show
# Expected: skip_permissions: true
```

### 1.2 Create with --share-credentials
```bash
./vibespace create test2 --share-credentials
./vibespace test2 config show
# Expected: share_credentials: true
```

### 1.3 Create with --allowed-tools
```bash
./vibespace create test3 --allowed-tools "Bash,Read,Write"
./vibespace test3 config show
# Expected: allowed_tools: Bash, Read, Write
```

### 1.4 Create with --model
```bash
./vibespace create test4 --model opus
./vibespace test4 config show
# Expected: model: opus
```

### 1.5 Create with --max-turns
```bash
./vibespace create test5 --max-turns 10
./vibespace test5 config show
# Expected: max_turns: 10
```

### 1.6 Create with multiple flags
```bash
./vibespace create test6 --skip-permissions --share-credentials --model sonnet
./vibespace test6 config show
# Expected: skip_permissions: true, share_credentials: true, model: sonnet
```

---

## 2. Spawn Command Tests

### 2.1 Spawn with --skip-permissions
```bash
./vibespace test1 spawn --skip-permissions --name trusted
./vibespace test1 config show trusted
# Expected: skip_permissions: true
```

### 2.2 Spawn with --allowed-tools override
```bash
./vibespace test1 spawn --allowed-tools "Bash,Read,Write,Edit" --name editor
./vibespace test1 config show editor
# Expected: allowed_tools: Bash, Read, Write, Edit
```

### 2.3 Spawn inherits vibespace defaults (no flags)
```bash
# Use test1 which has skip_permissions=true
./vibespace test1 spawn --name inherit
./vibespace test1 config show inherit
# Expected: skip_permissions should be false (spawn doesn't inherit by default)
```

---

## 3. Config Show Tests

### 3.1 Show all agents
```bash
./vibespace test1 config show
# Expected: Lists all agents with their configs
```

### 3.2 Show specific agent
```bash
./vibespace test1 config show claude-1
# Expected: Shows only claude-1 config
```

### 3.3 JSON output
```bash
./vibespace test1 config show --json
# Expected: Valid JSON with all agent configs
```

### 3.4 JSON output for specific agent
```bash
./vibespace test1 config show claude-1 --json
# Expected: Valid JSON with claude-1 config
```

---

## 4. Config Set Tests

### 4.1 Enable skip-permissions
```bash
./vibespace test2 config set claude-1 --skip-permissions
./vibespace test2 config show claude-1
# Expected: skip_permissions: true, pod restarts
```

### 4.2 Disable skip-permissions
```bash
./vibespace test1 config set claude-1 --no-skip-permissions
./vibespace test1 config show claude-1
# Expected: skip_permissions: false
```

### 4.3 Change allowed-tools
```bash
./vibespace test1 config set claude-1 --allowed-tools "Read,Glob,Grep"
./vibespace test1 config show claude-1
# Expected: allowed_tools: Read, Glob, Grep
```

### 4.4 Set model
```bash
./vibespace test1 config set claude-1 --model opus
./vibespace test1 config show claude-1
# Expected: model: opus
```

### 4.5 Set max-turns
```bash
./vibespace test1 config set claude-1 --max-turns 5
./vibespace test1 config show claude-1
# Expected: max_turns: 5
```

---

## 5. Connect Command Tests

### 5.1 Connect uses agent config
```bash
./vibespace test1 up
./vibespace test1 connect claude-1
# Expected: Claude runs with agent's config (--dangerously-skip-permissions if set)
# Verify by checking Claude's behavior or startup message
```

### 5.2 Connect to agent with custom allowed-tools
```bash
./vibespace test3 up
./vibespace test3 connect claude-1
# Expected: Claude runs with --allowedTools "Bash,Read,Write"
```

---

## 6. TUI/Multi Tests (Interactive)

### 6.1 Multi session uses agent config
```bash
./vibespace test1 up
./vibespace test1 multi
# Expected: TUI starts, Claude uses agent's config
# Send a message and verify Claude responds appropriately
```

### 6.2 Multi with multiple vibespaces
```bash
./vibespace test1 up
./vibespace test2 up
./vibespace multi test1 test2
# Expected: Both agents connect with their respective configs
```

### 6.3 Verify skip-permissions in TUI
```bash
# Create vibespace with skip-permissions
./vibespace create skiptest --skip-permissions
./vibespace skiptest up
./vibespace skiptest multi
# Expected: Claude should not prompt for permissions when using tools
# Test by asking Claude to read a file or run a command
```

---

## 7. Non-TTY / Scripting Tests

### 7.1 List agents with JSON
```bash
./vibespace test1 agents --json
# Expected: Valid JSON with agent list
```

### 7.2 Config show with JSON piped
```bash
./vibespace test1 config show --json | jq .
# Expected: Pretty-printed JSON
```

### 7.3 Non-interactive multi (send message)
```bash
echo "Hello" | ./vibespace test1 multi --agent claude-1
# Expected: Claude responds (uses agent config)
```

### 7.4 Batch mode
```bash
echo '{"agent": "claude-1@test1", "message": "What is 2+2?"}' | ./vibespace multi test1 --batch
# Expected: JSON response
```

---

## 8. Edge Cases

### 8.1 Config show for non-existent agent
```bash
./vibespace test1 config show nonexistent
# Expected: Error message
```

### 8.2 Config set for non-existent agent
```bash
./vibespace test1 config set nonexistent --skip-permissions
# Expected: Error message
```

### 8.3 Create with invalid allowed-tools format
```bash
./vibespace create badtools --allowed-tools ""
./vibespace badtools config show
# Expected: Uses default tools
```

### 8.4 Pod restart after config set
```bash
./vibespace test1 config set claude-1 --model opus
kubectl get pods -n vibespace -w
# Expected: Pod restarts (rolling update)
```

---

## 9. Regression Tests

### 9.1 Default behavior unchanged
```bash
./vibespace create defaulttest
./vibespace defaulttest config show
# Expected: All defaults (skip_permissions: false, default tools, etc.)
```

### 9.2 Connect without agent flag gives shell
```bash
./vibespace defaulttest up
./vibespace defaulttest connect
# Expected: Shell access (not Claude)
```

### 9.3 Connect with agent runs Claude
```bash
./vibespace defaulttest connect claude-1
# Expected: Claude interactive session
```

---

## Cleanup
```bash
./vibespace delete test1 test2 test3 test4 test5 test6 skiptest defaulttest badtools
```

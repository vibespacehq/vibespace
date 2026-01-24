# Daemon v2 Testing Guide

This document provides a comprehensive test plan for the new daemon architecture to ensure feature parity.

## Prerequisites

1. Fresh cluster initialized:
   ```bash
   vibespace init
   ```

2. Built binary:
   ```bash
   go build -o vibespace ./cmd/vibespace
   ```

3. Clean state (if needed):
   ```bash
   rm -rf ~/.vibespace/forwards ~/.vibespace/sessions ~/.vibespace/history
   rm -f ~/.vibespace/claude_sessions.json ~/.vibespace/daemon.*
   ```

---

## Test 1: Daemon Auto-Start

The daemon should auto-start when needed.

```bash
# Ensure daemon is not running
pkill -f "vibespace daemon" 2>/dev/null

# Check status (daemon not running)
./vibespace status
# Expected: "Daemon: not running"

# Create a vibespace (should NOT start daemon)
./vibespace create test1

# Scale up (should auto-start daemon)
./vibespace test1 up

# Check status (daemon should be running)
./vibespace status
# Expected: "Daemon: running (uptime: Xs, pid: XXXXX)"
```

**Expected behavior:**
- Daemon starts automatically on `up` command
- Status shows daemon running with uptime and PID

---

## Test 2: Port Forwarding via Watch API

The daemon should automatically create forwards when pods become ready.

```bash
# List forwards (should show SSH and ttyd)
./vibespace test1 forward list

# Expected output:
# Forwards for test1:
#   claude-1:
#     SSH:  localhost:10022 -> 22 (active)
#     ttyd: localhost:17681 -> 7681 (active)
```

**Expected behavior:**
- SSH forward on port 10022 (or similar)
- ttyd forward on port 17681 (or similar)
- Both marked as "active"

---

## Test 3: Connect via SSH

```bash
# Connect to the agent
./vibespace test1 connect

# Expected: SSH connection to Claude Code CLI
# Type 'exit' to disconnect
```

**Expected behavior:**
- SSH connects successfully
- Claude Code CLI is accessible
- Can run commands inside the container

---

## Test 4: Connect via Browser (ttyd)

```bash
# Connect with browser flag
./vibespace test1 connect --browser

# Expected: Opens browser to http://localhost:17681
```

**Expected behavior:**
- Browser opens to ttyd URL
- Terminal is accessible in browser

---

## Test 5: Multiple Agents (Spawn)

```bash
# Spawn a second agent
./vibespace test1 spawn

# Wait for pod to be ready (watch with kubectl or wait ~30s)
sleep 30

# List agents
./vibespace test1 agents

# Expected:
# Agents in test1:
#   claude-1: running
#   claude-2: running

# List forwards (should show both agents)
./vibespace test1 forward list

# Expected: Forwards for both claude-1 and claude-2
# claude-1: SSH 10022, ttyd 17681
# claude-2: SSH 10023, ttyd 17682 (or similar offset)
```

**Expected behavior:**
- Second agent spawns successfully
- Daemon automatically detects new pod via watch API
- Forwards created automatically for new agent

---

## Test 6: Agent Kill and Cleanup

```bash
# Kill the second agent
./vibespace test1 kill claude-2

# Wait for cleanup
sleep 10

# List forwards (should only show claude-1)
./vibespace test1 forward list

# Expected: Only claude-1 forwards remain
```

**Expected behavior:**
- Agent removed from Kubernetes
- Daemon detects pod deletion via watch API
- Forwards for claude-2 are cleaned up automatically

---

## Test 7: Scale Down and Up

```bash
# Scale down (replicas to 0)
./vibespace test1 down

# Check forwards (should be empty or daemon shows no active forwards)
./vibespace test1 forward list

# Scale back up
./vibespace test1 up

# Wait for pod ready
sleep 30

# Check forwards (should be restored)
./vibespace test1 forward list
```

**Expected behavior:**
- `down` scales to 0 replicas
- Forwards are cleaned up when pods go away
- `up` scales back to 1 replica
- Forwards are recreated when pod becomes ready

---

## Test 8: Multiple Vibespaces

```bash
# Create a second vibespace
./vibespace create test2
./vibespace test2 up

# Wait for ready
sleep 30

# Check status (should show both vibespaces)
./vibespace status

# Expected daemon output:
# Daemon: running (uptime: Xs, pid: XXXXX)
#   Managed vibespaces:
#     test1: 1 agent(s)
#     test2: 1 agent(s)

# List forwards for each
./vibespace test1 forward list
./vibespace test2 forward list

# Expected: Each has its own set of forwards with different local ports
```

**Expected behavior:**
- Single daemon manages multiple vibespaces
- Each vibespace has independent forwards
- No port conflicts between vibespaces

---

## Test 9: TUI Multi-Session

```bash
# Start TUI with multiple vibespaces
./vibespace multi test1 test2

# In the TUI:
# 1. Send a message to all: "hello"
# 2. Send to specific agent: @claude-1@test1 what vibespace are you in?
# 3. Try /help to see commands
# 4. Try /quit to exit
```

**Expected behavior:**
- TUI launches successfully
- Can see agents from both vibespaces
- Messages route correctly to specified agents
- `/quit` exits cleanly

---

## Test 10: TUI Session Persistence

```bash
# Create a named session
./vibespace session create mysession
./vibespace session add mysession test1

# Start the session
./vibespace session start mysession

# Send some messages, then /quit

# Restart the session
./vibespace session start mysession

# Expected: Previous messages visible in history
```

**Expected behavior:**
- Session persists between restarts
- Chat history is preserved

---

## Test 11: Non-Interactive Mode (Scripting)

```bash
# List agents in JSON
./vibespace multi test1 --list-agents --json

# Send a message and get JSON response
echo "what is 2+2?" | ./vibespace multi test1 --json

# Plain text mode
echo "hello" | ./vibespace multi test1 --plain
```

**Expected behavior:**
- JSON output is valid JSON
- Plain text output is parseable
- Works in non-TTY environments

---

## Test 12: Daemon Stop

```bash
# Stop the cluster (should stop daemon first)
./vibespace stop

# Check daemon is not running
./vibespace status
# Expected: Cluster stopped, daemon not running
```

**Expected behavior:**
- `stop` command stops daemon gracefully
- Daemon processes are cleaned up
- Socket file is removed

---

## Test 13: Daemon Recovery

```bash
# Start cluster
./vibespace init

# Create and start a vibespace
./vibespace create recovery-test
./vibespace recovery-test up
sleep 30

# Force kill the daemon
pkill -9 -f "vibespace daemon"

# Try to connect (should auto-restart daemon)
./vibespace recovery-test connect
# (Ctrl+D to exit)

# Check daemon is running again
./vibespace status
```

**Expected behavior:**
- Daemon auto-restarts when needed
- Forwards are recreated via reconciliation
- Connection works after recovery

---

## Test 14: Debug Mode

```bash
# Run with debug logging
VIBESPACE_DEBUG=1 ./vibespace test1 forward list

# Check debug log
cat ~/.vibespace/debug.log | tail -20

# Run daemon in foreground for debugging (optional)
VIBESPACE_DEBUG=1 ./vibespace daemon
# (Ctrl+C to stop)
```

**Expected behavior:**
- Debug logs written to `~/.vibespace/debug.log`
- Daemon logs to `~/.vibespace/daemon.log`
- Logs contain useful debugging information

---

## Test 15: JSON Output

```bash
# Status in JSON
./vibespace status --json

# List in JSON
./vibespace list --json

# Forward list in JSON
./vibespace test1 forward list --json
```

**Expected behavior:**
- All commands support `--json` flag
- Output is valid JSON
- Contains all relevant information

---

## Cleanup

```bash
# Delete test vibespaces
./vibespace delete test1 --force
./vibespace delete test2 --force
./vibespace delete recovery-test --force

# Stop cluster if done
./vibespace stop
```

---

## Summary Checklist

| Test | Feature | Status |
|------|---------|--------|
| 1 | Daemon auto-start | [ ] |
| 2 | Port forwarding via watch API | [ ] |
| 3 | SSH connect | [ ] |
| 4 | Browser connect (ttyd) | [ ] |
| 5 | Multiple agents (spawn) | [ ] |
| 6 | Agent kill and cleanup | [ ] |
| 7 | Scale down/up | [ ] |
| 8 | Multiple vibespaces | [ ] |
| 9 | TUI multi-session | [ ] |
| 10 | TUI session persistence | [ ] |
| 11 | Non-interactive mode | [ ] |
| 12 | Daemon stop | [ ] |
| 13 | Daemon recovery | [ ] |
| 14 | Debug mode | [ ] |
| 15 | JSON output | [ ] |

---

## Known Differences from Old Architecture

1. **Single daemon**: Previously one daemon per vibespace, now one global daemon
2. **Watch API**: Forwards are created reactively when pods change, not imperatively
3. **Auto-reconciliation**: Daemon automatically syncs state on pod changes
4. **Simplified commands**: No more `forward start/stop` - just `up/down` for scaling

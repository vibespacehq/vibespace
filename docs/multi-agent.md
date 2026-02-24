# Multi-Agent Sessions

<!-- TODO: add demo GIF of a multi-agent session -->

Vibespace can run multiple AI agents working on the same codebase simultaneously. Agents share a filesystem, so changes one agent makes are immediately visible to the others.

## Adding agents to a vibespace

A vibespace starts with one agent. Add more with:

```bash
vibespace agent create --vibespace my-project -t claude-code -s --name reviewer
```

The `-s` flag shares credentials with the primary agent (if the primary was also created with `-s`). Without it, you'd need to log in to the new agent separately.

List agents:

```bash
vibespace agent list --vibespace my-project
```

## Interactive sessions

Start a multi-agent chat session:

```bash
vibespace multi --vibespaces my-project
```

This opens an interactive TUI where you can send messages to specific agents or broadcast to all of them.

### Targeting agents

- `@claude-1 explain the auth flow` — send to a specific agent
- `@all write tests for the login page` — send to every agent in the session

### Session management

Sessions persist automatically. Resume a previous session:

```bash
vibespace multi --resume
```

This shows a picker if you have multiple sessions. Pass a session name to resume directly:

```bash
vibespace multi --resume my-session
```

### Spanning vibespaces

Sessions can include agents from multiple vibespaces:

```bash
vibespace multi --vibespaces frontend,backend
```

Or pick specific agents:

```bash
vibespace multi --agents claude-1@frontend,codex-1@backend
```

## Non-interactive usage

Send a single message and get a response:

```bash
vibespace multi "run the test suite" --vibespace my-project --agent claude-1
```

Stream the response as plain text:

```bash
vibespace multi "explain this codebase" --vibespace my-project --stream
```

Batch mode reads JSONL from stdin:

```bash
echo '{"agent":"claude-1","message":"run tests"}' | vibespace multi --vibespace my-project --batch
```

## Session commands

```bash
# List all sessions
vibespace session list

# Show session details
vibespace session show my-session

# Delete a session
vibespace session delete my-session
```

## Use cases

**Code review workflow:** One agent writes code, another reviews it. The reviewer sees the same files and can suggest changes.

**Divide and conquer:** Point one agent at the backend and another at the frontend. They share the repo but work on different parts.

**Second opinion:** Ask two different models (Claude and Codex) the same question and compare their approaches.

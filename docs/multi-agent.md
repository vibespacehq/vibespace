# Multi-Agent Sessions

<!-- TODO: add demo GIF of a multi-agent session -->

Vibespace can run multiple AI agents working on the same codebase simultaneously. By default, agents share a filesystem, so changes one agent makes are immediately visible to the others. For parallel development where agents need isolation, see [Worktree Mode](#worktree-mode) below.

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
vibespace multi "run the test suite" --vibespaces my-project --agent claude-1
```

Stream the response as plain text:

```bash
vibespace multi "explain this codebase" --vibespaces my-project --stream
```

Batch mode reads JSONL from stdin:

```bash
echo '{"agent":"claude-1","message":"run tests"}' | vibespace multi --vibespaces my-project --batch
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

## Worktree mode

By default, all agents share the same filesystem — great for collaboration but problematic when agents need to make independent changes. Worktree mode solves this by giving each agent its own git branch and working copy.

### Setup

Enable worktree mode when creating a vibespace with a GitHub repo:

```bash
vibespace create my-project -t claude-code -s --repo https://github.com/org/repo --worktree
```

This creates a bare git clone on the shared PVC. Each agent gets its own worktree checked out to a branch named after the agent (e.g., `claude-1`, `claude-2`).

### Custom branches

Override the default branch name:

```bash
# Primary agent on a custom branch
vibespace create my-project -t claude-code -s --repo https://github.com/org/repo --worktree --branch feat/api

# Additional agent on a custom branch
vibespace agent create --vibespace my-project -t claude-code -s --branch feat/frontend
```

### Filesystem layout

```
/vibespace/
  .bare-repo/              # bare git clone (shared history)
  worktrees/
    claude-1/              # agent 1's checkout (branch: claude-1)
    claude-2/              # agent 2's checkout (branch: claude-2)
```

Each agent's shell starts in its own worktree directory. Agents can see each other's worktrees but work independently on their own branches.

### When to use worktree mode

| Scenario | Mode |
|----------|------|
| Agents working on separate features | Worktree |
| One agent writes, another reviews | Shared (default) |
| Parallel refactoring across modules | Worktree |
| Pair programming on the same files | Shared (default) |

## Use cases

**Code review workflow:** One agent writes code, another reviews it. The reviewer sees the same files and can suggest changes. Use shared mode (default).

**Divide and conquer:** Point one agent at the backend and another at the frontend. Use worktree mode so they can commit independently without conflicts.

**Second opinion:** Ask two different models (Claude and Codex) the same question and compare their approaches.

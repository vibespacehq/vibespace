# TUI

<!-- TODO: add GIF/screenshots of each TUI tab -->

Run `vibespace` with no arguments to launch the terminal UI.

```bash
vibespace
```

The TUI has five tabs across the top. Switch between them with number keys (`1`-`5`), `Tab`/`Shift+Tab`, or mouse click.

## Tabs

### 1. Vibespaces

Browse and manage vibespaces. Three levels of navigation:

- **List** — all vibespaces with status, agents, resources. `Enter` to drill into one.
- **Agent view** — agents in the selected vibespace with their config, resources, and port forwards. `Enter` to see sessions, `x` to connect via SSH, `b` to open in browser.
- **Sessions** — agent conversation history. `Enter` to resume a session.

Actions: `n` new vibespace, `d` delete, `a` add agent (in agent view), `e` edit config, `f` manage forwards, `S` start/stop.

### 2. Chat

Multi-agent chat interface. Start a session from the Sessions tab or Vibespaces tab.

Slash commands: `/help`, `/list`, `/add`, `/remove`, `/focus`, `/clear`, `/session`, `/scroll`, `/quit`.

### 3. Monitor

Live resource dashboard with 5-second auto-refresh.

Shows per-agent CPU and memory usage with bar charts. When the terminal is tall enough, it also displays CPU and memory history as streaming line charts.

Controls: `R` force refresh, `p` pause/resume, `v` filter by vibespace.

### 4. Sessions

Browse, create, and resume multi-agent sessions.

Creating a new session (`n`) walks through three steps: name the session, pick vibespaces, then pick agents from each vibespace. The session opens in the Chat tab.

### 5. Remote

WireGuard remote mode controls. Shows different content depending on connection state:

- **Disconnected** — token input to connect
- **Connected** — server info, tunnel health, diagnostics. `D` to disconnect.
- **Serving** — endpoint, client list, token generation. `g` to generate a token.

## Overlays

**Help (`?`):** Keybinding reference grouped by context.

**Command palette (`:`):** Fuzzy-searchable action list. Type to filter, `Enter` to execute.

## Keybindings

### Global

| Key | Action |
|---|---|
| `1`-`5` | Switch tab |
| `Ctrl+1`-`Ctrl+5` | Switch tab (always, even when typing) |
| `Tab` / `Shift+Tab` | Next / previous tab |
| `?` | Help overlay |
| `:` | Command palette |
| `Ctrl+C` | Quit |

### Navigation (most tabs)

| Key | Action |
|---|---|
| `j` / `k` or arrow keys | Move up / down |
| `g` / `G` | Jump to first / last |
| `Enter` | Select / drill in |
| `Esc` | Back |

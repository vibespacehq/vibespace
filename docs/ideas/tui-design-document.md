# TUI Design v2

What you get when you type `vibespace` with no arguments.

## 1. Overview

**Tab-based navigation.** Five permanent tabs across the top. Switch with number keys
or mouse click. No stack, no drill-down — every major surface is one keypress away.

**Sessions span vibespaces.** A chat session can include agents from `myproject`,
`backend-api`, and `experiment` simultaneously. The session model already supports this
(`session.Session.Vibespaces []VibespaceEntry`). The TUI makes it obvious.

**Library-driven rendering.** Tables use `lipgloss/table`. Lists use `lipgloss/list`.
Monitoring charts use `ntcharts` (sparklines, bar charts). Mouse support via
`bubblezone`. Spring animations via `harmonica` for smooth transitions. No hand-rolled
ASCII layout.

## 2. Tab Layout

```
 ╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
 │                                                                         │
 │                         (active tab content)                            │
 │                                                                         │
 ├─────────────────────────────────────────────────────────────────────────┤
 │ status bar                                                     ? help  │
 ╰─────────────────────────────────────────────────────────────────────────╯
```

The active tab is highlighted in teal with bold text. Inactive tabs are dim. The tab
bar is rendered with `lipgloss` horizontal join and `bubblezone.Mark()` for click
targets. The status bar at the bottom shows contextual hints and cluster health.

**Tab switching:**

| Key | Tab |
|-----|-----|
| `1` | Vibespaces |
| `2` | Chat |
| `3` | Monitor |
| `4` | Sessions |
| `5` | Remote |
| `Tab` | Next tab |
| `Shift+Tab` | Previous tab |
| Mouse click | Any tab |

These keys work everywhere except inside the Chat tab's text input (where `1`-`5` are
regular characters). In Chat, use `Ctrl+1` through `Ctrl+5` or click the tabs.

## 3. Dependencies

| Library | Import | Purpose |
|---------|--------|---------|
| bubbletea | `tea "github.com/charmbracelet/bubbletea"` | Application framework |
| bubbles | `"github.com/charmbracelet/bubbles/*"` | Text input, viewport, spinner |
| lipgloss | `"github.com/charmbracelet/lipgloss"` | Styling, layout |
| lipgloss/table | `"github.com/charmbracelet/lipgloss/table"` | All data tables |
| lipgloss/list | `"github.com/charmbracelet/lipgloss/list"` | Agent lists, config display |
| lipgloss/tree | `"github.com/charmbracelet/lipgloss/tree"` | Vibespace → agent hierarchy |
| bubblezone | `zone "github.com/lrstanley/bubblezone"` | Mouse click regions |
| ntcharts | `"github.com/NimbleMarkets/ntcharts/*"` | Sparklines, bar charts |
| harmonica | `"github.com/charmbracelet/harmonica"` | Tab switch animation |

## 4. Tab 1: Vibespaces

The home tab. Shows cluster health at the top, then a `lipgloss/table` of vibespaces
with inline expansion for agents.

### 4.1 Collapsed View (default)

```
 ╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
 │                                                                         │
 │  Cluster running    Daemon running (pid 4821, up 3h)    Remote ──       │
 │                                                                         │
 │ ┌────────────┬──────────┬────────┬───────┬────────┬─────────┬─────────┐ │
 │ │ NAME       │ STATUS   │ AGENTS │ CPU   │ MEMORY │ STORAGE │ AGE     │ │
 │ ├────────────┼──────────┼────────┼───────┼────────┼─────────┼─────────┤ │
 │ │▸myproject  │ running  │ 3      │ 750m  │ 1.5Gi  │ 10Gi    │ 2d      │ │
 │ │ backend-api│ running  │ 2      │ 500m  │ 1Gi    │ 10Gi    │ 5d      │ │
 │ │ ml-pipeline│ stopped  │ 1      │ 250m  │ 512Mi  │ 20Gi    │ 12d     │ │
 │ │ experiment │ running  │ 4      │ 1000m │ 2Gi    │ 10Gi    │ 1h      │ │
 │ └────────────┴──────────┴────────┴───────┴────────┴─────────┴─────────┘ │
 │                                                                         │
 │                                                                         │
 │                                                                         │
 │                                                                         │
 │                                                                         │
 │                                                                         │
 │                                                                         │
 │                                                                         │
 ├─────────────────────────────────────────────────────────────────────────┤
 │ j/k navigate  Enter expand  x connect  b browser  n new  d delete  c chat│
 ╰─────────────────────────────────────────────────────────────────────────╯
```

The table is rendered with `lipgloss/table`:

```go
t := table.New().
    Border(lipgloss.RoundedBorder()).
    BorderStyle(lipgloss.NewStyle().Foreground(ui.ColorMuted)).
    StyleFunc(func(row, col int) lipgloss.Style {
        switch {
        case row == table.HeaderRow:
            return headerStyle
        case row == selectedRow:
            return selectedStyle
        default:
            return cellStyle
        }
    }).
    Headers("NAME", "STATUS", "AGENTS", "CPU", "MEMORY", "STORAGE", "AGE").
    Rows(rows...)
```

STATUS cells are colored: teal for running, yellow for stopped, red for error.
The selected row (`▸` prefix) gets brighter text. Each row is a `bubblezone.Mark()`
region for mouse click support.

**Responsive:** At 80 columns, AGE and STORAGE columns are hidden. At 60 columns, CPU
and MEMORY are also hidden. Table widths are set dynamically based on terminal width.

### 4.2 Agent View (Enter on a vibespace)

Pressing Enter on a vibespace row navigates into a full-screen agent view for that
vibespace, completely replacing the table. This is stack navigation — `Esc` or
`Backspace` returns to the vibespace list.

```
 │  ← myproject                                          running │
 │ ────────────────────────────────────────────────────────────── │
 │                                                               │
 │  myproject                                                    │
 │  ├── claude-1   claude-code  running  model=sonnet  skip=true │
 │  ├── claude-2   claude-code  running  model=opus    skip=false│
 │  └── codex-1    codex        running  model=default           │
 │                                                               │
 │  Resources   CPU 750m (limit 1000m)  Mem 1.5Gi (limit 2Gi)   │
 │  Storage     10Gi (PVC)                                       │
 │  Mounts      ~/code → /workspace (rw)                         │
 │  Forwards    :52341→:22 [ssh]  :3000→:3000                    │
 │  Image       ghcr.io/vibespacehq/vibespace/claude-code:latest │
 │                                                               │
 │  Recent Logs                                                  │
 │  ──────────────────────────────────────────────────────────── │
 │  2026-02-12 16:23:25 INFO spawned: 'sshd' with pid 28        │
 │  2026-02-12 16:23:25 INFO spawned: 'ttyd' with pid 29        │
 │  ...                                                          │
```

The agent tree is rendered with `lipgloss/tree`:

```go
t := tree.Root("myproject").
    Child("claude-1   claude-code  running  model=sonnet  skip=true").
    Child("claude-2   claude-code  running  model=opus    skip=false").
    Child("codex-1    codex        running  model=default").
    Enumerator(tree.RoundedEnumerator).
    RootStyle(rootStyle).
    ItemStyle(itemStyle)
```

Use `j`/`k` to move the cursor between agents. Press `x` to connect, `e` to edit
config, `a` to add agent, `b` for browser (actions implemented in phase 3c/3d).

### 4.3 Connect (SSH into Vibespace/Agent)

Two connect modes, both accessible from the Vibespaces tab:

**Shell connect (`x` on a vibespace row, collapsed):** Opens a raw SSH shell into the
vibespace container itself. No agent — just a terminal. Uses `tea.ExecProcess` to
suspend the TUI and run SSH as a child process. When the SSH session exits, the TUI
resumes exactly where it was.

**Agent connect (`x` on an agent row, expanded):** SSH into the specific agent container
and launches its interactive CLI (claude-code or codex). This is a direct connection —
the agent's own interface, not the multi-agent chat.

Both use the same mechanism under the hood:

```go
// Shell connect (no agent) — raw terminal
cmd := connectViaSSH(localPort, "")
return tea.ExecProcess(cmd, func(err error) tea.Msg {
    return connectFinishedMsg{err: err}
})

// Agent connect — launches agent CLI in container
cmd := connectViaSSH(localPort, agentRemoteCommand)
return tea.ExecProcess(cmd, func(err error) tea.Msg {
    return connectFinishedMsg{err: err}
})
```

Before connecting, the TUI ensures the daemon is running and an SSH forward exists
for the target. If no forward is active, it starts one automatically.

**Browser mode (`b`):** Same as `x` but opens the connection in the system browser
via ttyd (equivalent to `vibespace connect --browser`). Instead of `tea.ExecProcess`,
this launches ttyd on a local port and opens the URL. The TUI stays running — no
suspend needed. A status line shows "Browser session active on :7681" while it's open.

```go
// Browser connect — start ttyd, open browser
url, cleanup := startTtydSession(localPort, agentName)
exec.Command("open", url).Start()  // macOS
return browserSessionStartedMsg{url: url, cleanup: cleanup}
```

**Connect vs /focus:** These are different. `x` (connect) suspends the TUI for a
direct 1:1 SSH session. `/focus` (from the Chat tab) launches the agent CLI inside
tmux for detach/reattach within the multi-agent chat context.

### 4.4 Vibespace Actions

From the vibespaces tab, with a vibespace selected:

| Key | Action |
|-----|--------|
| `Enter` | Toggle inline expansion |
| `x` | Connect — shell into vibespace (collapsed) or agent (expanded) |
| `b` | Connect in browser via ttyd (same as `--browser` flag) |
| `n` | Create new vibespace (inline form) |
| `d` | Delete vibespace (inline confirmation) |
| `c` | Open Chat tab with this vibespace's agents |
| `m` | Open Monitor tab focused on this vibespace |
| `S` | Start/stop vibespace |
| `e` | Edit selected agent's config (when expanded) |
| `a` | Add agent to selected vibespace |
| `f` | Manage forwards for selected vibespace |
| `/` | Filter/search vibespaces |

### 4.5 Inline Create Form

Pressing `n` replaces the area below the table with an inline creation form.
Fields are sequential — one at a time.

```
 │                                                                         │
 │  Create Vibespace                                                       │
 │                                                                         │
 │  Name ─────────────── my-new-project                                    │
 │  Agent type ────────── claude-code ▾                                    │
 │  Repository ────────── (optional, press Tab to skip)                    │
 │  CPU ───────────────── 250m                                             │
 │  Memory ────────────── 512Mi                                            │
 │  Storage ───────────── 10Gi                                             │
 │                                                                         │
 │  Enter: next  Tab: skip  Ctrl+s: create  Esc: cancel                    │
```

The active field has the cursor. Completed fields show their value dimmed. The
`Agent type` field is a selector (j/k to cycle). `Ctrl+s` submits. This is not a
separate view — it renders inside the Vibespaces tab below the table.

### 4.6 Inline Config Editor

Pressing `e` on an expanded agent shows the config editor inline. Uses
`lipgloss/list` for the key-value pairs.

```
 │  Config: claude-1 (claude-code)                                         │
 │                                                                         │
 │  • skip_permissions ─── true            ← Enter to toggle               │
 │  • model ───────────── sonnet           ← type to change                │
 │  • max_turns ───────── 0 (unlimited)                                    │
 │  • allowed_tools ───── Bash,Read,Write  ← Enter to edit                 │
 │  • disallowed_tools ── (none)                                           │
 │  • system_prompt ───── (none)           ← Enter to edit                 │
 │                                                                         │
 │  j/k navigate  Enter edit  Esc done                                     │
```

Changes are applied immediately via `vibespace.Service.SetAgentConfig()`.

### 4.7 Inline Forward Manager

Pressing `f` shows forward management inline.

```
 │  Forwards: myproject                                                    │
 │                                                                         │
 │ ┌──────────┬───────┬────────┬────────┬──────────┐                       │
 │ │ AGENT    │ LOCAL │ REMOTE │ TYPE   │ STATUS   │                       │
 │ ├──────────┼───────┼────────┼────────┼──────────┤                       │
 │ │ claude-1 │ 52341 │ 22     │ ssh    │ active   │                       │
 │ │ claude-1 │ 3000  │ 3000   │ manual │ active   │                       │
 │ │ claude-2 │ 52342 │ 22     │ ssh    │ active   │                       │
 │ └──────────┴───────┴────────┴────────┴──────────┘                       │
 │                                                                         │
 │  Detected Ports                                                         │
 │  • claude-1 :5432 postgres (5m ago)                                     │
 │  • claude-2 :8080 python/flask (2m ago)                                 │
 │                                                                         │
 │  a add  d remove  Enter forward detected port  Esc done                 │
```

Detected ports use `lipgloss/list`. The forwards table uses `lipgloss/table`.

## 5. Tab 2: Chat

The existing multi-agent chat view. The ~5,000 lines in `pkg/tui/` become this tab.
The existing `Model` struct runs as-is — the tab wraps it.

### 5.1 Layout

```
 ╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
 │                                                                         │
 │  session: cross-project    5 agents from 2 vibespaces    ↕ 72%         │
 │─────────────────────────────────────────────────────────────────────────│
 │                                                                         │
 │  14:32 [You → all] refactor the auth module to use JWT tokens           │
 │                                                                         │
 │  14:32 [claude-1@myproject] I'll refactor the authentication module.    │
 │        Let me start by examining the current implementation.            │
 │                                                                         │
 │        [◀] Read → src/auth/handler.go                                   │
 │        [✎] Edit → src/auth/handler.go                                   │
 │        [$] Bash → go test ./src/auth/...                                │
 │                                                                         │
 │        I've updated the auth handler to use JWT tokens.                 │
 │                                                                         │
 │  14:33 [codex-1@backend-api] I'll update the API client.               │
 │        [$] Bash → npm test                                              │
 │                                                                         │
 │  14:33 [claude-2@myproject] ⠋ thinking...                               │
 │                                                                         │
 │─────────────────────────────────────────────────────────────────────────│
 │ > @all                                                                  │
 │   Sending to all agents                                                 │
 ├─────────────────────────────────────────────────────────────────────────┤
 │ /help /list /add /remove /focus /session /clear  Ctrl+] exit to tabs    │
 ╰─────────────────────────────────────────────────────────────────────────╯
```

**Key differences from v1:**
- The header line shows the session name, total agent count, **and which vibespaces**
  are involved (e.g. "5 agents from 2 vibespaces")
- `Ctrl+]` exits to the tab bar (re-enables tab switching keys). The chat state is
  preserved — switching back to Tab 2 resumes exactly where you left off
- Tab switching keys (`1`-`5`) are intercepted by the tab bar, not the chat input.
  While the chat input is focused, they type normally. `Ctrl+1` through `Ctrl+5` always
  work for tab switching.

### 5.2 Entering Chat

Multiple entry points, all resulting in the same Chat tab:

| From | Action | Result |
|------|--------|--------|
| Vibespaces tab | `c` on a vibespace | New session with that vibespace's agents |
| Vibespaces tab | `c` with multiple selected | New session with selected vibespaces |
| Sessions tab | `Enter` on a session | Resume session (may span multiple vibespaces) |
| Sessions tab | `n` | New session → vibespace picker |
| Command palette | `:chat myproject backend-api` | New session with named vibespaces |

**Multi-vibespace session creation flow (from Sessions tab, `n`):**

```
 │  New Session                                                            │
 │                                                                         │
 │  Select vibespaces (Space to toggle, Enter to confirm):                 │
 │                                                                         │
 │  [x] myproject       3 agents   running                                 │
 │  [x] backend-api     2 agents   running                                 │
 │  [ ] ml-pipeline     1 agent    stopped                                 │
 │  [ ] experiment      4 agents   running                                 │
 │                                                                         │
 │  Session name: cross-project                                            │
 │                                                                         │
 │  Space toggle  Enter create session  Esc cancel                         │
```

### 5.3 Chat Integration

The `ChatView` wraps the existing `tui.Model`:

```go
type ChatTab struct {
    inner   *tui.Model       // existing chat model, untouched
    active  bool             // whether this tab is focused
    session *session.Session // session metadata
}

func (t *ChatTab) Update(msg tea.Msg) tea.Cmd {
    // When active, all messages go to inner model
    // The inner model handles everything: input, scrolling, agents, permissions
    _, cmd := t.inner.Update(msg)
    return cmd
}

func (t *ChatTab) View() string {
    return t.inner.View()
}
```

Zero changes to existing chat code. The `ChatTab` is a pass-through.

### 5.4 Chat Keybindings

All existing keybindings from `pkg/tui/update.go` remain unchanged:

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Ctrl+C` | Quit TUI |
| `PgUp` / `PgDn` | Scroll viewport |
| `Home` / `End` | Top / bottom |
| `Tab` | Autocomplete |

Slash commands unchanged: `/help`, `/list`, `/add`, `/remove`, `/focus`, `/clear`,
`/session`, `/ports`, `/quit`, `/scroll`.

**Added:** `Ctrl+]` exits the chat tab and returns focus to the tab bar.

## 6. Tab 3: Monitor

Live dashboard. Uses `ntcharts` for visualization. Auto-refreshes every second.

### 6.1 Layout

```
 ╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
 │                                                       ↻ refreshing 1s  │
 │  Vibespace: myproject ▾                                                 │
 │                                                                         │
 │  Resources                                                              │
 │ ┌──────────┬────────────────────────────────┬─────────────────────┐     │
 │ │ AGENT    │ CPU                            │ MEMORY              │     │
 │ ├──────────┼────────────────────────────────┼─────────────────────┤     │
 │ │ claude-1 │ ██████░░░░ 62% (155m/250m)     │ ████░░░░ 48% (246Mi)│     │
 │ │ claude-2 │ ████░░░░░░ 45% (112m/250m)     │ ███░░░░░ 38% (195Mi)│     │
 │ │ codex-1  │ █████░░░░░ 55% (137m/250m)     │ █████░░░ 61% (312Mi)│     │
 │ └──────────┴────────────────────────────────┴─────────────────────┘     │
 │                                                                         │
 │  Agent Activity                                                         │
 │ ┌──────────┬────────┬──────┬───────┬────────┬────────┬─────────┐       │
 │ │ AGENT    │ UPTIME │ MSGS │ TOOLS │ TOKENS │ ERRORS │ STATE   │       │
 │ ├──────────┼────────┼──────┼───────┼────────┼────────┼─────────┤       │
 │ │ claude-1 │ 2h 14m │ 47   │ 128   │ 142.3k │ 0      │ active  │       │
 │ │ claude-2 │ 2h 14m │ 23   │ 64    │ 89.1k  │ 2      │ idle 2m │       │
 │ │ codex-1  │ 1h 02m │ 31   │ 92    │ ──     │ 0      │ active  │       │
 │ └──────────┴────────┴──────┴───────┴────────┴────────┴─────────┘       │
 │                                                                         │
 │  CPU History (cluster)                                                  │
 │  ▁▂▃▄▅▆▅▄▃▂▃▄▅▆▇▆▅▄▃▄▅▆▅▄▃▂▃▄▅▆▅▄▃▂▁▂▃▄▅▆▅▄▃▂▃▄▅▆▅▄▃▂▁▂▃▄           │
 │                                                                         │
 ├─────────────────────────────────────────────────────────────────────────┤
 │ R refresh  p pause  v vibespace picker  1 resources  2 activity  3 cpu  │
 ╰─────────────────────────────────────────────────────────────────────────╯
```

### 6.2 Components

**Resource table** — `lipgloss/table` with inline bar charts rendered as Unicode
block characters (`█░`). Bar colors: teal when < 60%, orange 60-80%, red > 80%.

**Activity table** — `lipgloss/table` with state column colored (teal for active,
dim for idle, red for error).

**CPU sparkline** — `ntcharts/sparkline` showing 60 data points (last 60 seconds).

```go
sl := sparkline.New(60, 5)
sl.PushAll(cpuHistory)
sl.Draw()
```

**Vibespace selector** — the `▾` dropdown at the top. `v` opens a `lipgloss/list` to
pick which vibespace to monitor. "All" option shows aggregate.

### 6.3 Monitor Sections

| Section | Data Source | Refresh |
|---------|-------------|---------|
| Resources | k8s pod metrics API | 1s |
| Agent Activity | daemon client + session manager | 1s |
| CPU History | k8s metrics, last 60 samples | 1s |
| Recent Events | k8s events + daemon events | 1s |
| Container Health | k8s pod status | 5s |
| Forwards | daemon client | 5s |

Number keys `1`/`2`/`3` toggle section visibility. At small terminal sizes (< 30
rows), only the active section renders. `p` pauses refresh (status shows "paused").

### 6.4 Future: Cost Tracking

When token tracking is implemented, the Monitor tab adds a cost section:

```
 │  Estimated Cost (session)                                               │
 │ ┌──────────┬──────────┬──────────┬──────────┐                           │
 │ │ AGENT    │ INPUT    │ OUTPUT   │ COST     │                           │
 │ ├──────────┼──────────┼──────────┼──────────┤                           │
 │ │ claude-1 │ 89.2k    │ 53.1k    │ $0.47    │                           │
 │ │ claude-2 │ 45.1k    │ 44.0k    │ $0.31    │                           │
 │ │ codex-1  │ ──       │ ──       │ ──       │                           │
 │ │ total    │ 134.3k   │ 97.1k    │ $0.78    │                           │
 │ └──────────┴──────────┴──────────┴──────────┘                           │
```

## 7. Tab 4: Sessions

Browse, resume, create, and delete multi-agent sessions. Each session can span
multiple vibespaces.

### 7.1 Layout

```
 ╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
 │                                                                         │
 │ ┌───────────────────┬────────────────────────┬────────┬────────────────┐ │
 │ │ SESSION           │ VIBESPACES             │ AGENTS │ LAST USED      │ │
 │ ├───────────────────┼────────────────────────┼────────┼────────────────┤ │
 │ │▸cross-project     │ myproject, backend-api │ 5      │ 2m ago         │ │
 │ │ backend-refactor  │ backend-api            │ 2      │ 1h ago         │ │
 │ │ experiment-42     │ experiment             │ 4      │ 3h ago         │ │
 │ │ quick-fix         │ myproject              │ 1      │ 2d ago         │ │
 │ └───────────────────┴────────────────────────┴────────┴────────────────┘ │
 │                                                                         │
 │  Session: cross-project                                                 │
 │  Created    2026-02-10 09:15:00                                         │
 │  Vibespaces                                                             │
 │  ├── myproject                                                          │
 │  │   ├── claude-1 (claude-code)                                         │
 │  │   ├── claude-2 (claude-code)                                         │
 │  │   └── codex-1 (codex)                                                │
 │  └── backend-api                                                        │
 │      ├── claude-1 (claude-code)                                         │
 │      └── claude-2 (claude-code)                                         │
 │                                                                         │
 ├─────────────────────────────────────────────────────────────────────────┤
 │ j/k navigate  Enter resume  n new  d delete  ? help                     │
 ╰─────────────────────────────────────────────────────────────────────────╯
```

The top half is a `lipgloss/table`. The VIBESPACES column shows comma-separated
names when a session spans multiple vibespaces.

The bottom half shows detail for the selected session using `lipgloss/tree`. The tree
makes the multi-vibespace structure clear — you can see exactly which agents from
which vibespaces are in the session.

### 7.2 Session Actions

| Key | Action |
|-----|--------|
| `Enter` | Resume session → switches to Chat tab |
| `n` | New session (vibespace picker, see 5.2) |
| `d` | Delete session (inline confirmation) |
| `/` | Filter sessions |

### 7.3 New Session Flow

Pressing `n` shows the multi-vibespace picker inline:

```
 │  New Session                                                            │
 │                                                                         │
 │  Select vibespaces (Space to toggle):                                   │
 │  [x] myproject       3 agents   running                                 │
 │  [x] backend-api     2 agents   running                                 │
 │  [ ] ml-pipeline     1 agent    stopped                                 │
 │  [ ] experiment      4 agents   running                                 │
 │                                                                         │
 │  Name: ____________                                                     │
 │                                                                         │
 │  Space: toggle  Enter: create  Esc: cancel                              │
```

Multiple vibespaces can be selected. `Enter` creates the session and switches to the
Chat tab. The session name is optional — if blank, a UUID is generated.

## 8. Tab 5: Remote

WireGuard remote mode status and controls.

### 8.1 Connected

```
 ╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
 │                                                                         │
 │  Remote Mode  connected                                                 │
 │                                                                         │
 │  • Server       49.13.120.186                                           │
 │  • Local IP     10.100.0.2                                              │
 │  • Server IP    10.100.0.1                                              │
 │  • Connected    2026-02-12 09:15:00 (3h ago)                            │
 │                                                                         │
 │  Tunnel Health                                                          │
 │ ┌──────────────┬──────────────────────────────────────────────────┐     │
 │ │ Handshake    │ 12s ago                                         │     │
 │ │ TX           │ 142.3 MiB                                       │     │
 │ │ RX           │ 89.7 MiB                                        │     │
 │ │ Packet loss  │ 0.0%                                            │     │
 │ └──────────────┴──────────────────────────────────────────────────┘     │
 │                                                                         │
 │  TX/RX History                                                          │
 │  ▁▂▃▄▅▆▅▄▃▂▃▄▅▆▇▆▅▄▃▄▅▆▅▄▃▂▃▄▅▆▅▄▃▂▁▂▃▄▅▆▅▄▃▂▃▄▅▆▅▄▃▂▁▂▃▄           │
 │                                                                         │
 ├─────────────────────────────────────────────────────────────────────────┤
 │ D disconnect  w watch mode                                              │
 ╰─────────────────────────────────────────────────────────────────────────╯
```

`lipgloss/list` for the connection details. `lipgloss/table` for tunnel health.
`ntcharts/sparkline` for TX/RX history.

### 8.2 Disconnected

```
 │  Remote Mode  disconnected                                              │
 │                                                                         │
 │  Paste an invite token to connect:                                      │
 │  > vs-eyJrIjoiYWJjMTI...                                               │
 │                                                                         │
 │  Enter: connect  Esc: cancel                                            │
```

A text input appears for the token. Enter connects. The connection process shows
a spinner (`bubbles/spinner`).

### 8.3 Server Mode

When running as a server (`vibespace serve`), the remote tab shows server info:

```
 │  Remote Mode  serving                                                   │
 │                                                                         │
 │  • Endpoint     49.13.120.186:51820                                     │
 │  • Uptime       3d 14h                                                  │
 │  • Clients      2                                                       │
 │                                                                         │
 │  Clients                                                                │
 │ ┌──────────────┬────────────────┬──────────────┬───────────────────┐    │
 │ │ NAME         │ IP             │ LAST SEEN    │ TX/RX             │    │
 │ ├──────────────┼────────────────┼──────────────┼───────────────────┤    │
 │ │ yagiz-mbp    │ 10.100.0.2     │ 12s ago      │ 142.3/89.7 MiB   │    │
 │ │ ci-runner    │ 10.100.0.3     │ 5m ago       │ 23.1/12.4 MiB    │    │
 │ └──────────────┴────────────────┴──────────────┴───────────────────┘    │
 │                                                                         │
 │  g generate token  r remove client                                      │
```

## 9. Overlays

Two overlays render on top of any tab. They are not tabs themselves.

### 9.1 Help Overlay (`?`)

```
 ╭───────────────────────── Help ─────────────────────────╮
 │                                                         │
 │  Global                                                 │
 │  1-5       switch tab        ?       toggle help        │
 │  Tab       next tab          :       command palette    │
 │  Ctrl+C    quit              Esc     close/cancel       │
 │                                                         │
 │  Navigation (lists & tables)                            │
 │  j / ↓     down              k / ↑   up                 │
 │  g         top               G       bottom             │
 │  Enter     expand/select     /       search/filter      │
 │  Space     toggle checkbox                              │
 │                                                         │
 │  Vibespaces Tab                                         │
 │  x         connect (ssh)     b       connect (browser)   │
 │  n         new vibespace     d       delete              │
 │  c         chat              S       start/stop          │
 │  e         edit config       f       forwards            │
 │  a         add agent                                     │
 │                                                         │
 │  Chat Tab                                               │
 │  Ctrl+]    exit to tab bar   Tab     autocomplete       │
 │  PgUp/Dn  scroll            /cmd    slash commands      │
 │                                                         │
 │  Monitor Tab                                            │
 │  R         refresh           p       pause               │
 │  v         vibespace picker  1/2/3   toggle sections    │
 │                                                         │
 │                                     ? or Esc to close   │
 ╰─────────────────────────────────────────────────────────╯
```

Rendered as a centered `lipgloss` box with rounded border in orange. The overlay
uses `lipgloss.Place()` to center on screen.

### 9.2 Command Palette (`:`)

```
 ╭──────────────────────────────────────────────────────╮
 │ : chat myproject                                      │
 │                                                       │
 │ ▸ Chat with myproject (3 agents)              c       │
 │   Chat with backend-api (2 agents)            c       │
 │   New session                                 n       │
 │   Monitor myproject                           m       │
 │   Create vibespace                            n       │
 │   Remote status                               r       │
 │                                                       │
 ╰──────────────────────────────────────────────────────╯
```

Fuzzy-filtered list of all actions. Uses `bubbles/textinput` for the search input.
Each result row is a `bubblezone.Mark()` region for click support.

### 9.3 Permission Prompt

The existing `PermissionPrompt` overlay from `pkg/tui/permission_prompt.go` is
unchanged. It renders centered on top of whatever tab is active.

## 10. Architecture

### 10.1 Root Model

```go
type App struct {
    tabs       []Tab
    activeTab  int
    width      int
    height     int
    zone       *zone.Manager      // bubblezone manager
    shared     *SharedState        // cluster, daemon, vibespace service
    help       *HelpOverlay        // nil when hidden
    palette    *PaletteOverlay     // nil when hidden
    spring     harmonica.Spring    // for tab transition animation
}

type Tab interface {
    tea.Model
    Title() string
    ShortHelp() []key.Binding
}
```

### 10.2 Tab Implementations

```
pkg/tui/
    app.go                # App model, tab bar, overlays, global keys
    tab_vibespaces.go     # VibespaceTab: table, expansion, forms
    tab_chat.go           # ChatTab: wraps existing tui.Model
    tab_monitor.go        # MonitorTab: charts, tables, polling
    tab_sessions.go       # SessionsTab: table, tree, picker
    tab_remote.go         # RemoteTab: status, connect/disconnect
    overlay_help.go       # Help overlay
    overlay_palette.go    # Command palette

    # Existing files (unchanged)
    model.go              # Existing chat model
    view.go               # Existing chat rendering
    update.go             # Existing chat event handling
    styles.go             # Existing chat styles
    permission_prompt.go  # Permission overlay
    input.go              # Input parsing
    agent.go              # SSH agent connections
    session_manager.go    # Agent session management
    headless.go           # Non-interactive mode
```

### 10.3 Shared State

```go
type SharedState struct {
    Cluster     *ClusterState           // cached cluster/daemon/remote status
    Daemon      *daemon.Client          // daemon client (port forwards, pod info)
    Vibespace   *vibespace.Service      // k8s CRUD
    Sessions    *session.Store          // session persistence
    Vibespaces  []model.VibespaceInfo   // cached vibespace list
}
```

Refreshed on tab activation. The Vibespaces tab refreshes on focus. The Monitor tab
refreshes on its own tick. The Sessions tab refreshes when you switch to it.

### 10.4 BubbleZone Integration

```go
func (a *App) Init() tea.Cmd {
    zone.NewGlobal()
    // ...
}

func (a *App) View() string {
    // tab bar with zone markers
    var tabs []string
    for i, t := range a.tabs {
        id := fmt.Sprintf("tab-%d", i)
        style := inactiveTabStyle
        if i == a.activeTab {
            style = activeTabStyle
        }
        tabs = append(tabs, zone.Mark(id, style.Render(t.Title())))
    }
    tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

    // active tab content
    content := a.tabs[a.activeTab].View()

    // assemble
    result := lipgloss.JoinVertical(lipgloss.Left, tabBar, content, statusBar)

    // overlays
    if a.help != nil {
        result = a.help.Render(result, a.width, a.height)
    }
    if a.palette != nil {
        result = a.palette.Render(result, a.width, a.height)
    }

    return zone.Scan(result)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.MouseMsg:
        // check tab clicks
        for i := range a.tabs {
            if zone.Get(fmt.Sprintf("tab-%d", i)).InBounds(msg) {
                a.activeTab = i
                return a, nil
            }
        }
    case tea.KeyMsg:
        // global keys handled before delegating to active tab
    }
    // delegate to active tab
    return a, a.tabs[a.activeTab].Update(msg)
}
```

### 10.5 Tab Transition Animation

When switching tabs, use `harmonica` to animate the tab highlight position:

```go
spring := harmonica.NewSpring(harmonica.FPS(60), 8.0, 0.7)

// On tab switch:
a.tabHighlightX, a.tabHighlightVelocity = spring.Update(
    a.tabHighlightX,
    a.tabHighlightVelocity,
    targetTabX,
)
```

This creates a subtle springy slide for the active tab indicator. The animation
runs at 60fps via `tea.Tick`. Keep it subtle — the underline or highlight color
smoothly slides to the new tab position over ~200ms.

### 10.6 ntcharts Integration

Monitor tab uses ntcharts for sparklines and bar charts:

```go
// CPU sparkline
sl := sparkline.New(termWidth-4, 5)
for _, sample := range cpuSamples {
    sl.Push(sample)
}
sl.Draw()
cpuView := sl.View()

// Per-agent bar chart (horizontal)
bc := barchart.New(termWidth-4, len(agents)*2)
for _, agent := range agents {
    bc.Push(barchart.BarData{
        Label: agent.Name,
        Values: []barchart.BarValue{
            {Label: "CPU", Value: agent.CPUPercent, Style: cpuStyle},
        },
    })
}
bc.Draw()
barView := bc.View()
```

## 11. Keybinding Reference

### 11.1 Global (all tabs)

| Key | Action |
|-----|--------|
| `1`-`5` | Switch to tab (except when typing in Chat) |
| `Ctrl+1`-`Ctrl+5` | Switch to tab (always works) |
| `Tab` | Next tab |
| `Shift+Tab` | Previous tab |
| `?` | Toggle help overlay |
| `:` | Open command palette |
| `Ctrl+C` | Quit |

### 11.2 Vibespaces Tab

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate rows |
| `g` / `G` | Top / bottom |
| `Enter` | Toggle expansion |
| `x` | Connect — shell (collapsed) or agent CLI (expanded) |
| `b` | Connect in browser (ttyd) |
| `n` | New vibespace |
| `d` | Delete vibespace |
| `c` | Chat with vibespace |
| `S` | Start/stop |
| `e` | Edit agent config (when expanded) |
| `a` | Add agent |
| `f` | Manage forwards |
| `/` | Search/filter |
| `Esc` | Collapse / cancel |

### 11.3 Chat Tab

Same as existing `pkg/tui/update.go` plus:

| Key | Action |
|-----|--------|
| `Ctrl+]` | Exit to tab bar |

### 11.4 Monitor Tab

| Key | Action |
|-----|--------|
| `R` | Force refresh |
| `p` | Pause/resume |
| `v` | Vibespace picker |
| `1` / `2` / `3` | Toggle sections |

### 11.5 Sessions Tab

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate |
| `Enter` | Resume session → Chat tab |
| `n` | New session |
| `d` | Delete session |
| `/` | Search/filter |

### 11.6 Remote Tab

| Key | Action |
|-----|--------|
| `c` | Connect (paste token) |
| `D` | Disconnect |
| `w` | Watch mode |
| `g` | Generate token (server mode) |
| `r` | Remove client (server mode) |

## 12. Color & Style Guide

### 12.1 Brand Palette

| Color | Hex | Usage |
|-------|-----|-------|
| Teal | `#00ABAB` | Active tab, running, success, primary actions |
| Pink | `#F102F3` | Thinking indicator, branding accent |
| Orange | `#FF7D4B` | Warnings, help overlay border, highlighted values |
| Yellow | `#F5F50A` | Stopped status, attention items |

### 12.2 Semantic Colors

| Purpose | Color | Hex |
|---------|-------|-----|
| Running / Active | Teal | `#00ABAB` |
| Error / Delete | Red | `#FF4D4D` |
| Warning / Degraded | Orange | `#FF7D4B` |
| Stopped | Yellow | `#F5F50A` |
| Dim text / Secondary | Gray | `#666666` |
| Borders / Muted | Dark gray | `#444444` |
| Primary text | White | `#FFFFFF` |

### 12.3 Table Styles

All tables use `lipgloss/table` with consistent styling:

```go
var (
    tableBorder = lipgloss.RoundedBorder()
    borderStyle = lipgloss.NewStyle().Foreground(ui.ColorMuted)
    headerStyle = lipgloss.NewStyle().Bold(true).Foreground(ui.Teal)
    cellStyle   = lipgloss.NewStyle().Padding(0, 1)
    selectedRow = cellStyle.Foreground(ui.ColorWhite).Bold(true)
)
```

### 12.4 Tab Bar Styles

```go
activeTab = lipgloss.NewStyle().
    Bold(true).
    Foreground(ui.Teal).
    Border(lipgloss.NormalBorder(), false, false, true, false).
    BorderForeground(ui.Teal).
    Padding(0, 2)

inactiveTab = lipgloss.NewStyle().
    Foreground(ui.ColorDim).
    Border(lipgloss.NormalBorder(), false, false, true, false).
    BorderForeground(ui.ColorMuted).
    Padding(0, 2)
```

### 12.5 Resource Bar Colors

| Range | Color | Hex |
|-------|-------|-----|
| 0-59% | Teal | `#00ABAB` |
| 60-79% | Orange | `#FF7D4B` |
| 80-100% | Red | `#FF4D4D` |

### 12.6 Agent Colors

Agents cycle through the existing palette for identification in chat:

| Index | Color | Hex |
|-------|-------|-----|
| 0 | Teal | `#00ABAB` |
| 1 | Pink | `#F102F3` |
| 2 | Orange | `#FF7D4B` |
| 3 | Yellow | `#F5F50A` |
| 4 | Cyan | `#00D9FF` |
| 5 | Purple | `#7B61FF` |
| 6 | Green | `#00FF9F` |
| 7 | Coral | `#FF6B6B` |

### 12.7 Responsive Behavior

| Terminal Width | Behavior |
|----------------|----------|
| < 80 | Tab titles shorten to icons/numbers. Tables drop low-priority columns. |
| 80-120 | Standard layout. Most columns visible. |
| > 120 | Full columns, wider tables, more sparkline data points. |

| Terminal Height | Behavior |
|-----------------|----------|
| < 24 | Monitor shows only active section. Tables reduce visible rows. |
| 24-40 | Standard layout. |
| > 40 | Monitor shows all sections. More table rows. Expanded session detail. |

Tables never require horizontal scrolling. Long values truncate with `...`.

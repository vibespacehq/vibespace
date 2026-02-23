# TUI Reference

The tab-based terminal UI launched by `vibespace` with no arguments.

## Overview

Five permanent tabs across the top. Switch with number keys, `Tab`/`Shift+Tab`,
`Ctrl+1`-`5`, or mouse click. Overlays (help, command palette) render on top of
any tab.

Sessions span vibespaces — a chat session can include agents from multiple
vibespaces simultaneously.

## Dependencies

| Library | Import | Usage |
|---------|--------|-------|
| bubbletea | `tea "github.com/charmbracelet/bubbletea"` | Application framework |
| bubbles/textinput | `"github.com/charmbracelet/bubbles/textinput"` | Text fields (chat, forms, palette, token input) |
| bubbles/key | `"github.com/charmbracelet/bubbles/key"` | Status bar key bindings |
| lipgloss | `"github.com/charmbracelet/lipgloss"` | Styling, layout, centering |
| lipgloss/table | `"github.com/charmbracelet/lipgloss/table"` | Data tables (vibespaces, sessions, remote, monitor) |
| bubblezone | `zone "github.com/lrstanley/bubblezone"` | Mouse zone scanning |
| ntcharts | `"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"` | CPU/memory history charts (monitor tab) |
| harmonica | `"github.com/charmbracelet/harmonica"` | Spring physics for tab transition animation |
| go-colorful | `"github.com/lucasb-eyer/go-colorful"` | OKLab gradient interpolation for tab bar |

**Not used** (mentioned in original design but not implemented): `lipgloss/list`,
`lipgloss/tree`, `zone.Mark()` click regions.

## File Layout

```
pkg/tui/
    app.go                # App model, tab bar, overlays, global keys
    app_styles.go         # Border, tab bar, overlay shared styles
    tab.go                # Tab interface, indices, inter-tab messages
    tab_vibespaces.go     # Vibespaces: table, agent view, sessions, forms
    tab_chat.go           # Chat: wraps existing tui.Model
    tab_monitor.go        # Monitor: resource tables, charts, polling
    tab_sessions.go       # Sessions: list, create flow, delete
    tab_remote.go         # Remote: connect/disconnect, diagnostics, serve
    overlay_help.go       # Help keybinding reference
    overlay_palette.go    # Command palette with fuzzy search
    shared.go             # SharedState (cluster, daemon, vibespace service)

    # Chat engine (pre-existing)
    model.go              # Chat model
    view.go               # Chat rendering
    update.go             # Chat event handling
    styles.go             # Chat styles
    input.go              # Input parsing
    history_store.go      # Chat history persistence
```

## Architecture

### Root Model

```go
type App struct {
    tabs      []Tab           // 5 tabs
    activeTab int
    width     int
    height    int
    shared    *SharedState
    help      *HelpOverlay
    palette   *PaletteOverlay
    spring    harmonica.Spring // tab transition animation
}

type Tab interface {
    tea.Model
    Title() string
    ShortHelp() []key.Binding
    SetSize(width, height int)
}
```

### Update Priority

1. Window resize → propagate to overlays and active tab
2. Spring animation ticks → update tab highlight position
3. Inter-tab messages (`SwitchTabMsg`, `SwitchToChatMsg`, palette messages)
4. Key messages → overlays intercept first when visible, then global keys, then active tab
5. All other messages → delegate to active tab

### Shared State

```go
type SharedState struct {
    Cluster    *ClusterState
    Daemon     *daemon.Client
    Vibespace  *vibespace.Service
    Sessions   *session.Store
}
```

Refreshed on tab activation. Monitor tab refreshes on its own 1-second tick.

### Tab Transition Animation

Tab switching animates the highlight underline using `harmonica.Spring` at 60fps.
The gradient slides from the old tab position to the new one via `tea.Tick`.

## Tab Layout

```
╭─ 1 Vibespaces ─ 2 Chat ─ 3 Monitor ─ 4 Sessions ─ 5 Remote ─╮
│                                                                │
│                     (active tab content)                       │
│                                                                │
├────────────────────────────────────────────────────────────────┤
│ status bar (context-dependent keybinding hints)                │
╰────────────────────────────────────────────────────────────────╯
```

Active tab label has a teal→pink gradient (bold). Inactive tabs are dim.
The underline row uses the same gradient under the active tab.

## Tab 1: Vibespaces

Three-level stack navigation: list → agent view → session list.

### List Mode (default)

Column-aligned table of vibespaces with status coloring. Selected row
rendered with teal→pink gradient. Right panel shows recent logs for the
selected vibespace.

### Agent View (Enter on a vibespace)

Full-screen replacement. Top: agent table (Name, Type, Status). Bottom:
detail panel for selected agent showing configuration, resource limits,
and active port forwards.

Actions available: add agent (`a`), edit config (`e`), forward manager (`f`),
connect (`x`), session list (`Enter`).

### Session List (Enter on an agent)

Lists agent sessions parsed from SSH history (`~/.claude/history.jsonl` for
claude-code, `~/.codex/sessions/` for codex). Enter resumes via
`tea.ExecProcess` — TUI suspends, agent CLI opens, TUI resumes on exit.

### Inline Forms

- **Create vibespace** (`n`): Sequential fields — name, agent type (j/k selector),
  CPU, memory, storage. `Ctrl+S` submits.
- **Delete vibespace** (`d`): Inline confirmation prompt.
- **Add agent** (`a` in agent view): Name, type, model, max turns, share credentials,
  skip permissions, allowed/disallowed tools (multi-select with j/k + space).
- **Edit config** (`e` in agent view): Model, max turns, skip permissions toggle,
  allowed/disallowed tools multi-select.
- **Forward manager** (`f` in agent view): List active forwards with add (`a`),
  remove (`d`), refresh (`r`). Add form: remote port, local port, DNS toggle,
  DNS name.

## Tab 2: Chat

Wraps the existing `pkg/tui/model.go` chat engine as a pass-through tab.
The ~5,000-line chat model runs unchanged. `ChatTab` delegates all messages
to the inner model when a session is loaded.

Entry points: resume from Sessions tab, create from Vibespaces tab (`c` key
is not yet wired), or start with `vibespace chat`.

## Tab 3: Monitor

Live resource dashboard with 1-second auto-refresh.

### Sections

1. **Vibespace picker** — Filter by specific vibespace or "all"
2. **Node resources** — CPU/memory table (only in "all" view)
3. **Agent resources** — Per-agent CPU/memory with Unicode bar charts (`█░`),
   colored by utilization (teal <60%, orange 60-80%, red >80%)
4. **Totals** — Aggregate CPU/memory (single-vibespace view)
5. **CPU history** — `ntcharts` streaming line chart (visible when height ≥ 30)
6. **Memory history** — `ntcharts` streaming line chart (visible when height > 40)

### Controls

`R` force refresh, `p` pause/resume, `v` vibespace picker.

## Tab 4: Sessions

Browse, resume, create, and delete multi-agent sessions.

### List View

Table of sessions showing name, vibespaces, agent count, last used. Right
panel shows chat preview (last N messages) for the selected session.

### New Session Flow (three steps)

1. **Name input** — Text field for session name
2. **Vibespace picker** — Toggle vibespaces with `space`/`x`, confirm with `Enter`
3. **Agent picker** — Per-vibespace agent selection, repeated for each selected vibespace

Creates the session and switches to Chat tab.

## Tab 5: Remote

WireGuard remote mode status and controls.

### Three States

- **Disconnected**: Shows token input prompt. `c` to start, paste token, `Enter` to connect.
- **Connected**: Shows server IP, local IP, connection time, tunnel health (handshake,
  TX/RX, endpoint). `D` to disconnect (with confirmation). `R` to refresh.
  Diagnostics section with ping, DNS, API checks (runs with sudo via inline password prompt).
- **Serving**: Shows endpoint, uptime, client count, client table. `g` to generate token.
  `R` to refresh.

## Overlays

### Help (`?`)

Centered box with orange rounded border. Shows keybindings grouped by
Global, Vibespaces, Chat, Sessions. `?` or `Esc` to close.

### Command Palette (`:`)

Fuzzy-filtered action list using `bubbles/textinput`. Multi-word
case-insensitive search. `up`/`down` or `Ctrl+P`/`Ctrl+N` to navigate,
`Enter` to execute, `Esc` to close.

**Actions:**
- Go to Vibespaces/Chat/Monitor/Sessions/Remote (tab switching)
- New vibespace (switches tab + enters create form)
- New session (switches tab + enters name input)
- Toggle help
- Quit

## Keybinding Reference

### Global

| Key | Action |
|-----|--------|
| `1`-`5` | Switch tab (except when typing) |
| `Ctrl+1`-`Ctrl+5` | Switch tab (always) |
| `Tab` / `Shift+Tab` | Next / previous tab |
| `?` | Toggle help overlay |
| `:` | Toggle command palette |
| `Ctrl+C` | Quit |

### Vibespaces Tab

| Key | List | Agent View | Session List |
|-----|------|------------|--------------|
| `j`/`k`/`up`/`down` | Navigate vibespaces | Navigate agents | Navigate sessions |
| `g` / `G` | First / last | First / last | First / last |
| `Enter` | Agent view | Session list | Resume session |
| `x` | SSH shell (primary) | SSH + agent CLI | — |
| `b` | Browser (ttyd) | — | — |
| `n` | New vibespace | — | — |
| `d` | Delete vibespace | — | — |
| `S` | Start/stop | — | — |
| `a` | — | Add agent | — |
| `e` | — | Edit config | — |
| `f` | — | Forward manager | — |
| `Esc` | — | Back to list | Back to agents |

### Forward Manager

| Key | List | Add Form |
|-----|------|----------|
| `j`/`k` | Navigate | — |
| `a` | Add forward | — |
| `d` | Delete forward | — |
| `r` | Refresh | — |
| `Enter`/`Tab` | — | Next field |
| `Space` | — | Toggle DNS |
| `Ctrl+S` | — | Submit |
| `Esc` | Exit manager | Cancel add |

### Chat Tab

All existing keybindings from `pkg/tui/update.go`:

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `PgUp` / `PgDn` | Scroll viewport |
| `Home` / `End` | Top / bottom |
| `Tab` | Autocomplete |
| `Ctrl+]` | Exit to tab bar |

Slash commands: `/help`, `/list`, `/add`, `/remove`, `/focus`, `/clear`,
`/session`, `/ports`, `/quit`, `/scroll`.

### Monitor Tab

| Key | Action |
|-----|--------|
| `R` | Force refresh |
| `p` | Pause/resume |
| `v` | Vibespace picker |

Picker mode: `j`/`k` navigate, `Enter` select, `Esc` close.

### Sessions Tab

| Key | List | Delete Confirm | New Session |
|-----|------|----------------|-------------|
| `j`/`k` | Navigate | — | Navigate picker |
| `g`/`G` | First/last | — | — |
| `Enter` | Resume session | — | Submit/confirm |
| `n` | New session | — | — |
| `d` | Delete | — | — |
| `x`/`Space` | — | — | Toggle selection |
| `y` | — | Confirm | — |
| `Esc` | — | Cancel | Cancel |

### Remote Tab

| Key | Disconnected | Connected | Serving |
|-----|-------------|-----------|---------|
| `c` | Token input | — | — |
| `D` | — | Disconnect | — |
| `R` | — | Refresh | Refresh |
| `g` | — | — | Generate token |
| `Enter` | Submit token | — | — |
| `Esc` | Cancel input | — | — |

## Color Palette

### Brand Colors

| Color | Hex | Usage |
|-------|-----|-------|
| Teal | `#00ABAB` | Active tab, running status, success |
| Pink | `#F102F3` | Thinking indicator, gradient accent |
| Orange | `#FF7D4B` | Overlay borders, warnings |
| Yellow | `#F5F50A` | Stopped status |

### Semantic Colors

| Purpose | Color | Hex |
|---------|-------|-----|
| Running / Active | Teal | `#00ABAB` |
| Error / Delete | Red | `#FF4D4D` |
| Warning | Orange | `#FF7D4B` |
| Stopped | Yellow | `#F5F50A` |
| Dim text | Gray | `#666666` |
| Borders | Dark gray | `#444444` |
| Primary text | White | `#FFFFFF` |

### Resource Bar Colors

| Range | Color |
|-------|-------|
| 0-59% | Teal |
| 60-79% | Orange |
| 80-100% | Red |

## Responsive Behavior

| Terminal Width | Behavior |
|----------------|----------|
| < 80 | Tables drop low-priority columns |
| 80-120 | Standard layout |
| > 120 | Full columns, wider tables |

| Terminal Height | Behavior |
|-----------------|----------|
| < 30 | Monitor hides charts |
| 30-40 | CPU chart visible |
| > 40 | CPU + memory charts visible |

## Not Implemented

Features from the original design that are not yet built:

- `zone.Mark()` click regions on table rows
- `lipgloss/list` and `lipgloss/tree` rendering
- Monitor activity table (uptime, messages, tools, tokens, errors, state)
- Monitor cost tracking section
- Remote watch mode (`w` key)
- Sessions `/` search/filter
- Agent color cycling in chat (indexed palette)

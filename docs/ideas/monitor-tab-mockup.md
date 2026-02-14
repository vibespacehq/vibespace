# Monitor Tab Mockups

Based on real cluster data (5 agent pods across 2 vibespaces).

## All Vibespaces View (default)

```
╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
│                                                       ↻ refreshing 5s  │
│  Vibespace: all ▾                                                       │
│                                                                         │
│  Node Resources                                                         │
│ ┌──────────────────┬────────────────────────────────┬──────────────────┐│
│ │ NODE             │ CPU                            │ MEMORY           ││
│ ├──────────────────┼────────────────────────────────┼──────────────────┤│
│ │ colima-vibespace │ █░░░░░░░░░  7% (295m/4000m)   │ █░░░░░░░ 13%    ││
│ │                  │                                │ (1051Mi/8192Mi) ││
│ └──────────────────┴────────────────────────────────┴──────────────────┘│
│                                                                         │
│  Agent Resources                                                        │
│ ┌──────────┬──────┬──────────────────────────────┬─────────────────────┐│
│ │ AGENT    │ VS   │ CPU                          │ MEMORY              ││
│ ├──────────┼──────┼──────────────────────────────┼─────────────────────┤│
│ │ claude-1 │ test │ ░░░░░░░░░░  0% (1m/1000m)   │ ███░░░░ 30% (153Mi)││
│ │ codex-1  │ test │ ░░░░░░░░░░  0% (1m/1000m)   │ ██░░░░░ 25% (129Mi)││
│ │ yyyy     │ test │ ░░░░░░░░░░  0% (1m/1000m)   │ ██░░░░░ 27% (141Mi)││
│ │ claude-1 │test9 │ ░░░░░░░░░░  0% (1m/1000m)   │ ██░░░░░ 27% (141Mi)││
│ │ claude-2 │test9 │ ░░░░░░░░░░  0% (1m/1000m)   │ ██░░░░░ 27% (141Mi)││
│ └──────────┴──────┴──────────────────────────────┴─────────────────────┘│
│                                                                         │
│  CPU History (cluster)                                                  │
│  ▁▁▁▁▁▁▁▁▂▁▁▁▂▁▁▁▁▁▁▂▁▁▁▁▁▂▁▁▁▁▁▁▂▁▁▁▂▁▁▁▁▁▁▂▁▁▁▁▁▂▁▁▁▁           │
│                                                              now  7%    │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│ R refresh  p pause  v vibespace picker                          ? help  │
╰─────────────────────────────────────────────────────────────────────────╯
```

## Single Vibespace View (after pressing `v` and selecting "test")

```
╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
│                                                       ↻ refreshing 5s  │
│  Vibespace: test ▾                                                      │
│                                                                         │
│  Agent Resources                                                        │
│ ┌──────────┬──────────────────────────────────┬────────────────────────┐│
│ │ AGENT    │ CPU                              │ MEMORY                 ││
│ ├──────────┼──────────────────────────────────┼────────────────────────┤│
│ │ claude-1 │ ░░░░░░░░░░  0% (1m/1000m)       │ █████░░░░░ 30% (153Mi)││
│ │ codex-1  │ ░░░░░░░░░░  0% (1m/1000m)       │ ████░░░░░░ 25% (129Mi)││
│ │ yyyy     │ ░░░░░░░░░░  0% (1m/1000m)       │ ████░░░░░░ 27% (141Mi)││
│ └──────────┴──────────────────────────────────┴────────────────────────┘│
│                                                                         │
│  Totals: 3 agents  3m CPU  423Mi memory                                 │
│                                                                         │
│  CPU History                                                            │
│  ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁           │
│                                                                         │
│  Memory History                                                         │
│  ████████████████████████████████████████████████████████████████        │
│                                                              now 25%    │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│ R refresh  p pause  v vibespace picker                          ? help  │
╰─────────────────────────────────────────────────────────────────────────╯
```

## Under Load (simulated — agents actively coding)

```
╭─ 1 Vibespaces ─┬─ 2 Chat ─┬─ 3 Monitor ─┬─ 4 Sessions ─┬─ 5 Remote ─╮
│                                                       ↻ refreshing 5s  │
│  Vibespace: all ▾                                                       │
│                                                                         │
│  Node Resources                                                         │
│ ┌──────────────────┬────────────────────────────────┬──────────────────┐│
│ │ NODE             │ CPU                            │ MEMORY           ││
│ ├──────────────────┼────────────────────────────────┼──────────────────┤│
│ │ colima-vibespace │ ███████░░░ 68% (2720m/4000m)  │ █████░░░ 62%    ││
│ │                  │                                │ (5079Mi/8192Mi) ││
│ └──────────────────┴────────────────────────────────┴──────────────────┘│
│                                                                         │
│  Agent Resources                                                        │
│ ┌──────────┬──────┬──────────────────────────────┬─────────────────────┐│
│ │ AGENT    │ VS   │ CPU                          │ MEMORY              ││
│ ├──────────┼──────┼──────────────────────────────┼─────────────────────┤│
│ │ claude-1 │ test │ ████████░░ 78% (780m/1000m)  │ ███████░ 82% (840Mi)│
│ │ codex-1  │ test │ ██████░░░░ 62% (620m/1000m)  │ █████░░░ 55% (564Mi)│
│ │ yyyy     │ test │ ░░░░░░░░░░  2% (20m/1000m)   │ ██░░░░░░ 27% (141Mi)│
│ │ claude-1 │test9 │ █████████░ 85% (850m/1000m)  │ ████████ 91% (932Mi)│
│ │ claude-2 │test9 │ ███████░░░ 72% (720m/1000m)  │ ██████░░ 65% (665Mi)│
│ └──────────┴──────┴──────────────────────────────┴─────────────────────┘│
│                                                                         │
│  CPU History (cluster)                                                  │
│  ▁▂▃▄▅▆▅▄▃▂▃▄▅▆▇▆▅▄▃▄▅▆▅▄▃▂▃▄▅▆▅▇▇▇▆▆▇▇▆▇▆▇▇▆▇▇▆▆▇▇▆▇▇▇           │
│                                                              now 68%    │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│ R refresh  p pause  v vibespace picker                          ? help  │
╰─────────────────────────────────────────────────────────────────────────╯
```

## Paused State

```
│                                                       ⏸ paused          │
│  Vibespace: all ▾                                                       │
```

## Metrics Unavailable State

```
│                                                                         │
│  ⚠ Metrics server not available                                         │
│                                                                         │
│  The Kubernetes metrics API is not responding.                           │
│  This usually resolves within 60 seconds after cluster startup.         │
│                                                                         │
│  Retrying in 10s...                                                     │
│                                                                         │
```

## Bar Color Legend

- `█` teal (#00ABAB): usage < 60%
- `█` orange (#FF7D4B): usage 60-79%
- `█` red (#FF4D4D): usage >= 80%
- `░` dark gray (#444444): unused capacity

## Keybindings

| Key | Action |
|-----|--------|
| `R` | Force refresh now |
| `p` | Pause / resume auto-refresh |
| `v` | Open vibespace picker |

## Responsive Behavior

| Width | Behavior |
|-------|----------|
| < 80  | VS column hidden, shorter bars |
| 80-120 | Standard layout |
| > 120 | Wider bars, more sparkline points |

| Height | Behavior |
|--------|----------|
| < 30 | Sparklines hidden |
| 30-40 | Standard layout |
| > 40 | Show both CPU and Memory sparklines |

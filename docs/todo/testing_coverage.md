# CLI Testing Coverage — Output Modes

Audit of which CLI commands support each output mode and whether tests exist for them.

## Legend

- **Supports**: The command has code handling this output mode
- **Tested (E2E)**: The E2E test suite exercises this mode
- **Tested (Unit)**: Unit tests exist for this mode

"N/A" means the command doesn't support that mode (no `IsPlainMode()` / `IsJSONMode()` check in code). These are action commands whose default output is spinner + `printSuccess()` messages — plain mode would just suppress the spinner but produce the same text.

---

## Commands with Table/List Output

These commands render structured data (tables or key-value lists) and support all 3 modes.

| Command | JSON | Plain | Default |
|---------|------|-------|---------|
| `list` | supported, **E2E** | supported, **E2E** | supported, no test |
| `<name> info` | supported, **E2E** | supported, **E2E** | supported, no test |
| `<name> agent list` | supported, **E2E** | supported, **E2E** | supported, no test |
| `<name> config show` | supported, **E2E** | supported, **E2E** | supported, no test |
| `<name> config show <agent>` | supported, **E2E** | supported, **E2E** | supported, no test |
| `<name> forward list` | supported, **E2E** | supported, **E2E** | supported, no test |
| `<name> ports` | supported, **E2E** | supported, **E2E** | supported, no test |
| `multi --list-sessions` | supported, **E2E** | supported, **E2E** | supported, no test |
| `multi --list-agents` | supported, **E2E** | supported, **E2E** | supported, no test |
| `session list` | supported, **E2E** | supported, **E2E** | supported, no test |

## Action Commands (Spinner + Success Message)

These commands perform an action and output a success/failure message. Their default output is spinner + `printSuccess()`. They support `--json` but do **not** implement `--plain` (no `IsPlainMode()` check) — plain mode falls through to the same default output minus the spinner.

| Command | JSON | Plain | Default |
|---------|------|-------|---------|
| `create` | supported, **E2E** | N/A | supported, no test |
| `delete` | supported, **E2E** | N/A | supported, no test |
| `<name> agent create` | supported, **E2E** | N/A | supported, no test |
| `<name> agent delete` | supported, **E2E** | N/A | supported, no test |
| `<name> start` | supported, **E2E** | N/A | supported, no test |
| `<name> stop` | supported, **E2E** | N/A | supported, no test |
| `<name> config set` | supported, **E2E** | N/A | supported, no test |
| `<name> forward add` | supported, **E2E** | N/A | supported, no test |
| `<name> forward remove` | supported, **E2E** | N/A | supported, no test |

## Special Commands

| Command | JSON | Plain | Default |
|---------|------|-------|---------|
| `status` | supported, **E2E** | N/A | supported, no test |
| `init` | supported, no test | N/A | supported, no test |
| `stop` (cluster) | supported, no test | N/A | supported, no test |
| `uninstall` | N/A | N/A | supported, no test |
| `version` | supported, no test | N/A | supported, no test |
| `<name> exec` | supported, **E2E** | N/A (default prints raw stdout/stderr) | supported, no test |
| `<name> connect` | N/A (interactive) | N/A | interactive terminal |
| `multi` (interactive) | N/A (launches TUI) | N/A | launches TUI |
| `multi` (non-interactive) | supported, **E2E** | supported, no test | N/A (requires --json or --plain) |
| `serve` | supported, no test | N/A | supported, no test |
| `remote connect` | supported, no test | N/A | supported, no test |
| `remote disconnect` | supported, no test | N/A | supported, no test |
| `remote status` | supported, no test | N/A | supported, no test |

## Unit Test Coverage (Table Infrastructure)

The table rendering mechanism is unit-tested in `pkg/ui/table_test.go`:

| Test | What it covers |
|------|---------------|
| `TestNewTableColumnAligned` | Headers + data present, column-aligned output |
| `TestTableColumnsAligned` | Second column starts at same offset on all lines |
| `TestTableWithAnsiColorsInCells` | ANSI-colored cells don't break alignment |
| `TestStripAnsi` | Regex correctly strips SGR codes |
| `TestNewTableEmptyRows` | Header-only output for empty data |
| `TestNewTableNoBoxCharacters` | No box-drawing characters in default mode |
| `TestNewTablePlain` | Plain mode produces tab-separated output |
| `TestPlainTableRows` | Rows-only output (no headers) |
| `TestPlainTableWithHeader` | Header inclusion toggle |

Color helpers are unit-tested in `internal/cli/output_test.go`:

| Test | What it covers |
|------|---------------|
| `TestOutputColorHelpersNoColor` | Green/Red/Bold/etc return plain text when noColor=true |
| `TestOutputColorHelpersWithColor` | Color helpers add ANSI codes when color is forced on |
| `TestOutputTableDefaultMode` | `NewTable()` produces column-aligned output without box chars |

## Gaps

1. **No E2E tests for default output mode** — Every E2E test uses `mustSucceed()` (appends `--json`) or `mustSucceedPlain()` (appends `--plain`). No test runs a command without output mode flags to verify the default colored/table output.

2. **No E2E tests for `init` JSON mode** — `init` is tested in default mode only (the E2E runner calls it without `--json`).

3. **No unit tests for individual command output** — Tests validate the table rendering infrastructure but not that e.g. `list` passes the right headers/rows to `out.Table()`.

## E2E Test Helpers

| Helper | Mode | Used by |
|--------|------|---------|
| `mustSucceed()` | Appends `--json`, parses JSON envelope, asserts `success=true` | All JSON subtests |
| `runJSON()` | Appends `--json`, parses JSON envelope (no success assertion) | `multi-message` |
| `mustSucceedPlain()` | Appends `--plain`, asserts exit code 0 | All `plain/*` subtests |
| `run()` | Raw execution, no flags added | `init`, `wait-for-ready` |

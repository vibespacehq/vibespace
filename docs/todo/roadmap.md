# Roadmap

## P1: Help & Diagnostics

| Task | Description | Effort |
|------|-------------|--------|
| `vibespace doctor` | Diagnose common issues (Docker, ports, disk) | M |
| `vibespace wait` | Wait for cluster ready state | S |
| `<vs> wait` | Wait for agent(s) ready | S |
| `vibespace help --json` | Machine-readable command schema | M |
| Improve `--help` text | Consistent examples, better descriptions | S |

## P2: Testing

| Task | Description | Effort |
|------|-------------|--------|
| Unit tests for `internal/cli` | Test each command handler | L |
| Unit tests for `pkg/errors` | Test exit code mapping | S |
| Unit tests for `pkg/ui` | Test table/style rendering | S |
| Integration tests | End-to-end CLI workflows | L |
| Contract tests | Validate JSON/plain output schemas | M |
| CI pipeline | GitHub Actions on PR | M |
| CI badges | Build, coverage, Go Report Card in README | S |
| Codecov integration | Coverage reporting and PR comments | S |

## P3: Declarative Config

| Task | Description | Effort |
|------|-------------|--------|
| `vibespace apply -f spec.yaml` | Create/update from YAML | L |
| `vibespace diff -f spec.yaml` | Show what apply would change | M |
| `vibespace export <vs>` | Export vibespace to YAML | M |
| Spec schema | Define YAML schema, validate | M |

## P4: Automation Enhancements

| Task | Description | Effort |
|------|-------------|--------|
| `--plan` / `--apply` for delete | Two-step confirmation | S |
| `--status` filter for list | Filter by running/stopped/error | S |
| `--sort` flag for list | Sort by name/created/status | S |
| `--limit`/`--cursor` pagination | For large result sets | M |
| `--timeout` global flag | Timeout for all operations | S |
| `--non-interactive` global flag | Disable all prompts | S |
| `multi --stream --json` | JSONL streaming output | M |

## P5: Versioning & Releases

| Task | Description | Effort |
|------|-------------|--------|
| GitHub Actions release | Auto-release on tag | M |
| Update checker | Notify when new version available | M |

## P6: Distribution

| Task | Description | Effort |
|------|-------------|--------|
| GitHub Releases | Pre-built binaries (darwin/linux, amd64/arm64) | M |
| Homebrew formula | `brew install vibespace` | S |
| Install script | `curl -sSL .../install.sh \| bash` | S |
| Go install | `go install github.com/.../vibespace@latest` | S |
| APT repository | For Debian/Ubuntu | M |
| RPM repository | For RHEL/Fedora | M |
| AUR package | For Arch Linux | S |

## P7: Feature Flags

| Task | Description | Effort |
|------|-------------|--------|
| `--experimental` global flag | Enable experimental features | S |
| `VIBESPACE_EXPERIMENTAL=1` env | Environment variable toggle | S |
| `vibespace features list` | Show available flags | S |
| `vibespace features enable <flag>` | Enable specific flag | S |
| Feature graduation process | Promote stable features | S |

## P8: JSON Enhancements

| Task | Description | Effort |
|------|-------------|--------|
| `request_id` in meta | Correlation ID with `--verbose` | S |
| `duration_ms` in meta | Operation timing with `--verbose` | S |
| `warnings` array in meta | Non-fatal warnings | S |

---

## Effort Key

| Symbol | Meaning |
|--------|---------|
| S | Small (< 1 day) |
| M | Medium (1-3 days) |
| L | Large (> 3 days) |

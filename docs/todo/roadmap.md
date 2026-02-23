# Roadmap

## P0: Testing (WIP)

| Task | Description | Status |
|------|-------------|--------|
| Unit tests (77 tests, 17 files) | Pure logic across all packages | Done |
| K8s service layer tests (10 tests) | CRUD against real k3s | Done |
| E2E binary lifecycle (~37 subtests × 3 platforms) | JSON + plain mode coverage | Done |
| CI pipeline (ci.yml + ci-e2e.yml) | Lint + unit + integration + E2E | Done |
| Codecov integration | Binary coverage + PR comments | Done |
| CI badges | Build, coverage in README | Done |
| Default (human-readable) output E2E | Exercise table/color/spinner paths | Todo |
| Error path E2E | Invalid args, not found, conflict | Todo |
| Remote mode E2E | WireGuard tunnel: serve + connect from runners | Todo |

## P1: Help & Diagnostics

| Task | Description | Effort |
|------|-------------|--------|
| `vibespace doctor` | Diagnose common issues (Docker, ports, disk) | M |
| `vibespace wait` | Wait for cluster ready state | S |
| `<vs> wait` | Wait for agent(s) ready | S |
| `vibespace help --json` | Machine-readable command schema | M |
| Improve `--help` text | Consistent examples, better descriptions | S |

## P2: Declarative Config

| Task | Description | Effort |
|------|-------------|--------|
| `vibespace apply -f spec.yaml` | Create/update from YAML | L |
| `vibespace diff -f spec.yaml` | Show what apply would change | M |
| `vibespace export <vs>` | Export vibespace to YAML | M |
| Spec schema | Define YAML schema, validate | M |

## P3: Automation Enhancements

| Task | Description | Effort |
|------|-------------|--------|
| `--plan` / `--apply` for delete | Two-step confirmation | S |
| `--status` filter for list | Filter by running/stopped/error | S |
| `--sort` flag for list | Sort by name/created/status | S |
| `--limit`/`--cursor` pagination | For large result sets | M |
| `--timeout` global flag | Timeout for all operations | S |
| `--non-interactive` global flag | Disable all prompts | S |
| `multi --stream --json` | JSONL streaming output | M |

## P4: Versioning & Releases

| Task | Description | Effort |
|------|-------------|--------|
| GitHub Actions release | Auto-release on tag | M |
| Update checker | Notify when new version available | M |

## P5: Distribution

| Task | Description | Effort |
|------|-------------|--------|
| GitHub Releases | Pre-built binaries (darwin/linux, amd64/arm64) | M |
| Homebrew formula | `brew install vibespace` | S |
| Install script | `curl -sSL .../install.sh \| bash` | S |
| Go install | `go install github.com/.../vibespace@latest` | S |
| APT repository | For Debian/Ubuntu | M |
| RPM repository | For RHEL/Fedora | M |
| AUR package | For Arch Linux | S |

## P6: Feature Flags

| Task | Description | Effort |
|------|-------------|--------|
| `--experimental` global flag | Enable experimental features | S |
| `VIBESPACE_EXPERIMENTAL=1` env | Environment variable toggle | S |
| `vibespace features list` | Show available flags | S |
| `vibespace features enable <flag>` | Enable specific flag | S |
| Feature graduation process | Promote stable features | S |

## P7: JSON Enhancements

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

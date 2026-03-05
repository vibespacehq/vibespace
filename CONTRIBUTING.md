# Contributing

## Development setup

**Requirements:** Go 1.22+, macOS or Linux

```bash
git clone https://github.com/vibespacehq/vibespace.git
cd vibespace
go build ./...
```

### Running tests

```bash
# Unit tests only (fast, no infra needed)
go test -short ./...

# All tests including k8s integration (needs k3s running)
go test ./...

# Lint
go vet ./...
staticcheck ./...
```

### Building

```bash
./scripts/build.sh                  # build with version injection
./scripts/install.sh                # install to /usr/local/bin (needs sudo)
```

### Debug mode

```bash
VIBESPACE_DEBUG=1 ./vibespace <command>     # logs to ~/.vibespace/debug.log
VIBESPACE_LOG_LEVEL=debug ./vibespace ...   # set log level
```

## Project structure

```
cmd/vibespace/          # main entry point
internal/cli/           # cobra commands, TUI, output handling
internal/platform/      # cluster management (colima, lima, baremetal)
pkg/remote/             # WireGuard, server, client, TLS
pkg/dns/                # embedded DNS server
pkg/daemon/             # port forwarding daemon
pkg/agent/              # agent config interface
pkg/vibespace/          # vibespace CRUD via k8s client
pkg/deployment/         # k8s deployment/service management
pkg/k8s/                # k8s client wrapper
pkg/session/            # multi-agent session persistence
pkg/tui/                # tab-based TUI
pkg/metrics/            # k8s metrics collection
pkg/ui/                 # colors, styles, tables
pkg/errors/             # error types, exit codes
build/                  # Dockerfiles for agent containers
test/e2e/               # E2E tests (require Linux + k3s)
```

## Submitting changes

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Run `go build ./... && go vet ./... && go test -short ./...` to verify
4. Commit using [conventional commits](#commit-conventions)
5. Open a pull request against `main`

### What makes a good PR

- Focused on a single change
- Includes tests for new functionality where applicable
- Doesn't introduce unnecessary dependencies
- Passes CI (lint + unit + integration)

### Commit conventions

We use conventional commits:

```
feat: add remote mode via WireGuard
fix(daemon): handle connection timeout
refactor(tui): simplify session management
docs: update CLI reference
test: add config set integration test
chore: bump dependencies
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`, `ci`, `build`, `style`

Keep the subject line under 72 characters, imperative mood ("add" not "added"), no period at the end.

## Reporting bugs

Open a [GitHub issue](https://github.com/vibespacehq/vibespace/issues) with:

- What you were doing
- What you expected
- What happened instead
- Output of `vibespace version` and `vibespace status`
- Debug logs if relevant (`VIBESPACE_DEBUG=1`)

## Suggesting features

Open a [GitHub issue](https://github.com/vibespacehq/vibespace/issues) with the `enhancement` label. Describe the use case, not just the feature.

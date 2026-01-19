# vibespace

Multi-Claude development environments via CLI.

## Quick Start

```bash
# Build
go build -o vibespace ./cmd/vibespace

# Initialize cluster (downloads Colima, k3s, kubectl)
./vibespace init

# Create a vibespace
./vibespace create myproject

# Start port-forwarding and connect
./vibespace myproject up
./vibespace myproject connect
```

## Installation

```bash
# Build and install
go build -o vibespace ./cmd/vibespace
sudo mv vibespace /usr/local/bin/

# Or run directly
go run ./cmd/vibespace <command>
```

## Commands

### Cluster Management
```bash
vibespace init          # Initialize local Kubernetes cluster
vibespace status        # Show cluster status
vibespace stop          # Stop the cluster
```

### Vibespace Management
```bash
vibespace create <name>     # Create a vibespace
vibespace list              # List all vibespaces
vibespace delete <name>     # Delete a vibespace
vibespace <name> start      # Start a vibespace
vibespace <name> stop       # Stop a vibespace
```

### Agent Management
```bash
vibespace <name> agents         # List Claude agents
vibespace <name> spawn          # Add another Claude agent
vibespace <name> kill <agent>   # Remove an agent
```

### Connection
```bash
vibespace <name> up             # Start port-forward daemon
vibespace <name> down           # Stop port-forward daemon
vibespace <name> connect        # Connect to Claude terminal
vibespace <name> ports          # List detected ports
vibespace <name> forward list   # List active forwards
```

### Multi-Session
```bash
vibespace multi <v1> <v2>       # Quick multi-vibespace TUI
vibespace session create <name> # Create named session
vibespace session start <name>  # Start session TUI
```

See `docs/CLI_SPEC.md` for complete command reference.

## Project Structure

```
api/
├── cmd/vibespace/       # CLI entry point
├── internal/
│   ├── cli/             # Cobra command implementations
│   └── platform/        # Colima/k3s management
├── pkg/
│   ├── k8s/             # Kubernetes client
│   ├── knative/         # Knative service management
│   ├── vibespace/       # Vibespace business logic
│   ├── model/           # Data models
│   └── image/           # Container image (ttyd + Claude)
└── README.md
```

## Development

```bash
# Run tests
go test ./...

# Build
go build ./cmd/vibespace

# Run directly
go run ./cmd/vibespace init
```

## State Files

```
~/.vibespace/
├── bin/           # Bundled binaries (colima, lima, kubectl)
├── daemons/       # Port-forward daemon state
├── sessions/      # Multi-session state
└── config.json    # Global config
```

## Requirements

- Go 1.21+
- macOS (Colima) or Linux (native k3s)
- Docker (for building container images)

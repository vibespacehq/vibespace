# Workspace API Server

Go backend API for managing Kubernetes workspaces.

## Prerequisites

- Go 1.24+
- Access to a Kubernetes cluster (k3s recommended)
- `KUBECONFIG` set to your Kubernetes config file

## Quick Start

```bash
# Install dependencies
go mod download

# Run the server
go run cmd/server/main.go

# Or build and run
go build -o server cmd/server/main.go
./server
```

The API server will start on port `8090` by default. Override with `PORT` environment variable.

## API Endpoints

### Health

- `GET /api/v1/health` - Health check

### Workspaces

- `GET /api/v1/workspaces` - List all workspaces
- `POST /api/v1/workspaces` - Create a new workspace
- `GET /api/v1/workspaces/:id` - Get workspace details
- `DELETE /api/v1/workspaces/:id` - Delete a workspace
- `POST /api/v1/workspaces/:id/start` - Start a workspace
- `POST /api/v1/workspaces/:id/stop` - Stop a workspace

### Templates

- `GET /api/v1/templates` - List available templates
- `GET /api/v1/templates/:id` - Get template details

## Example Requests

### Create a workspace

```bash
curl -X POST http://localhost:8090/api/v1/workspaces \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "template": "nextjs",
    "persistent": true
  }'
```

### List workspaces

```bash
curl http://localhost:8090/api/v1/workspaces
```

### List templates

```bash
curl http://localhost:8090/api/v1/templates
```

## Development

```bash
# Run tests
go test ./...

# Build
go build ./cmd/server

# Run with auto-reload (install air first: go install github.com/air-verse/air@latest)
air
```

## Project Structure

```
api/
├── cmd/
│   └── server/          # Main entry point
├── pkg/
│   ├── handler/         # HTTP handlers
│   ├── workspace/       # Workspace service
│   ├── k8s/             # Kubernetes client wrapper
│   └── model/           # Data models
├── go.mod
└── README.md
```

## Configuration

The API server can be configured via environment variables:

- `PORT` - Server port (default: 8090)
- `KUBECONFIG` - Path to Kubernetes config file (default: `~/.kube/config`)

## Notes

- The API will run in limited mode if Kubernetes is not available (for development)
- Workspace persistence and auto-scaling features require Knative (will be implemented in Phase 2)
- Template images must be available in the local registry or Kubernetes cluster

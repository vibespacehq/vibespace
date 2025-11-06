# Vibespace API Server

Go backend API for managing Kubernetes vibespaces.

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

### Vibespaces

- `GET /api/v1/vibespaces` - List all vibespaces
- `POST /api/v1/vibespaces` - Create a new vibespace
- `GET /api/v1/vibespaces/:id` - Get vibespace details
- `DELETE /api/v1/vibespaces/:id` - Delete a vibespace
- `POST /api/v1/vibespaces/:id/start` - Start a vibespace
- `POST /api/v1/vibespaces/:id/stop` - Stop a vibespace

### Templates

- `GET /api/v1/templates` - List available templates
- `GET /api/v1/templates/:id` - Get template details

## Example Requests

### Create a vibespace

```bash
curl -X POST http://localhost:8090/api/v1/vibespaces \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-project",
    "template": "nextjs",
    "persistent": true
  }'
```

### List vibespaces

```bash
curl http://localhost:8090/api/v1/vibespaces
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
│   ├── vibespace/       # Vibespace service
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
- Vibespace persistence and auto-scaling features require Knative (will be implemented in Phase 2)
- Template images must be available in the local registry or Kubernetes cluster

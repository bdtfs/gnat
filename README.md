# gnat

[![Go](https://img.shields.io/badge/Go-1.25.3-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](/)
[![Lint](https://img.shields.io/badge/lint-golangci--lint-purple)](https://golangci-lint.run/)

A fast, scenario-oriented load testing service written in Go. Define test setups via HTTP API, execute runs, and collect detailed latency/throughput statistics with percentile breakdowns.

## Features

- **Simple HTTP API** - Create setups, start runs, and fetch stats via REST endpoints
- **Configurable Load** - Set requests per second (RPS) and duration per test
- **Detailed Metrics** - Latency percentiles (P50/P90/P95/P99), success rates, status code distribution
- **Structured Logging** - JSON logs via `log/slog` for easy parsing
- **Tunable HTTP Client** - Configure connection pools, timeouts, and keep-alive settings

## Quick Start

### Prerequisites

- Go 1.25.3 or later
- Git

### Installation

```bash
git clone https://github.com/bdtfs/gnat.git
cd gnat
make build
```

### Run

```bash
# Start the server (default port 8778)
./bin/gnat-backend

# Or with custom port
APPLICATION_PORT=9000 ./bin/gnat-backend
```

### Example Usage

```bash
# Create a test setup
curl -X POST http://localhost:8778/api/setups \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Example Test",
    "method": "GET",
    "url": "https://httpbin.org/get",
    "rps": 50,
    "duration": "10s"
  }'

# Start a run (use the setup_id from the response above)
curl -X POST http://localhost:8778/api/runs \
  -H "Content-Type: application/json" \
  -d '{"setup_id": "SETUP_ID_HERE"}'

# Check run status and stats
curl http://localhost:8778/api/runs/RUN_ID_HERE
```

## Development

### Prerequisites

Install development tools:

```bash
make tools
```

This installs:
- [golangci-lint](https://golangci-lint.run/) v1.64.8 - Linting
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) v1.1.4 - Vulnerability scanning

### Commands

| Command | Description |
|---------|-------------|
| `make build` | Build backend and frontend binaries |
| `make test` | Run all tests |
| `make test-race` | Run tests with race detector |
| `make lint` | Run golangci-lint |
| `make coverage` | Generate coverage report |
| `make run-all` | Run backend and frontend servers |

### Test Coverage

Tests use table-driven patterns with `t.Parallel()` for concurrent execution.

| Package | Coverage |
|---------|----------|
| `internal/config` | 100% |
| `internal/models` | 100% |
| `internal/storage/memory` | 100% |
| `internal/converters` | 100% |
| `internal/service` | 95.8% |
| `internal/server` | 89.6% |
| `pkg/clients/http` | 100% |

### Linting

The project uses [golangci-lint](https://golangci-lint.run/) with strict settings:

**Enabled Linters:**
- **Security**: gosec
- **Correctness**: errcheck, staticcheck, govet
- **Style**: gofmt, goimports, revive
- **Complexity**: gocyclo (max 10), gocognit (max 15)
- **Quality**: unused, ineffassign, misspell, goconst

Run the linter:

```bash
make lint
# or directly
golangci-lint run
```

## API Reference

Base URL: `http://localhost:8778`

### Setups

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/setups` | Create a new test setup |
| `GET` | `/api/setups` | List all setups |
| `GET` | `/api/setups/{id}` | Get setup by ID |
| `DELETE` | `/api/setups/{id}` | Delete setup |

**Create Setup Request:**
```json
{
  "name": "My Test",
  "description": "Optional description",
  "method": "GET",
  "url": "https://api.example.com/endpoint",
  "headers": {"Authorization": "Bearer token"},
  "body": "",
  "rps": 100,
  "duration": "30s"
}
```

### Runs

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/runs` | Start a new run |
| `GET` | `/api/runs` | List all runs |
| `GET` | `/api/runs?setup_id={id}` | List runs for a setup |
| `GET` | `/api/runs/{id}` | Get run details with stats |
| `POST` | `/api/runs/{id}/cancel` | Cancel an active run |
| `GET` | `/api/runs/{id}/stats` | Get run statistics only |

**Run Status Values:** `pending` | `running` | `completed` | `failed` | `cancelled`

**Stats Response:**
```json
{
  "total": 3000,
  "success": 2950,
  "failed": 50,
  "avg_latency_ms": 45.2,
  "min_latency_ms": 12.0,
  "max_latency_ms": 250.0,
  "p50_latency_ms": 42.0,
  "p90_latency_ms": 78.0,
  "p95_latency_ms": 95.0,
  "p99_latency_ms": 150.0,
  "success_rate": 0.983,
  "rps": 99.5,
  "bytes_read": 1048576,
  "status_codes": {"200": 2950, "500": 50},
  "errors": []
}
```

## Configuration

### Environment Variables

**Application:**

| Variable | Default | Description |
|----------|---------|-------------|
| `APPLICATION_PORT` | `8778` | HTTP server port |

**HTTP Client Tuning:**

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_MAX_IDLE_CONNS` | `10000` | Max idle connections |
| `HTTP_MAX_IDLE_CONNS_PER_HOST` | `10000` | Max idle connections per host |
| `HTTP_IDLE_CONN_TIMEOUT` | `90s` | Idle connection timeout |
| `HTTP_DISABLE_COMPRESSION` | `false` | Disable HTTP compression |
| `HTTP_DIAL_TIMEOUT` | `5s` | TCP dial timeout |
| `HTTP_KEEPALIVE` | `30s` | Keep-alive duration |
| `HTTP_TLS_HANDSHAKE_TIMEOUT` | `5s` | TLS handshake timeout |
| `HTTP_EXPECT_TIMEOUT` | `1s` | Expect 100-continue timeout |
| `HTTP_REQUEST_TIMEOUT` | `10s` | Overall request timeout |

## Project Structure

```
.
├── cmd/
│   ├── gnat-backend/          # REST API server
│   └── gnat-frontend/         # Web UI server
├── internal/
│   ├── config/                # Environment-based configuration
│   ├── converters/            # Model <-> DTO transformations
│   ├── di/                    # Dependency injection container
│   ├── models/                # Domain models (Setup, Run, Stats)
│   ├── runner/                # Load generator and stats collector
│   ├── server/                # HTTP handlers and middlewares
│   ├── service/               # Business logic layer
│   ├── storage/memory/        # In-memory repository
│   └── web/                   # Frontend templates and handlers
├── pkg/
│   └── clients/http/          # Tuned HTTP client builder
├── .golangci.yml              # Linter configuration
├── Makefile                   # Build and development commands
└── README.md
```

## Notes

- **In-Memory Storage**: Data is not persisted across restarts
- **Go 1.22+ Required**: Uses `http.ServeMux` path patterns
- **Graceful Shutdown**: Server handles SIGINT/SIGTERM signals

## Roadmap

- [ ] Web UI for real-time monitoring
- [ ] Persistent storage backend
- [ ] CLI for local runs and config generation
- [ ] Distributed workers support
- [ ] Export results to JSON/CSV

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

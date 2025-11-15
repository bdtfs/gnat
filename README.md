# gnat

Gnat is a fast, scenario-oriented load testing service and CLI written in Go. It provides simple HTTP-based APIs to define test setups and start runs, 
collects latency/throughput statistics, and logs requests in JSON. It currently uses in-memory storage.

Note: Some features mentioned in earlier descriptions (e.g., a real-time Web UI, distributed workers) are not present in this repository yet. See TODOs below.

## Overview

- Language/stack: Go (module-based, Go modules)
- Module: `github.com/bdtfs/gnat`
- Package manager: `go` (Go modules)
- Entry point (binary): `./cmd/gnat/main.go`
- Server framework: standard library `net/http` with Go 1.22+ style patterns (ServeMux)
- Logging: `log/slog` JSON to stdout
- Storage: in-memory repository (non-persistent)
- HTTP client: custom-configured `http.Client` (see env vars)

## Requirements

- Go toolchain installed (version as declared in `go.mod`: `go 1.25.3`)
  - If you are on an older Go version, try with the latest stable Go and update if needed.
- Git (to clone the repo)

## Getting started

### Clone

```
git clone https://github.com/bdtfs/gnat.git
cd gnat
```

### Build

```
go build -o bin/gnat ./cmd/gnat
```

### Run (development)

By default, the server binds to `0.0.0.0:${APPLICATION_PORT}`. The default is `8778`.

```
# default port 8778
APPLICATION_PORT=8778 go run ./cmd/gnat

# or run the built binary
APPLICATION_PORT=8778 ./bin/gnat
```

Important: The startup banner currently prints `:8080`, but the server actually binds to `:${APPLICATION_PORT}` (default `8778`).

TODO: Align the banner with the configured port.

## API

Base URL: `http://localhost:${APPLICATION_PORT}` (default `http://localhost:8778`).

### Create setup

`POST /api/setups`

Request (JSON):
```
{
  "name": "My test",
  "description": "Simple GET",
  "method": "GET",
  "url": "https://example.com/api",
  "body": "",
  "headers": {"Accept": "application/json"},
  "rps": 100,
  "duration": "30s"
}
```
Notes:
- `duration` is parsed by Go's `time.ParseDuration` (examples: `"10s"`, `"2m"`, `"1h"`).
- `body` is a JSON string; when provided it will be parsed as base64 by Go's JSON decoder for `[]byte` fields. For plain-text payloads, provide base64-encoded content.

Response `201 Created` (JSON): `dto.Setup`
```
{
  "id": "...",
  "name": "...",
  "description": "...",
  "method": "GET",
  "url": "https://example.com/api",
  "rps": 100,
  "duration": 30000000000,
  "status": "active",
  "created_at": "2025-11-15T20:46:00Z",
  "updated_at": "2025-11-15T20:46:00Z"
}
```

### List setups

`GET /api/setups`

Response `200 OK`: array of `dto.Setup`.

### Get setup

`GET /api/setups/{id}` → `200 OK` with `dto.Setup` or `404`.

### Delete setup

`DELETE /api/setups/{id}` → `204 No Content` or `404`.

### Start run

`POST /api/runs`

Request:
```
{ "setup_id": "{setup_id}" }
```

Response `201 Created`: `dto.Run`.

### List runs

`GET /api/runs` → list all runs.

Optional filter: `GET /api/runs?setup_id={setup_id}` → runs for a specific setup.

### Get run

`GET /api/runs/{id}` → `200 OK` with `dto.Run` or `404`.

### Cancel run

`POST /api/runs/{id}/cancel` → `204 No Content` or `400` if cannot cancel.

### Get run stats

`GET /api/runs/{id}/stats` → `200 OK` with `dto.Stats`.

`dto.Run` fields (simplified):
```
{
  "id": "...",
  "setup_id": "...",
  "status": "pending|running|completed|failed|cancelled",
  "started_at": "...",
  "elapsed": "1m2s",
  "ended_at": "...",           // optional
  "error": "...",               // optional
  "stats": { /* see below */ }
}
```

`dto.Stats` fields:
```
{
  "total": 0,
  "success": 0,
  "failed": 0,
  "avg_latency_ms": 0,
  "min_latency_ms": 0,
  "max_latency_ms": 0,
  "p50_latency_ms": 0,
  "p90_latency_ms": 0,
  "p95_latency_ms": 0,
  "p99_latency_ms": 0,
  "success_rate": 0,
  "rps": 0,
  "bytes_read": 0,
  "status_codes": {"200": 123},
  "errors": ["..."]
}
```

## Environment variables

Application:
- `APPLICATION_PORT` (int) — HTTP port to bind; default: `8778`.

HTTP client tuning (affects outbound load generation):
- `HTTP_MAX_IDLE_CONNS` (int) — default: `10000`.
- `HTTP_MAX_IDLE_CONNS_PER_HOST` (int) — default: `10000`.
- `HTTP_IDLE_CONN_TIMEOUT` (duration) — default: `90s`.
- `HTTP_DISABLE_COMPRESSION` (bool) — default: `false`.
- `HTTP_DIAL_TIMEOUT` (duration) — default: `5s`.
- `HTTP_KEEPALIVE` (duration) — default: `30s`.
- `HTTP_TLS_HANDSHAKE_TIMEOUT` (duration) — default: `5s`.
- `HTTP_EXPECT_TIMEOUT` (duration) — default: `1s`.
- `HTTP_REQUEST_TIMEOUT` (duration) — default: `10s`.

## Logging

- Structured JSON logs via `log/slog` to stdout.
- Built-in middlewares: panic recovery (returns 500 JSON) and request logging (method, path, status, duration, remote).

## Testing

There are currently no test files in this repository.

TODO:
- Add unit tests for converters, service, and runner.
- Add integration tests for HTTP API.

## Project structure

```
.
├── cmd/gnat/                   # Main entry point (binary)
│   ├── main.go                 # App bootstrap & graceful shutdown
│   └── welcome.go              # ASCII banner and endpoints list (TODO: port alignment)
├── internal/
│   ├── config/                 # Config loading from env
│   ├── converters/             # Model -> DTO transformations
│   ├── di/                     # Dependency injection container
│   ├── models/                 # Domain models & statuses
│   ├── runner/                 # Load generator, stats collector
│   ├── server/                 # HTTP server, middlewares, DTOs
│   ├── service/                # Business logic for setups/runs
│   └── storage/memory/         # In-memory repository
├── pkg/clients/http/           # Tuned HTTP client builder
├── go.mod, go.sum              # Module definition
├── LICENSE                     # MIT License
└── README.md                   # This file
```

## Development notes

- Storage is in-memory: process restart clears setups and runs.
- Run cancellation endpoint attempts to cancel active runs; completed runs cannot be cancelled.
- The server uses Go's `http.ServeMux` with path patterns (Go 1.22+).

## License

This project is licensed under the MIT License — see the `LICENSE` file for details.

## Roadmap / TODOs

- Web UI for real-time monitoring. (Not present in this repo.)
- Persist setups and results (add storage backend).
- Distributed workers and coordinator.
- CLI UX for local runs and config generation.
- Align startup banner with configured port and advertised endpoints.

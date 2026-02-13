# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GoMockAPI is a mock API server built in Go that lets users define REST endpoints with configurable responses, conditional logic, simulated delays, and dynamic response generation. It uses an embedded BoltDB database and embedded static assets, requiring no external dependencies to run.

## Build & Run Commands

```bash
# Run the application (default port 4041)
go run ./cmd/web

# Run with flags
go run ./cmd/web -port=4041 -domain=localhost -host=0.0.0.0

# Build binary
go build -o gomockapi ./cmd/web

# Cross-platform build (outputs to ./bin/)
./builder.sh ./cmd/web

# Run tests
go test ./...
go test -v ./utils/stringutils/    # single package

# Vet/format (no linter config exists)
go vet ./...
go fmt ./...
```

## Architecture

**Module:** `github.com/onlysumitg/GoMockAPI` (Go 1.20, Chi router, BoltDB)

### Key Directories

- `cmd/web/` — Main application: server setup, all HTTP handlers, routes, middleware, template rendering, caching
- `internal/models/` — Data models and all BoltDB operations (repository pattern)
- `utils/` — Utility packages: `httputils`, `jsonutils`, `xmlutils`, `stringutils`, `typeutils`, `concurrent`
- `ui/html/` — Go html/templates (base.tmpl, pages/, partials/, accounts/, emails/)
- `ui/static/` — Static assets (CSS, JS, fonts), served via `go:embed`
- `env/` — Environment config loaded from `.env` file

### Request Flow for Mock API Calls

```
Request → Chi Router → Middleware (CSRF, auth, rate-limit, CORS)
  → Handler → EndPoint lookup (cache or DB) → ApiCall processing
  → ConditionGroup evaluation → ResponseParam resolution → Response
  → (async) log saving
```

### Core Domain Model

- **EndPoint** — A mock API endpoint (URL pattern, HTTP method, sample request/response). Cached in memory with key `{collection}_{name}_{method}`.
- **ApiCall** (`internal/models/api_call.go`) — Processes a single request: parses headers/body/query, evaluates conditions, builds response. Can optionally proxy to an actual URL.
- **ConditionGroup** — A set of conditions evaluated with AND logic. When all pass, applies response mappings and sets HTTP status.
- **Condition** — Compares a request parameter against an expected value (operators: equals, contains, starts/ends with, regex).
- **ResponseParam** — A placeholder in the response body with default values or random data generation (via gofakeit).
- **Collection** — Groups related endpoints for organization.

### Data Storage

BoltDB files in `db/` directory:
- `db/internal.db` — Main database (endpoints, users, collections, conditions, params)
- `db/log_YYYYMMDD.db` — Daily API call log files

### Application Bootstrap

The `application` struct in `cmd/web/config.go` holds all dependencies (models, session manager, template cache, DB connections). Handlers are methods on this struct.

### Concurrency

- Endpoint cache is mutex-protected (`cmd/web/cache.go`)
- Log saving and stats run in goroutines with recovery wrappers (`utils/concurrent/`)
- Background worker system in `internal/worker/`

## Environment Configuration

Configured via `env/.env` (see `env/.env.sample` for template). Key vars: `PORT`, `DOMAIN`, `ALLOWEDORIGINS`, `HTTPS`, `USELETSENCRYPT`, `REQUESTS_PER_HOUR_BY_IP`, `REQUESTS_PER_HOUR_BY_USER`, SMTP settings.

Default credentials: `admin2@example.com` / `adminpass`

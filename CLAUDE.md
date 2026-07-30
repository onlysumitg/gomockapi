# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GoMockAPI is a mock API server built in Go that lets users define REST endpoints with configurable responses, conditional logic, simulated delays, and dynamic response generation. Endpoints are managed through a server-rendered admin UI (or imported from Postman collections) and served under `/api/...`. It uses an embedded BoltDB database and embedded static assets/templates, requiring no external dependencies to run.

## Build & Run Commands

```bash
# Run the application (default port 4041). Must run from the repo root:
# env/.env, db/, lic/ and uploads/ are resolved relative to the working directory.
go run ./cmd/web

# Run with flags (env vars in env/.env override PORT / DOMAIN / USELETSENCRYPT)
go run ./cmd/web -port=4041 -domain=localhost -host=0.0.0.0

# Build binary
go build -o gomockapi ./cmd/web

# Cross-platform build (outputs to ./bin/)
./builder.sh ./cmd/web

# Build + push versioned docker image (bump APP_VERSION inside the script first)
./builder_docker.sh

# Tests — only utils/stringutils has any
go test ./...
go test -v ./utils/stringutils/

go fmt ./...
go vet ./...   # currently reports pre-existing findings (unkeyed struct literals in
               # cmd/web/ApiHandler.go, unreachable code in response_param.go) — not regressions
```

`ui/html` and `ui/static` are embedded via `go:embed` (`ui/efs.go`), so template and asset edits require a rebuild/restart, not just a browser refresh.

## Architecture

**Module:** `github.com/onlysumitg/GoMockAPI` (Go 1.20, Chi router, BoltDB/bbolt)

### Key Directories

- `cmd/web/` — Main application: server setup, all HTTP handlers, routes, middleware, template rendering, caching
- `internal/models/` — Domain types **and** all BoltDB access (repository pattern: one `XxxModel{DB *bolt.DB}` per bucket)
- `utils/` — `httputils`, `jsonutils`, `xmlutils`, `stringutils`, `typeutils`, `concurrent`
- `ui/html/` — Go html/templates: `base.tmpl` + `account_base.tmpl`/`email_base.tmpl`/`public_base.tmpl`, plus `pages/`, `partials/`, `accounts/`, `emails/`, `public/`
- `env/` — `env.GetEnvVariable` re-reads `env/.env` on **every** call (godotenv); there is no `.env.sample`, create `env/.env` yourself
- `testdata/` — Sample Postman collections and OpenAPI examples for import testing

Unused/dormant: `internal/worker/` and `internal/iwebsocket/` are not imported anywhere, `cmd/web/batches.go` is an empty stub, and `cmd/postman/main.go` is a standalone scratch binary for parsing Postman JSON.

### Request Flow for Mock API Calls

`/api/*` is the mock-serving surface and has **no auth and no CSRF** — only CORS, chi's base middleware stack, and the optional rate limiters apply. Auth (`app.RequireAuthentication`) and CSRF (`noSurf`) are attached per route group inside each `Xxx Handlers(router)` function (see `cmd/web/EndPointHandler.go`, `ApiHandler.go`, etc.), never globally in `routes.go`.

```
GET/DELETE → app.GET     (query string + path params → flat map)
POST/PUT/PATCH → app.POST (JSON or XML body per endpoint.SampleRequestType,
                           + query params + path params + form values)
  → app.ProcessAPICall (cmd/web/ApiHandler.go)
      → app.GetEndPoint(collection, name, method)      [cache, cmd/web/cache.go]
      → build models.ApiCall with RequestFlatMap
      → apiCall.CopyResponseString(endPoint)           [snapshot of every response variant]
      → endPoint.BuildResponse(apiCall)                [condition groups, then defaults]
      → apiCall.Finalize()                             [proxy, delay, pick final response]
      → app.writeJSONorXML(...)
      → goroutines: save logs to LogDB, append EndPointCallLog
```

### Response Model & Placeholder Substitution

This is the core mechanic and is spread across several files:

1. An **EndPoint** owns `ResponseMap []*EndPointResponse` — several *named* response variants, each with its own `HttpCode`, headers, body and `ResponseType` (`JSON`/`XML`/`TEXT`).
2. On save, `EndPointResponse.BuildResponsePlaceholder()` converts the sample body into `ResponsePlaceholder`, where every leaf value becomes the token `{{FLATKEY}}` (`jsonutils.JsonToMapPlaceholder` / `xmlutils.XmlToFlatMapAndPlaceholder`; `TEXT` is copied verbatim). In parallel, `RebuildResponseParams` creates one `EndPointResponseParam` per flat key.
3. At request time, `EndPointResponseParam.process()` does a plain `strings.ReplaceAll` of `"{{KEY}}"` in the response variant it belongs to. The replacement is quoted or unquoted based on `DefaultDatatype` (`INT`, `FLOAT64`, `BOOL`, `XMLSTRING`, `INVALID`→`null`, default→quoted string).
4. `ApiCall.HasSetCache` / `SetKey` enforce first-write-wins: once a key (e.g. the HTTP status) has been assigned by a condition group, later assignments are logged and skipped.
5. `ApiCall.GetResponseToUse()` picks the variant: the one whose ID a condition group selected, else `EndPoint.GetDefaultResponseID()` (a variant named `DEFAULT` → first `200` → first `2xx` → first in list).

**Override value syntax** (`EndPointResponseParam.OverrideValue`, also used for condition-group assignments):
- `REQUEST[...]:<flat-key>` — copy a value out of the incoming request's flat map
- `*RANDOM:<NAME>` — generate fake data; the names are the keys of `models.RandomFunctionMap` (`internal/models/random.go`, gofakeit)
- anything else — used as a literal

**Special `*` keys** in the flat maps (injected by `cmd/web/ApiHandler.go`, consumed by `EndPointResponseParam.processSpecials`):
- Request side: `*HEADER_<NAME>` (uppercased request headers), `*PATH_<n>` (extra path segments after `/api/{collection}/{endpoint}`), `*CLIENT_IP`
- Response side: `*HTTP_STATUS_CODE`, `*HEADER_<NAME>`, `*DELAY_RESPONSE_MILLI_SEC` — the delay accepts a plain integer or a `(start,end)` range for a random delay

### Core Domain Model

- **EndPoint** (`internal/models/endpoint.go`) — mock endpoint definition. Cached in memory keyed `{collection}_{name}_{method}` (all lowercased). `ActualURL` is required even for pure mocks; its parsed host/path drive the optional proxy mode and the derived `MockUrl`.
- **EndPointResponse** (`internal/models/endpoint_response_model.go`) — one response variant of an endpoint.
- **ApiCall** (`internal/models/api_call.go`) — per-request state: request flat map, per-variant response copies, accumulated log lines, final status/headers/body. When a condition group sets `CallActualEndPoint`, `Finalize()` proxies to `ActualURL` and uses the real response instead.
- **ConditionGroup** (`internal/models/condition_group.go`) — conditions ANDed together (short-circuits on first failure). On success it selects a `ResponseID` and applies its `ConditionGroupParameters` to response params. All groups are evaluated in name order; earlier assignments win.
- **Condition** (`internal/models/condition.go`) — one request param vs. a literal. Operators are exactly the keys of `models.OperatorFuncMap` (`internal/models/operators.go`): `EQUALS_TO`, `NOT_EQUALS_TO`, `LESS_THAN`, `LESS_THAN_OR_EQUALS_TO`, `GREATER_THAN`, `GREATER_THAN_OR_EQUALS_TO`, `CONTAINS`, `STARTS_WITH`, `ENDS_WITH` — no regex. A missing request key fails the condition; an unknown operator *passes* it.
- **Collection** — groups endpoints; its name is the first path segment of the mock URL (defaults to `V1`).

### Data Storage

BoltDB files in `db/` (created at startup):
- `db/internal.db` — buckets: `endpoints`, `users`, `usersverification`, `userspasword` (sic), `collections`, `endpointconditions`, `endpointconditionsgroup`, `responseparam`, `requestparam`, plus the scs/boltstore session store
- `db/log_YYYYMMDD.db` — that day's API call logs (bucket `apilogs`); `models.DailyDataCleanup` runs nightly at 21:30 via gocron

Repository conventions to follow when adding models:
- Bucket name comes from an unexported `getTableName() []byte` method; keys are always `strings.ToUpper(id)`.
- Child records embed the parent ID in the key (`<ownerID>_<something>`) and are listed by `bytes.HasPrefix` cursor scans — so IDs must never contain a stray `_` boundary that breaks a prefix.
- `Get`/`List` eagerly hydrate children (request params, response params, condition groups) after unmarshalling.

### Caching & Concurrency

- `app.GetEndPoint` (`cmd/web/cache.go`) rebuilds the **entire** endpoint cache from the DB on a miss or when `invalidEndPointCache` is set; mutations call `app.invalidateEndPointCache()` rather than patching a single entry.
- `EndPointModel.Save` fans out the request/response/header param rebuilds across goroutines with a `sync.WaitGroup`.
- Request-path goroutines (log saving, call-log append) are wrapped in `concurrent.Recoverer`; long-lived goroutines use `concurrent.RecoverAndRestart`.

### Application Bootstrap

`main()` (`cmd/web/main.go`) creates `db/`, `lic/`, `env/`, opens both Bolt files, then `baseAppConfig` (`cmd/web/config.go`) builds the `application` struct holding every dependency (models, session manager, template cache, loggers). All handlers are methods on `*application`. `parameters.Load()` (`cmd/web/cliParameters.go`) parses flags first, then lets env vars override.

Test mode (`DEBUG=true`) caps endpoints at 50 globally / 2 per user and enables an extra aggressive cleanup job.

### Templates

`newTemplateCache()` globs `html/pages/*.tmpl`, `html/accounts/*.tmpl`, `html/emails/*.tmpl`, `html/public/*.tmpl` from the embedded FS and parses each with all four base templates plus `html/partials/*.tmpl`. Which base is executed is chosen **by page filename prefix** in `render()`: `account_*` → `account_base`, `public_*` → `public_base`, `email_*` → `email_base` (in `templateToString`), everything else → `base`. Data passed to templates must be a field on `templateData` (`cmd/web/templates.go`); custom funcs live in `cmd/web/templatesFunctions.go`.

## Environment Configuration

Create `env/.env` (not checked in, no sample file exists). Vars actually read by the code: `PORT`, `DOMAIN`, `DEBUG`, `HTTPS` (defaults to **true** when unset), `USELETSENCRYPT`, `REQUESTS_PER_HOUR_BY_IP`, `REQUESTS_PER_HOUR_BY_USER`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`. Boolean vars accept `TRUE`/`YES`/`Y` (case-insensitive). Note `ALLOWEDORIGINS` appears in `readme.md` but is never read — CORS origins are hardcoded to `https://*`, `http://*` in `cmd/web/routes.go`.

Default superuser (created on startup, overridable with `-superuseremail` / `-superuserpwd`): `admin2@example.com` / `adminpass`

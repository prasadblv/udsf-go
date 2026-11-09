# Architecture

## Overview

```
 NF clients ──HTTPS/HTTP2──▶ ┌───────────────────────────── udsf ─────────────────────────────┐
                             │ Gin engine                                                       │
                             │  otelgin → RouteLatency → Recovery → shutdown gate →             │
                             │  Redis gate → CORS → pprof                                       │
                             │   ├─ /healthz /readyz /dbhealthz                                 │
                             │   └─ /nudsf-dr/*path ──▶ generated gorilla/mux router            │
                             │                           └─ controllers → services              │
                             │                                              └─ database.Service │
                             └───────────────────────────────────────────────┬──────────────────┘
                                                                             ▼
                                                                          Redis
                       OTLP/gRPC ◀── traces + metrics ── otel-collector ──▶ VictoriaMetrics / VictoriaTraces
```

## Components

| Package | Responsibility |
|---------|----------------|
| `cmd/api` | Parses `-config`, loads configuration, initialises tracing and metrics, starts the TLS server and handles `SIGINT`/`SIGTERM`. |
| `internal/config` | Defines `UdsfConfig`, applies defaults with Viper, validates `server.mode`, builds client `tls.Config` objects for Redis and OTLP. |
| `internal/server` | Creates the `http.Server`, registers middleware and routes, implements health handlers and the 503 gates. |
| `internal/database` | `Service` interface and Redis implementation (create, get, update, delete, scan, multi-get, ping, health). |
| `internal/observability` | OTLP trace and metric providers, samplers, resource attributes, and the route latency middleware. |
| `internal/udsf/api` | Controllers (generated) and services (hand-written business logic) per API group. |
| `internal/udsf/model` | Generated data types from the OpenAPI definition. |
| `internal/udsf/route` | Generated gorilla/mux router assembled from the controllers. |
| `internal/udsf/util` | Key construction, block (de)serialisation, multipart writer, logging, response helpers. |

## Request flow

1. The Gin engine receives the request. When OpenTelemetry is enabled, `otelgin` starts a span and `RouteLatency` times the request.
2. `gin.Recovery` converts panics into 500 responses.
3. `rejectReqWhenShuttingDown` returns `503` for everything except `/healthz` once shutdown has started.
4. `rejectReqWhenRedisDown` returns `503` (with `Retry-After: 5`) for everything except `/healthz` while Redis does not answer `PING`. The probe result is cached for 1 second and only one goroutine probes at a time, so a request burst costs one `PING`.
5. CORS allows `http://localhost:5173` with credentials.
6. Requests under `/nudsf-dr/*` are matched against the generated gorilla/mux router. The matched route template replaces Gin's wildcard path in the span name and the `http.route` metric label.
7. The controller parses path, query, header and body parameters and calls the matching service method, which returns an `ImplResponse` (status code and body).

Only paths under `/nudsf-dr/` are forwarded to the generated router; see the known limitations below.

## Storage layout

Blocks are stored as Redis string values.

- **Key:** `udsf:{realmId}:{storageId}:{recordId}:{blockId}`
- **Value:** `<content-type>\n<raw bytes>`. The first newline separates the media type from the payload; the payload is never interpreted. A missing `Content-Type` is stored as `application/octet-stream`.
- **Listing:** a record's blocks are found with an incremental `SCAN` using the pattern `udsf:{realmId}:{storageId}:{recordId}:*` (never `KEYS`). Glob characters in identifiers are escaped. Values are then fetched with a multi-get and sorted by key.
- **Expiry:** blocks are created without a TTL; updates preserve the existing TTL.

Because identifiers are joined with `:`, identifiers should not contain `:`.

## Server behaviour

- TLS certificate and key are loaded at start-up; a failure stops the process with a hint to run `./generate-cert.sh`.
- ALPN advertises `h2` and `http/1.1`; minimum TLS version is 1.2.
- On `SIGINT`/`SIGTERM` the server stops accepting connections, drains in-flight requests for up to `server.shutdown_timeout` seconds, then flushes traces and metrics. A second signal forces exit.
- `pprof` handlers are exposed under `/debug/pprof`. Restrict network access to the port in shared environments.

## Known limitations

- Only the Block CRUD service is implemented; the other `Nudsf_DataRepository` services return `501 Not Implemented`.
- The Gin catch-all route is `/nudsf-dr/*path`, so requests to `/nudsf-timer/...` do not reach the generated router until that route is added in `internal/server/routes.go`.
- Conditional request headers (`If-Match`, `If-None-Match`, `If-Modified-Since`) and `supported-features` are accepted but not evaluated.
- Block deletion does not yet update a parent Record, because Record storage is not implemented.

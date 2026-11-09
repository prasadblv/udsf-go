# udsf-go

A Go implementation of the **Unstructured Data Storage Function (UDSF)** of the 5G Core, exposing the 3GPP
`Nudsf_DataRepository` and `Nudsf_Timer` service APIs (TS 29.598) over HTTPS/HTTP2, with Redis as the backing store.

Background on the network function: <https://5gctextbook.com/en/nf/udsf/>

## Features

- HTTPS server (TLS 1.2+) with HTTP/2 and HTTP/1.1, built on Gin and a generated OpenAPI router.
- **Block CRUD** fully implemented: create/replace, read, list (`multipart/parallel`) and delete of opaque blocks.
- Redis storage with configurable pool, timeouts and optional TLS / mTLS.
- Health endpoints (`/healthz`, `/readyz`, `/dbhealthz`) and automatic `503` while Redis is down or the server is shutting down.
- Graceful shutdown that drains in-flight requests and flushes telemetry.
- OpenTelemetry traces, metrics and logs (OTLP/gRPC), including a per-route latency histogram.
- YAML configuration, Docker image, and a Docker Compose stack with Redis, OpenTelemetry Collector, VictoriaMetrics, VictoriaTraces and VictoriaLogs.
- Shell functional test and k6 load tests.

## API implementation status

| API group | Base path | Status |
|-----------|-----------|--------|
| Block CRUD | `/nudsf-dr/v1/{realmId}/{storageId}/records/{recordId}/blocks[/{blockId}]` | Implemented |
| Record CRUD / search | `/nudsf-dr/v1/{realmId}/{storageId}/records[/{recordId}]` | Routes registered, not yet implemented |
| Meta schema CRUD | `/nudsf-dr/v1/{realmId}/{storageId}/meta-schemas/{schemaId}` | Routes registered, not yet implemented |
| Notification subscriptions | `/nudsf-dr/v1/{realmId}/{storageId}/subs-to-notify[/{subscriptionId}]` | Routes registered, not yet implemented |
| Timers | `/nudsf-timer/v1/{realmId}/{storageId}/timers[/{timerId}]` | Not yet implemented, and not yet routed by the server |

See [docs/api.md](docs/api.md) for details.

## Quick start

### Prerequisites

- Go (version in `go.mod`)
- A reachable Redis (7.x recommended), or Docker for the Compose stack
- `openssl` to generate a development certificate

### Run locally

```bash
./generate-cert.sh                 # creates certs/server.pem and certs/server.key
docker run -d --name redis -p 6379:6379 redis:7.2.4
make run                           # uses config/udsf_config.yaml
```

Or build and run a binary with an explicit config file:

```bash
make build
./udsf -config config/udsf_config.yaml
```

Try it:

```bash
curl --http2 -k https://127.0.0.1:9443/healthz

curl --http2 -k -X PUT -H 'Content-Type: application/json' \
  -d '{"hello":"world"}' \
  https://127.0.0.1:9443/nudsf-dr/v1/Realm01/Storage01/records/Record01/blocks/Block01

curl --http2 -k https://127.0.0.1:9443/nudsf-dr/v1/Realm01/Storage01/records/Record01/blocks/Block01
```

### Run with Docker Compose

```bash
./generate-cert.sh
make docker-run                    # builds the image and starts the full stack
make docker-down                   # stops it
```

The stack uses `config/udsf_config.docker.yaml`, which enables OpenTelemetry and points to the other containers.

| Service | Purpose | Default host port |
|---------|---------|-------------------|
| `udsf` | This application (HTTPS) | 9443 |
| `redis_bp` | Redis data store | internal |
| `otel-collector` | OTLP receiver | internal |
| `victoria-metrics` | Metrics storage / UI (`/vmui`) | 8428 |
| `victoria-traces` | Trace storage / UI (`/select/vmui`) | 10428 |
| `victoria-logs` | Log storage / UI (`/select/vmui`) | 9428 |

Ports and resource limits can be overridden through the environment: `UDSF_PORT`, `UDSF_CPUS`, `UDSF_MEMORY`,
`REDIS_CPUS`, `REDIS_MEMORY`, `VM_PORT`, `VM_RETENTION`, `VT_PORT`, `VT_RETENTION`, `VL_PORT`, `VL_RETENTION`.

## Command line

```
udsf [-config <file>]     (shorthand: -c <file>)
```

Without `-config`, `config/udsf_config.yaml` is used; if that default file is missing, built-in defaults apply.
An explicitly given config file must exist. Configuration is read from the YAML file only (environment variables are not read).
See [docs/configuration.md](docs/configuration.md).

## Make targets

| Target | Description |
|--------|-------------|
| `make all` | Build, then run the tests |
| `make build` | Build the `udsf` binary |
| `make run` | Run from source |
| `make test` | Run all tests (`go test ./... -v`) |
| `make itest` | Run the Redis integration tests (needs Docker; uses Testcontainers) |
| `make docker-run` / `make docker-down` | Start / stop the Compose stack |
| `make watch` | Live reload with [air](https://github.com/air-verse/air) |
| `make clean` | Remove the built binary |

## Documentation

- [Architecture](docs/architecture.md): components, request flow, storage layout
- [API reference](docs/api.md): endpoints, status codes, examples
- [Configuration](docs/configuration.md): every setting and its default
- [Observability](docs/observability.md): health probes, tracing, metrics, logging
- [Development](docs/development.md): project layout, testing, load tests, regenerating code

## Repository layout

```
cmd/api/                 Application entry point
config/                  Example configuration (local and Docker)
internal/config/         Configuration loading and TLS helpers
internal/database/       Redis-backed storage service
internal/observability/  OpenTelemetry tracing and metrics
internal/server/         HTTP server, middleware, health routes
internal/udsf/           OpenAPI-generated API, models, router, and utilities
openapi/                 OpenAPI definition
spec/                    3GPP TS 29.598 specification
functional-tests/        curl-based functional test
k6/                      Load test scripts
observability/           OpenTelemetry Collector configuration
```

## License

Apache License 2.0. See [LICENSE](LICENSE).

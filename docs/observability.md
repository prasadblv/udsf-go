# Observability

## Health endpoints

| Endpoint | Behaviour |
|----------|-----------|
| `GET /healthz` | Liveness. Always `200`, even during shutdown or a Redis outage |
| `GET /readyz` | Readiness. `200` while the process serves traffic |
| `GET /dbhealthz` | Redis status and statistics as a flat JSON object; `503` if the database is not configured |

While Redis does not answer `PING`, or after shutdown has begun, every other endpoint answers `503` with a
`Retry-After: 5` header, so load balancers can route around the instance. Because `/readyz` itself passes through the
gate, it also returns `503` in those states.

`/dbhealthz` fields include `redis_status` (`up`/`down`), `redis_message`, `redis_version`, `redis_mode`,
`redis_connected_clients`, `redis_used_memory`, `redis_used_memory_peak`, `redis_max_memory`, `redis_uptime_in_seconds`
and connection pool figures (`redis_total_connections`, `redis_idle_connections`, `redis_stale_connections`,
`redis_hits_connections`, `redis_misses_connections`, `redis_timeouts_connections`, `redis_pool_size_percentage`).
`redis_message` warns about high client counts, stale connections, high memory use, recent restarts and pool pressure.

## Logging

- Application logs go to standard output and error.
- Gin access and recovery logs are also written to `log.log_file` when set.
- `log.level` controls the verbosity of the service-layer logger (`debug`, `info`, `warn`, `error`).
- The shutdown sequence and Redis availability changes are logged; repeated rejections while Redis is down are rate-limited to one warning every 5 seconds.

## OpenTelemetry diagnostics

The telemetry pipeline logs its own state through the normal logger, so problems are visible without a collector:

| When | Level | Message |
|------|-------|---------|
| Start-up, per signal | INFO | `otel tracing/metrics/logs enabled: endpoint=... tls=... service=...` (tracing adds sampler, metrics adds export interval) |
| Start-up, signal off | INFO | `otel <signal> disabled (<setting>)` |
| Misconfiguration | WARN | endpoint contains a scheme (the gRPC exporter expects `host:port`), `tls.insecure_skip_verify` on, or `redis.enable_opentelemetry` set while `open_telemetry.enabled` is false |
| Failed export | WARN | `otel: <error> (N similar errors suppressed)`, at most once every 10 seconds |
| Shutdown | INFO / WARN | `otel <signal> flushed (<duration>)`, or a shutdown error with the time spent |

## Log export (VictoriaLogs)

Set `open_telemetry.enabled: true` and `open_telemetry.logs.enabled: true` to also ship logs over OTLP/gRPC to the
collector, which forwards them to VictoriaLogs. Console logging is unchanged.

- Covers the service-layer logger (`util.LogInfo` and friends) and the standard `log` package output (start-up, Redis availability, shutdown). Gin access logs are not exported.
- The `log.level` setting applies to exported logs as well.
- Records carry the same resource attributes as traces and metrics (`service.name`, `service.version`, and so on).
- Pending logs are flushed during graceful shutdown.
- When logs export is on, standard `log` lines on the console use the structured `time=... level=... msg=...` format.

Query in the VictoriaLogs UI at <http://localhost:9428/select/vmui>, for example `service.name:udsf AND _msg:~"block"`,
or with `curl http://localhost:9428/select/logsql/query -d 'query=service.name:udsf' -d limit=20`.

## Tracing

Enable with `open_telemetry.enabled: true`. Spans are exported over OTLP/gRPC to `open_telemetry.otlp_endpoint`.

- Inbound requests are traced by `otelgin`. Spans for the OpenAPI routes are renamed to `METHOD /route/template` and carry `http.route`.
- With `redis.enable_opentelemetry: true`, Redis commands produce client spans. `ping`, `hello`, `client` and `select` are skipped.
- Resource attributes: `service.name`, `service.version`, `library.language=go`, plus `open_telemetry.attributes`.
- Sampling is set with `open_telemetry.sampler`. Use a `*traceidratio` sampler to reduce volume under load.
- Buffered spans are flushed during graceful shutdown.

## Metrics

Metrics are pushed over OTLP/gRPC every 15 seconds.

| Metric | Source | Notes |
|--------|--------|-------|
| `http.server.*` | `otelgin` | Standard HTTP server metrics, without a route label |
| `udsf.http.route.duration` | UDSF | Histogram in seconds. Labels: `http.request.method`, `http.route`, `http.response.status_code`. Buckets: 5 ms to 10 s. Unmatched requests use `unmatched` |
| Redis pool and command metrics | `redisotel` | Present when `redis.enable_opentelemetry` is true |

## Local stack

`docker compose up` (or `make docker-run`) starts the collector pipeline defined in `observability/otel-collector.yaml`:

```
udsf ──OTLP gRPC :4317──▶ otel-collector ──▶ VictoriaMetrics  (metrics, http://localhost:8428/vmui)
                                         ├─▶ VictoriaTraces   (traces,  http://localhost:10428/select/vmui)
                                         └─▶ VictoriaLogs     (logs,    http://localhost:9428/select/vmui)
```

VictoriaTraces also exposes a Jaeger-compatible query API at `http://localhost:10428/select/jaeger`.
VictoriaMetrics is started with `-opentelemetry.usePrometheusNaming=true`, so dots in metric names appear as underscores
(for example `udsf_http_route_duration_seconds_bucket`).

Retention is set with `VM_RETENTION` (months, default `1`), `VT_RETENTION` (default `7d`) and `VL_RETENTION` (default `7d`).

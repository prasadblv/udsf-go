# Configuration

UDSF is configured from a single YAML file. Environment variables are not read.

- Pass the file with `-config <path>` (or `-c <path>`).
- Without the flag, `config/udsf_config.yaml` is used. If that default file does not exist, built-in defaults apply.
- A file given explicitly must exist, otherwise start-up fails.
- Any key you omit takes the default listed below.

Two ready-made files are provided:

| File | Use |
|------|-----|
| `config/udsf_config.yaml` | Local development (debug mode, local Redis, telemetry off) |
| `config/udsf_config.docker.yaml` | Docker Compose (release mode, `redis_bp` host, telemetry on, stdout logging, certificates under `/certs`) |

Time values are in seconds unless noted.

## `server`

| Key | Default | Description |
|-----|---------|-------------|
| `https_port` | `:9443` | Listen address. A bare port such as `9443` gets a `:` prefix |
| `mode` | `release` | Gin mode: `debug`, `release` or `test`. Any other value is rejected |
| `read_timeout` | `10` | Request read timeout |
| `write_timeout` | `30` | Response write timeout |
| `idle_timeout` | `60` | Keep-alive idle timeout |
| `shutdown_timeout` | `5` | Time allowed to drain in-flight requests and flush telemetry on shutdown |
| `max_header_bytes` | `1048576` | Maximum request header size. Leave empty for the default |
| `cert_pem_location` | `certs/server.pem` | TLS certificate (PEM) |
| `cert_key_location` | `certs/server.key` | TLS private key (PEM) |

## `app`

| Key | Default | Description |
|-----|---------|-------------|
| `name` | `UDSF` | Application name; used as the service name when `open_telemetry.service_name` is empty |

## `redis`

| Key | Default | Description |
|-----|---------|-------------|
| `host` | `127.0.0.1` | Redis host |
| `port` | `6379` | Redis port |
| `password` | empty | Redis password |
| `database` | `0` | Redis database number |
| `dial_timeout` | `5` | Connect timeout |
| `read_timeout` | `3` | Socket read timeout |
| `write_timeout` | `3` | Socket write timeout |
| `pool_size` | `10 × GOMAXPROCS` | Maximum connections |
| `min_idle_conns` | `0` | Idle connections kept open |
| `max_idle_conns` | `0` | Maximum idle connections |
| `conn_max_idletime` | `30` | Minutes before an idle connection is closed |
| `conn_max_lifetime` | `0` | Minutes before a connection is recycled (`0` = no limit) |
| `enable_opentelemetry` | `false` | Add Redis client spans and pool metrics. Needs `open_telemetry.enabled: true` to have any effect |
| `tls.*` | see below | Optional TLS to Redis |

## `open_telemetry`

| Key | Default | Description |
|-----|---------|-------------|
| `enabled` | `false` | Turn tracing and metrics export on |
| `service_name` | `udsf` | `service.name` resource attribute |
| `service_version` | `0.0.1` | `service.version` resource attribute |
| `otlp_endpoint` | empty | OTLP/gRPC collector as `host:port`, for example `otel-collector:4317` |
| `jaeger_endpoint` | empty | Fallback used only when `otlp_endpoint` is empty |
| `otlp_headers` | empty | Reserved |
| `otlp_compression` | `gzip` | Reserved |
| `otlp_timeout` | `10` | Timeout for each export request |
| `attributes` | none | Extra resource attributes added to every span and metric. Keys are lower-cased |
| `logs.enabled` | `false` | Export application logs over OTLP, in addition to the console. Needs `enabled: true` |
| `sampler.type` | `always_on` | `always_on`, `always_off`, `traceidratio`, `parentbased_always_on`, `parentbased_always_off`, `parentbased_traceidratio`. Unknown values fall back to `always_on` with a warning |
| `sampler.ratio` | `1.0` | Sampling ratio from 0.0 to 1.0 for the `*traceidratio` types; out-of-range values are clamped |
| `tls.*` | see below | Optional TLS to the collector |

## `log`

| Key | Default | Description |
|-----|---------|-------------|
| `level` | `info` | `debug`, `info`, `warn` or `error` |
| `log_file` | `logs/udsf.log` | Also write Gin logs to this file. Empty or `stdout` logs to the console only (recommended in containers) |

## Client TLS blocks

`redis.tls` and `open_telemetry.tls` share the same fields. TLS is off by default.

| Key | Default | Description |
|-----|---------|-------------|
| `enabled` | `false` | Use TLS for this connection. For OTLP, `false` means plaintext gRPC |
| `ca_cert_location` | empty | PEM CA bundle used to verify the server. Empty uses system roots |
| `client_cert_location` | empty | Client certificate for mutual TLS |
| `client_key_location` | empty | Client key; must be set together with the certificate |
| `server_name` | empty | Name used for certificate verification. Defaults to `redis.host`, or the host of `otlp_endpoint` |
| `insecure_skip_verify` | `false` | Skip server certificate verification. Development only |

The minimum TLS version for client connections is 1.2. Invalid TLS settings are reported at start-up.

## Server certificate

`./generate-cert.sh [output_dir]` creates a self-signed certificate and key (default directory `certs/`). It refuses to
overwrite existing files.

| Variable | Default | Meaning |
|----------|---------|---------|
| `CERT_CN` | `localhost` | Common name and primary DNS name |
| `CERT_DAYS` | `365` | Validity in days |
| `CERT_SANS` | none | Extra subject alternative names, comma separated, e.g. `DNS:udsf.local,IP:10.0.0.5` |

Use a certificate from your own CA in production.

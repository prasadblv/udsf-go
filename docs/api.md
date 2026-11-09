# API reference

The service follows 3GPP TS 29.598. The OpenAPI definition is in [`openapi/openapi.yaml`](../openapi/openapi.yaml) and the
specification is in [`spec/`](../spec). All endpoints are served over HTTPS; HTTP/2 is negotiated through ALPN.

Path parameters used below:

| Parameter | Meaning |
|-----------|---------|
| `realmId` | Realm (namespace) of the data, e.g. `Realm01` |
| `storageId` | Storage within the realm, e.g. `Storage01` |
| `recordId` | Record identifier |
| `blockId` | Block identifier within the record |

## Block CRUD (implemented)

Base path: `/nudsf-dr/v1/{realmId}/{storageId}/records/{recordId}/blocks`

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| `GET` | `/blocks` | List all blocks of a record | `200` `multipart/parallel` |
| `GET` | `/blocks/{blockId}` | Read one block | `200` with the stored media type |
| `PUT` | `/blocks/{blockId}` | Create or replace a block | `201` created, `204` replaced, `200` replaced with `get-previous=true` |
| `DELETE` | `/blocks/{blockId}` | Delete a block | `204` |

### Block content

Blocks are opaque. The request body is stored byte for byte together with its `Content-Type`, and returned unchanged
on read. Without a `Content-Type` the block is stored as `application/octet-stream`.

### List response

`GET .../blocks` returns `multipart/parallel`. Each part carries `Content-Type`, `Content-Id` (the block id) and
`Content-Transfer-Encoding: binary`. A record with no blocks yields an empty multipart body. Parts are ordered by block id.

### Query parameters

| Parameter | Applies to | Description |
|-----------|------------|-------------|
| `get-previous` | `PUT`, `DELETE` | On `PUT` of an existing block, return the previous content with `200` instead of `204` |

### Error responses

Errors use `application/problem+json` (`ProblemDetails`).

| Status | Cause |
|--------|-------|
| `400` | Missing request body on `PUT` |
| `404` | Block does not exist (`GET`, `DELETE`) |
| `500` | Redis failure or a corrupt stored value |
| `503` | Redis unreachable or server shutting down (`Retry-After` header set) |

### Examples

```bash
BASE=https://127.0.0.1:9443/nudsf-dr/v1/Realm01/Storage01/records/Record01/blocks
CURL="curl --http2 -k"

# Create (201)
$CURL -X PUT -H 'Content-Type: application/json' -d '{"name":"first","value":42}' $BASE/Block01

# Replace (204), or return the previous value (200)
$CURL -X PUT -H 'Content-Type: application/json' -d '{"value":43}' $BASE/Block01
$CURL -X PUT -H 'Content-Type: application/json' -d '{"value":44}' "$BASE/Block01?get-previous=true"

# Binary block
$CURL -X PUT -H 'Content-Type: application/octet-stream' --data-binary @file.bin $BASE/Block02

# Read / list / delete
$CURL $BASE/Block01
$CURL $BASE
$CURL -X DELETE $BASE/Block01
```

## Other Nudsf_DataRepository endpoints (not yet implemented)

Routes are registered and respond `501 Not Implemented`.

| Group | Method and path |
|-------|-----------------|
| Record CRUD | `GET`, `DELETE /records`; `GET`, `PUT`, `PATCH`, `DELETE /records/{recordId}`; `GET`, `PATCH /records/{recordId}/meta` |
| Meta schema | `GET`, `PUT`, `DELETE /meta-schemas/{schemaId}` |
| Notification subscriptions | `GET /subs-to-notify`; `GET`, `PUT`, `PATCH`, `DELETE /subs-to-notify/{subscriptionId}` |

All are under `/nudsf-dr/v1/{realmId}/{storageId}`.

## Nudsf_Timer endpoints (not yet implemented)

Base path: `/nudsf-timer/v1/{realmId}/{storageId}`

The generated router defines these routes, but the Gin engine currently forwards only `/nudsf-dr/*` to it, so
requests to `/nudsf-timer/...` return `404` until the Gin route is added (see [architecture](architecture.md#known-limitations)).

| Operation | Method and path |
|-----------|-----------------|
| Search timers | `GET /timers` |
| Delete timers | `DELETE /timers` |
| Get / start / update / stop a timer | `GET`, `PUT`, `PATCH`, `DELETE /timers/{timerId}` |

## Operational endpoints

| Path | Description |
|------|-------------|
| `GET /healthz` | Liveness; always answers `200 "OK"` and is never gated |
| `GET /readyz` | Readiness; `200 "OK"` |
| `GET /dbhealthz` | Redis health and statistics (see [observability](observability.md)) |
| `/debug/pprof/*` | Go profiling handlers |

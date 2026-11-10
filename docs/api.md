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

## Record CRUD (partly implemented)

Base path: `/nudsf-dr/v1/{realmId}/{storageId}/records/{recordId}`

| Method | Path | Description | Success |
|--------|------|-------------|---------|
| `PUT` | `/records/{recordId}` | Create or modify a record (and add or replace blocks) | `201` created, `204` modified, `200` modified with `get-previous=true` |
| `GET` | `/records/{recordId}` | Read a record | `200` `Record` (meta and block ids) |
| `GET` | `/records/{recordId}/meta` | Read a record's meta | `200` `RecordMeta` |
| `DELETE` | `/records/{recordId}` | Delete a record and all its blocks | `204`, or `200` with `get-previous=true` |

Not implemented yet (they answer `501`): `GET /records` (search), `DELETE /records` (bulk delete), `PATCH /records/{recordId}`
and `PATCH /records/{recordId}/meta`. The generated `PatchOperation` model is an empty struct, so JSON Patch operations
cannot be read until the model is fixed.

### Create or modify (`PUT`)

The body is `multipart/form-data`:

| Part | Required | Description |
|------|----------|-------------|
| `meta` | yes | JSON `RecordMeta`: `ttl` (date-time), `callbackReference` (URI), `tags` (`{"name": ["value", ...]}`) |
| `blocks` | no | One file part per block. The block id is the part's `Content-Id` header (angle brackets are stripped), falling back to the form filename. The part's `Content-Type` is stored with the block |

- A new record answers `201`. If it already exists the meta is replaced and the answer is `204`, or `200` with the previous record when `get-previous=true`.
- `If-None-Match: *` on an existing record answers `412`.
- A `ttl` in the past answers `400`. The TTL is applied when the record is created; later modifications keep the existing expiry.
- Blocks sent with the request are added or replaced. Existing blocks that are not part of the request are left untouched.
- An empty or duplicate block id answers `400` and nothing is written. If storing the blocks fails while creating a record, the record is rolled back.
- Only parts with a filename reach the service (Go's multipart parser treats parts without one as plain fields), for example `curl -F 'blocks=@file;filename=Block01;type=application/json'`.

### Read (`GET`)

`GET /records/{recordId}` returns `{"meta": {...}, "blocks": ["Block01", ...]}`. `blocks` lists the ids of the record's blocks, sorted;
fetch the content through the [Block API](#block-crud-implemented). `GET .../meta` returns only the meta.

### Delete (`DELETE`)

Removes the record, every block belonging to it, and its block index in one operation. With `get-previous=true` the deleted record's
meta is returned with `200`.

### Error responses

| Status | Cause |
|--------|-------|
| `400` | Missing or malformed `meta`, invalid `get-previous`, `ttl` in the past, or invalid blocks (missing or duplicate id) |
| `404` | Record does not exist (`GET`, `DELETE`) |
| `412` | `If-None-Match: *` and the record already exists |
| `500` | Redis failure or a corrupt stored value |
| `503` | Redis unreachable or server shutting down (`Retry-After` header set) |

### Examples

```bash
BASE=https://127.0.0.1:9443/nudsf-dr/v1/Realm01/Storage01/records/Record01
CURL="curl --http2 -k"

# Create a record with meta and one block (201)
$CURL -X PUT \
  -F 'meta={"callbackReference":"http://cb.example/one","tags":{"imsi":["001010000000001"]}};type=application/json' \
  -F 'blocks=@block.json;filename=Block01;type=application/json' $BASE

# Modify (204), or return the previous record (200)
$CURL -X PUT -F 'meta={"callbackReference":"http://cb.example/two"}' $BASE
$CURL -X PUT -F 'meta={}' "$BASE?get-previous=true"

# Read the record, its meta, and a block
$CURL $BASE
$CURL $BASE/meta
$CURL $BASE/blocks/Block01

# Delete the record and its blocks
$CURL -X DELETE $BASE
```

## Other Nudsf_DataRepository endpoints (not yet implemented)

Routes are registered and respond `501 Not Implemented`.

| Group | Method and path |
|-------|-----------------|
| Record CRUD (rest of the group) | `GET`, `DELETE /records`; `PATCH /records/{recordId}`; `PATCH /records/{recordId}/meta` |
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
| `/debug/pprof/*` | Go profiling handlers; served only when `server.enable_pprof` is `true`, otherwise `404` |

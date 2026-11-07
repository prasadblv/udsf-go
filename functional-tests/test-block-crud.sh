#!/usr/bin/env bash
# Exercises the Block CRUD endpoints defined in openapi/openapi.yaml.
#   PUT    /{realmId}/{storageId}/records/{recordId}/blocks/{blockId}
#   GET    /{realmId}/{storageId}/records/{recordId}/blocks/{blockId}
#   GET    /{realmId}/{storageId}/records/{recordId}/blocks
#   DELETE /{realmId}/{storageId}/records/{recordId}/blocks/{blockId}
#
# Usage: ./test-block-crud.sh [base_url]     (default https://127.0.0.1:9443)
# Requires: curl. jq is optional (pretty-prints / validates the list).

set -u

export UDSF_SERVER_HTTPS_PORT=:9443

export UDSF_REDIS_HOST=127.0.0.1
export UDSF_REDIS_PORT=6379
export UDSF_REDIS_PASSWORD=
export UDSF_REDIS_DATABASE=1

BASE_URL="${1:-${BASE_URL:-https://127.0.0.1:9443}}/nudsf-dr/v1"
REALM="Realm01"       # example values from openapi.yaml
STORAGE="Storage01"
RECORD="Record$(date +%s)"   # unique per run so reruns start clean
BLOCK1="Block01"
BLOCK2="Block02"
REC="$BASE_URL/$REALM/$STORAGE/records/$RECORD/blocks"

# Shared curl options: HTTP/2 over TLS, self-signed cert accepted (-k), silent.
# No -I: that sends header-only requests and drops the response body.
CURL_OPTS=(--http2 -k -s)

PASS=0; FAIL=0
BODY_FILE="$(mktemp)"; trap 'rm -f "$BODY_FILE"' EXIT

# call METHOD URL EXPECTED_STATUS [data]  -> sets $BODY
call() {
  local method="$1" url="$2" expected="$3" data="${4-}" status
  local args=("${CURL_OPTS[@]}" -o "$BODY_FILE" -w '%{http_code}' -X "$method" "$url")
  [[ -n "$data" ]] && args+=(-H 'Content-Type: application/json' --data "$data")
  status="$(curl "${args[@]}")"
  BODY="$(tr -d '\0' < "$BODY_FILE")"
  if [[ "$status" == "$expected" ]]; then
    PASS=$((PASS+1)); printf 'PASS  %-6s %s -> %s\n' "$method" "${url#"$BASE_URL"}" "$status"
  else
    FAIL=$((FAIL+1)); printf 'FAIL  %-6s %s -> %s (expected %s)\n' "$method" "${url#"$BASE_URL"}" "$status" "$expected"
  fi
  [[ -n "$BODY" ]] && printf '      body: %s\n' "$BODY"
}

# expect_contains DESCRIPTION NEEDLE   (checks $BODY)
expect_contains() {
  if [[ "$BODY" == *"$2"* ]]; then PASS=$((PASS+1)); echo "PASS  $1"
  else FAIL=$((FAIL+1)); echo "FAIL  $1 (missing: $2)"; fi
}

echo "== Target: $REC"

HDR_FILE="$(mktemp)"; trap 'rm -f "$BODY_FILE" "$HDR_FILE"' EXIT

# list_blocks  -> GET the block list; sets $STATUS, $CT (Content-Type), $BODY
# The list response is multipart/parallel per openapi.yaml (one part per block).
list_blocks() {
  STATUS="$(curl "${CURL_OPTS[@]}" -D "$HDR_FILE" -o "$BODY_FILE" -w '%{http_code}' "$REC")"
  CT="$(grep -i '^content-type:' "$HDR_FILE" | head -1 | tr -d '\r')"
  BODY="$(tr -d '\0' < "$BODY_FILE")"
}

check() {  # check DESCRIPTION CONDITION_EXIT_CODE
  if [[ "$2" == "0" ]]; then PASS=$((PASS+1)); echo "PASS  $1"; else FAIL=$((FAIL+1)); echo "FAIL  $1"; fi
}

echo "-- list empty record (multipart/parallel with no parts)"
list_blocks
printf '      status: %s  %s\n' "$STATUS" "$CT"
[[ "$STATUS" == "200" ]]; check "list empty -> 200" $?
shopt -s nocasematch
[[ "$CT" == *multipart/parallel* && "$CT" == *boundary=* ]]; check "list Content-Type is multipart/parallel with boundary" $?
shopt -u nocasematch
[[ "$(grep -ci '^content-id:' "$BODY_FILE" || true)" == "0" ]]; check "list has no parts" $?

echo "-- create blocks (Block schema accepts any JSON value)"
call PUT "$REC/$BLOCK1" 201 '{"name":"first","value":42}'
call PUT "$REC/$BLOCK2" 201 '"QmxvY2sgY29udGVudA=="'

echo "-- get block"
call GET "$REC/$BLOCK1" 200
expect_contains "block1 content" '"value":42'

echo "-- modify existing block"
call PUT "$REC/$BLOCK1" 204 '{"name":"first","value":43}'
call GET "$REC/$BLOCK1" 200
expect_contains "block1 updated" '"value":43'

echo "-- modify with get-previous=true returns the previous value (200)"
call PUT "$REC/$BLOCK1?get-previous=true" 200 '{"name":"first","value":44}'
expect_contains "previous value returned" '"value":43'

echo "-- list blocks (multipart/parallel, one part per block)"
list_blocks
printf '      status: %s  %s\n' "$STATUS" "$CT"
[[ "$STATUS" == "200" ]]; check "GET list -> 200" $?
shopt -s nocasematch
[[ "$CT" == *multipart/parallel* && "$CT" == *boundary=* ]]; check "list Content-Type is multipart/parallel with boundary" $?
shopt -u nocasematch
expect_contains "list has block1 content" '"name":"first"'
expect_contains "list has block2 content" 'QmxvY2sgY29udGVudA=='
[[ "$(grep -ci '^content-id:' "$BODY_FILE" || true)" == "2" ]]; check "2 parts with Content-Id headers" $?
grep -qi '^content-id: *Block01' "$BODY_FILE"; check "part Content-Id carries the block id" $?

echo "-- octet-stream blocks (binary content, all 256 byte values)"
BIN_IN="$(mktemp)"; BIN_OUT="$(mktemp)"
trap 'rm -f "$BODY_FILE" "$HDR_FILE" "$BIN_IN" "$BIN_OUT"' EXIT
BLOCK3="Block03"
LC_ALL=C awk 'BEGIN{for(i=0;i<256;i++)printf "%c",i}' > "$BIN_IN"
[[ "$(wc -c < "$BIN_IN" | tr -d ' ')" == "256" ]] || echo "WARN  could not generate 256-byte payload"

status="$(curl "${CURL_OPTS[@]}" -o "$BODY_FILE" -w '%{http_code}' -X PUT -H 'Content-Type: application/octet-stream' \
  --data-binary @"$BIN_IN" "$REC/$BLOCK3")"
if [[ "$status" == "201" ]]; then PASS=$((PASS+1)); echo "PASS  PUT octet-stream block -> 201"
else FAIL=$((FAIL+1)); echo "FAIL  PUT octet-stream block -> $status (expected 201)"; fi

status="$(curl "${CURL_OPTS[@]}" -o "$BIN_OUT" -w '%{http_code}' -H 'Accept: application/octet-stream' "$REC/$BLOCK3")"
if [[ "$status" == "200" ]]; then PASS=$((PASS+1)); echo "PASS  GET octet-stream block -> 200"
else FAIL=$((FAIL+1)); echo "FAIL  GET octet-stream block -> $status (expected 200)"; fi
if cmp -s "$BIN_IN" "$BIN_OUT"; then PASS=$((PASS+1)); echo "PASS  octet-stream round-trip is byte-identical"
else FAIL=$((FAIL+1)); echo "FAIL  octet-stream round-trip differs from the uploaded bytes"; fi

echo "-- multipart/parallel list now includes the binary part"
list_blocks
printf '      status: %s  %s\n' "$STATUS" "$CT"
[[ "$STATUS" == "200" ]]; check "list -> 200" $?
grep -qi '^content-type: *application/octet-stream' "$BODY_FILE"; check "a part has Content-Type: application/octet-stream" $?
[[ "$(grep -ci '^content-id:' "$BODY_FILE" || true)" == "3" ]]; check "3 parts with Content-Id headers" $?
echo "-- delete block"
call DELETE "$REC/$BLOCK1" 204
call GET "$REC/$BLOCK1" 404
call DELETE "$REC/$BLOCK1" 404

echo "-- cleanup"
call DELETE "$REC/$BLOCK2" 204
call DELETE "$REC/$BLOCK3" 204

echo
echo "Passed: $PASS  Failed: $FAIL"


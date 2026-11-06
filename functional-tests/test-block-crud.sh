#!/usr/bin/env bash
# Exercises the Block CRUD endpoints defined in openapi/openapi.yaml.
#   PUT    /{realmId}/{storageId}/records/{recordId}/blocks/{blockId}
#   GET    /{realmId}/{storageId}/records/{recordId}/blocks/{blockId}
#   GET    /{realmId}/{storageId}/records/{recordId}/blocks
#   DELETE /{realmId}/{storageId}/records/{recordId}/blocks/{blockId}
#
# Usage: ./test-block-crud.sh [base_url]     (default http://localhost:9090)
# Requires: curl. jq is optional (pretty-prints / validates the list).

set -u

export PORT=9090

export UDSF_DB_ADDRESS=127.0.0.1
export UDSF_DB_PORT=6379
export UDSF_DB_PASSWORD=
export UDSF_DB_DATABASE=1

BASE_URL="${1:-${BASE_URL:-http://localhost:9090}}/nudsf-dr/v1"
REALM="Realm01"       # example values from openapi.yaml
STORAGE="Storage01"
RECORD="Record$(date +%s)"   # unique per run so reruns start clean
BLOCK1="Block01"
BLOCK2="Block02"
REC="$BASE_URL/$REALM/$STORAGE/records/$RECORD/blocks"

PASS=0; FAIL=0
BODY_FILE="$(mktemp)"; trap 'rm -f "$BODY_FILE"' EXIT

# call METHOD URL EXPECTED_STATUS [data]  -> sets $BODY
call() {
  local method="$1" url="$2" expected="$3" data="${4-}" status
  local args=(-s -o "$BODY_FILE" -w '%{http_code}' -X "$method" "$url")
  [[ -n "$data" ]] && args+=(-H 'Content-Type: application/json' --data "$data")
  status="$(curl "${args[@]}")"
  BODY="$(cat "$BODY_FILE")"
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

echo "-- list empty record"
call GET "$REC" 200
# the model uses omitempty, so an empty list serializes as {}
if [[ "$BODY" == "{}" || "$BODY" == *'"blocks":[]'* ]]; then PASS=$((PASS+1)); echo "PASS  list is empty"
else FAIL=$((FAIL+1)); echo "FAIL  list should be empty"; fi

echo "-- create blocks (Block schema accepts any JSON value)"
call PUT "$REC/$BLOCK1" 201 '{"name":"first","value":42}'
call PUT "$REC/$BLOCK2" 201 '"QmxvY2sgY29udGVudA=="'

echo "-- get block"
call GET "$REC/$BLOCK1" 200
expect_contains "block1 content" '"value":42'

echo "-- modify existing block"
call PUT "$REC/$BLOCK1" 200 '{"name":"first","value":43}'
call GET "$REC/$BLOCK1" 200
expect_contains "block1 updated" '"value":43'

echo "-- list blocks"
call GET "$REC" 200
expect_contains "list has block1" '"name":"first"'
expect_contains "list has block2" 'QmxvY2sgY29udGVudA=='
command -v jq >/dev/null && echo "$BODY" | jq -e '.blocks | length == 2' >/dev/null \
  && { PASS=$((PASS+1)); echo "PASS  list length == 2"; } \
  || { command -v jq >/dev/null && { FAIL=$((FAIL+1)); echo "FAIL  list length != 2"; }; }

echo "-- delete block"
call DELETE "$REC/$BLOCK1" 204
call GET "$REC/$BLOCK1" 404
call DELETE "$REC/$BLOCK1" 404

echo "-- cleanup"
call DELETE "$REC/$BLOCK2" 204

echo
echo "Passed: $PASS  Failed: $FAIL"


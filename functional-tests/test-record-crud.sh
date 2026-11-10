#!/usr/bin/env bash
# Exercises the Record CRUD endpoints (the ones that are implemented).
#   PUT    /{realmId}/{storageId}/records/{recordId}        multipart/form-data: meta + blocks
#   GET    /{realmId}/{storageId}/records/{recordId}
#   GET    /{realmId}/{storageId}/records/{recordId}/meta
#   DELETE /{realmId}/{storageId}/records/{recordId}
# Search, bulk delete and PATCH (record and meta) are not implemented yet and are not tested.
#
# Usage: ./test-record-crud.sh [base_url]     (default https://127.0.0.1:9443)
# Requires: curl.

set -u

BASE_URL="${1:-${BASE_URL:-https://127.0.0.1:9443}}/nudsf-dr/v1"
REALM="Realm01"
STORAGE="Storage01"
RECORD="Record$(date +%s)"   # unique per run so reruns start clean
REC="$BASE_URL/$REALM/$STORAGE/records/$RECORD"

CURL_OPTS=(--http2 -k -s)

PASS=0; FAIL=0
BODY_FILE="$(mktemp)"; TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$BODY_FILE" "$TMP_DIR"' EXIT

# call METHOD URL EXPECTED_STATUS [curl args...]  -> sets $BODY
call() {
  local method="$1" url="$2" expected="$3" status
  shift 3
  status="$(curl "${CURL_OPTS[@]}" -o "$BODY_FILE" -w '%{http_code}' -X "$method" "$@" "$url")"
  BODY="$(tr -d '\0' < "$BODY_FILE")"
  if [[ "$status" == "$expected" ]]; then
    PASS=$((PASS+1)); printf 'PASS  %-6s %s -> %s\n' "$method" "${url#"$BASE_URL"}" "$status"
  else
    FAIL=$((FAIL+1)); printf 'FAIL  %-6s %s -> %s (expected %s)\n' "$method" "${url#"$BASE_URL"}" "$status" "$expected"
  fi
  [[ -n "$BODY" ]] && printf '      body: %s\n' "$BODY"
}

# put_record URL EXPECTED_STATUS META_JSON [block args: ID:CONTENT_TYPE:FILE ...]
# Sends multipart/form-data with a "meta" field and one "blocks" file part per block.
put_record() {
  local url="$1" expected="$2" meta="$3" b id ct file
  shift 3
  local form=(-F "meta=$meta;type=application/json")
  for b in "$@"; do
    id="${b%%:*}"; b="${b#*:}"; ct="${b%%:*}"; file="${b#*:}"
    form+=(-F "blocks=@$file;filename=$id;type=$ct;headers=\"Content-Id: $id\"")
  done
  call PUT "$url" "$expected" "${form[@]}"
}

contains() {  # contains DESCRIPTION NEEDLE   (checks $BODY)
  if [[ "$BODY" == *"$2"* ]]; then PASS=$((PASS+1)); echo "PASS  $1"
  else FAIL=$((FAIL+1)); echo "FAIL  $1 (missing: $2)"; fi
}

not_contains() {  # not_contains DESCRIPTION NEEDLE   (checks $BODY)
  if [[ "$BODY" != *"$2"* ]]; then PASS=$((PASS+1)); echo "PASS  $1"
  else FAIL=$((FAIL+1)); echo "FAIL  $1 (unexpected: $2)"; fi
}

check() {  # check DESCRIPTION CONDITION_EXIT_CODE
  if [[ "$2" == "0" ]]; then PASS=$((PASS+1)); echo "PASS  $1"; else FAIL=$((FAIL+1)); echo "FAIL  $1"; fi
}

echo "== Target: $REC"

JSON_A="$TMP_DIR/a.json";  printf '{"name":"first","value":42}' > "$JSON_A"
TEXT_B="$TMP_DIR/b.txt";   printf 'hello block' > "$TEXT_B"
JSON_A2="$TMP_DIR/a2.json"; printf '{"name":"first","value":43}' > "$JSON_A2"
TEXT_C="$TMP_DIR/c.txt";   printf 'third' > "$TEXT_C"
BIN_IN="$TMP_DIR/bin.dat"; BIN_OUT="$TMP_DIR/bin.out"
LC_ALL=C awk 'BEGIN{for(i=0;i<256;i++)printf "%c",i}' > "$BIN_IN"

echo "-- missing record"
call GET "$REC" 404
call GET "$REC/meta" 404
call DELETE "$REC" 404

echo "-- create record without blocks (201)"
put_record "$REC" 201 '{"callbackReference":"http://cb.example/one","tags":{"imsi":["001010000000001"]}}'
call GET "$REC" 200
contains "record has callbackReference" '"callbackReference":"http://cb.example/one"'
contains "record has tags" '"imsi":["001010000000001"]'
not_contains "record has no blocks" '"blocks":["'

echo "-- get meta"
call GET "$REC/meta" 200
contains "meta has callbackReference" '"callbackReference":"http://cb.example/one"'
contains "meta has tags" '"imsi":["001010000000001"]'

echo "-- modify record (204), then confirm"
put_record "$REC" 204 '{"callbackReference":"http://cb.example/two"}'
call GET "$REC/meta" 200
contains "meta updated" '"callbackReference":"http://cb.example/two"'
not_contains "old tags replaced" 'imsi'

echo "-- modify with get-previous=true returns the previous record (200)"
put_record "$REC?get-previous=true" 200 '{"callbackReference":"http://cb.example/three"}'
contains "previous meta returned" '"callbackReference":"http://cb.example/two"'
call GET "$REC/meta" 200
contains "new meta stored" '"callbackReference":"http://cb.example/three"'

echo "-- If-None-Match: * on an existing record fails (412)"
body_args=(-H 'If-None-Match: *' -F 'meta={};type=application/json')
call PUT "$REC" 412 "${body_args[@]}"
call GET "$REC/meta" 200
contains "record unchanged after 412" '"callbackReference":"http://cb.example/three"'

echo "-- ttl in the past is rejected (400)"
put_record "$BASE_URL/$REALM/$STORAGE/records/${RECORD}-expired" 400 '{"ttl":"2000-01-01T00:00:00Z"}'
call GET "$BASE_URL/$REALM/$STORAGE/records/${RECORD}-expired" 404

echo "-- malformed meta is rejected (400)"
put_record "$BASE_URL/$REALM/$STORAGE/records/${RECORD}-bad" 400 'not-json'
call GET "$BASE_URL/$REALM/$STORAGE/records/${RECORD}-bad" 404

echo "-- create record with blocks (json + text)"
REC2="$BASE_URL/$REALM/$STORAGE/records/${RECORD}-blocks"
put_record "$REC2" 201 '{"tags":{"k":["v"]}}' "Block01:application/json:$JSON_A" "Block02:text/plain:$TEXT_B"
call GET "$REC2" 200
contains "record lists Block01" '"Block01"'
contains "record lists Block02" '"Block02"'

echo "-- blocks are readable through the Block API with their content types"
call GET "$REC2/blocks/Block01" 200
contains "Block01 content" '"value":42'
call GET "$REC2/blocks/Block02" 200
contains "Block02 content" 'hello block'
ct="$(curl "${CURL_OPTS[@]}" -o /dev/null -w '%{content_type}' "$REC2/blocks/Block02")"
[[ "$ct" == text/plain* ]]; check "Block02 Content-Type is text/plain (got: $ct)" $?

echo "-- update upserts blocks: Block01 replaced, Block03 (binary) added, Block02 untouched"
put_record "$REC2" 204 '{"tags":{"k":["v2"]}}' "Block01:application/json:$JSON_A2" "Block03:application/octet-stream:$BIN_IN"
call GET "$REC2/blocks/Block01" 200
contains "Block01 replaced" '"value":43'
call GET "$REC2/blocks/Block02" 200
contains "Block02 still present" 'hello block'
curl "${CURL_OPTS[@]}" -o "$BIN_OUT" "$REC2/blocks/Block03"
cmp -s "$BIN_IN" "$BIN_OUT"; check "binary block round-trip is byte-identical" $?
call GET "$REC2" 200
contains "record lists Block03" '"Block03"'

echo "-- duplicate block ids are rejected (400) and nothing is written"
put_record "$REC2" 400 '{"tags":{"k":["dup"]}}' "Dup:text/plain:$TEXT_C" "Dup:text/plain:$TEXT_C"
call GET "$REC2/blocks/Dup" 404
call GET "$REC2/meta" 200
contains "meta unchanged after rejected update" '"k":["v2"]'

echo "-- delete record removes its blocks"
call DELETE "$REC2" 204
call GET "$REC2" 404
call GET "$REC2/blocks/Block01" 404
call GET "$REC2/blocks/Block03" 404

echo "-- delete with get-previous=true returns the deleted record (200)"
call DELETE "$REC?get-previous=true" 200
contains "deleted record returned" '"callbackReference":"http://cb.example/three"'
call GET "$REC" 404
call DELETE "$REC" 404

echo
echo "Passed: $PASS  Failed: $FAIL"
[[ "$FAIL" == "0" ]]

#!/usr/bin/env bash
# Demonstrates the Record CRUD API (and its blocks) with curl over HTTP/2 in verbose mode.
#   PUT    /nudsf-dr/v1/{realm}/{storage}/records/{record}        create / modify (multipart: meta + blocks)
#   GET    .../records/{record}                                    read record (meta + block ids)
#   GET    .../records/{record}/meta                               read meta only
#   DELETE .../records/{record}                                    delete the record and its blocks
# Search, bulk delete and PATCH are not implemented yet and are not shown.
#
# Usage: ./demo-record-crud.sh [base_url]         (default https://127.0.0.1:9443)
#        TRACE=1 ./demo-record-crud.sh            also dump request/response bodies (curl --trace-ascii),
#                                                 including the multipart request body
#
# Verbose mode (-v) prints the request line and headers (">") and the response status and headers ("<").
# Response bodies are printed after each call. TRACE=1 additionally shows the request body on the wire.

set -u

BASE_URL="${1:-${BASE_URL:-https://127.0.0.1:9443}}"
REALM="Realm01"
STORAGE="Storage01"
RECORD="DemoRecord$(date +%s)"   # unique per run
REC="$BASE_URL/nudsf-dr/v1/$REALM/$STORAGE/records/$RECORD"

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

if [[ "${TRACE:-0}" == "1" ]]; then VERBOSE=(--trace-ascii -); else VERBOSE=(-v); fi
CURL=(curl -sS --http2 -k "${VERBOSE[@]}")

step() { printf '\n\n================================================================\n# %s\n================================================================\n' "$1"; }
show() { printf '\n--- (response body is the text printed just above) ---\n'; }

echo "Target: $REC"

printf '{"name":"first","value":42}' > "$TMP/block1.json"
printf 'hello block' > "$TMP/block2.txt"
printf '{"name":"first","value":43}' > "$TMP/block1-v2.json"

step "1. Read a record that does not exist (expect 404 Not Found)"
"${CURL[@]}" "$REC"; show

step "2. Create a record with meta and two blocks (expect 201 Created)"
# multipart/form-data: part 'meta' is JSON RecordMeta; each 'blocks' part is one block whose id is its filename
# (or Content-Id header) and whose Content-Type is stored with the block.
META='{"callbackReference":"http://cb.example/one","tags":{"imsi":["001010000000001"]}}'
echo "meta part: $META"
echo "block parts: Block01 (application/json), Block02 (text/plain)"
"${CURL[@]}" -X PUT \
  -F "meta=$META;type=application/json" \
  -F "blocks=@$TMP/block1.json;filename=Block01;type=application/json" \
  -F "blocks=@$TMP/block2.txt;filename=Block02;type=text/plain" \
  "$REC"

step "3. Read the record (expect 200: meta plus the ids of its blocks)"
"${CURL[@]}" "$REC"; show

step "4. Read only the meta (expect 200)"
"${CURL[@]}" "$REC/meta"; show

step "5. Read a block of the record through the Block API (expect 200, original bytes and type)"
"${CURL[@]}" "$REC/blocks/Block02"; show

step "6. Modify the record: new meta and a replacement for Block01 (expect 204 No Content)"
META='{"callbackReference":"http://cb.example/two"}'
echo "meta part: $META"
"${CURL[@]}" -X PUT \
  -F "meta=$META;type=application/json" \
  -F "blocks=@$TMP/block1-v2.json;filename=Block01;type=application/json" \
  "$REC"

step "7. Confirm: meta replaced, Block01 replaced, Block02 untouched"
"${CURL[@]}" "$REC"; show
"${CURL[@]}" "$REC/blocks/Block01"; show

step "8. Modify again with get-previous=true (expect 200 with the previous record)"
META='{"callbackReference":"http://cb.example/three"}'
echo "meta part: $META"
"${CURL[@]}" -X PUT -F "meta=$META;type=application/json" "$REC?get-previous=true"; show

step "9. PUT with If-None-Match: * on an existing record (expect 412 Precondition Failed)"
"${CURL[@]}" -X PUT -H 'If-None-Match: *' -F 'meta={};type=application/json' "$REC"; show

step "10. Reject a ttl in the past (expect 400 Bad Request)"
"${CURL[@]}" -X PUT -F 'meta={"ttl":"2000-01-01T00:00:00Z"};type=application/json' "$REC-expired"; show

step "11. Reject duplicate block ids (expect 400, nothing written)"
"${CURL[@]}" -X PUT \
  -F 'meta={};type=application/json' \
  -F "blocks=@$TMP/block2.txt;filename=Dup;type=text/plain" \
  -F "blocks=@$TMP/block2.txt;filename=Dup;type=text/plain" \
  "$REC"; show

step "12. Delete the record (expect 204); its blocks go with it"
"${CURL[@]}" -X DELETE "$REC"

step "13. Confirm the record and its blocks are gone (expect 404, 404)"
"${CURL[@]}" "$REC"; show
"${CURL[@]}" "$REC/blocks/Block01"; show

step "14. Delete again with get-previous=true on a missing record (expect 404)"
"${CURL[@]}" -X DELETE "$REC?get-previous=true"; show

echo
echo "Done."

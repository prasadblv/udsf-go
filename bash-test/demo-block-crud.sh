#!/usr/bin/env bash
# Demonstrates the Block CRUD API with curl over HTTP/2 in verbose mode.
#   PUT    /nudsf-dr/v1/{realm}/{storage}/records/{record}/blocks/{block}   create / replace
#   GET    .../blocks/{block}                                                read one
#   GET    .../blocks                                                        list (multipart/parallel)
#   DELETE .../blocks/{block}                                                delete
#
# Usage: ./demo-block-crud.sh [base_url]          (default https://127.0.0.1:9443)
#        TRACE=1 ./demo-block-crud.sh             also dump request/response bodies (curl --trace-ascii)
#
# Verbose mode (-v) prints the request line and headers (">") and the response status and headers ("<").
# Response bodies are printed after each call. TRACE=1 additionally shows the request body on the wire.

set -u

BASE_URL="${1:-${BASE_URL:-https://127.0.0.1:9443}}"
REALM="Realm01"
STORAGE="Storage01"
RECORD="DemoBlocks$(date +%s)"   # unique per run
REC="$BASE_URL/nudsf-dr/v1/$REALM/$STORAGE/records/$RECORD/blocks"

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

# --http2: negotiate HTTP/2 via ALPN; -k: accept the self-signed dev certificate; -v: verbose.
if [[ "${TRACE:-0}" == "1" ]]; then VERBOSE=(--trace-ascii -); else VERBOSE=(-v); fi
CURL=(curl -sS --http2 -k "${VERBOSE[@]}")

step() { printf '\n\n================================================================\n# %s\n================================================================\n' "$1"; }
show() { printf '\n--- (response body is the text printed just above) ---\n'; }

echo "Target: $REC"

step "1. Create a JSON block (expect 201 Created)"
PAYLOAD='{"name":"first","value":42}'
echo "request body: $PAYLOAD"
"${CURL[@]}" -X PUT -H 'Content-Type: application/json' -d "$PAYLOAD" "$REC/Block01"

step "2. Create a binary block with all 256 byte values (expect 201 Created)"
LC_ALL=C awk 'BEGIN{for(i=0;i<256;i++)printf "%c",i}' > "$TMP/in.bin"
echo "request body: 256 raw bytes (0x00-0xff) from a file"
"${CURL[@]}" -X PUT -H 'Content-Type: application/octet-stream' --data-binary @"$TMP/in.bin" "$REC/Block02"

step "3. Read the JSON block (expect 200, Content-Type: application/json)"
"${CURL[@]}" "$REC/Block01"; show

step "4. Replace the JSON block (expect 204 No Content)"
PAYLOAD='{"name":"first","value":43}'
echo "request body: $PAYLOAD"
"${CURL[@]}" -X PUT -H 'Content-Type: application/json' -d "$PAYLOAD" "$REC/Block01"

step "5. Replace again with get-previous=true (expect 200 with the previous content)"
PAYLOAD='{"name":"first","value":44}'
echo "request body: $PAYLOAD"
"${CURL[@]}" -X PUT -H 'Content-Type: application/json' -d "$PAYLOAD" "$REC/Block01?get-previous=true"; show

step "6. List all blocks (expect 200, Content-Type: multipart/parallel; one part per block with Content-Id)"
"${CURL[@]}" -o "$TMP/list.out" "$REC"
echo "--- response body (binary parts shown as text where printable) ---"
LC_ALL=C tr -c '[:print:]\r\n' '.' < "$TMP/list.out"

step "7. Read the binary block and verify the round trip (expect byte-identical)"
"${CURL[@]}" -o "$TMP/out.bin" "$REC/Block02"
echo "--- first 32 bytes of the response ---"; xxd -l 32 "$TMP/out.bin"
cmp -s "$TMP/in.bin" "$TMP/out.bin" && echo "round trip: IDENTICAL (256 bytes)" || echo "round trip: DIFFERENT"

step "8. Delete a block (expect 204), then read it again (expect 404 Not Found)"
"${CURL[@]}" -X DELETE "$REC/Block01"
"${CURL[@]}" "$REC/Block01"; show

step "9. Cleanup: delete the remaining block (expect 204)"
"${CURL[@]}" -X DELETE "$REC/Block02"

echo
echo "Done."

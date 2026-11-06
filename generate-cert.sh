#!/usr/bin/env bash
# Generates a self-signed TLS certificate (server.pem) and private key (server.key)
# for local development. Requires openssl.
#
# Usage: ./generate-cert.sh [output_dir]        (default: ./certs)
# Env:   CERT_CN    common name / primary DNS name   (default: localhost)
#        CERT_DAYS  validity in days                 (default: 365)
#        CERT_SANS  extra subjectAltName entries, comma separated,
#                   e.g. "DNS:udsf.local,IP:10.0.0.5"

set -euo pipefail

OUT_DIR="${1:-certs}"
CN="${CERT_CN:-localhost}"
DAYS="${CERT_DAYS:-365}"
SANS="DNS:localhost,IP:127.0.0.1,IP:::1"
[[ "$CN" != "localhost" ]] && SANS="DNS:${CN},${SANS}"
SANS="${SANS}${CERT_SANS:+,${CERT_SANS}}"

command -v openssl >/dev/null || { echo "openssl not found" >&2; exit 1; }

mkdir -p "$OUT_DIR"
KEY="$OUT_DIR/server.key"
PEM="$OUT_DIR/server.pem"

if [[ -e "$KEY" || -e "$PEM" ]]; then
  echo "Refusing to overwrite existing $KEY / $PEM (delete them first)" >&2
  exit 1
fi

# 2048-bit RSA key + self-signed cert; -addext works on OpenSSL 1.1.1+ and LibreSSL 3.1+
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout "$KEY" -out "$PEM" \
  -days "$DAYS" \
  -subj "/CN=${CN}" \
  -addext "subjectAltName=${SANS}"

chmod 600 "$KEY"
chmod 644 "$PEM"

echo "Created:"
echo "  certificate: $PEM"
echo "  private key: $KEY"
openssl x509 -in "$PEM" -noout -subject -enddate -ext subjectAltName

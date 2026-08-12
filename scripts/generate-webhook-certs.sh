#!/usr/bin/env sh
set -eu
OUT="${1:-.local/webhook-tls}"
SERVICE_DNS="${SERVICE_DNS:-authorization-admission.authz-system.svc}"
mkdir -p "$OUT"
openssl req -x509 -newkey rsa:2048 -nodes -days 30 \
  -keyout "$OUT/tls.key" -out "$OUT/tls.crt" \
  -subj "/CN=$SERVICE_DNS" \
  -addext "subjectAltName=DNS:$SERVICE_DNS,DNS:authorization-admission.authz-system.svc.cluster.local"
echo "Created local-only webhook certificate in $OUT"

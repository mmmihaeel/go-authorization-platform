#!/usr/bin/env sh
set -eu
BASE_URL="${BASE_URL:-http://localhost:10000}"

curl --fail-with-body --silent --show-error \
  -H 'X-Subject-ID: alice' \
  -H 'X-Subject-Roles: tenant_admin' \
  -H 'X-Tenant-ID: acme' \
  -H 'X-Resource-Type: document' \
  -H 'X-Resource-ID: roadmap' \
  -H 'X-Device-Trusted: true' \
  -H 'X-MFA: true' \
  "$BASE_URL/documents/roadmap"
printf '\n'

status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  -H 'X-Subject-ID: bob' \
  -H 'X-Tenant-ID: acme' \
  -H 'X-Resource-Type: document' \
  -H 'X-Resource-ID: roadmap' \
  "$BASE_URL/documents/roadmap")"
[ "$status" = "403" ] || { echo "Expected 403, got $status" >&2; exit 1; }
echo "PASS Envoy denied unauthorized request with HTTP 403"

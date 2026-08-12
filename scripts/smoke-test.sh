#!/usr/bin/env sh
set -eu
BASE_URL="${BASE_URL:-http://localhost:8080}"

request() {
  expected="$1"
  body="$2"
  response="$(curl --fail-with-body --silent --show-error \
    -H 'Content-Type: application/json' \
    -H "X-Request-ID: smoke-$expected" \
    --data "$body" \
    "$BASE_URL/v1/decisions/check")"
  allowed="$(printf '%s' "$response" | sed -n 's/.*"allowed":\(true\|false\).*/\1/p')"
  [ "$allowed" = "$expected" ] || { echo "Expected allowed=$expected, got: $response" >&2; exit 1; }
  printf 'PASS allowed=%s decision=%s\n' "$expected" "$response"
}

request true '{"subject":{"id":"alice","tenant_id":"acme","roles":["tenant_admin"]},"resource":{"type":"document","id":"roadmap","tenant_id":"acme","attributes":{"classification":"internal"}},"action":"view","context":{"device_trusted":true,"mfa":true,"risk_score":10}}'
request false '{"subject":{"id":"bob","tenant_id":"acme"},"resource":{"type":"document","id":"roadmap","tenant_id":"globex"},"action":"view","context":{"device_trusted":true,"mfa":true,"risk_score":10}}'

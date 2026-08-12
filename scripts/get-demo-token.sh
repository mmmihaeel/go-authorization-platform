#!/usr/bin/env sh
set -eu
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8082}"
USERNAME="${1:-alice}"
PASSWORD="${2:-alice-local-only}"

curl --fail-with-body --silent --show-error \
  -X POST "$KEYCLOAK_URL/realms/authz-demo/protocol/openid-connect/token" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'client_id=demo-cli' \
  --data-urlencode 'grant_type=password' \
  --data-urlencode "username=$USERNAME" \
  --data-urlencode "password=$PASSWORD"

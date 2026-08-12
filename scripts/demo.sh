#!/usr/bin/env sh
set -eu

KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8082}"
ENVOY_URL="${ENVOY_URL:-http://localhost:10000}"
TOKEN_URL="$KEYCLOAK_URL/realms/authz-demo/protocol/openid-connect/token"

for command in curl jq docker; do
  command -v "$command" >/dev/null 2>&1 || { echo "$command is required" >&2; exit 1; }
done

temporary_directory="$(mktemp -d)"
trap 'rm -rf "$temporary_directory"' EXIT HUP INT TERM

user_token() {
  username="$1"
  password="$2"
  curl --fail-with-body --silent --show-error \
    -X POST "$TOKEN_URL" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'client_id=demo-cli' \
    --data-urlencode 'grant_type=password' \
    --data-urlencode "username=$username" \
    --data-urlencode "password=$password" | jq -er '.access_token'
}

service_token() {
  curl --fail-with-body --silent --show-error \
    -X POST "$TOKEN_URL" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode 'client_id=reporting-service' \
    --data-urlencode 'client_secret=reporting-local-only' \
    --data-urlencode 'grant_type=client_credentials' | jq -er '.access_token'
}

request() {
  name="$1"
  expected_status="$2"
  method="$3"
  token="$4"
  path="$5"
  shift 5
  output="$temporary_directory/$(printf '%s' "$name" | tr ' ' '-').json"
  status="$(curl --silent --show-error --output "$output" --write-out '%{http_code}' \
    -X "$method" -H "Authorization: Bearer $token" "$@" "$ENVOY_URL$path")"
  if [ "$status" != "$expected_status" ]; then
    echo "FAIL $name: expected HTTP $expected_status, got $status" >&2
    cat "$output" >&2
    exit 1
  fi
  printf 'PASS %-34s HTTP %s  ' "$name" "$status"
  jq -c '{decision_id:(.decision_id // .authorized_by), allowed:(if has("allowed") then .allowed else true end), reasons:(.reason_codes // ["forwarded_to_resource"]), resource_id:(.id // null)}' "$output"
}

echo "Obtaining audience-scoped Keycloak access tokens..."
alice_token="$(user_token alice alice-local-only)"
bob_token="$(user_token bob bob-local-only)"
mallory_token="$(user_token mallory mallory-local-only)"
reporting_token="$(service_token)"

baseline="$(docker compose exec -T postgres psql -U authz -d authz -Atc \
  "select (select count(*) from authorization_decisions),(select count(*) from outbox_events where status='published'),(select count(*) from consumed_audit_events);")"
baseline_decisions="$(printf '%s' "$baseline" | cut -d'|' -f1)"
baseline_published="$(printf '%s' "$baseline" | cut -d'|' -f2)"
baseline_consumed="$(printf '%s' "$baseline" | cut -d'|' -f3)"

request "RBAC admin allow" 200 GET "$alice_token" /documents/roadmap
request "relationship denial" 403 GET "$mallory_token" /documents/roadmap
request "context denial" 403 GET "$bob_token" /documents/security-plan
request "context allow" 200 GET "$bob_token" /documents/security-plan \
  -H 'X-Device-Trusted: true' -H 'X-Access-Purpose: incident-response'
request "cross-tenant denial" 403 GET "$bob_token" /documents/globex-plan
request "MFA-required denial" 403 DELETE "$alice_token" /documents/roadmap
request "service identity allow" 200 GET "$reporting_token" /documents/roadmap

expected_decisions=$((baseline_decisions + 7))
expected_published=$((baseline_published + 7))
expected_consumed=$((baseline_consumed + 7))
attempt=0
while [ "$attempt" -lt 30 ]; do
  counts="$(docker compose exec -T postgres psql -U authz -d authz -Atc \
    "select (select count(*) from authorization_decisions),(select count(*) from outbox_events where status='published'),(select count(*) from consumed_audit_events);")"
  decisions="$(printf '%s' "$counts" | cut -d'|' -f1)"
  published="$(printf '%s' "$counts" | cut -d'|' -f2)"
  consumed="$(printf '%s' "$counts" | cut -d'|' -f3)"
  if [ "$decisions" -ge "$expected_decisions" ] && [ "$published" -ge "$expected_published" ] && [ "$consumed" -ge "$expected_consumed" ]; then
    printf 'PASS audit pipeline                     decisions=%s published=%s consumed=%s\n' "$decisions" "$published" "$consumed"
    echo "Demo completed successfully."
    exit 0
  fi
  attempt=$((attempt + 1))
  sleep 1
done

echo "FAIL audit pipeline: decisions=$decisions published=$published consumed=$consumed" >&2
exit 1

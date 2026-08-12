#!/usr/bin/env sh
set -eu

base_url="${BASE_URL:-http://localhost:10000}"
keycloak_url="${KEYCLOAK_URL:-http://localhost:8082}"

status="$(curl --silent --output /dev/null --write-out '%{http_code}' "$base_url/documents/roadmap")"
[ "$status" = "401" ] || { echo "expected an unauthenticated request to return 401, got $status" >&2; exit 1; }

token="$(curl --fail --silent --show-error \
  --data-urlencode grant_type=password \
  --data-urlencode client_id=demo-cli \
  --data-urlencode username=alice \
  --data-urlencode password=alice-local-only \
  "$keycloak_url/realms/authz-demo/protocol/openid-connect/token" | jq -er .access_token)"

baseline="$(docker compose exec -T postgres psql -U authz -d authz -Atc \
  "select (select count(*) from authorization_decisions),(select count(*) from outbox_events where status='published'),(select count(*) from consumed_audit_events);")"
baseline_count="$(printf '%s' "$baseline" | cut -d'|' -f1)"

restore_opa() { docker compose start opa >/dev/null; }
trap restore_opa EXIT
docker compose stop opa >/dev/null

status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  -H "Authorization: Bearer $token" \
  -H 'X-Action: view' \
  -H 'X-Resource-Type: document' \
  -H 'X-Resource-ID: roadmap' \
  "$base_url/documents/roadmap")"
[ "$status" = "503" ] || { echo "expected OPA failure to return 503, got $status" >&2; exit 1; }

restore_opa
for _ in $(seq 1 30); do
  [ "$(docker inspect --format '{{.State.Health.Status}}' go-authorization-platform-opa-1)" = "healthy" ] && break
  sleep 1
done
[ "$(docker inspect --format '{{.State.Health.Status}}' go-authorization-platform-opa-1)" = "healthy" ] || {
  echo 'OPA did not recover' >&2
  exit 1
}
trap - EXIT

expected_count=$((baseline_count + 1))
for _ in $(seq 1 30); do
  counts="$(docker compose exec -T postgres psql -U authz -d authz -Atc \
    "select (select count(*) from authorization_decisions),(select count(*) from outbox_events where status='published'),(select count(*) from consumed_audit_events);")"
  [ "$counts" = "$expected_count|$expected_count|$expected_count" ] && break
  sleep 1
done
[ "$counts" = "$expected_count|$expected_count|$expected_count" ] || {
  echo "dependency failure audit did not drain: $counts" >&2
  exit 1
}

printf 'PASS no-token=401 OPA-down=503 OPA-recovered=healthy audited=%s\n' "$expected_count"

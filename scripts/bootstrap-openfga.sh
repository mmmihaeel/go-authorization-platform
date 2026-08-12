#!/usr/bin/env sh
set -eu

API_URL="${FGA_API_URL:-http://localhost:8081}"
STORE_FILE="${1:-policies/openfga/store.fga.yaml}"

if ! command -v fga >/dev/null 2>&1; then
  echo "The OpenFGA CLI is required. Install it, or run:" >&2
  echo "docker run --rm -v \"$PWD:/workspace\" openfga/cli:v0.7.20 model test --tests /workspace/${STORE_FILE}" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to create .env.runtime" >&2
  exit 1
fi

OUTPUT="$(fga store import --api-url "$API_URL" --file "$STORE_FILE")"
printf '%s\n' "$OUTPUT"
STORE_ID="$(printf '%s' "$OUTPUT" | jq -r '.store.id // .store_id // empty')"
MODEL_ID="$(printf '%s' "$OUTPUT" | jq -r '.model.authorization_model_id // .authorization_model_id // .model_id // empty')"

if [ -z "$STORE_ID" ]; then
  echo "Import succeeded, but the CLI output format did not expose a store ID. Copy it from the JSON above." >&2
  exit 0
fi

cat > .env.runtime <<EOF
AUTHZ_MODE=remote
OPENFGA_URL=$API_URL
OPENFGA_STORE_ID=$STORE_ID
OPENFGA_MODEL_ID=$MODEL_ID
OPA_URL=http://localhost:8181
OPA_DECISION_PATH=authz/decision
EOF

echo "Wrote .env.runtime. Load it with: set -a; . ./.env.runtime; set +a"

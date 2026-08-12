#!/usr/bin/env sh
set -eu

if [ -z "${TOKEN:-}" ]; then
  TOKEN="$(./scripts/get-demo-token.sh | jq -er .access_token)"
  export TOKEN
fi

mount_root="$PWD"
case "$(uname -s)" in
  MINGW*|MSYS*)
    mount_root="$(pwd -W)"
    export MSYS_NO_PATHCONV=1
    ;;
esac

exec docker run --rm \
  --add-host=host.docker.internal:host-gateway \
  -e TOKEN \
  -e BASE_URL="${BASE_URL:-http://host.docker.internal:10000}" \
  -e RATE="${RATE:-25}" \
  -e DURATION="${DURATION:-30s}" \
  -v "$mount_root/tests/load:/scripts:ro" \
  "grafana/k6:${K6_VERSION:-2.2.0}" run /scripts/decision-api.js

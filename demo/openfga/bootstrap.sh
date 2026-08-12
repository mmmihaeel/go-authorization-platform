#!/bin/sh
set -eu

store_checksum="$(sha256sum "$OPENFGA_STORE_FILE" | cut -d' ' -f1)"
if [ -f /state/bootstrapped ] && [ "$(cat /state/bootstrapped)" = "$store_checksum" ]; then
  echo "OpenFGA demo store is already bootstrapped."
  exit 0
fi

stores="$(fga store list --api-url "$OPENFGA_URL")"
store_id="$(printf '%s' "$stores" | sed -n 's/.*"id":"\([^"]*\)","name":"Go Authorization Platform".*/\1/p')"

if [ -n "$store_id" ]; then
  fga store import --api-url "$OPENFGA_URL" --store-id "$store_id" --file "$OPENFGA_STORE_FILE"
else
  fga store import --api-url "$OPENFGA_URL" --file "$OPENFGA_STORE_FILE"
fi

printf '%s\n' "$store_checksum" > /state/bootstrapped

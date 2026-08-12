#!/usr/bin/env sh
set -eu
OUT="${1:-.local/webhook-tls}"
SERVICE_DNS="${SERVICE_DNS:-authorization-admission.authz-system.svc}"
TEMPLATE="deploy/kubernetes/webhook/validatingwebhookconfiguration.yaml"
mkdir -p "$OUT"
case "$(uname -s)" in
  MINGW*|MSYS*) export MSYS_NO_PATHCONV=1 ;;
esac
openssl req -x509 -newkey rsa:2048 -nodes -days 30 \
  -keyout "$OUT/tls.key" -out "$OUT/tls.crt" \
  -subj "/CN=$SERVICE_DNS" \
  -addext "subjectAltName=DNS:$SERVICE_DNS,DNS:authorization-admission.authz-system.svc.cluster.local"

certificate="$(base64 < "$OUT/tls.crt" | tr -d '\r\n')"
private_key="$(base64 < "$OUT/tls.key" | tr -d '\r\n')"
sed "s|REPLACE_WITH_BASE64_CA_BUNDLE|$certificate|" "$TEMPLATE" > "$OUT/validatingwebhookconfiguration.yaml"
cat > "$OUT/secret.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: authorization-admission-tls
  namespace: authz-system
type: kubernetes.io/tls
data:
  tls.crt: $certificate
  tls.key: $private_key
EOF

echo "Created local-only webhook certificate and rendered manifests in $OUT"

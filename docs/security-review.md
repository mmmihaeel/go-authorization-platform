# Security review

Reviewed 2026-08-12 against the default Go build and local Compose path. No unresolved critical or high-severity finding was identified. The review fixed the former trust in caller identity headers, made JWT verification mandatory, bounded HTTP bodies and dependency calls, validated dependency URL schemes, sanitized dependency errors at the HTTP boundary, required atomic audit persistence for allows, added RabbitMQ publisher confirms, and separated completed inbox events from active leases.

The remaining findings are explicit demo boundaries, not hidden production claims.

## SEC-001: contextual headers require a trusted edge

**Severity:** Medium if the demo edge configuration is reused outside an isolated environment; Low in the loopback-only Compose deployment.

**Location:** `internal/httpapi/router.go:241-271`, `demo/envoy/envoy.yaml`, `README.md` security boundaries.

The ext_authz request accepts device trust, risk score, and access purpose as context headers. Identity, tenant, role, service type, and MFA cannot be supplied this way and are always derived from a verified token. In a real deployment, an ingress or risk service must strip caller copies, derive or attest these context values, and authenticate the hop to the authorization service. Until that exists, these fields must not protect internet-facing resources.

## SEC-002: Compose credentials and plaintext service links are local only

**Severity:** Medium if exposed or promoted unchanged; Low in the documented loopback-only environment.

**Location:** `compose.yaml:25-27`, `compose.yaml:96-117`, `compose.yaml:144-223`.

Compose contains named development passwords and uses plaintext links on a private Docker network. Published ports bind to `127.0.0.1`, and the README states that these values are not reusable. A shared deployment must use generated secrets, TLS or mTLS, restricted management interfaces, OpenFGA authentication, Keycloak hardening, and a namespace-level network policy.

## SEC-003: resource metadata is a static demonstration catalog

**Severity:** Low.

**Location:** `cmd/authz-api/main.go:89`, `internal/httpapi/router.go:33`.

The API replaces client-provided tenant and classification values with canonical entries, which prevents request metadata spoofing. The catalog is intentionally static, so changing resources requires a process restart and it is not an authoritative ownership system. A larger deployment should resolve immutable metadata from the resource owner with authenticated calls and versioned cache invalidation.

## Release posture

The local reference implementation has no known release-blocking security issue within its stated scope. It is not suitable for external exposure without resolving SEC-001 and SEC-002 for the target environment. Dependency scanning, secret scanning, and CodeQL remain CI controls rather than substitutes for deployment hardening.

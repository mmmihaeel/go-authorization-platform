# Testing strategy

## Test pyramid and evidence

### Unit tests

Cover deterministic domain behavior:

- request normalization and validation;
- relationship deny and policy deny;
- dependency errors and fail-closed semantics;
- audit-required behavior;
- outbox retries and concurrent publishing;
- idempotent event consumption;
- admission validation.

### HTTP contract tests

Use `httptest` for:

- direct decision endpoint;
- strict JSON decoding;
- metrics endpoint;
- Envoy `ext_authz` status and decision-header contract;
- OpenFGA request object naming;
- OPA response decoding.

### Policy/model tests

- Rego v1 positive and negative cases;
- tenant mismatch;
- MFA, risk, device, purpose;
- OpenFGA RBAC inheritance;
- team/folder ReBAC path;
- service-principal relationship;
- cross-tenant denial.

### Concurrency tests

`go test -race ./...` is mandatory when changing:

- memory recorders/inboxes;
- outbox worker concurrency;
- metric counters;
- shutdown/lifecycle behavior.

### Benchmarks

The included benchmark measures only in-process composition with fake dependencies. It is useful for allocation regressions, not a claim about end-to-end policy latency.

Record:

- Go version and CPU;
- commit SHA;
- `ns/op`, `B/op`, allocations;
- before/after comparison when changing the hot path.

### Remote integration tests — completion task

Run disposable PostgreSQL, OpenFGA, OPA, and RabbitMQ containers and verify:

1. store/model import;
2. actual OpenFGA checks;
3. actual Rego decisions;
4. decision + outbox transaction;
5. event publication and idempotent consumption;
6. dependency timeout and recovery;
7. model/policy revision pinning.

### Deployment tests

- `docker compose config`;
- Helm lint and render;
- server-side dry run against a disposable cluster;
- Argo CD sync in a local Kind cluster;
- webhook certificate and AdmissionReview path;
- protected Deployment allowed/denied fixtures;
- NetworkPolicy connectivity checks.

### Security tests

- malformed JWT, wrong issuer/audience, expired token;
- spoofed identity headers;
- cross-tenant object;
- policy/model mutation cases;
- high-cardinality input does not become metric labels;
- request-body size limits;
- dependency error body does not leak credentials;
- container runs non-root/read-only.

## CI gates

A merge-ready change should pass:

```text
gofmt → vet → unit/integration → race → benchmark smoke → build
OPA tests → OpenFGA model tests
Helm lint/render → Compose validation → container build
CodeQL → govulncheck → dependency review
```

## Evidence naming

Store sanitized evidence under a release or pull request, not as ambiguous screenshots:

```text
evidence/<version>/
  ci-summary.md
  policy-tests.txt
  model-tests.txt
  benchmark.txt
  k6-summary.json
  grafana-overview.png
  trace-allowed.png
  trace-denied.png
  outage-drill.md
  admission-test.md
```

No production tokens, customer IDs, or sensitive policy data belong in evidence artifacts.

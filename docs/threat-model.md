# Threat model

## Scope

Authorization decisions for multi-tenant HTTP services, policy/model delivery, audit evidence, token delegation, and Kubernetes enforcement.

## Assets

- tenant boundaries and relationship tuples;
- OPA policies and OpenFGA model revisions;
- access and exchanged tokens;
- decision records, audit events, and correlation IDs;
- service credentials and policy-store identifiers;
- webhook TLS material and GitOps credentials;
- availability of the authorization path.

## Trust boundaries

1. untrusted client to Envoy/gateway;
2. gateway to authorization API;
3. authorization API to OpenFGA/OPA/PostgreSQL;
4. audit worker to RabbitMQ;
5. CI/GitOps to cluster and policy engines;
6. Kubernetes API server to admission webhook.

## Threats and mitigations

| Threat | Impact | Primary mitigations | Remaining work |
|---|---|---|---|
| Spoofed identity or tenant headers | privilege escalation | API validates JWT and derives subject/tenant/roles/MFA from claims; Envoy forwards no identity headers | require mTLS or workload identity on the edge-to-PDP hop |
| Cross-tenant object/subject mix | data exposure | canonical resource metadata; tenant-scoped object IDs; OPA equality check; negative model tests | replace the static demo catalog with an authoritative resource lookup |
| Malicious or accidental broad OpenFGA tuple | privilege escalation | reviewed model/tuple changes; immutable revisions; replay tests; audit changes | signed promotion artifact |
| OPA policy grants despite graph denial | privilege escalation | policy explicitly requires `relationship_allowed`; service also combines with logical AND | mutation tests for policy invariants |
| Stale model/policy during rollout | inconsistent decisions | pin model ID/package revision; staged rollout; decision metadata; rollback runbook | add revision fields to persisted record |
| Token accepted by wrong service | confused deputy | audience and issuer validation; explicit exchange permissions; `azp`/`act` checks | full exchange integration test |
| Replayed event creates duplicate side effect | duplicate audit/action | event ID; PostgreSQL inbox lease; idempotent insert; manual acknowledgement | require the same invariant for every added handler |
| Decision stored but event not published | missing downstream audit | transactional outbox; retry/backoff; backlog alert | dead-letter and replay tooling |
| Broker published but acknowledgement lost | duplicate delivery | publisher confirms; at-least-once contract; idempotent consumer | failure-injection drills |
| OpenFGA/OPA unavailable | unauthorized bypass or outage | fail closed, short timeout, 503, alerting, no `failure_mode_allow` | controlled read-only fallback evaluation |
| Sidecar omitted from protected workload | bypass | admission webhook requires policy ref and sidecar | controller reconciliation and image verification |
| Admission webhook unavailable | unprotected deployment | `failurePolicy: Fail`, multiple replicas, PDB | certificate rotation automation |
| Audit payload leaks sensitive data | privacy/compliance issue | data minimization; structured fields; access controls; retention policy | field-level redaction and encryption policy |
| Policy supply-chain compromise | systemic authorization change | CODEOWNERS, CI tests, review, pinned images, GitOps | signing/SBOM/attestation enforcement |
| High-cardinality telemetry | observability outage | bounded labels; decision IDs in logs/traces, not metric labels | cardinality tests and budgets |
| Timing differences reveal relationship existence | information disclosure | generic external denial; bounded timeouts; avoid raw engine errors | latency-distribution analysis |

## Security invariants

- Missing or malformed input never becomes an allow.
- Dependency failure never becomes an allow in the default mode.
- OPA cannot override an OpenFGA denial.
- External callers cannot author their trusted identity context.
- A resource's tenant is not accepted solely from a caller-controlled token/header.
- Audit/event processing is at least once and side effects are idempotent.
- Policy/model changes are code-reviewed and tested before promotion.
- Protected workloads cannot opt out of enforcement through a normal deployment change.

## Abuse cases to demonstrate

1. Change `resource.tenant_id` to another tenant and show denial.
2. Submit a valid relationship with `risk_score=95` and show contextual denial.
3. Delete a protected resource without MFA and show denial.
4. Stop OPA/OpenFGA and show Envoy returns failure rather than forwarding.
5. Submit the same event twice and show the handler runs once.
6. Create a protected Deployment without a sidecar and show admission denial.
7. Attempt an audience-mismatched exchanged token and show verification failure.

## Data minimization

Decision records should include identifiers and security-relevant context, not entire access tokens or arbitrary application payloads. Before production use, define:

- field allowlist and redaction rules;
- retention by decision class;
- access roles for audit search;
- encryption requirements;
- deletion/export procedures where applicable.

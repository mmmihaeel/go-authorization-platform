# Failure modes

## Default rule

Authorization failures are security decisions, not ordinary downstream errors. The baseline mode is **fail closed**.

| Failure | API behavior | Envoy behavior | Recovery evidence |
|---|---|---|---|
| OpenFGA timeout/error | denied decision, reason `relationship_dependency_unavailable`, HTTP 503 | request blocked | dependency metrics/logs, trace, decision audit when available |
| OPA timeout/error | denied decision, reason `policy_dependency_unavailable`, HTTP 503 | request blocked | dependency metrics/logs, trace, decision audit when available |
| Invalid request | denied decision, HTTP 400 | request blocked | validation reason code |
| Audit database failure, audit optional | decision may proceed; `audit_recorded=false` | follows decision | alert and explicit loss window |
| Audit database failure, audit required | deny, reason `audit_persistence_failed` | request blocked | database incident evidence |
| RabbitMQ unavailable | decision unaffected; outbox accumulates | no effect on synchronous path | backlog depth, retry state, recovery publish |
| Worker crash mid-batch | locked rows recovered/retried after policy | no effect on synchronous path | attempts, lock age, duplicate-safe consumer |
| Duplicate broker delivery | consumer skips completed event ID | no duplicate side effect | inbox state |
| Keycloak unavailable | new login/exchange fails; valid cached tokens may continue until expiry | depends on local/JWKS cache policy | IdP health and token validation logs |
| Admission webhook unavailable | Kubernetes API rejects matching changes (`failurePolicy: Fail`) | existing workloads continue | API-server admission error, webhook health |
| Bad policy/model release | elevated deny/allow risk | follows bad revision until rollback | revision, decision diff, rollback audit |

## Audit availability decision

`AUTHZ_AUDIT_REQUIRED` makes the trade-off explicit:

- `false`: prioritize the synchronous authorization decision when the audit store is unavailable, but expose `audit_recorded=false` and alert. Appropriate only when a separately approved loss window exists.
- `true`: no decision can allow unless its audit evidence is durably stored. Appropriate for operations that require non-repudiation.

A larger system may make this operation-specific, but a single explicit switch keeps this reference implementation auditable.

## Read-only degraded mode

The CRD documents `allow-read-only` as a possible deployment policy, but the runtime does not implement it. Such a mode would require all of the following:

- a cryptographically or operationally trusted cache of recent positive entitlements;
- only explicitly classified read relations;
- short maximum staleness;
- no contextual high-risk resource;
- mandatory degraded-mode audit and alert;
- a tested automatic exit when dependencies recover.

Without this evidence, the safe behavior remains deny.

## Retry behavior

The outbox worker uses bounded concurrency, exponential delay capped by attempt count, publisher confirms, lazy RabbitMQ reconnection, and a two-minute recovery lease for stale `publishing` rows. Because at-least-once recovery can republish an event after an ambiguous worker failure, consumers must remain idempotent. Further operational controls may add:

- randomized jitter;
- configurable lease duration and ownership tokens;
- maximum attempts and dead-letter state;
- replay tooling with an operator audit trail.

## Failure-injection matrix

| Drill | Expected result |
|---|---|
| add 1 s latency to OPA with 750 ms client timeout | 503, no protected request forwarded |
| terminate one API replica | readiness removes it; remaining replica serves |
| stop RabbitMQ for five minutes | outbox grows; decisions continue; backlog drains after recovery |
| kill worker after publish before mark-complete | duplicate delivery possible; consumer side effect remains once |
| deploy policy that denies fixture corpus | canary detects diff; GitOps rollback restores revision |
| remove sidecar from protected Deployment | admission webhook denies CREATE/UPDATE |

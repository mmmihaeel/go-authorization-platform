# Runbook: authorization dependency degradation

## Trigger

- `AuthorizationServiceDown` or `AuthorizationDependencyErrors` alert;
- elevated HTTP 503 from `/v1/decisions/check` or Envoy;
- traces show OpenFGA/OPA timeout/error;
- users report protected operations blocked.

## Safety rule

Do not enable broad `failure_mode_allow` to restore traffic. Preserve tenant boundaries and gather evidence before any scoped degraded-mode decision.

## Triage

1. Confirm incident start time, affected tenants/relations, and recent deployments.
2. Check API readiness, resource saturation, and error rate.
3. Separate OpenFGA from OPA failures using reason codes and traces.
4. Verify DNS/network/TLS before changing policy/model state.
5. Check dependency health, latency, connection pools, replicas, and datastore health.
6. Identify the last known-good application, model, and policy revisions.
7. Confirm denied requests are not reaching the protected workload.

## Mitigation

- roll back the most recent relevant revision when correlation is strong;
- scale a saturated dependency within tested bounds;
- restore database/network dependency;
- reduce nonessential load at the caller, not authorization correctness;
- for an approved read-only degraded policy, enable only its documented relations, tenants, duration, and audit controls.

## Verification

- synthetic allow and deny fixtures return expected results;
- cross-tenant fixture still denies;
- Envoy forwards only the allowed fixture;
- dependency-error rate returns to baseline;
- decision traces use the expected model/policy revision;
- no unauthorized bypass was introduced.

## Recovery

1. Remove temporary mitigations.
2. Confirm all replicas use the intended revision.
3. Review audit gaps and outbox backlog.
4. Preserve sanitized incident timelines, queries, traces, and decision IDs.
5. Create follow-up work with owner, severity, and acceptance evidence.

## Post-incident questions

- Was detection faster than user reports?
- Did reason codes identify the failed component?
- Was timeout/retry behavior bounded?
- Did a rollout create mixed revisions?
- Did the runbook avoid unsafe availability shortcuts?
- What test or alert would have prevented recurrence?

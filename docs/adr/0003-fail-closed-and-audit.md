# ADR 0003: Fail closed and make audit durability explicit

- Status: Accepted
- Date: 2026-08-12

## Context

Authorization dependencies can fail. Silently allowing requests protects availability at the cost of an unbounded security bypass. Audit persistence can also fail, and different operations may have different non-repudiation requirements.

## Decision

- OpenFGA or OPA failure returns a denied decision and HTTP 503.
- Envoy uses `failure_mode_allow: false`.
- Missing/malformed context denies.
- OPA cannot override a relationship denial.
- `AUTHZ_AUDIT_REQUIRED` explicitly chooses whether audit-store failure denies the request.
- RabbitMQ failure does not affect the synchronous path because the event is stored in PostgreSQL outbox first.

## Consequences

- authorization dependency incidents can reduce application availability;
- this behavior is visible and testable rather than an accidental fallback;
- high-assurance operations can require durable audit;
- lower-risk operations may use a reviewed audit-loss window;
- a future read-only degraded mode needs strict, separately approved invariants.

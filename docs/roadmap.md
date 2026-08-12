# Roadmap

## Scope rule

A task belongs only when it strengthens one of these signals:

- authorization correctness;
- secure identity/delegation boundary;
- enforcement reliability;
- audit/event integrity;
- deployability/operability;
- reproducible engineering evidence.

A dashboard, service, controller, or abstraction without a concrete acceptance criterion is out of scope.

## Phase 0 — Starter baseline ✅

- executable Go API in deterministic demo mode;
- remote OpenFGA and OPA HTTP clients;
- strict decision contract and Envoy ext-authz contract;
- OpenFGA model/fixtures/tests;
- OPA policy/tests;
- audit SQL/outbox worker/idempotent consumer;
- race/benchmark/HTTP/integration tests;
- Compose, Helm, Argo CD, Terraform/Terragrunt;
- CRD and tested admission webhook;
- threat model, ADRs, failure modes, runbooks.

## Phase 1 — Real vertical integration

### 1. Wire durable decision persistence

**Work**

- enable pgx adapter in `authz-api`;
- run migrations explicitly;
- replace in-memory recorder with SQL repository;
- add readiness checks for required dependencies.

**Acceptance**

- allowed and denied decisions persist;
- decision/outbox rows commit or roll back together;
- integration test proves no orphan decision/event;
- audit-required mode denies on database failure.

### 2. Wire RabbitMQ audit delivery

**Work**

- production worker composition;
- publisher confirms;
- jittered retry and stale-lock recovery;
- PostgreSQL inbox implementation and demo consumer.

**Acceptance**

- broker outage grows outbox without losing decisions;
- recovery drains backlog;
- duplicate delivery runs side effect once;
- dead-letter/replay path is documented.

### 3. Wire Keycloak OIDC and token exchange

**Work**

- require bearer middleware on direct API;
- validate issuer, audience, time, tenant, authorized party, actor;
- configure explicit source→target token exchange;
- derive subject from token, never body/header.

**Acceptance**

- valid delegated token succeeds;
- wrong audience/actor/tenant fails;
- body subject cannot override token subject;
- decision audit records effective subject and actor.

### 4. Remote OpenFGA/OPA integration suite

**Acceptance**

- disposable stack imports exact revisions;
- positive/negative/cross-tenant checks pass through real engines;
- OPA/OpenFGA timeout tests produce 503 and no upstream forwarding;
- CI publishes sanitized test logs.

## Phase 2 — Platform evidence

### 5. OpenTelemetry correlation

- instrument OpenFGA, OPA, database, and HTTP spans;
- attach decision ID, allowed/denied, and dependency outcome with bounded attributes;
- never place subject/resource IDs in metric labels.

**Acceptance:** one allowed and one denied trace correlate Envoy, API, engines, and database.

### 6. Kind + Helm + Argo CD scenario

- local Kind cluster bootstrap;
- install CRD/webhook/chart;
- Argo CD sync from a pinned revision;
- deployment and rollback script.

**Acceptance:** healthy/synced app, admission denial fixture, successful rollback evidence.

### 7. Failure drills and runbook proof

- OPA outage;
- OpenFGA latency;
- RabbitMQ outage/recovery;
- bad policy revision;
- webhook outage.

**Acceptance:** each runbook records detection, impact, mitigation, rollback, and follow-up.

### 8. Reproducible load evidence

- baseline, remote engines, audit-enabled, and failure scenarios;
- resource/connection-pool profiling;
- publish raw summary and environment metadata.

**Acceptance:** thresholds are supported by evidence or revised with rationale.

## Phase 3 — Optional advanced Kubernetes work

### 9. `AuthorizationPolicyBinding` controller

The controller should:

- watch bindings and selected workloads;
- resolve policy/model references;
- publish readiness/status conditions;
- inject or configure enforcement only where ownership permits;
- detect drift;
- use finalizers only when cleanup is necessary;
- expose reconcile metrics and tests.

**Stop condition:** do not implement this before the simpler admission/Helm flow is complete and demonstrable. The controller is valuable evidence only when it has a clear reconciliation contract and failure behavior.

## Explicit non-roadmap

- custom replacement for OpenFGA/OPA/Keycloak;
- dozens of sample microservices;
- a UI unrelated to policy operations;
- cloud-specific production infrastructure across multiple providers;
- artificial benchmark targets detached from hardware and model size;
- untested service-mesh complexity.

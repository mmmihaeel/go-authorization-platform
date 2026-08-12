# Portfolio evidence guide

## Positioning

This project should be presented as a **designed and implemented public portfolio platform**, not as undisclosed production experience.

Accurate résumé wording:

> Designed and built a Go authorization-platform portfolio project combining OpenFGA ReBAC, OPA contextual ABAC, OIDC delegation boundaries, Envoy external authorization, transactional outbox delivery, Kubernetes admission controls, GitOps, IaC, and observability; backed by executable Go tests, race and benchmark checks, policy/model test suites, architecture decisions, and an explicit integration-evidence roadmap.

Avoid wording such as “operated bank authorization at scale” unless that is independently true.

## Competency map

| Engineering competency | Repository evidence |
|---|---|
| Idiomatic Go and interfaces | decision service, dependency ports, strict HTTP handlers |
| Concurrency | bounded outbox publisher, synchronization, race tests |
| RBAC/ReBAC/Zanzibar | OpenFGA model, tuples, inheritance tests |
| ABAC/Rego | OPA decision policy and negative tests |
| OAuth/OIDC/delegation | Keycloak realm, OIDC adapter, delegation design |
| Sidecar authorization | Envoy `ext_authz` endpoint/config, fail-closed smoke test |
| PostgreSQL modeling | decision/outbox/inbox schema and indexes |
| Messaging reliability | transactional outbox, retry state, idempotent consumer |
| Kubernetes advanced concepts | Helm hardening, CRD, validating admission webhook |
| GitOps/IaC | Argo CD app, Terraform module, Terragrunt example |
| Observability | metrics, optional OTLP tracing, alerts, dashboard, runbooks |
| Architecture leadership | diagrams, ADRs, threat model, trade-offs, non-goals |
| Incident ownership | degradation and policy rollback runbooks |

## Evidence sequence for a reviewer

1. Read the first screen of `README.md` and architecture diagram.
2. Inspect `internal/authorization/service.go` for fail-closed composition.
3. Inspect `policies/openfga/store.fga.yaml` and `policies/opa/authz_test.rego`.
4. Run `make check`, `make policy-test`, and `make model-test`.
5. Run the Envoy sidecar smoke test.
6. Inspect the SQL outbox and idempotent consumer.
7. Review the threat model and ADRs.
8. Inspect Helm/Argo CD/Terraform and the admission webhook.
9. Review measured load/failure evidence only after the first release records it.

## What makes the final release senior-level

Not endpoint count. The strongest signals are:

- a narrow, defensible architecture;
- explicit ownership boundaries between identity, relationships, and context;
- positive and negative policy/model tests;
- failure behavior and rollback plans;
- concurrency/race evidence;
- measurable performance with reproducible metadata;
- secure deployment controls;
- honest trade-offs and non-goals.

## Suggested repository screenshots

Add only after the flows work:

- architecture diagram rendered in README;
- OpenFGA model test output;
- OPA policy test output;
- allowed and denied correlated traces;
- Grafana overview with synthetic demo traffic;
- Envoy request allowed and denied;
- admission webhook denying a non-compliant Deployment;
- Argo CD application healthy/synced in a local cluster.

A screenshot without commit, scenario, and explanation is decoration rather than evidence.

# Six-minute portfolio demo

## 0:00–0:45 — Problem and boundary

“Applications often spread permission checks across handlers and databases. This project centralizes the decision contract without building a custom identity provider or policy language. OpenFGA owns relationships, OPA owns contextual constraints, and Envoy enforces the result before the protected service is reached.”

Show the README architecture diagram and the implemented/roadmap distinction.

## 0:45–1:45 — Authorization semantics

Open `policies/openfga/model.fga` and explain:

- organization admin as tenant-level RBAC;
- team membership and parent-folder inheritance as ReBAC;
- direct service-principal relation for workload access.

Open `policies/opa/authz.rego` and explain tenant equality, risk, MFA, device trust, and purpose.

Run:

```bash
make policy-test
make model-test
```

## 1:45–2:45 — Go decision path

Open `internal/authorization/service.go`:

- normalization and validation;
- OpenFGA check;
- OPA evaluation with relationship result;
- logical AND / deny by default;
- stable reason codes and decision ID;
- audit behavior.

Run:

```bash
make check
make smoke
```

## 2:45–3:35 — Sidecar enforcement

Run:

```bash
make sidecar-up
make sidecar-smoke
```

Explain that Envoy blocks the original resource request and propagates a decision ID only after allow. Point out `failure_mode_allow: false` and the trusted-header caveat.

## 3:35–4:25 — Reliability and audit

Show:

- SQL transaction storing decision + outbox event;
- `FOR UPDATE SKIP LOCKED` claim;
- bounded worker concurrency;
- inbox/idempotent consumer test.

Explain at-least-once delivery and why a distributed transaction with RabbitMQ is avoided.

## 4:25–5:15 — Platform controls

Show:

- Helm deployment with OPA sidecar, probes, resources, non-root/read-only security;
- Argo CD application;
- Terraform module;
- `AuthorizationPolicyBinding` CRD;
- admission webhook test requiring sidecar and policy reference.

State clearly that the CRD controller is future work; the admission webhook is implemented.

## 5:15–6:00 — Operations and evidence

Show:

- metrics/dashboard/alerts;
- threat model and failure table;
- runbook;
- load-test scenario and empty result template.

Close with: “The project is intentionally measured by policy tests, failure behavior, deployment controls, and reproducible evidence—not by the number of endpoints.”

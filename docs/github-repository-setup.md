# GitHub repository setup

## Repository identity

**Repository slug**

```text
go-authorization-platform
```

The slug is intentionally descriptive and search-friendly. A branded codename would be less discoverable and would hide the core subject from recruiters and engineers scanning a profile.

**GitHub About description**

```text
Production-style Go authorization control plane with OpenFGA ReBAC, OPA/Rego ABAC, Keycloak OIDC, Envoy ext_authz, transactional outbox, Kubernetes, GitOps, IaC, and observability.
```

**Topics**

```text
golang
authorization
authz
openfga
opa
rego
keycloak
oauth2
openid-connect
rebac
abac
rbac
zanzibar
kubernetes
envoy
opentelemetry
postgresql
rabbitmq
terraform
argocd
```

GitHub supports up to 20 topics; this list uses all 20 and balances direct technologies with authorization search terms.

## Initial repository settings

- visibility: public;
- default branch: `main`;
- license: Apache-2.0;
- Issues and Discussions: enabled;
- Projects/Wiki: optional, disabled initially to reduce empty surfaces;
- security policy: enabled from `SECURITY.md`;
- Dependabot alerts and security updates: enabled;
- secret scanning and push protection: enabled where available;
- auto-delete head branches: enabled;
- merge method: squash merge only for a linear portfolio history.

## Branch protection

Require on `main`:

- pull request before merge;
- one approval when collaborating; self-review evidence when solo;
- dismissal of stale approvals;
- conversation resolution;
- signed commits if practical;
- passing `CI / go`, `CI / policies`, `CI / packaging`, `Security / codeql`, and `Security / govulncheck`;
- linear history;
- no force pushes or deletions.

A solo portfolio can still use pull requests to preserve decision context and evidence.

## Suggested release milestones

- `v0.1.0` — executable demo, model/policy tests, architecture/ADRs;
- `v0.2.0` — real PostgreSQL/RabbitMQ/OIDC wiring and remote integration tests;
- `v0.3.0` — Kind/Helm/Argo CD deployment, traces, dashboards, failure drill;
- `v1.0.0` — reproducible load evidence, security hardening, stable documented contracts.

Do not tag `v1.0.0` merely because the folder structure is complete.

## Pinned-repository presentation

Pin this project near the existing Go/PostgreSQL and delivery-platform repositories. Its distinct message is **authorization/security platform engineering**, so its README thumbnail and description should not look like another CRUD or generic infrastructure lab.

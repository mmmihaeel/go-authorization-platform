# Authorization model

## Domain

The model represents a multi-tenant collaboration product:

- `organization` is the tenant root;
- `team` groups users inside an organization;
- `folder` and `document` form a resource hierarchy;
- `user` and `service` are principals.

This domain is large enough to demonstrate realistic authorization relationships while remaining inspectable.

## Division of responsibility

| Concern | Engine | Rationale |
|---|---|---|
| Organization admin/auditor/member | OpenFGA | durable role relationship |
| Team membership/management | OpenFGA | graph relationship |
| Folder/document inheritance | OpenFGA | Zanzibar-style parent traversal |
| Direct service-principal grant | OpenFGA | explicit least-privilege entitlement |
| Tenant equality | OPA and object naming | defense in depth |
| Account suspension | OPA | fast-changing identity context |
| MFA and device trust | OPA | request/session context |
| Risk score and purpose | OPA | contextual policy |
| Token audience/actor | OIDC verifier / OPA extension | identity and delegation boundary |

## OpenFGA examples

### RBAC

```text
user:alice admin organization:acme
organization:acme organization document:acme/roadmap
```

The document's `can_view`, `can_edit`, and `can_delete` relations include `admin from organization`.

### ReBAC

```text
user:bob member team:acme/platform
team:acme/platform#member viewer folder:acme/engineering
folder:acme/engineering parent document:acme/roadmap
```

Bob can view the document through team membership and parent-folder inheritance, but cannot edit or delete it.

### Workload principal

```text
service:service-account-reporting-service viewer document:acme/roadmap
```

The service receives only the relation required for reporting. Identity/token delegation determines when the service is acting for itself versus on behalf of a user.

## OPA examples

A relationship is necessary but not sufficient. A request can still be denied when:

- `subject.tenant_id != resource.tenant_id`;
- `subject.claims.account_suspended == true`;
- `context.risk_score > 70`;
- delete/manage lacks MFA;
- a restricted resource is accessed from an untrusted device;
- a restricted resource has no declared access purpose.

The final policy result contains machine-readable reason codes. User-facing applications should map these codes to safe messages rather than expose raw policy internals.

## Multi-tenant invariants

1. Every tenant-owned resource has an `organization` relation.
2. Resource IDs are tenant-scoped in OpenFGA (`type:tenant/id`).
3. The request carries subject and resource tenant IDs.
4. OPA denies mismatched tenants even if a malformed tuple accidentally exists.
5. Audit rows are indexed by tenant and decision time.
6. No wildcard cross-tenant administrator exists in the baseline model.

A future platform-wide operator role would need a separate type, explicit approval, stronger audit semantics, and targeted model tests.

## Policy change workflow

1. Create a model/policy change branch.
2. Add positive, negative, and tenant-boundary tests first.
3. Run `make policy-test` and `make model-test`.
4. Document semantic changes in an ADR or RFC.
5. Deploy to a non-production store/package revision.
6. Replay a sanitized decision corpus and compare results.
7. Promote an immutable revision through GitOps.
8. Watch denial/error rates and retain a tested rollback revision.

## Example decision matrix

| Principal | Resource | Relationship | Context | Result |
|---|---|---|---|---|
| Alice, tenant admin | Acme roadmap | inherited admin | trusted, low risk | allow view/edit/delete |
| Bob, platform member | Acme roadmap | viewer via team + folder | trusted, low risk | allow view only |
| Reporting service | Acme roadmap | direct viewer | valid workload token | allow view only |
| Bob | Globex plan | no path | any | deny |
| Alice | restricted document | relationship allowed | untrusted device | deny |
| Alice | document delete | relationship allowed | no MFA | deny |

The executable source of truth is under `policies/openfga/` and `policies/opa/`; this table is explanatory only.

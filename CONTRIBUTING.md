# Contributing

## Workflow

1. Open an issue/RFC for behavior or architecture changes.
2. Create a focused branch.
3. Add the smallest evidence that proves the change.
4. Run `make check`; run policy/model/deployment validation when relevant.
5. Update ADRs, threat model, runbooks, or API docs when contracts change.
6. Open a pull request using the supplied template.

## Authorization changes

Every model or policy change must include:

- intended allow case;
- intended deny case;
- tenant-boundary case;
- failure/rollback consideration;
- compatibility impact on callers and stored relationships.

## Commit guidance

Use clear conventional prefixes where useful:

```text
feat(authz): add delegated service-principal relation
fix(policy): deny restricted access without purpose
infra(helm): add topology spread constraint
docs(adr): record policy revision strategy
test(openfga): cover cross-tenant inheritance
```

Never commit real tokens, model/store IDs, customer data, private endpoints, or production screenshots.

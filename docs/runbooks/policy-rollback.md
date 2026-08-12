# Runbook: policy or model rollback

## Trigger

- sudden denial spike after policy/model promotion;
- replay/canary diff shows unexpected allows;
- tenant-boundary or high-risk negative test fails;
- incident is correlated with an immutable revision change.

## Immediate actions

1. Freeze further policy/model promotions.
2. Record current application, OpenFGA model, OPA bundle/package, and Git commit revisions.
3. Determine whether the risk is over-permission or over-denial.
4. For possible over-permission, restrict affected operations/tenants at the nearest safe enforcement point while preparing rollback.
5. Select the last tested revision; do not edit the bad revision in place.

## Rollback

- update GitOps configuration to the known-good immutable revision;
- sync through Argo CD or the approved deployment mechanism;
- preserve the failed revision for analysis;
- avoid tuple deletion unless the incident specifically involves tuple data and a reviewed restoration plan exists.

## Verification fixtures

- tenant admin intended allow;
- team member inherited view;
- no inherited edit/delete;
- cross-tenant deny;
- restricted resource contextual deny;
- service-principal least-privilege check;
- synthetic Envoy allow and deny.

## Completion

- denial/allow rates stabilize;
- sampled traces and decision records show the restored revision;
- affected tenants are assessed;
- failed tests are added before a corrected promotion;
- root cause and change-control improvements are documented.

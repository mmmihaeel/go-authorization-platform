# ADR 0001: Compose OpenFGA and OPA

- Status: Accepted
- Date: 2026-08-12

## Context

The platform must demonstrate RBAC, ReBAC, Zanzibar-style inheritance, and contextual ABAC. Putting every concept into one engine would either duplicate application data in Rego or force rapidly changing request context into durable relationship tuples.

## Decision

Use:

- OpenFGA for durable subject–resource relationships and inheritance;
- OPA for contextual constraints and the final policy document;
- the Go service as the stable orchestration and audit boundary.

The Go service performs the OpenFGA check first, passes `relationship_allowed` into OPA, and only allows when both results are true.

## Consequences

### Positive

- each engine has a narrow, explainable responsibility;
- relationship models and contextual policies can be tested independently;
- applications integrate through one decision contract;
- reason codes and audit evidence are centralized;
- engines can evolve without exposing their raw APIs to every service.

### Negative

- two dependencies add latency and availability considerations;
- revision coordination is required;
- debugging needs correlated traces across both engines;
- policy authors must understand where each rule belongs.

## Rejected alternatives

### OpenFGA only

Conditions can express some context, but using the graph store for all session/risk/device policy would blur ownership and increase contextual tuple/model complexity.

### OPA only

Large relationship graphs and hierarchical sharing would require substantial data replication and custom graph semantics.

### Custom Go authorization engine

It would create a proprietary policy language and distract from integration, operations, and evidence.

### Let each service choose independently

This recreates inconsistent authorization behavior and prevents centralized audit/failure controls.

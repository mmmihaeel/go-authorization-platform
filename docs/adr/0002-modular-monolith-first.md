# ADR 0002: Modular service before microservice fleet

- Status: Accepted
- Date: 2026-08-12

## Context

Additional deployables without independent scaling, ownership, or isolation needs add operational cost without improving the authorization boundary.

## Decision

Start with:

- one stateless authorization API;
- one independently runnable audit worker;
- one minimal protected demo API;
- one admission webhook.

Keep domain, adapters, audit, events, authentication, and HTTP contracts as explicit Go packages. Split deployment units only after profiling or ownership requirements demonstrate value.

## Consequences

- local setup remains understandable;
- tests exercise real boundaries without orchestration overhead;
- the worker can scale independently where asynchronous load differs;
- future extraction remains possible through interfaces and event contracts;
- the project avoids implying a production topology that has not been justified.

## Extraction criteria

A module becomes a separate service only when at least one is true:

- independent scaling materially reduces cost or latency;
- security/isolation requires a separate trust boundary;
- deployment cadence or team ownership is independent;
- availability objectives differ;
- measured contention cannot be solved cleanly in-process.

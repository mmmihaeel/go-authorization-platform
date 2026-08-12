# Load testing

## Purpose

Measure decision-path behavior and identify a safe operating envelope. The repository intentionally does not publish invented throughput or latency numbers.

## Scenario

`tests/load/decision-api.js` generates a controlled mix:

- 80% allowed same-tenant requests;
- 20% denied cross-tenant requests;
- stable request IDs;
- configurable arrival rate and duration;
- p95 and p99 thresholds;
- decision correctness checks, not only HTTP status.

Run against demo mode:

```bash
make run
RATE=100 DURATION=60s make load
```

Run directly:

```bash
docker run --rm --network host \
  -e BASE_URL=http://localhost:8080 \
  -e RATE=250 \
  -e DURATION=5m \
  -v "$PWD/tests/load:/scripts:ro" \
  grafana/k6:2.2.0 run --summary-export=/tmp/summary.json /scripts/decision-api.js
```

## Test stages

1. **In-process baseline:** demo checker/policy to find Go/HTTP overhead.
2. **Remote policy baseline:** OpenFGA + OPA on the same host.
3. **Networked dependencies:** realistic latency and connection pools.
4. **Audit-enabled:** PostgreSQL transaction on every decision.
5. **Failure injection:** dependency latency, errors, broker outage, restart.
6. **Horizontal scaling:** multiple replicas and load balancing.

## Required metadata

Every published result must include:

- commit SHA and dirty state;
- Go and dependency versions;
- CPU, memory, OS, container limits;
- topology and network placement;
- OpenFGA/OPA/PostgreSQL configuration;
- model/tuple size;
- request distribution and payload size;
- warm-up period and run duration;
- arrival rate, achieved throughput, p50/p95/p99/max;
- error, deny, and timeout rates;
- resource utilization;
- raw k6 summary.

## Candidate budgets, not claims

Initial engineering targets for a local/containerized portfolio demo:

- p95 below 100 ms at the selected steady arrival rate;
- p99 below 250 ms;
- transport/evaluation error rate below 1%;
- decision correctness 100%;
- no race detector failures or unbounded goroutine growth;
- predictable denial during dependency failure.

Replace these with measured service objectives only after representative tests.

## Result template

```markdown
# Load result — <tag/commit>

Environment:
Scenario:
Policy/model size:
Arrival rate:
Duration:

| Metric | Result | Threshold | Pass |
|---|---:|---:|---:|
| achieved requests/s | | | |
| p50 | | | |
| p95 | | 100 ms | |
| p99 | | 250 ms | |
| transport errors | | <1% | |
| incorrect decisions | | 0 | |

Observations:
Bottleneck evidence:
Changes proposed:
Raw summary artifact:
```

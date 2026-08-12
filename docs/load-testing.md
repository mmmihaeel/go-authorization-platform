# Load testing

`tests/load/decision-api.js` sends requests through Envoy to the protected document API. It uses a real Keycloak access token and a controlled mix of 80% allowed requests and 20% OPA risk denials.

Start the stack; the script obtains Alice's local token when `TOKEN` is unset:

```bash
make up
make load
```

`RATE` defaults to 25 requests/second and `DURATION` to 30 seconds. Set them explicitly for a longer run:

```bash
TOKEN="$TOKEN" RATE=100 DURATION=2m make load
```

The scenario accepts HTTP 200 and 403 as transport successes, then independently checks the expected enforcement status and decision-ID header. Its latency thresholds are local smoke budgets, not service-level objectives.

Record the commit, dirty state, machine/container resources, model size, request mix, warm-up, achieved rate, latency distribution, error/deny rates, and raw k6 summary with any published result. An in-process Go benchmark and a local Compose test are not evidence of production capacity.

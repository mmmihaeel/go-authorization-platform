import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const decisionErrors = new Rate('decision_errors');
const decisionLatency = new Trend('decision_latency', true);

export const options = {
  scenarios: {
    steady_decisions: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 100),
      timeUnit: '1s',
      duration: __ENV.DURATION || '60s',
      preAllocatedVUs: 20,
      maxVUs: 100,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    decision_errors: ['rate<0.01'],
    decision_latency: ['p(95)<100', 'p(99)<250'],
  },
};

const baseURL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const allowCase = __ITER % 5 !== 0;
  const tenant = allowCase ? 'acme' : 'globex';
  const payload = JSON.stringify({
    subject: { id: 'alice', tenant_id: 'acme', roles: ['tenant_admin'] },
    resource: {
      type: 'document', id: 'roadmap', tenant_id: tenant,
      attributes: { classification: 'internal' },
    },
    action: 'view',
    context: { device_trusted: true, mfa: true, risk_score: 10 },
  });
  const response = http.post(`${baseURL}/v1/decisions/check`, payload, {
    headers: { 'Content-Type': 'application/json', 'X-Request-ID': `k6-${__VU}-${__ITER}` },
  });
  const parsed = response.json();
  const expected = allowCase;
  const valid = check(response, {
    'status is 200': (r) => r.status === 200,
    'decision matches scenario': () => parsed.allowed === expected,
    'decision ID is present': () => Boolean(parsed.decision_id),
  });
  decisionErrors.add(!valid);
  decisionLatency.add(response.timings.duration);
}

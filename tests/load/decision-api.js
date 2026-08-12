import http from 'k6/http';
import { check, fail } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import exec from 'k6/execution';

const scenarioErrors = new Rate('scenario_errors');
const enforcementLatency = new Trend('enforcement_latency', true);

export const options = {
  scenarios: {
    steady_enforcement: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RATE || 25),
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: 10,
      maxVUs: 50,
    },
  },
  thresholds: {
    scenario_errors: ['rate<0.01'],
    enforcement_latency: ['p(95)<250', 'p(99)<500'],
  },
};

const baseURL = __ENV.BASE_URL || 'http://localhost:10000';
const token = __ENV.TOKEN;

export function setup() {
  if (!token) fail('TOKEN must contain an authz-api audience-scoped Keycloak access token');
}

export default function () {
  const shouldAllow = exec.scenario.iterationInTest % 5 !== 0;
  const response = http.get(`${baseURL}/documents/roadmap`, {
    headers: {
      Authorization: `Bearer ${token}`,
      'X-Risk-Score': shouldAllow ? '10' : '95',
      'X-Request-ID': `k6-${__VU}-${__ITER}`,
    },
    responseCallback: http.expectedStatuses(200, 403),
  });
  const valid = check(response, {
    'enforcement status matches': (r) => r.status === (shouldAllow ? 200 : 403),
    'decision ID is present': (r) => Boolean(r.headers['X-Authorization-Decision-Id']),
  });
  scenarioErrors.add(!valid);
  enforcementLatency.add(response.timings.duration);
}

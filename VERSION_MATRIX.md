# Version matrix

Pinned baseline selected on **2026-08-12** from current stable releases. Dependabot and scheduled review can propose upgrades, but every change still requires policy, integration, and deployment evidence.

| Component | Baseline |
|---|---:|
| Go | 1.26.5 |
| PostgreSQL | 18.4 |
| OpenFGA server | 1.18.3 |
| OpenFGA CLI | 0.7.20 |
| OPA | 1.19.0 |
| Keycloak | 26.7.1 |
| Envoy | 1.39.0 |
| RabbitMQ | 4.3.4 |
| Kubernetes | 1.36.3 target |
| Argo CD | 3.5.0 target |
| OpenTelemetry Collector | 0.158.0 |
| Prometheus | 3.13.2 |
| Grafana | 13.1.3 |
| k6 | 2.2.0 |
| Terraform | 1.15.8 |
| Terraform Kubernetes provider | 3.2.1 |
| Terraform Helm provider | 3.2.0 |

All executable images and developer tools used by the repository are pinned. The matrix records a reproducible starting point, not a claim that every external integration has already been exercised in every supported environment.

# Starter validation boundary

- Snapshot date: **2026-08-12**
- Source target: **Go 1.26.0 module / Go 1.26.5 container image**
- Assembly runner available: **Go 1.23.2, linux/amd64**

This document prevents configuration presence from being confused with executed evidence.

## Executed successfully

The module directive was temporarily lowered only in an isolated validation copy because the assembly runner did not contain Go 1.26; the checked-in `go.mod` remained at Go 1.26.0.

```text
gofmt check
go vet ./...
go test ./...
go test -race ./...
build cmd/authz-api
build cmd/audit-worker
build cmd/demo-resource-api
build cmd/admission-webhook
HTTP allow/deny smoke flow
Prometheus metrics smoke flow
strict JSON extra-value and malformed-tail checks
```

The in-process benchmark completed on an AMD EPYC 9V74 runner:

```text
BenchmarkDecisionService-5  1330401  900.9 ns/op  136 B/op  6 allocs/op
```

This benchmark uses fake in-memory dependencies. It is useful only as an allocation/regression baseline and is **not** an end-to-end OpenFGA/OPA latency claim.

Static checks also completed for JSON/YAML parsing, shell syntax, local Markdown links, and trailing whitespace. Helm templates were excluded from generic YAML parsing because they contain Go template expressions.

## Not executed in the assembly runner

The runner did not provide Docker, OPA CLI, OpenFGA CLI, Helm, Terraform, or external Go-module network resolution. Therefore these checks remain mandatory before the first public tag:

```bash
go mod tidy
go test -tags 'oidc postgres rabbitmq otel' ./...
make policy-test
make model-test
make compose-validate
make helm-lint
terraform -chdir=infrastructure/terraform/environments/dev init -backend=false
terraform -chdir=infrastructure/terraform/environments/dev validate
```

Then run the real integration path:

```bash
make infra-up
./scripts/bootstrap-openfga.sh
make sidecar-up
make sidecar-smoke
```

Record the outputs in the first pull request or release evidence. Do not claim the Helm, Terraform, token-exchange, remote-engine, or load-test paths as verified until those commands pass.

## First-machine exit criteria

1. Commit the generated `go.sum` from Go 1.26.5.
2. Confirm CI executes the Go 1.26 module graph and optional build tags.
3. Run OPA and OpenFGA policy/model tests using the pinned images.
4. Validate Compose, Helm, and Terraform with their pinned baselines.
5. Capture one allowed and one denied Envoy request with the same decision ID visible at the protected API.
6. Keep benchmark and k6 evidence labeled with hardware, commit SHA, scenario, and configuration.

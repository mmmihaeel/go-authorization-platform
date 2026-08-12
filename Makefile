SHELL := /bin/sh
GO ?= go
DOCKER ?= docker
COMPOSE ?= docker compose
OPA_VERSION ?= 1.19.0-static
FGA_CLI_VERSION ?= v0.7.20
K6_VERSION ?= 2.2.0
CHART := deploy/helm/go-authorization-platform

.PHONY: help fmt vet test race bench build check policy-test model-test helm-lint compose-validate validate \
        up observability-up down reset demo smoke fail-closed-smoke load

help:
	@printf '%s\n' \
	  'check             format check, vet, unit/integration tests, race tests' \
	  'validate          check + OPA/OpenFGA policy tests + Helm lint' \
	  'up                build and start the complete local stack' \
	  'demo              exercise real JWT, Envoy, OpenFGA, OPA, outbox and RabbitMQ flows' \
	  'fail-closed-smoke verify missing-token and OPA-outage enforcement' \
	  'observability-up   start optional Prometheus and Grafana services' \
	  'reset             remove local containers and development volumes' \
	  'smoke             alias for demo' \
	  'load              run k6 scenario against the decision API'

fmt:
	@unformatted="$$(find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -l)"; \
	  test -z "$$unformatted" || { printf '%s\n' "$$unformatted"; echo 'Run gofmt on the files listed above.'; exit 1; }

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

bench:
	$(GO) test -run '^$$' -bench . -benchmem ./internal/authorization

build:
	mkdir -p bin
	$(GO) build -trimpath -o bin/authz-api ./cmd/authz-api
	$(GO) build -trimpath -o bin/audit-worker ./cmd/audit-worker
	$(GO) build -trimpath -o bin/audit-consumer ./cmd/audit-consumer
	$(GO) build -trimpath -o bin/demo-resource-api ./cmd/demo-resource-api
	$(GO) build -trimpath -o bin/admission-webhook ./cmd/admission-webhook

check: fmt vet test race build

policy-test:
	$(DOCKER) run --rm -v "$(CURDIR)/policies/opa:/policies:ro" openpolicyagent/opa:$(OPA_VERSION) test -v /policies

model-test:
	$(DOCKER) run --rm -v "$(CURDIR):/workspace:ro" openfga/cli:$(FGA_CLI_VERSION) model test --tests /workspace/policies/openfga/store.fga.yaml

helm-lint:
	helm lint $(CHART)
	helm template authz-platform $(CHART) >/dev/null

compose-validate:
	$(COMPOSE) config --quiet

validate: check policy-test model-test helm-lint compose-validate

up:
	$(COMPOSE) up -d --build --wait

observability-up:
	$(COMPOSE) --profile observability up -d --wait prometheus grafana

down:
	$(COMPOSE) --profile observability down --remove-orphans

reset:
	$(COMPOSE) --profile observability down --remove-orphans --volumes

demo:
	sh ./scripts/demo.sh

smoke:
	sh ./scripts/demo.sh

fail-closed-smoke:
	sh ./scripts/fail-closed-smoke.sh

load:
	K6_VERSION=$(K6_VERSION) sh ./scripts/load-smoke.sh

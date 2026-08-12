SHELL := /bin/sh
GO ?= go
DOCKER ?= docker
COMPOSE ?= docker compose
OPA_VERSION ?= 1.19.0-static
FGA_CLI_VERSION ?= v0.7.20
K6_VERSION ?= 2.2.0
CHART := deploy/helm/go-authorization-platform

.PHONY: help fmt vet test race bench build run check policy-test model-test helm-lint compose-validate validate \
        app-up sidecar-up infra-up identity-up observability-up down smoke sidecar-smoke load

help:
	@printf '%s\n' \
	  'check             format check, vet, unit/integration tests, race tests' \
	  'validate          check + OPA/OpenFGA policy tests + Helm lint' \
	  'run               run the API in self-contained demo mode' \
	  'app-up            build and start API + demo resource service' \
	  'sidecar-up        start Envoy ext_authz demo on localhost:10000' \
	  'infra-up          start PostgreSQL, OpenFGA, OPA, RabbitMQ' \
	  'identity-up       start local Keycloak on localhost:8082' \
	  'observability-up  start OTel Collector, Prometheus, Grafana' \
	  'smoke             test allow and deny API decisions' \
	  'load              run k6 scenario against the decision API'

fmt:
	@test -z "$$($(GO)fmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || \
	  { echo 'Run gofmt on the files listed above.'; $(GO)fmt -l $$(find . -name '*.go' -not -path './vendor/*'); exit 1; }

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
	$(GO) build -trimpath -o bin/demo-resource-api ./cmd/demo-resource-api
	$(GO) build -trimpath -o bin/admission-webhook ./cmd/admission-webhook

run:
	AUTHZ_MODE=demo $(GO) run ./cmd/authz-api

check: fmt vet test race build

policy-test:
	$(DOCKER) run --rm -v "$(CURDIR)/policies/opa:/policies:ro" openpolicyagent/opa:$(OPA_VERSION) test -v /policies

model-test:
	$(DOCKER) run --rm -v "$(CURDIR):/workspace:ro" openfga/cli:$(FGA_CLI_VERSION) model test --tests /workspace/policies/openfga/store.fga.yaml

helm-lint:
	helm lint $(CHART)
	helm template authz-platform $(CHART) >/dev/null

compose-validate:
	$(COMPOSE) --profile app --profile sidecar --profile infra --profile identity --profile observability config --quiet

validate: check policy-test model-test helm-lint compose-validate

app-up:
	$(COMPOSE) --profile app up -d --build

sidecar-up:
	$(COMPOSE) --profile sidecar up -d --build

infra-up:
	$(COMPOSE) --profile infra up -d

identity-up:
	$(COMPOSE) --profile identity up -d

observability-up:
	$(COMPOSE) --profile observability up -d

down:
	$(COMPOSE) --profile app --profile sidecar --profile infra --profile identity --profile observability down --remove-orphans

smoke:
	./scripts/smoke-test.sh

sidecar-smoke:
	./scripts/sidecar-smoke-test.sh

load:
	$(DOCKER) run --rm --network host -v "$(CURDIR)/tests/load:/scripts:ro" grafana/k6:$(K6_VERSION) run /scripts/decision-api.js

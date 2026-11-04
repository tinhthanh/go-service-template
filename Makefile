# Every command goes through here, so a person, an agent, the hooks and CI
# run the same thing with the same flags. `make help` lists them.
#
#   make check     the inner loop: generated code current, lint, every test (the Stop hook runs it)
#   make verify    what CI runs, here, before a push: check, tidy modules, known vulnerabilities

.DEFAULT_GOAL := help
SHELL := /bin/bash

GEN_TMP := $(shell mktemp -d 2>/dev/null || echo /tmp/gen-check)

help: ## List the targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / { printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
.PHONY: help

setup: ## Point git at .githooks (format and lint on commit, make check on push)
	git config core.hooksPath .githooks
.PHONY: setup

generate: ## Write internal/api from api/openapi.json
	go generate ./internal/api/...
.PHONY: generate

gen-check: ## Fail when internal/api is not what api/openapi.json makes
	@go tool oapi-codegen -config api/oapi-codegen.yaml -o $(GEN_TMP)/api.gen.go api/openapi.json
	@go run ./tools/stubgen -in $(GEN_TMP)/api.gen.go -out $(GEN_TMP)/unimplemented.gen.go
	@diff -q $(GEN_TMP)/api.gen.go internal/api/api.gen.go >/dev/null && diff -q $(GEN_TMP)/unimplemented.gen.go internal/api/unimplemented.gen.go >/dev/null \
		|| (echo "gen-check: internal/api is not what api/openapi.json makes: run make generate"; exit 1)
.PHONY: gen-check

fmt: ## Format every Go file (gofumpt, goimports)
	go tool golangci-lint fmt
.PHONY: fmt

lint: ## The style and correctness checks (.golangci.yml)
	go tool golangci-lint run
.PHONY: lint

test: ## Every test, with the race detector
	go test -race -count=1 ./...
.PHONY: test

check: gen-check lint test ## The inner loop: generated code current, lint, tests
.PHONY: check

mod-check: ## Fail when go.mod or go.sum aren't tidy
	go mod tidy -diff
.PHONY: mod-check

vuln: ## Known vulnerabilities in the code the service calls
	go tool govulncheck ./...
.PHONY: vuln

verify: check mod-check vuln ## What CI runs: make verify passing here is CI passing
.PHONY: verify

contract-sync: ## Copy the client's contract in: make contract-sync FROM=<the client's repository>
	@test -n "$(FROM)" || (echo "contract-sync: make contract-sync FROM=<the client's repository, with api/openapi.json>"; exit 1)
	cp "$(FROM)/api/openapi.json" api/openapi.json
	$(MAKE) generate
.PHONY: contract-sync

run: ## Run the server on :8080 (settings from the environment, .env.example)
	go run ./cmd/server
.PHONY: run

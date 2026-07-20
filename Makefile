SHELL := /bin/sh

COMPOSE ?= docker compose
GO_PACKAGES := ./cmd/... ./internal/...

.DEFAULT_GOAL := help

.PHONY: help build up down restart ps logs health seed telegram db-shell clean test test-integration lint

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_-]+:.*##/ {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build Docker images
	$(COMPOSE) build

up: ## Build and start the standard local demo in the background
	$(COMPOSE) up --build -d

down: ## Stop containers while preserving the PostgreSQL volume
	$(COMPOSE) down

restart: down up ## Restart the standard local demo

ps: ## Show Compose service status
	$(COMPOSE) ps

logs: ## Follow logs for all services
	$(COMPOSE) logs -f

health: ## Check the public readiness endpoint
	curl --fail --silent --show-error http://localhost:18080/api/readyz

seed: ## Re-index the built-in demo documents
	$(COMPOSE) up -d postgres
	$(COMPOSE) run --rm seed

telegram: ## Start the optional Telegram bot profile
	$(COMPOSE) --profile telegram up --build -d

db-shell: ## Open a PostgreSQL shell in the database container
	$(COMPOSE) exec postgres psql -U docmind -d docmind

clean: ## Stop containers and permanently remove local PostgreSQL data
	$(COMPOSE) down -v

test: ## Run Go checks and build the admin interface locally
	GOCACHE=/tmp/docmind-go-build go test $(GO_PACKAGES)
	GOCACHE=/tmp/docmind-go-build go vet $(GO_PACKAGES)
	GOCACHE=/tmp/docmind-go-build go test -race $(GO_PACKAGES)
	npm --prefix web/admin ci
	npm --prefix web/admin run lint
	npm --prefix web/admin run test
	npm --prefix web/admin run build

test-integration: ## Run PostgreSQL integration tests (requires DOCMIND_TEST_DATABASE_URL)
	DOCMIND_TEST_DATABASE_URL="$(DOCMIND_TEST_DATABASE_URL)" GOCACHE=/tmp/docmind-go-build go test -tags=integration ./tests/integration

lint: ## Run the Go linter
	GOCACHE=/tmp/docmind-go-build GOLANGCI_LINT_CACHE=/tmp/docmind-golangci-cache golangci-lint run $(GO_PACKAGES)

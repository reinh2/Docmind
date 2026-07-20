SHELL := /bin/sh

COMPOSE ?= docker compose
GO_PACKAGES := ./cmd/... ./internal/...
DEMO_SITE_PORT ?= 8899
DEMO_SITE_DIR := examples
DEMO_SITE_PID := /tmp/docmind-demo-site.pid
OPEN_CMD := $(shell command -v open || command -v xdg-open || echo true)

.DEFAULT_GOAL := help

.PHONY: help build up down restart ps logs health seed telegram db-shell clean test test-integration lint demo demo-site demo-site-stop open

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z_-]+:.*##/ {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build Docker images
	$(COMPOSE) build

up: demo-site ## Build and start the standard local demo, plus the customer-facing widget demo site
	$(COMPOSE) up --build -d
	@echo "Admin console: http://localhost:18080 (token: demo-admin-token)"

down: demo-site-stop ## Stop containers (and the demo site server) while preserving the PostgreSQL volume
	$(COMPOSE) down

restart: down up ## Restart the standard local demo

demo: up open ## One-shot: start the stack (with the widget demo site) and open both in a browser

demo-site: ## Serve the standalone widget demo page (examples/demo-site.html) in the background
	@$(MAKE) demo-site-stop --no-print-directory
	@(cd $(DEMO_SITE_DIR) && nohup python3 -m http.server $(DEMO_SITE_PORT) >/tmp/docmind-demo-site.log 2>&1 & echo $$! > $(DEMO_SITE_PID))
	@sleep 1
	@echo "Demo site: http://localhost:$(DEMO_SITE_PORT)/demo-site.html"

demo-site-stop: ## Stop the demo site static server
	@[ -f $(DEMO_SITE_PID) ] && kill $$(cat $(DEMO_SITE_PID)) 2>/dev/null; rm -f $(DEMO_SITE_PID)

open: ## Open the admin console and demo site in your default browser
	@$(OPEN_CMD) http://localhost:18080 >/dev/null 2>&1 || true
	@$(OPEN_CMD) http://localhost:$(DEMO_SITE_PORT)/demo-site.html >/dev/null 2>&1 || true

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

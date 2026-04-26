SHELL := /usr/bin/env bash

DB ?= bite.db
MIGRATIONS_DIR := internal/db/migrations

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ─── Day-to-day ─────────────────────────────────────────────────────────────

.PHONY: setup
setup: ## First-time bootstrap: go mod tidy + sqlc generate
	go mod tidy
	sqlc generate

.PHONY: run
run: ## Run bite (opens chat TUI)
	go run ./cmd/bite

.PHONY: build
build: ## Build ./bin/bite
	mkdir -p bin
	go build -o bin/bite ./cmd/bite

.PHONY: test
test: ## Run tests
	go test -race ./...

.PHONY: cover
cover: ## Run tests with coverage
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	gofmt -w -s .

# ─── Codegen ────────────────────────────────────────────────────────────────

.PHONY: sqlc
sqlc: ## Regenerate Go code from SQL via sqlc
	sqlc generate

# ─── Migrations (auto-applied on startup; these are for manual control) ─────

.PHONY: migrate-up migrate-down migrate-status migrate-new
migrate-up: ## Apply pending migrations
	goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB) up

migrate-down: ## Roll back the most recent migration
	goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB) down

migrate-status: ## Show migration status
	goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB) status

migrate-new: ## Create a new migration: make migrate-new name=foo
	@[ -n "$(name)" ] || (echo "usage: make migrate-new name=<migration_name>" && exit 1)
	goose -dir $(MIGRATIONS_DIR) create $(name) sql

# ─── Release / Docker ───────────────────────────────────────────────────────

.PHONY: release-snapshot
release-snapshot: ## Local snapshot release (no publish, no docker)
	goreleaser release --snapshot --clean --skip=publish,docker,docker-manifest

.PHONY: docker
docker: build ## Build local Docker image bite:dev
	docker build -t bite:dev -f Dockerfile bin/

# ─── Housekeeping ───────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts and local DB
	rm -rf bin dist coverage.out
	rm -f $(DB) $(DB)-journal

.PHONY: help build run test lint vet clean docker migrate-up migrate-down migrate-version migrate-baseline migrate-create schema-diff schema-diff-sql stale-cols dev backup restore dex-up dex-down dex-logs frontend-build frontend-dev

# Default target: print the help screen.
.DEFAULT_GOAL := help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.Version=$(VERSION)"

# Help — list non-obvious targets with a one-line description each.
help:
	@echo "Paper LMS — Makefile targets"
	@echo ""
	@echo "Build & run:"
	@echo "  build              Build the server + migrate binaries into ./bin"
	@echo "  run                Build then run the server"
	@echo "  dev                Run backend + frontend dev servers concurrently"
	@echo "  clean              Remove build artifacts (bin/, web/dist/)"
	@echo ""
	@echo "Quality:"
	@echo "  test               go test ./..."
	@echo "  vet                go vet ./..."
	@echo "  lint               vet + golangci-lint (if installed)"
	@echo ""
	@echo "Frontend:"
	@echo "  frontend-build     Build the Vite React bundle (web/dist)"
	@echo "  frontend-dev       Start the Vite dev server (HMR on :5174)"
	@echo ""
	@echo "Database migrations (prod requires AUTO_MIGRATE=false):"
	@echo "  migrate-up         Apply all pending migrations"
	@echo "  migrate-down       Roll back the last migration"
	@echo "  migrate-version    Print current migration version"
	@echo "  migrate-baseline   Mark current schema as baseline (existing prod DB)"
	@echo "  migrate-create     Scaffold a new NNNNNN_name.{up,down}.sql migration pair"
	@echo ""
	@echo "Schema parity (run before merging a model change):"
	@echo "  schema-diff        Report tables/indexes the SQL chain is missing vs GORM AutoMigrate"
	@echo "  schema-diff-sql    Same, but emit paste-ready CREATE TABLE/INDEX statements"
	@echo "  stale-cols         Categorize SQL-chain columns AutoMigrate doesn't know about; writes STALE_COLUMNS.md"
	@echo ""
	@echo "Local OIDC (Phase 10-A.7 — Dex testing):"
	@echo "  dex-up             Start the local Dex OIDC server (docker-compose --profile dex)"
	@echo "  dex-down           Stop the local Dex server"
	@echo "  dex-logs           Tail Dex logs"
	@echo ""
	@echo "Docker & ops:"
	@echo "  docker             docker build -t paper-lms ."
	@echo "  backup             Run scripts/backup.sh"
	@echo "  restore            Run scripts/restore.sh BACKUP_FILE=<path>"
	@echo ""
	@echo "See PROJECT.md and docs/adr/ for the 'why' behind these targets."

# Build
build:
	go build $(LDFLAGS) -o bin/paper-lms ./cmd/server
	go build -o bin/migrate ./cmd/migrate

run: build
	./bin/paper-lms

dev:
	cd web && npm run dev &
	go run ./cmd/server

# Quality
test:
	go test ./...

vet:
	go vet ./...

lint: vet
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

# Frontend
frontend-build:
	cd web && npm run build

frontend-dev:
	cd web && npm run dev

# Database migrations
migrate-up: build
	./bin/migrate up

migrate-down: build
	./bin/migrate down

migrate-version: build
	./bin/migrate version

migrate-baseline: build
	./bin/migrate baseline

migrate-create:
	@read -p "Migration name: " name; \
	version=$$(ls -1 internal/db/migrations/*.up.sql 2>/dev/null | wc -l | tr -d ' '); \
	version=$$((version + 1)); \
	padded=$$(printf "%06d" $$version); \
	touch "internal/db/migrations/$${padded}_$${name}.up.sql"; \
	touch "internal/db/migrations/$${padded}_$${name}.down.sql"; \
	echo "Created internal/db/migrations/$${padded}_$${name}.up.sql"; \
	echo "Created internal/db/migrations/$${padded}_$${name}.down.sql"

# Schema parity: compare GORM AutoMigrate against the SQL migration chain.
# Spins up two scratch databases on the configured Postgres, runs both schema
# builders, and reports tables/indexes the SQL chain is missing. Add --emit-sql
# to print paste-ready CREATE TABLE / CREATE INDEX statements.
#
# CI enforces parity via TestSchemaParity_Wave1; run this locally when you add
# a new model to discover what migration content to author.
schema-diff:
	@go run ./cmd/schemadiff

schema-diff-sql:
	@go run ./cmd/schemadiff --emit-sql

# Wave 2a: categorize stale columns (SQL chain has, AutoMigrate doesn't) into
# RENAME_CANDIDATE, SOFT_DELETE_LEFTOVER, POLYMORPHIC_REFACTOR, or UNKNOWN, and
# write the report to STALE_COLUMNS.md for human review before authoring
# cleanup migrations.
stale-cols:
	@go run ./cmd/stalecols

# Docker
docker:
	docker build -t paper-lms .

# Database backup/restore
backup:
	@./scripts/backup.sh

restore:
	@./scripts/restore.sh $(BACKUP_FILE)

# Dex (Phase 10-A.7) — local OIDC server for OIDC E2E testing.
# Only started explicitly; not part of `docker-compose up`.
dex-up:
	docker-compose --profile dex up -d dex

dex-down:
	docker-compose --profile dex down dex

dex-logs:
	docker-compose --profile dex logs -f dex

# Cleanup
clean:
	rm -rf bin/
	rm -rf web/dist/

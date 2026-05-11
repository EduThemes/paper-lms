.PHONY: build run test lint vet clean docker migrate-up migrate-down migrate-version migrate-baseline migrate-create schema-diff schema-diff-sql stale-cols dev backup restore

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.Version=$(VERSION)"

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

# Cleanup
clean:
	rm -rf bin/
	rm -rf web/dist/

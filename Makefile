.PHONY: setup run run-api run-db build test sqlc \
        migrate-diff migrate-apply migrate-hash migrate-status \
        up down logs

# Load .env for host runs (run / run-api / migrate-*). Optional + guarded. Docker compose
# loads .env on its own, so this only affects `go run`/`atlas` on the host.
ifneq (,$(wildcard .env))
include .env
export
endif

# Resolve dependencies and create go.sum (run this first).
setup:
	go mod tidy

# Fast dev loop: Postgres in Docker (+ migrations), API on the host. Frees :8080 first.
run:
	-docker compose stop api 2>/dev/null
	$(MAKE) run-db
	$(MAKE) run-api

# Start Postgres in Docker and apply Atlas migrations (no local Postgres/atlas needed).
run-db:
	docker compose up -d --wait postgres
	docker compose run --rm migrate

run-api:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

# Requires: github.com/sqlc-dev/sqlc on PATH. Regenerates internal/repo/sqlc.
sqlc:
	sqlc generate

# --- Atlas migrations (schema source of truth: db/query/schema.sql) ---
# Draft a migration from schema.sql changes:  make migrate-diff name=add_something
migrate-diff:
	atlas migrate diff $(name) --env local

# Apply pending migrations to $DATABASE_URL (host):
migrate-apply:
	atlas migrate apply --env local --url "$(MIGRATE_DATABASE_URL)"

# Recompute checksums after manual edits to a migration:
migrate-hash:
	atlas migrate hash --env local

migrate-status:
	atlas migrate status --env local --url "$(MIGRATE_DATABASE_URL)"

# Run the whole stack (postgres + migrate + api) in Docker.
up:
	docker compose up --build

down:
	docker compose down

# Tail the API logs (Docker stack).
logs:
	docker compose logs -f api

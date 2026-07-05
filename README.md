# Student Success — Backend

Multi-tenant SaaS backend for the Student Success retention platform, sold per
educational center (o'quv markazi). Each center is a tenant isolated by `org_id` +
Postgres Row-Level Security. Clean architecture, mirroring the Yurtal/Jarvis backend
conventions (Huma + Echo).

## Stack

| Concern        | Choice |
|----------------|--------|
| Language       | Go 1.25 |
| HTTP           | Echo + **Huma v2** (OpenAPI 3.1 + Swagger UI at `/docs`) |
| Database       | Postgres (pgx) |
| Queries        | sqlc (type-safe, no ORM) |
| Migrations     | **Atlas** (`db/query/schema.sql` is the source of truth) |
| Auth           | JWT **access + refresh** (refresh sessions in Postgres), bcrypt |
| Observability  | zerolog + Prometheus (`/metrics`) |
| i18n           | uz / ru / en localized error messages |
| Bot            | Telegram (single platform bot + deep-link onboarding) — planned |

## Architecture

Clean architecture; dependencies point inward
(`controller/http → usecase → repo → entity`):

```
cmd/api/main.go                      entrypoint (load config → app.New → Run)
internal/
  app/                               composition root (wiring + errgroup Run)
  config/                            cleanenv + godotenv (config.yml optional, env wins)
  controller/http/                   Echo + Huma: server, middleware, register helpers,
                                     auth operations (validation tags + i18n errors)
  usecase/                           business logic (auth: register/login/refresh)
  repo/                              persistence ports (contracts.go) +
    postgres/                          sqlc-backed adapters (RLS-scoped)
    sqlc/                              generated (DO NOT EDIT)
  entity/                            domain models (Organization, User, Principal, …)
  security/                          JWT (access+refresh) + bcrypt
  i18n/                              localized messages
  observability/                     zerolog logger + Prometheus metrics
  platform/postgres/                 pgx pool + tenant (RLS) helpers
  risk/                              PURE risk-scoring engine (done + tested)
db/query/schema.sql                  desired schema (sqlc + Atlas source of truth)
db/query/query.sql                   named sqlc queries
db/migrations/                       Atlas versioned migrations (+ atlas.sum)
```

### Tenant isolation (the key SaaS decision)

Shared schema, every row tagged with `org_id`. Two layers of defense:
1. Handlers read `org_id` from the JWT principal (never the request body).
2. `platform/postgres.WithTenant` sets `app.current_org` per transaction; **RLS**
   policies (with `FORCE ROW LEVEL SECURITY`) filter every row — a query that forgets
   its filter still can't leak across tenants.

## Run it

```bash
cp .env.example .env          # then set a strong JWT_SECRET (openssl rand -base64 48)

make up                       # all in Docker: postgres + Atlas migrate + api
# or, fast host dev loop (Postgres in Docker, API on host):
make run

curl localhost:8087/healthz   # {"status":"ok"}
open  http://localhost:8087/docs   # Swagger UI
```

Auth endpoints: `POST /api/v1/auth/{register,login,refresh}`.

## Database workflow (Atlas + sqlc)

```bash
# 1. edit db/query/schema.sql (and db/query/query.sql for new queries)
make migrate-diff name=add_students   # draft a migration from the schema change
make migrate-apply                    # apply to $DATABASE_URL
make sqlc                             # regenerate internal/repo/sqlc
```

`make test` runs the unit tests (risk engine).

## Status

Foundation complete: clean-arch layering, config/observability/security/i18n, RLS tenant
isolation, and the **auth vertical (register / login / refresh) — done + verified**. The
risk engine is implemented + tested. Next: students CRUD wired to the risk engine, then
surveys/attendance → intervention tasks, Telegram bot, integrations, AI advisor.

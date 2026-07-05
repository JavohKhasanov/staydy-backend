// Atlas config. Schema source of truth: db/query/schema.sql.
// Dev DB is an ephemeral Docker Postgres used only for diffing/linting (no local PG needed).
//
//   make migrate-diff name=add_something   # draft a migration from schema.sql changes
//   make migrate-apply                      # apply pending migrations to $DATABASE_URL
env "local" {
  src = "file://db/query/schema.sql"
  dev = "docker://postgres/16/dev?search_path=public"
  migration {
    dir = "file://db/migrations"
  }
}

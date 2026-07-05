-- Row-Level Security for tenant isolation.
--
-- IMPORTANT: Atlas Community's `migrate diff` engine silently drops RLS/policy DDL, so
-- this migration is HAND-WRITTEN (Atlas only drops RLS on diff, not on apply). Keep it in
-- sync with the RLS block in db/query/schema.sql. After editing, run `make migrate-hash`.
--
-- FORCE makes policies apply to the table owner too (the app currently connects as the
-- owning role `ssp`). Follow-up hardening: run the app as a dedicated non-owner role.

ALTER TABLE "users"               ENABLE ROW LEVEL SECURITY;
ALTER TABLE "students"            ENABLE ROW LEVEL SECURITY;
ALTER TABLE "attendance_records"  ENABLE ROW LEVEL SECURITY;
ALTER TABLE "surveys"             ENABLE ROW LEVEL SECURITY;
ALTER TABLE "notes"               ENABLE ROW LEVEL SECURITY;
ALTER TABLE "intervention_tasks"  ENABLE ROW LEVEL SECURITY;
ALTER TABLE "bot_conversations"   ENABLE ROW LEVEL SECURITY;
ALTER TABLE "invite_tokens"       ENABLE ROW LEVEL SECURITY;

ALTER TABLE "users"               FORCE ROW LEVEL SECURITY;
ALTER TABLE "students"            FORCE ROW LEVEL SECURITY;
ALTER TABLE "attendance_records"  FORCE ROW LEVEL SECURITY;
ALTER TABLE "surveys"             FORCE ROW LEVEL SECURITY;
ALTER TABLE "notes"               FORCE ROW LEVEL SECURITY;
ALTER TABLE "intervention_tasks"  FORCE ROW LEVEL SECURITY;
ALTER TABLE "bot_conversations"   FORCE ROW LEVEL SECURITY;
ALTER TABLE "invite_tokens"       FORCE ROW LEVEL SECURITY;

CREATE POLICY org_isolation ON "users"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON "students"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON "attendance_records"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON "surveys"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON "notes"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON "intervention_tasks"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON "bot_conversations"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON "invite_tokens"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

-- Dedicated NON-superuser application role. RLS is bypassed by superusers and by the
-- table owner (without FORCE), so the runtime must NOT connect as `ssp` (the bootstrap
-- superuser/owner). The app connects as `app`; migrations still run as `ssp`.
-- Dev password is 'app' — set a real secret for prod.
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app') THEN
        CREATE ROLE app LOGIN PASSWORD 'app';
    END IF;
END $$;
GRANT USAGE ON SCHEMA public TO app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app;

-- In-app notification feed for back-office users (staff alerts stay inside the platform, no
-- Telegram needed for staff). Each row targets one user within an org; RLS scopes by org, and
-- queries additionally filter by the recipient user_id.
CREATE TABLE "notifications" (
    "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"     uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "user_id"    uuid NOT NULL REFERENCES "users" ("id") ON DELETE CASCADE,
    "kind"       text NOT NULL,
    "title"      text NOT NULL,
    "body"       text NOT NULL DEFAULT '',
    "link"       text NOT NULL DEFAULT '',
    "read_at"    timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX "idx_notifications_user" ON "notifications" ("org_id", "user_id", "created_at" DESC);

ALTER TABLE "notifications" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "notifications" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "notifications"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

-- escalated_at dedupes the SLA escalation: a stale open task escalates to the director once.
ALTER TABLE "intervention_tasks" ADD COLUMN "escalated_at" timestamptz;

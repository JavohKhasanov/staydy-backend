-- Faza A2 (CRM): activities — a polymorphic communication timeline (call/sms/note/meeting) that
-- attaches to a lead or a student via (subject_type, subject_id). RLS hand-written.

CREATE TABLE "activities" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "subject_type" text NOT NULL, "subject_id" uuid NOT NULL, "type" text NOT NULL DEFAULT 'note', "body" text NOT NULL DEFAULT '', "author" text NOT NULL DEFAULT '', "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "activities_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
CREATE INDEX "idx_activities_subject" ON "activities" ("org_id", "subject_type", "subject_id");

ALTER TABLE "activities" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "activities" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "activities"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

-- Phase 0: student groups (cohorts) + teacher assignment + students.group_id.
-- (Hand-trimmed from `atlas migrate diff`: dropped unrelated constraint-name normalization
-- on attendance/surveys/notes/intervention_tasks and the unused idx_invite_tokens_student
-- drop — those are pre-existing schema.sql drift, not part of this feature.)

-- Create "groups" table
CREATE TABLE "groups" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "name" text NOT NULL, "teacher_id" uuid NULL, "direction" text NULL, "schedule_days" text NULL, "created_at" timestamptz NOT NULL DEFAULT now(), "updated_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "groups_org_id_id_key" UNIQUE ("org_id", "id"), CONSTRAINT "groups_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "groups_teacher_id_fkey" FOREIGN KEY ("teacher_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL);
-- Create index "idx_groups_org" to table: "groups"
CREATE INDEX "idx_groups_org" ON "groups" ("org_id");
-- Create index "idx_groups_teacher" to table: "groups"
CREATE INDEX "idx_groups_teacher" ON "groups" ("teacher_id");
-- Modify "students" table: add group_id with a composite (org_id, group_id) FK to groups.
ALTER TABLE "students" ADD COLUMN "group_id" uuid NULL, ADD CONSTRAINT "students_org_id_group_id_fkey" FOREIGN KEY ("org_id", "group_id") REFERENCES "groups" ("org_id", "id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "idx_students_group" to table: "students"
CREATE INDEX "idx_students_group" ON "students" ("group_id");

-- Row-Level Security for the new groups table (HAND-WRITTEN: Atlas Community can't represent
-- RLS, so it is omitted from `migrate diff`. Keep in sync with the RLS block in schema.sql.)
ALTER TABLE "groups" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "groups" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "groups"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

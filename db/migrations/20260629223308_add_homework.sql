-- Phase 1: homework completion tracking (mirrors attendance_records).
-- (Hand-trimmed from `atlas migrate diff`: dropped the unrelated constraint-name
-- normalization on attendance/surveys/notes/intervention_tasks and the idx_invite_tokens_student
-- drop — pre-existing schema.sql drift, not part of this feature.)

-- Create "homework_records" table
CREATE TABLE "homework_records" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "student_id" uuid NOT NULL, "date" date NOT NULL, "is_done" boolean NOT NULL, "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "homework_records_org_id_student_id_date_key" UNIQUE ("org_id", "student_id", "date"), CONSTRAINT "homework_records_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "homework_records_org_id_student_id_fkey" FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_homework_student" to table: "homework_records"
CREATE INDEX "idx_homework_student" ON "homework_records" ("student_id");

-- Row-Level Security for homework_records (HAND-WRITTEN: Atlas Community can't represent RLS).
ALTER TABLE "homework_records" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "homework_records" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "homework_records"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

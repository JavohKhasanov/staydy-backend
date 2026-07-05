-- Faza B3 (CRM): lessons — scheduled class sessions (the timetable). Additive; attendance is not
-- reworked here (still per-day). RLS hand-written.

CREATE TABLE "lessons" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "group_id" uuid NULL, "teacher_id" uuid NULL, "date" date NOT NULL, "start_time" text NOT NULL DEFAULT '', "end_time" text NOT NULL DEFAULT '', "room" text NOT NULL DEFAULT '', "topic" text NOT NULL DEFAULT '', "status" text NOT NULL DEFAULT 'scheduled', "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "lessons_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "lessons_teacher_id_fkey" FOREIGN KEY ("teacher_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL);
CREATE INDEX "idx_lessons_org_date" ON "lessons" ("org_id", "date");
CREATE INDEX "idx_lessons_group" ON "lessons" ("group_id");

ALTER TABLE "lessons" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "lessons" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "lessons"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

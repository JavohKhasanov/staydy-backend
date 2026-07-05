-- Faza B2 (CRM): enrollments — a student's enrolment in a group/course with lifecycle status,
-- price and dates. Additive successor to students.group_id (which stays for now). RLS is
-- hand-written since Atlas Community can't represent it.

-- Create "enrollments" table
CREATE TABLE "enrollments" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "student_id" uuid NOT NULL, "group_id" uuid NULL, "course_id" uuid NULL, "status" text NOT NULL DEFAULT 'active', "start_date" date NULL, "end_date" date NULL, "price" bigint NOT NULL DEFAULT 0, "discount" integer NOT NULL DEFAULT 0, "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "enrollments_org_id_student_id_fkey" FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_enrollments_student" to table: "enrollments"
CREATE INDEX "idx_enrollments_student" ON "enrollments" ("student_id");
-- Create index "idx_enrollments_org" to table: "enrollments"
CREATE INDEX "idx_enrollments_org" ON "enrollments" ("org_id");

-- Row-Level Security for enrollments (HAND-WRITTEN: Atlas Community can't represent RLS).
ALTER TABLE "enrollments" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "enrollments" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "enrollments"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

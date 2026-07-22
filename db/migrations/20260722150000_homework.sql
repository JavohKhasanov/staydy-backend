-- Homework: a teacher/admin attaches an assignment to a group (optionally a lesson day); students
-- submit text + links; the teacher grades (status + score). Distinct from the legacy
-- homework_records boolean (a per-date retention flag) — this is the richer LMS-style flow.
CREATE TABLE "homework_assignments" (
    "id"          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"      uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "group_id"    uuid NOT NULL REFERENCES "groups" ("id") ON DELETE CASCADE,
    "lesson_date" date,
    "title"       text NOT NULL,
    "description" text NOT NULL DEFAULT '',
    "deadline"    timestamptz,
    "max_score"   int NOT NULL DEFAULT 100,
    "created_at"  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX "idx_hw_assignments_group" ON "homework_assignments" ("group_id");
CREATE INDEX "idx_hw_assignments_org" ON "homework_assignments" ("org_id");

CREATE TABLE "homework_submissions" (
    "id"            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"        uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "assignment_id" uuid NOT NULL REFERENCES "homework_assignments" ("id") ON DELETE CASCADE,
    "student_id"    uuid NOT NULL,
    "text"          text NOT NULL DEFAULT '',
    "links"         text NOT NULL DEFAULT '',              -- newline-separated URLs
    "status"        text NOT NULL DEFAULT 'submitted',     -- submitted | accepted | rejected
    "score"         int,
    "review_note"   text NOT NULL DEFAULT '',
    "submitted_at"  timestamptz NOT NULL DEFAULT now(),
    "reviewed_at"   timestamptz,
    "updated_at"    timestamptz NOT NULL DEFAULT now(),
    UNIQUE ("assignment_id", "student_id"),
    FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON DELETE CASCADE
);
CREATE INDEX "idx_hw_submissions_assignment" ON "homework_submissions" ("assignment_id");
CREATE INDEX "idx_hw_submissions_student" ON "homework_submissions" ("student_id");

ALTER TABLE "homework_assignments" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "homework_assignments" FORCE ROW LEVEL SECURITY;
ALTER TABLE "homework_submissions" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "homework_submissions" FORCE ROW LEVEL SECURITY;
CREATE POLICY "org_isolation" ON "homework_assignments"
    USING ("org_id" = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY "org_isolation" ON "homework_submissions"
    USING ("org_id" = nullif(current_setting('app.current_org', true), '')::uuid);

-- Exams: structured graded assessments per group (distinct from homework). A teacher creates an
-- exam for a group, then records each student's score; a good result awards XP (feeds the rating).
CREATE TABLE "exams" (
    "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"     uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "group_id"   uuid NOT NULL REFERENCES "groups" ("id") ON DELETE CASCADE,
    "title"      text NOT NULL,
    "exam_date"  date,
    "max_score"  int NOT NULL DEFAULT 100,
    "created_at" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX "idx_exams_group" ON "exams" ("group_id");

CREATE TABLE "exam_results" (
    "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"     uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "exam_id"    uuid NOT NULL REFERENCES "exams" ("id") ON DELETE CASCADE,
    "student_id" uuid NOT NULL,
    "score"      int NOT NULL DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "updated_at" timestamptz NOT NULL DEFAULT now(),
    UNIQUE ("exam_id", "student_id")
);
CREATE INDEX "idx_exam_results_exam" ON "exam_results" ("exam_id");

ALTER TABLE "exams" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "exams" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "exams"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

ALTER TABLE "exam_results" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "exam_results" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "exam_results"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

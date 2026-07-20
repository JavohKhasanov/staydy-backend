-- Many-to-many student<->group membership: a student can study in several groups (courses) at
-- once. students.group_id stays as the PRIMARY group (legacy display/risk); group_members is the
-- source of truth for rosters, group payments, and attendance sheets.
CREATE TABLE "group_members" (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    group_id   uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    student_id uuid NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, student_id)
);
CREATE INDEX "idx_group_members_group" ON "group_members" ("group_id");
CREATE INDEX "idx_group_members_student" ON "group_members" ("student_id");

ALTER TABLE "group_members" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "group_members" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "group_members"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

-- Seed memberships from the existing single-group assignments.
INSERT INTO "group_members" (org_id, group_id, student_id)
SELECT s.org_id, s.group_id, s.id FROM students s WHERE s.group_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Attendance can now carry which group's session it was (nullable; old rows stay NULL).
ALTER TABLE "attendance_records" ADD COLUMN "group_id" uuid REFERENCES groups(id) ON DELETE SET NULL;

-- One attendance row per student per date PER GROUP (NULL group counts as its own bucket),
-- so a student in two groups can be marked twice on the same day.
ALTER TABLE "attendance_records" DROP CONSTRAINT "attendance_records_org_student_date_key";
CREATE UNIQUE INDEX "attendance_records_day_group_key"
    ON "attendance_records" (org_id, student_id, date, group_id) NULLS NOT DISTINCT;

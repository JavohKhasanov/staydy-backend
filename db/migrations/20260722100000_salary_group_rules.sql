-- Per-group salary overrides: a teacher's variable pay can differ per group (course).
-- Absent an override, a group uses the teacher's default rule (salary_rules). The fixed base
-- stays teacher-level (salary_rules.base_amount); only the variable kind/rate varies per group.
CREATE TABLE "salary_group_rules" (
    "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"     uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "teacher_id" uuid NOT NULL REFERENCES "users" ("id") ON DELETE CASCADE,
    "group_id"   uuid NOT NULL REFERENCES "groups" ("id") ON DELETE CASCADE,
    "kind"       text NOT NULL DEFAULT 'per_student',
    "rate"       bigint NOT NULL DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "updated_at" timestamptz NOT NULL DEFAULT now(),
    UNIQUE ("org_id", "teacher_id", "group_id")
);
CREATE INDEX "idx_salary_group_rules_teacher" ON "salary_group_rules" ("teacher_id");

-- Gamification: cached XP/coin totals on the student, plus an append-only points ledger. Awards are
-- idempotent via UNIQUE (org_id, student_id, kind, ref) — recording the same attendance/submission
-- twice never double-counts.
ALTER TABLE "students" ADD COLUMN "xp" int NOT NULL DEFAULT 0;
ALTER TABLE "students" ADD COLUMN "coins" int NOT NULL DEFAULT 0;

CREATE TABLE "student_points" (
    "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"     uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "student_id" uuid NOT NULL,
    "kind"       text NOT NULL,           -- attendance | homework_submit | homework_accept | checkin | purchase | manual
    "xp"         int NOT NULL DEFAULT 0,
    "coins"      int NOT NULL DEFAULT 0,  -- negative for spends (purchases)
    "ref"        text NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT now(),
    UNIQUE ("org_id", "student_id", "kind", "ref"),
    FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON DELETE CASCADE
);
CREATE INDEX "idx_student_points_student" ON "student_points" ("student_id");

ALTER TABLE "student_points" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "student_points" FORCE ROW LEVEL SECURITY;
CREATE POLICY "org_isolation" ON "student_points"
    USING ("org_id" = nullif(current_setting('app.current_org', true), '')::uuid);

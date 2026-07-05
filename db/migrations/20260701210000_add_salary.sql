-- Faza F1 (finance completion): teacher payroll. NONE of the studied OSS (Gibbon/Frappe/OpenEduCat/
-- openSIS/SkyLearn) ship payroll, so this is our design. A salary_rule configures how a teacher is
-- paid; a salary_slip is one period's computed pay (gross ± bonus/deduction = net). Gross is computed
-- from real data (done lessons / active students / revenue) at preview time. RLS hand-written.

CREATE TABLE "salary_rules" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "org_id" uuid NOT NULL,
  "teacher_id" uuid NOT NULL,
  "kind" text NOT NULL DEFAULT 'fixed', -- fixed | per_lesson | per_student | percent_revenue
  "rate" bigint NOT NULL DEFAULT 0,      -- so'm for fixed/per_lesson/per_student; percent(0-100) for percent_revenue
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "salary_rules_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON DELETE CASCADE,
  CONSTRAINT "salary_rules_teacher_fkey" FOREIGN KEY ("teacher_id") REFERENCES "users" ("id") ON DELETE CASCADE,
  CONSTRAINT "salary_rules_org_teacher_key" UNIQUE ("org_id", "teacher_id")
);
CREATE INDEX "idx_salary_rules_org" ON "salary_rules" ("org_id");

CREATE TABLE "salary_slips" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "org_id" uuid NOT NULL,
  "teacher_id" uuid NOT NULL,
  "period_start" date NOT NULL,
  "period_end" date NOT NULL,
  "gross" bigint NOT NULL DEFAULT 0,
  "bonus" bigint NOT NULL DEFAULT 0,
  "deduction" bigint NOT NULL DEFAULT 0,
  "net" bigint NOT NULL DEFAULT 0,
  "status" text NOT NULL DEFAULT 'draft', -- draft | paid
  "note" text NOT NULL DEFAULT '',
  "paid_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "salary_slips_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON DELETE CASCADE,
  CONSTRAINT "salary_slips_teacher_fkey" FOREIGN KEY ("teacher_id") REFERENCES "users" ("id") ON DELETE CASCADE
);
CREATE INDEX "idx_salary_slips_org" ON "salary_slips" ("org_id");
CREATE INDEX "idx_salary_slips_teacher" ON "salary_slips" ("teacher_id");

ALTER TABLE "salary_rules" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "salary_rules" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "salary_rules"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

ALTER TABLE "salary_slips" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "salary_slips" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "salary_slips"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

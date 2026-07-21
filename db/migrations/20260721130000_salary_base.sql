-- Hybrid salary: a fixed base PLUS an optional variable component. gross = base + variable(kind,rate).
-- kind 'fixed' now means "no variable part" (base only). Existing rules: fold the flat kinds into
-- base where they were effectively fixed; per_lesson/per_student/percent_revenue keep their variable.
ALTER TABLE "salary_rules" ADD COLUMN "base_amount" bigint NOT NULL DEFAULT 0;

-- Old 'fixed' rules paid `rate` as the whole salary → move it to base and null the variable.
UPDATE "salary_rules" SET base_amount = rate, rate = 0 WHERE kind = 'fixed';

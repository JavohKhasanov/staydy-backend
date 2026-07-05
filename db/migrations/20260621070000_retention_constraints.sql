-- Retention data-integrity constraints (review fixes):
--  * one attendance row per student per date, one survey per student per week (upserts).
--  * at most one non-resolved intervention task per student → DB-enforced auto-open dedup,
--    so the check-then-create race can't produce duplicate Open tasks.
-- Kept in sync with db/query/schema.sql; run `make migrate-hash` after edits.

ALTER TABLE attendance_records
    ADD CONSTRAINT attendance_records_org_student_date_key UNIQUE (org_id, student_id, date);

ALTER TABLE surveys
    ADD CONSTRAINT surveys_org_student_week_key UNIQUE (org_id, student_id, week_number);

CREATE UNIQUE INDEX intervention_tasks_one_open
    ON intervention_tasks (org_id, student_id) WHERE status <> 'Resolved';

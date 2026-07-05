-- DB-level tenant integrity: a child row's org_id can never disagree with its student's
-- org_id. Replaces the single-column student FK with a COMPOSITE (org_id, student_id) FK
-- referencing students(org_id, id), so isolation no longer rests solely on an app-layer
-- existence check. Requires UNIQUE(org_id, id) on the parent.
--
-- Hand-written (kept in sync with db/query/schema.sql; run `make migrate-hash` after edits).

ALTER TABLE students ADD CONSTRAINT students_org_id_id_key UNIQUE (org_id, id);

ALTER TABLE notes DROP CONSTRAINT notes_student_id_fkey;
ALTER TABLE notes ADD CONSTRAINT notes_org_student_fkey
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE;

ALTER TABLE attendance_records DROP CONSTRAINT attendance_records_student_id_fkey;
ALTER TABLE attendance_records ADD CONSTRAINT attendance_records_org_student_fkey
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE;

ALTER TABLE surveys DROP CONSTRAINT surveys_student_id_fkey;
ALTER TABLE surveys ADD CONSTRAINT surveys_org_student_fkey
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE;

ALTER TABLE intervention_tasks DROP CONSTRAINT intervention_tasks_student_id_fkey;
ALTER TABLE intervention_tasks ADD CONSTRAINT intervention_tasks_org_student_fkey
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE;

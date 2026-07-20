-- Invoices belong to a group (course) so a multi-group student's payments don't bleed between
-- groups: paying for English must not show as paid in Math. Legacy invoices are backfilled to
-- the student's primary group.
ALTER TABLE "invoices" ADD COLUMN "group_id" uuid REFERENCES groups(id) ON DELETE SET NULL;

UPDATE "invoices" i
SET group_id = s.group_id
FROM students s
WHERE i.student_id = s.id AND i.group_id IS NULL AND s.group_id IS NOT NULL;

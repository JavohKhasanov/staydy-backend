-- assigned_to makes an intervention accountable: a director/administrator assigns a Red/at-risk
-- student's task to a staff member (teacher/mentor) who owns the follow-up. ON DELETE SET NULL so
-- removing a staff member doesn't drop the task.
ALTER TABLE "intervention_tasks" ADD COLUMN "assigned_to" uuid REFERENCES "users" ("id") ON DELETE SET NULL;

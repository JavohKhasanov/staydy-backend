-- reminded_at dedupes the "deadline in 2 hours" push: an assignment is reminded once when it first
-- enters the reminder window, then this timestamp keeps the scheduler from pinging it again.
ALTER TABLE "homework_assignments" ADD COLUMN "reminded_at" timestamptz;

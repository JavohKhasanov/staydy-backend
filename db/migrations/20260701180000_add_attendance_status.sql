-- Faza B3-full: richer attendance status (present | absent | late | excused).
-- is_present stays the risk-facing signal and the risk engine is UNTOUCHED: is_present is derived
-- as (status <> 'absent'), so 'late' and 'excused' count as attended and do NOT lower the
-- attendance rate. Existing rows are backfilled from the current is_present boolean.
ALTER TABLE "attendance_records" ADD COLUMN "status" text NOT NULL DEFAULT 'present';
UPDATE "attendance_records" SET "status" = CASE WHEN "is_present" THEN 'present' ELSE 'absent' END;

-- Group owns its recurring schedule: class time + room. schedule_days already holds the weekday
-- pattern (the UI now writes canonical codes like 'mon,wed,fri' via the weekday picker).
ALTER TABLE "groups"
    ADD COLUMN "start_time" text NOT NULL DEFAULT '',
    ADD COLUMN "end_time"   text NOT NULL DEFAULT '',
    ADD COLUMN "room_id"    uuid REFERENCES rooms(id) ON DELETE SET NULL;

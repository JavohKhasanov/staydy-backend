-- Center-configurable billing rule: how many attended sessions a new student may have before a
-- missing invoice is flagged (default 3).
ALTER TABLE "organizations" ADD COLUMN "grace_lessons" int NOT NULL DEFAULT 3;

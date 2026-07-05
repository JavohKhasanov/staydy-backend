-- Faza F3: rooms (xonalar) + double-booking prevention. A room is a physical space (optionally at a
-- branch). lessons get a nullable room_id; the usecase rejects a lesson that reuses a room at an
-- overlapping time on the same date (OpenEduCat's op.session.check_timetable pattern). RLS hand-written.
CREATE TABLE "rooms" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "org_id" uuid NOT NULL,
  "branch_id" uuid NULL,
  "name" text NOT NULL,
  "capacity" int NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "rooms_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON DELETE CASCADE,
  CONSTRAINT "rooms_branch_id_fkey" FOREIGN KEY ("branch_id") REFERENCES "branches" ("id") ON DELETE SET NULL
);
CREATE INDEX "idx_rooms_org" ON "rooms" ("org_id");

ALTER TABLE "lessons" ADD COLUMN "room_id" uuid NULL REFERENCES "rooms" ("id") ON DELETE SET NULL;

ALTER TABLE "rooms" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "rooms" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "rooms"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

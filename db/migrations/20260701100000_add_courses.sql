-- Faza B1 (CRM): courses — the sellable programs a center offers. A group runs one course;
-- enrollments + fees reference it. RLS is hand-written since Atlas Community can't represent it.

-- Create "courses" table
CREATE TABLE "courses" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "name" text NOT NULL, "level" text NOT NULL DEFAULT '', "price" bigint NOT NULL DEFAULT 0, "duration_weeks" integer NOT NULL DEFAULT 0, "description" text NOT NULL DEFAULT '', "is_active" boolean NOT NULL DEFAULT true, "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "courses_org_id_name_key" UNIQUE ("org_id", "name"), CONSTRAINT "courses_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_courses_org" to table: "courses"
CREATE INDEX "idx_courses_org" ON "courses" ("org_id");

-- Row-Level Security for courses (HAND-WRITTEN: Atlas Community can't represent RLS).
ALTER TABLE "courses" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "courses" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "courses"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

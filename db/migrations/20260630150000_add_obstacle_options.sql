-- Phase: center-configurable "biggest obstacle" choices for the weekly check-in. The bot builds
-- its obstacle keyboard from a center's active options (falling back to a default set when none
-- are configured). RLS is hand-written since Atlas Community can't represent it.

-- Create "obstacle_options" table
CREATE TABLE "obstacle_options" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "label" text NOT NULL, "position" integer NOT NULL DEFAULT 0, "is_active" boolean NOT NULL DEFAULT true, "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "obstacle_options_org_id_label_key" UNIQUE ("org_id", "label"), CONSTRAINT "obstacle_options_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_obstacle_options_org" to table: "obstacle_options"
CREATE INDEX "idx_obstacle_options_org" ON "obstacle_options" ("org_id");

-- Row-Level Security for obstacle_options (HAND-WRITTEN: Atlas Community can't represent RLS).
ALTER TABLE "obstacle_options" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "obstacle_options" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "obstacle_options"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

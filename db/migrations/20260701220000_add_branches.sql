-- Faza F2: multi-branch (filiallar). Soft-filter model (OpenSIS/Modme): branches live INSIDE one org
-- (org_id + RLS stays the hard tenant boundary); a nullable branch_id (NULL = org-wide / unassigned)
-- scopes students/groups/expenses/users to a branch. COURSES stay SHARED across branches. The owner
-- sees all branches and filters/aggregates by branch_id. RLS hand-written.
CREATE TABLE "branches" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "org_id" uuid NOT NULL,
  "name" text NOT NULL,
  "address" text NOT NULL DEFAULT '',
  "phone" text NOT NULL DEFAULT '',
  "is_active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "branches_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON DELETE CASCADE
);
CREATE INDEX "idx_branches_org" ON "branches" ("org_id");

ALTER TABLE "students" ADD COLUMN "branch_id" uuid NULL REFERENCES "branches" ("id") ON DELETE SET NULL;
ALTER TABLE "groups"   ADD COLUMN "branch_id" uuid NULL REFERENCES "branches" ("id") ON DELETE SET NULL;
ALTER TABLE "expenses" ADD COLUMN "branch_id" uuid NULL REFERENCES "branches" ("id") ON DELETE SET NULL;
ALTER TABLE "users"    ADD COLUMN "branch_id" uuid NULL REFERENCES "branches" ("id") ON DELETE SET NULL;

ALTER TABLE "branches" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "branches" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "branches"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

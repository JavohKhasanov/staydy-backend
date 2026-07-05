-- Faza A (CRM): leads — the sales funnel (prospective students before enrolment). RLS hand-written.

CREATE TABLE "leads" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "name" text NOT NULL, "phone" text NULL, "email" text NULL, "source" text NOT NULL DEFAULT '', "stage" text NOT NULL DEFAULT 'new', "interest" text NOT NULL DEFAULT '', "note" text NOT NULL DEFAULT '', "assigned_to" uuid NULL, "student_id" uuid NULL, "created_at" timestamptz NOT NULL DEFAULT now(), "updated_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "leads_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "leads_assigned_to_fkey" FOREIGN KEY ("assigned_to") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL);
CREATE INDEX "idx_leads_org" ON "leads" ("org_id");
CREATE INDEX "idx_leads_stage" ON "leads" ("org_id", "stage");

ALTER TABLE "leads" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "leads" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "leads"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

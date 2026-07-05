-- Faza F1 (finance completion): expenses. Centers record operating costs (ijara, kommunal, maosh,
-- reklama, ...) so profit = income − expenses. Manual ledger; RLS hand-written (Atlas can't emit it).
CREATE TABLE "expenses" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "org_id" uuid NOT NULL,
  "category" text NOT NULL DEFAULT 'boshqa',
  "amount" bigint NOT NULL,
  "spent_at" date NOT NULL DEFAULT CURRENT_DATE,
  "note" text NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "expenses_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE INDEX "idx_expenses_org" ON "expenses" ("org_id");
CREATE INDEX "idx_expenses_spent_at" ON "expenses" ("spent_at");

ALTER TABLE "expenses" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "expenses" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "expenses"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

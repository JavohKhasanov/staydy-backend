-- Faza C (CRM): finance — invoices + payments (manual ledger, no payment gateway). Reception
-- records what students owe and pay; balance/debt is derived. RLS is hand-written.

-- Create "invoices" table
CREATE TABLE "invoices" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "student_id" uuid NOT NULL, "enrollment_id" uuid NULL, "amount" bigint NOT NULL, "paid_amount" bigint NOT NULL DEFAULT 0, "due_date" date NULL, "period" text NOT NULL DEFAULT '', "note" text NOT NULL DEFAULT '', "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "invoices_org_id_student_id_fkey" FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE);
CREATE INDEX "idx_invoices_student" ON "invoices" ("student_id");
CREATE INDEX "idx_invoices_org" ON "invoices" ("org_id");

-- Create "payments" table
CREATE TABLE "payments" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "invoice_id" uuid NOT NULL, "student_id" uuid NOT NULL, "amount" bigint NOT NULL, "method" text NOT NULL DEFAULT 'cash', "paid_at" timestamptz NOT NULL DEFAULT now(), "note" text NOT NULL DEFAULT '', "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "payments_invoice_id_fkey" FOREIGN KEY ("invoice_id") REFERENCES "invoices" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "payments_org_id_student_id_fkey" FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE);
CREATE INDEX "idx_payments_student" ON "payments" ("student_id");
CREATE INDEX "idx_payments_invoice" ON "payments" ("invoice_id");
CREATE INDEX "idx_payments_org" ON "payments" ("org_id");

-- Row-Level Security (HAND-WRITTEN: Atlas Community can't represent RLS).
ALTER TABLE "invoices" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "invoices" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "invoices"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

ALTER TABLE "payments" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "payments" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "payments"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

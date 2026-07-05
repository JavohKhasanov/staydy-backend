-- Faza D-prep: org billing foundation (1-month free trial + status). The Payme/Click gateway is
-- post-MVP; this makes the system gateway-ready by tracking trial/active/expired per center.
-- trial_ends_at defaults to +30 days so every new center gets a free month automatically.
ALTER TABLE "organizations" ADD COLUMN "trial_ends_at" timestamptz NULL DEFAULT (now() + interval '30 days');
ALTER TABLE "organizations" ADD COLUMN "billing_status" text NOT NULL DEFAULT 'trial';

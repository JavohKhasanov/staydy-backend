-- Landing pricing plans, editable from the superadmin panel (platform-level, NON-RLS like
-- signup_requests/organizations). The public landing renders these; the super_admin edits them.
CREATE TABLE "plans" (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_key    text NOT NULL DEFAULT '',       -- trial|basic|pro — preselects the signup CTA
    name        text NOT NULL,                   -- "Sinov" / "Asosiy" / "Pro"
    price       text NOT NULL DEFAULT '',        -- "Bepul" / "299 000 so'm" / "So'rov bo'yicha"
    period      text NOT NULL DEFAULT '',        -- "oy" / "yil" / ""
    tagline     text NOT NULL DEFAULT '',        -- short line under the name
    features    text[] NOT NULL DEFAULT '{}',    -- feature checklist
    highlighted boolean NOT NULL DEFAULT false,  -- "Ommabop" badge
    sort_order  int NOT NULL DEFAULT 0,
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX "idx_plans_sort" ON "plans" ("sort_order");

-- Seed the 3 default plans so the landing has content immediately (prices are placeholders the
-- super_admin fills in).
INSERT INTO "plans" (plan_key, name, price, period, tagline, features, highlighted, sort_order) VALUES
  ('trial', 'Sinov', 'Bepul', '', '1 oy, to''liq imkoniyatlar',
     ARRAY['Barcha asosiy modullar','1 oy sinov','Karta kerak emas'], false, 1),
  ('basic', 'Asosiy', '— so''m', 'oy', 'Kichik va o''rta markazlar',
     ARRAY['CRM + Moliya','Davomat + EWS','Telegram bot','Hisobotlar'], true, 2),
  ('pro', 'Pro', '— so''m', 'oy', 'Katta markazlar',
     ARRAY['Asosiydagi barchasi','AI maslahatchi','Kengaytirilgan hisobotlar','Ustuvor qo''llab-quvvatlash'], false, 3);

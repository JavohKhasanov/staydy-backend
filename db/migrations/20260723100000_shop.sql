-- Reward shop: each center defines items students buy with coins. A purchase is one-per-item
-- (owned model); the coin spend + ledger entry + purchase row happen atomically.
CREATE TABLE "shop_items" (
    "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"     uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "name"       text NOT NULL,
    "icon"       text NOT NULL DEFAULT '',
    "price"      int NOT NULL DEFAULT 0,
    "is_active"  boolean NOT NULL DEFAULT true,
    "created_at" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX "idx_shop_items_org" ON "shop_items" ("org_id");

CREATE TABLE "shop_purchases" (
    "id"         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "org_id"     uuid NOT NULL REFERENCES "organizations" ("id") ON DELETE CASCADE,
    "student_id" uuid NOT NULL,
    "item_id"    uuid NOT NULL REFERENCES "shop_items" ("id") ON DELETE CASCADE,
    "price"      int NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    UNIQUE ("student_id", "item_id"),
    FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON DELETE CASCADE
);
CREATE INDEX "idx_shop_purchases_student" ON "shop_purchases" ("student_id");

ALTER TABLE "shop_items"     ENABLE ROW LEVEL SECURITY;
ALTER TABLE "shop_items"     FORCE ROW LEVEL SECURITY;
ALTER TABLE "shop_purchases" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "shop_purchases" FORCE ROW LEVEL SECURITY;
CREATE POLICY "org_isolation" ON "shop_items"
    USING ("org_id" = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY "org_isolation" ON "shop_purchases"
    USING ("org_id" = nullif(current_setting('app.current_org', true), '')::uuid);

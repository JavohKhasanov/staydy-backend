-- Seed the platform-owner (super_admin) account used by the superadmin panel (/admin/login).
-- pgcrypto's crypt(pw, gen_salt('bf',10)) produces a standard bcrypt hash that the Go backend
-- (golang.org/x/crypto/bcrypt) verifies. Idempotent (safe to re-run / re-apply).
--
-- schema.sql declares pgcrypto but Atlas never emitted CREATE EXTENSION (PG16's gen_random_uuid
-- is built-in), so install it here — crypt()/gen_salt() need it.
--
-- SECURITY: the password is embedded here (and thus in git). Fine for local dev; for production,
-- change it after first login (or seed via the SUPERADMIN_* env instead and delete this row).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO "organizations" ("name", "slug", "plan", "status")
VALUES ('Platform', 'platform', 'internal', 'active')
ON CONFLICT ("slug") DO NOTHING;

INSERT INTO "users" ("org_id", "email", "password_hash", "full_name", "role")
SELECT o."id", 'xadminsup@gmail.com', crypt('@superpassw0rd$', gen_salt('bf', 10)), 'Super Admin', 'super_admin'
FROM "organizations" o
WHERE o."slug" = 'platform'
ON CONFLICT ("org_id", "email") DO NOTHING;

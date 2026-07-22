-- Student login: a password on the student record, plus a phone index for lookups.
ALTER TABLE "students" ADD COLUMN "password_hash" text NOT NULL DEFAULT '';
CREATE INDEX "idx_students_phone" ON "students" ("phone");

-- Cross-org login lookup. Students log in by phone + password without first picking a center, so the
-- lookup must see across tenants — which RLS (FORCE) otherwise blocks. This SECURITY DEFINER
-- function runs as its owner (the superuser that applies migrations), bypassing RLS, and returns
-- only the fields auth needs and only students that actually have a login password set.
-- Match on the last 9 digits (the Uzbek national number) so formatting and the +998 prefix don't
-- cause misses.
CREATE FUNCTION "student_auth_lookup"("p_phone" text)
RETURNS TABLE ("id" uuid, "org_id" uuid, "password_hash" text, "name" text)
LANGUAGE sql SECURITY DEFINER SET search_path = public STABLE AS $$
  SELECT s."id", s."org_id", s."password_hash", s."name"
  FROM "students" s
  WHERE s."password_hash" <> ''
    AND right(regexp_replace(coalesce(s."phone", ''), '\D', '', 'g'), 9)
      = right(regexp_replace("p_phone", '\D', '', 'g'), 9)
    AND right(regexp_replace("p_phone", '\D', '', 'g'), 9) <> '';
$$;

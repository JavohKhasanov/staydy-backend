-- salary_group_rules was added (20260722100000) WITHOUT row-level security, unlike its siblings
-- salary_rules/salary_slips. Its queries filter by teacher_id only, so absent RLS an authenticated
-- finance/director could read or delete ANOTHER org's per-group salary rates by passing a foreign
-- teacher UUID. Enable + force org isolation like every other tenant table; every repo call already
-- runs inside WithTenant (sets app.current_org), so this transparently scopes the existing queries.
ALTER TABLE "salary_group_rules" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "salary_group_rules" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "salary_group_rules"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

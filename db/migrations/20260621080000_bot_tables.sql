-- Telegram bot tables: take bot_conversations and invite_tokens OUT of RLS.
--
-- An incoming Telegram update carries only a chat_id (bot_conversations) or a secret start
-- token (invite_tokens) — there is NO tenant scope to resolve them under. So, like
-- refresh_sessions/organizations, these tables are non-RLS: they carry org_id explicitly and
-- are only ever looked up by their unique chat_id or by a secret token. The bot resolves the
-- org from the row, THEN enters WithTenant(org) for any tenant-table write (students/surveys).
--
-- Hand-written (Atlas Community drops RLS DDL on diff). Run `make migrate-hash` after editing.

-- Student-level deep link: bind the invite to a specific student, not just the org.
ALTER TABLE invite_tokens ADD COLUMN IF NOT EXISTS student_id uuid REFERENCES students(id) ON DELETE CASCADE;

DROP POLICY IF EXISTS org_isolation ON bot_conversations;
DROP POLICY IF EXISTS org_isolation ON invite_tokens;

ALTER TABLE bot_conversations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE bot_conversations DISABLE ROW LEVEL SECURITY;
ALTER TABLE invite_tokens     NO FORCE ROW LEVEL SECURITY;
ALTER TABLE invite_tokens     DISABLE ROW LEVEL SECURITY;

-- Lookup indexes for the bot's hot paths.
CREATE INDEX IF NOT EXISTS idx_invite_tokens_student ON invite_tokens(student_id);

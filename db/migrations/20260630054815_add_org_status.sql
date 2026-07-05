-- Superadmin: organizations.status ('active' | 'suspended'). A suspended center's users
-- can't log in.
-- (Hand-trimmed from `atlas migrate diff`: dropped the unrelated pre-existing constraint-name
-- normalization on attendance/surveys/notes/intervention_tasks + the idx_invite_tokens_student
-- drop — not part of this feature.)
ALTER TABLE "organizations" ADD COLUMN "status" text NOT NULL DEFAULT 'active';

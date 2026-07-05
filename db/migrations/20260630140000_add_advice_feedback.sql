-- Phase: AI advice feedback (TZ §7.4). Mentors mark a recommendation useful / not useful;
-- one rating per mentor per student, upserted. (Mirrors homework_records; RLS is hand-written
-- since Atlas Community can't represent it.)

-- Create "ai_advice_feedback" table
CREATE TABLE "ai_advice_feedback" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "org_id" uuid NOT NULL, "student_id" uuid NOT NULL, "user_id" uuid NOT NULL, "useful" boolean NOT NULL, "created_at" timestamptz NOT NULL DEFAULT now(), PRIMARY KEY ("id"), CONSTRAINT "ai_advice_feedback_org_id_student_id_user_id_key" UNIQUE ("org_id", "student_id", "user_id"), CONSTRAINT "ai_advice_feedback_org_id_fkey" FOREIGN KEY ("org_id") REFERENCES "organizations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "ai_advice_feedback_org_id_student_id_fkey" FOREIGN KEY ("org_id", "student_id") REFERENCES "students" ("org_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "idx_advice_feedback_student" to table: "ai_advice_feedback"
CREATE INDEX "idx_advice_feedback_student" ON "ai_advice_feedback" ("student_id");

-- Row-Level Security for ai_advice_feedback (HAND-WRITTEN: Atlas Community can't represent RLS).
ALTER TABLE "ai_advice_feedback" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "ai_advice_feedback" FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON "ai_advice_feedback"
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

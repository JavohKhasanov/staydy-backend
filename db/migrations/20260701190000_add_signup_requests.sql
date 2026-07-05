-- Landing-page "request a plan" funnel. Platform-level (NON-RLS, like organizations/plans): a
-- prospective center submits interest from the public marketing site; a super_admin reviews it in
-- the panel and follows up manually (Payme/Click purchase is post-MVP).
CREATE TABLE "signup_requests" (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    center_name  text NOT NULL,
    contact_name text NOT NULL,
    phone        text NOT NULL,
    email        text NOT NULL DEFAULT '',
    plan         text NOT NULL DEFAULT '', -- requested tier: trial|basic|pro (informational)
    message      text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'new', -- new|contacted|converted|rejected
    created_at   timestamptz NOT NULL DEFAULT now()
);

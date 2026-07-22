-- Desired database schema — single source of truth for sqlc codegen and Atlas
-- migrations (Atlas diffs this file against a dev database to draft migrations).
-- Multi-tenant: every tenant table carries org_id and is isolated by Postgres RLS
-- keyed on the app.current_org GUC (set per-transaction by platform/postgres.SetTenant).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Tenants. One organization == one educational center (o'quv markazi).
CREATE TABLE organizations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    slug       text NOT NULL UNIQUE,
    plan       text NOT NULL DEFAULT 'trial',
    status     text NOT NULL DEFAULT 'active', -- 'active' | 'suspended' (suspended → login blocked)
    trial_ends_at  timestamptz DEFAULT (now() + interval '30 days'), -- free month; Payme/Click gateway is post-MVP
    billing_status text NOT NULL DEFAULT 'trial', -- 'trial' | 'active' (paid) | 'expired'
    grace_lessons  int NOT NULL DEFAULT 3, -- attended sessions allowed before missing-invoice flag
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Landing-page "request a plan" funnel (platform-level, NON-RLS): prospective centers submit
-- interest from the public site; a super_admin reviews + follows up (purchase is post-MVP).
CREATE TABLE signup_requests (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    center_name  text NOT NULL,
    contact_name text NOT NULL,
    phone        text NOT NULL,
    email        text NOT NULL DEFAULT '',
    plan         text NOT NULL DEFAULT '',
    message      text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'new', -- new|contacted|converted|rejected
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- F2 multi-branch (filiallar). A branch is a physical location inside one org (org_id + RLS is the
-- hard boundary). students/groups/expenses/users carry a nullable branch_id (NULL = org-wide).
CREATE TABLE branches (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       text NOT NULL,
    address    text NOT NULL DEFAULT '',
    phone      text NOT NULL DEFAULT '',
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_branches_org ON branches(org_id);

-- Landing pricing plans (platform-level, NON-RLS), edited by super_admin, shown on the public site.
CREATE TABLE plans (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_key    text NOT NULL DEFAULT '',
    name        text NOT NULL,
    price       text NOT NULL DEFAULT '',
    period      text NOT NULL DEFAULT '',
    tagline     text NOT NULL DEFAULT '',
    features    text[] NOT NULL DEFAULT '{}',
    highlighted boolean NOT NULL DEFAULT false,
    sort_order  int NOT NULL DEFAULT 0,
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_plans_sort ON plans(sort_order);

-- Back-office accounts (center admins, mentors).
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email         text NOT NULL,
    password_hash text NOT NULL,
    full_name     text NOT NULL,
    role          text NOT NULL DEFAULT 'mentor',
    phone         text,
    is_active     boolean NOT NULL DEFAULT true,
    branch_id     uuid, -- F2: primary branch (FK to branches, added by migration); NULL = org-wide
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, email)
);

-- Refresh-token sessions. The opaque refresh token is returned to the client; only
-- its SHA-256 hash is stored. Looked up by hash (no RLS — lookup precedes org scope).
CREATE TABLE refresh_sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    user_agent text,
    ip         text,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_sessions_user ON refresh_sessions(user_id);

-- Student groups (cohorts). Each group optionally belongs to a teacher (a users row with
-- role 'teacher'). teacher_id is a simple FK (SET NULL on teacher removal); tenant integrity
-- for teacher assignment is enforced by the usecase (RLS-scoped lookup) before assigning.
CREATE TABLE groups (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name          text NOT NULL,
    teacher_id    uuid REFERENCES users(id) ON DELETE SET NULL,
    course_id     uuid, -- FK added after courses is defined (see ALTER below)
    branch_id     uuid, -- F2: branch this group runs at (FK to branches, added by migration)
    direction     text,
    schedule_days text,
    capacity      int NOT NULL DEFAULT 0,
    start_date    date,
    end_date      date,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    start_time    text NOT NULL DEFAULT '',    -- recurring class time "HH:MM"
    end_time      text NOT NULL DEFAULT '',
    room_id       uuid, -- FK to rooms(id) added by migration (rooms defined later)
    -- needed so students can carry a composite (org_id, group_id) FK.
    UNIQUE (org_id, id)
);
CREATE INDEX idx_groups_org ON groups(org_id);
CREATE INDEX idx_groups_teacher ON groups(teacher_id);

-- Many-to-many student<->group membership (students.group_id = primary group, legacy).
CREATE TABLE group_members (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    group_id   uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    student_id uuid NOT NULL, -- FK to students added below (students defined later)
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (group_id, student_id)
);
CREATE INDEX idx_group_members_group ON group_members(group_id);
CREATE INDEX idx_group_members_student ON group_members(student_id);

CREATE TABLE students (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id             uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name               text NOT NULL,
    phone              text,
    password_hash      text NOT NULL DEFAULT '', -- student mini-app login (phone + password)
    xp                 int  NOT NULL DEFAULT 0,   -- gamification: cached total XP
    coins              int  NOT NULL DEFAULT 0,   -- gamification: spendable coin balance
    telegram_id        text,
    telegram_chat_id   bigint,
    course_name        text,
    group_name         text,
    group_id           uuid,
    mentor_name        text,
    start_date         date,
    onboarding_goal    text,
    six_month_target   text,
    weekly_study_hours text,
    confidence_level   int  NOT NULL DEFAULT 5,
    risk_score         int  NOT NULL DEFAULT 0,
    risk_tier          text NOT NULL DEFAULT 'Green',
    -- CRM contact / identity / lifecycle (2026-07-01, grounded in mature education systems)
    email              text,
    birth_date         date,
    gender             text NOT NULL DEFAULT '',
    second_phone       text,
    address            text,
    parent_name        text,
    parent_phone       text,
    student_code       text,
    status             text NOT NULL DEFAULT 'active', -- active|inactive|graduated|lead|dropped
    mentor_id          uuid REFERENCES users(id) ON DELETE SET NULL,
    branch_id          uuid, -- F2: branch this student belongs to (FK to branches, added by migration)
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    -- needed so child tables can carry a composite (org_id, student_id) FK.
    UNIQUE (org_id, id),
    -- a student's group must belong to the same org (DB-level tenant integrity). NO ACTION:
    -- a group can't be hard-deleted while students reference it (usecase nulls them first).
    FOREIGN KEY (org_id, group_id) REFERENCES groups (org_id, id) ON DELETE NO ACTION
);
CREATE INDEX idx_students_org ON students(org_id);
CREATE INDEX idx_students_org_risk ON students(org_id, risk_score DESC);
CREATE INDEX idx_students_group ON students(group_id);

CREATE TABLE attendance_records (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    group_id   uuid, -- which group's session (nullable; legacy rows NULL)
    date       date NOT NULL,
    is_present boolean NOT NULL,
    status     text NOT NULL DEFAULT 'present', -- present|absent|late|excused; is_present = status<>'absent'
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE,
    UNIQUE NULLS NOT DISTINCT (org_id, student_id, date, group_id) -- one row per date per group (upsert)
);
CREATE INDEX idx_attendance_student ON attendance_records(student_id);

CREATE TABLE surveys (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id       uuid NOT NULL,
    week_number      int  NOT NULL,
    motivation_score int  NOT NULL,
    progress_score   int  NOT NULL,
    biggest_obstacle text,
    comment          text,
    submitted_at     timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE,
    UNIQUE (org_id, student_id, week_number) -- one survey per week (upsert on resubmit)
);
CREATE INDEX idx_surveys_student ON surveys(student_id);

CREATE TABLE homework_records (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    date       date NOT NULL,
    is_done    boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE,
    UNIQUE (org_id, student_id, date) -- one homework row per lesson date (upsert on re-record)
);
CREATE INDEX idx_homework_student ON homework_records(student_id);

-- Homework assignments + submissions (LMS-style): a teacher attaches an assignment to a group;
-- students submit text + links; the teacher grades (status + score).
CREATE TABLE homework_assignments (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    group_id    uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    lesson_date date,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    deadline    timestamptz,
    max_score   int NOT NULL DEFAULT 100,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_hw_assignments_group ON homework_assignments(group_id);
CREATE INDEX idx_hw_assignments_org ON homework_assignments(org_id);

CREATE TABLE homework_submissions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    assignment_id uuid NOT NULL REFERENCES homework_assignments(id) ON DELETE CASCADE,
    student_id    uuid NOT NULL,
    text          text NOT NULL DEFAULT '',
    links         text NOT NULL DEFAULT '',
    status        text NOT NULL DEFAULT 'submitted', -- submitted | accepted | rejected
    score         int,
    review_note   text NOT NULL DEFAULT '',
    submitted_at  timestamptz NOT NULL DEFAULT now(),
    reviewed_at   timestamptz,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (assignment_id, student_id),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_hw_submissions_assignment ON homework_submissions(assignment_id);
CREATE INDEX idx_hw_submissions_student ON homework_submissions(student_id);

-- Gamification points ledger (append-only; idempotent via the unique key).
CREATE TABLE student_points (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    kind       text NOT NULL,
    xp         int NOT NULL DEFAULT 0,
    coins      int NOT NULL DEFAULT 0,
    ref        text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, student_id, kind, ref),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_student_points_student ON student_points(student_id);

-- Reward shop: per-center items bought with coins (one-per-item owned model).
CREATE TABLE shop_items (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name       text NOT NULL,
    icon       text NOT NULL DEFAULT '',
    price      int NOT NULL DEFAULT 0,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_shop_items_org ON shop_items(org_id);

CREATE TABLE shop_purchases (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    item_id    uuid NOT NULL REFERENCES shop_items(id) ON DELETE CASCADE,
    price      int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (student_id, item_id),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_shop_purchases_student ON shop_purchases(student_id);

-- Mentor feedback on an AI recommendation (TZ §7.4): one rating per mentor per student, upserted.
CREATE TABLE ai_advice_feedback (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    user_id    uuid NOT NULL,
    useful     boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE,
    UNIQUE (org_id, student_id, user_id)
);
CREATE INDEX idx_advice_feedback_student ON ai_advice_feedback(student_id);

-- Center-configurable choices for the check-in's "biggest obstacle" question. The bot builds its
-- keyboard from a center's active options; the web survey form reads the same set.
CREATE TABLE obstacle_options (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    label      text NOT NULL,
    position   int NOT NULL DEFAULT 0,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, label)
);
CREATE INDEX idx_obstacle_options_org ON obstacle_options(org_id);

-- Sellable programs a center offers (e.g. "IELTS Intermediate"). A group runs one course;
-- enrollments + fees reference it. price is whole UZS so'm.
CREATE TABLE courses (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name           text NOT NULL,
    code           text NOT NULL DEFAULT '',
    level          text NOT NULL DEFAULT '',
    price          bigint NOT NULL DEFAULT 0,
    duration_weeks int NOT NULL DEFAULT 0,
    description    text NOT NULL DEFAULT '',
    is_active      boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
CREATE INDEX idx_courses_org ON courses(org_id);

-- groups.course_id FK (declared here, after courses exists). A group runs one course.
ALTER TABLE groups ADD CONSTRAINT groups_course_id_fkey
    FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE SET NULL;

-- A student's enrolment in a group/course with lifecycle status, price and dates. Additive
-- successor to students.group_id (kept for now; coexists). status: active|completed|dropped|frozen.
CREATE TABLE enrollments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    group_id   uuid,
    course_id  uuid,
    status     text NOT NULL DEFAULT 'active',
    start_date date,
    end_date   date,
    price      bigint NOT NULL DEFAULT 0,
    discount   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_enrollments_student ON enrollments(student_id);
CREATE INDEX idx_enrollments_org ON enrollments(org_id);

-- Finance (manual ledger, no payment gateway). invoices = what a student owes; payments = what
-- was recorded as paid. paid_amount denormalises the payment sum; balance/status are derived.
CREATE TABLE invoices (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id    uuid NOT NULL,
    group_id      uuid, -- which group (course) the charge is for; NULL = general
    enrollment_id uuid,
    amount        bigint NOT NULL,
    paid_amount   bigint NOT NULL DEFAULT 0,
    due_date      date,
    period        text NOT NULL DEFAULT '',
    note          text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_invoices_student ON invoices(student_id);
CREATE INDEX idx_invoices_org ON invoices(org_id);

CREATE TABLE payments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    amount     bigint NOT NULL,
    method     text NOT NULL DEFAULT 'cash',
    paid_at    timestamptz NOT NULL DEFAULT now(),
    note       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_payments_student ON payments(student_id);
CREATE INDEX idx_payments_invoice ON payments(invoice_id);
CREATE INDEX idx_payments_org ON payments(org_id);

-- Operating costs (ijara/kommunal/maosh/reklama/...). profit = payments − expenses over a period.
CREATE TABLE expenses (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    category   text NOT NULL DEFAULT 'boshqa',
    amount     bigint NOT NULL,
    spent_at   date NOT NULL DEFAULT CURRENT_DATE,
    note       text NOT NULL DEFAULT '',
    branch_id  uuid, -- F2: branch this cost belongs to (FK to branches, added by migration)
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_expenses_org ON expenses(org_id);
CREATE INDEX idx_expenses_spent_at ON expenses(spent_at);

-- Teacher payroll: a rule configures how a teacher is paid; a slip is one period's computed pay.
CREATE TABLE salary_rules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    teacher_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       text NOT NULL DEFAULT 'fixed', -- variable part: fixed(none) | per_lesson | per_student | percent_revenue
    rate       bigint NOT NULL DEFAULT 0,       -- variable rate (per lesson/student, or % for percent_revenue)
    base_amount bigint NOT NULL DEFAULT 0,      -- fixed monthly base (added on top of the variable part)
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, teacher_id)
);
CREATE INDEX idx_salary_rules_org ON salary_rules(org_id);

-- Per-group salary overrides (a teacher's variable pay can differ per group/course). Absent a row,
-- the group falls back to the teacher's default salary_rules kind/rate. Base stays teacher-level.
CREATE TABLE salary_group_rules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    teacher_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id   uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    kind       text NOT NULL DEFAULT 'per_student',
    rate       bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, teacher_id, group_id)
);
CREATE INDEX idx_salary_group_rules_teacher ON salary_group_rules(teacher_id);

CREATE TABLE salary_slips (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    teacher_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period_start date NOT NULL,
    period_end   date NOT NULL,
    gross        bigint NOT NULL DEFAULT 0,
    bonus        bigint NOT NULL DEFAULT 0,
    deduction    bigint NOT NULL DEFAULT 0,
    net          bigint NOT NULL DEFAULT 0,
    status       text NOT NULL DEFAULT 'draft', -- draft | paid
    note         text NOT NULL DEFAULT '',
    paid_at      timestamptz NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_salary_slips_org ON salary_slips(org_id);
CREATE INDEX idx_salary_slips_teacher ON salary_slips(teacher_id);

-- Sales funnel: prospective students before enrolment. stage: new|contacted|trial|enrolled|lost.
CREATE TABLE leads (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        text NOT NULL,
    phone       text,
    email       text,
    source      text NOT NULL DEFAULT '',
    stage       text NOT NULL DEFAULT 'new',
    interest    text NOT NULL DEFAULT '',
    note        text NOT NULL DEFAULT '',
    assigned_to uuid REFERENCES users(id) ON DELETE SET NULL,
    student_id  uuid,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_leads_org ON leads(org_id);
CREATE INDEX idx_leads_stage ON leads(org_id, stage);

-- Polymorphic communication timeline (call/sms/note/meeting) on a lead or student.
CREATE TABLE activities (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    subject_type text NOT NULL,
    subject_id   uuid NOT NULL,
    type         text NOT NULL DEFAULT 'note',
    body         text NOT NULL DEFAULT '',
    author       text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_activities_subject ON activities(org_id, subject_type, subject_id);

-- Scheduled class sessions (the timetable). start_time/end_time are "HH:MM" strings.
CREATE TABLE lessons (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    group_id   uuid,
    teacher_id uuid REFERENCES users(id) ON DELETE SET NULL,
    date       date NOT NULL,
    start_time text NOT NULL DEFAULT '',
    end_time   text NOT NULL DEFAULT '',
    room       text NOT NULL DEFAULT '',
    room_id    uuid, -- F3: FK to rooms (added by migration); double-booking checked by usecase
    topic      text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'scheduled',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Physical rooms (xonalar), optionally at a branch. A lesson references a room; the usecase
-- prevents double-booking a room at overlapping times on the same date.
CREATE TABLE rooms (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    branch_id  uuid REFERENCES branches(id) ON DELETE SET NULL,
    name       text NOT NULL,
    capacity   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_rooms_org ON rooms(org_id);
CREATE INDEX idx_lessons_org_date ON lessons(org_id, date);
CREATE INDEX idx_lessons_group ON lessons(group_id);

CREATE TABLE notes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid NOT NULL,
    author     text NOT NULL,
    text       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_notes_student ON notes(student_id);

CREATE TABLE intervention_tasks (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id             uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id         uuid NOT NULL,
    reasons            text[] NOT NULL DEFAULT '{}',
    status             text NOT NULL DEFAULT 'Open',
    resolution_comment text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    resolved_at        timestamptz,
    FOREIGN KEY (org_id, student_id) REFERENCES students (org_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_tasks_org_status ON intervention_tasks(org_id, status);
-- At most one non-resolved intervention task per student (DB-enforced auto-open dedup).
CREATE UNIQUE INDEX intervention_tasks_one_open
    ON intervention_tasks (org_id, student_id) WHERE status <> 'Resolved';

-- Telegram bot FSM state, one row per chat.
CREATE TABLE bot_conversations (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           uuid REFERENCES organizations(id) ON DELETE CASCADE,
    telegram_chat_id bigint NOT NULL,
    student_id       uuid REFERENCES students(id) ON DELETE SET NULL,
    flow             text,
    step             text,
    state            jsonb NOT NULL DEFAULT '{}',
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (telegram_chat_id)
);

-- One-time deep-link onboarding tokens (t.me/<bot>?start=<token>). student_id binds the link
-- to a specific student so the bot knows who connected. NOT under RLS: an incoming Telegram
-- update carries only the secret token, with no tenant scope — resolved like refresh_sessions.
CREATE TABLE invite_tokens (
    token      text PRIMARY KEY,
    org_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    student_id uuid REFERENCES students(id) ON DELETE CASCADE,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    used_at    timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- --- Row-Level Security: every tenant table filters on app.current_org ---
-- platform/postgres.SetTenant runs SELECT set_config('app.current_org', <org>, true)
-- per transaction. nullif(...,'') guards the unset case so an unscoped query sees nothing.
-- FORCE makes RLS apply to the table owner too (the app connects as the owner in dev).
--
-- NOTE: bot_conversations and invite_tokens are intentionally NOT here. The bot resolves them
-- with no tenant scope (only a chat_id / secret token), so they follow the refresh_sessions
-- pattern: non-RLS, carrying org_id explicitly, accessed by unique chat_id or secret token.

ALTER TABLE users               ENABLE ROW LEVEL SECURITY;
ALTER TABLE groups              ENABLE ROW LEVEL SECURITY;
ALTER TABLE students            ENABLE ROW LEVEL SECURITY;
ALTER TABLE attendance_records  ENABLE ROW LEVEL SECURITY;
ALTER TABLE homework_records    ENABLE ROW LEVEL SECURITY;
ALTER TABLE surveys             ENABLE ROW LEVEL SECURITY;
ALTER TABLE notes               ENABLE ROW LEVEL SECURITY;
ALTER TABLE intervention_tasks  ENABLE ROW LEVEL SECURITY;

ALTER TABLE users               FORCE ROW LEVEL SECURITY;
ALTER TABLE groups              FORCE ROW LEVEL SECURITY;
ALTER TABLE students            FORCE ROW LEVEL SECURITY;
ALTER TABLE attendance_records  FORCE ROW LEVEL SECURITY;
ALTER TABLE homework_records    FORCE ROW LEVEL SECURITY;
ALTER TABLE surveys             FORCE ROW LEVEL SECURITY;
ALTER TABLE notes               FORCE ROW LEVEL SECURITY;
ALTER TABLE intervention_tasks  FORCE ROW LEVEL SECURITY;
ALTER TABLE group_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE group_members FORCE ROW LEVEL SECURITY;
ALTER TABLE homework_assignments  ENABLE ROW LEVEL SECURITY;
ALTER TABLE homework_assignments  FORCE ROW LEVEL SECURITY;
ALTER TABLE homework_submissions  ENABLE ROW LEVEL SECURITY;
ALTER TABLE homework_submissions  FORCE ROW LEVEL SECURITY;
ALTER TABLE student_points        ENABLE ROW LEVEL SECURITY;
ALTER TABLE student_points        FORCE ROW LEVEL SECURITY;
ALTER TABLE shop_items            ENABLE ROW LEVEL SECURITY;
ALTER TABLE shop_items            FORCE ROW LEVEL SECURITY;
ALTER TABLE shop_purchases        ENABLE ROW LEVEL SECURITY;
ALTER TABLE shop_purchases        FORCE ROW LEVEL SECURITY;

CREATE POLICY org_isolation ON users
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON groups
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON students
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON homework_records
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON group_members
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON homework_assignments
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON homework_submissions
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON student_points
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON shop_items
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON shop_purchases
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON attendance_records
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON surveys
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON notes
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);
CREATE POLICY org_isolation ON intervention_tasks
    USING (org_id = nullif(current_setting('app.current_org', true), '')::uuid);

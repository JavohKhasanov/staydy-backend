-- sqlc queries. Run `make sqlc` to regenerate internal/repo/sqlc.
--
-- Tenant tables (users, students, …) have NO "WHERE org_id" clause: callers run them
-- inside platform/postgres.WithTenant, and RLS filters every row. org_id is set on INSERT.
-- organizations and refresh_sessions are NOT under RLS (looked up before org scope exists).

-- name: CreateOrganization :one
INSERT INTO organizations (name, slug, plan)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: SlugExists :one
SELECT EXISTS (SELECT 1 FROM organizations WHERE slug = $1);

-- name: ListAllOrgIDs :many
-- NON-RLS: used by the cross-tenant scheduler to iterate every center.
SELECT id FROM organizations ORDER BY created_at ASC;

-- name: ListAllOrganizations :many
-- NON-RLS: superadmin center list (full rows; the caller filters out the platform org).
SELECT * FROM organizations ORDER BY created_at DESC;

-- name: UpdateOrgPlanStatus :one
UPDATE organizations SET plan = $2, status = $3, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateOrgBilling :one
UPDATE organizations SET plan = $2, billing_status = $3, trial_ends_at = $4, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: DeleteOrganization :exec
DELETE FROM organizations WHERE id = $1;

-- name: CreateSignupRequest :one
INSERT INTO signup_requests (center_name, contact_name, phone, email, plan, message)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListSignupRequests :many
SELECT * FROM signup_requests ORDER BY created_at DESC;

-- name: UpdateSignupRequestStatus :one
UPDATE signup_requests SET status = $2 WHERE id = $1 RETURNING *;

-- name: ListActivePlans :many
SELECT * FROM plans WHERE is_active = true ORDER BY sort_order ASC, created_at ASC;

-- name: ListAllPlans :many
SELECT * FROM plans ORDER BY sort_order ASC, created_at ASC;

-- name: CreatePlan :one
INSERT INTO plans (plan_key, name, price, period, tagline, features, highlighted, sort_order, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdatePlan :one
UPDATE plans SET plan_key = $2, name = $3, price = $4, period = $5, tagline = $6,
    features = $7, highlighted = $8, sort_order = $9, is_active = $10
WHERE id = $1 RETURNING *;

-- name: DeletePlan :exec
DELETE FROM plans WHERE id = $1;

-- name: CreateBranch :one
INSERT INTO branches (org_id, name, address, phone) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListBranches :many
SELECT * FROM branches WHERE org_id = $1 ORDER BY created_at ASC;

-- name: UpdateBranch :one
UPDATE branches SET name = $2, address = $3, phone = $4, is_active = $5 WHERE id = $1 RETURNING *;

-- name: DeleteBranch :exec
DELETE FROM branches WHERE id = $1;

-- name: CreateRoom :one
INSERT INTO rooms (org_id, branch_id, name, capacity) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: ListRooms :many
SELECT * FROM rooms WHERE org_id = $1 ORDER BY name ASC;

-- name: UpdateRoom :one
UPDATE rooms SET name = $2, branch_id = $3, capacity = $4 WHERE id = $1 RETURNING *;

-- name: DeleteRoom :exec
DELETE FROM rooms WHERE id = $1;

-- name: CountRoomConflicts :one
-- Room double-booking: same room + date with time overlap (existing.start < new.end AND
-- existing.end > new.start). Excludes cancelled lessons, timeless lessons, and the given id (self).
SELECT COUNT(*)::bigint FROM lessons
WHERE room_id = sqlc.arg(room_id)::uuid AND date = sqlc.arg(date) AND id <> sqlc.arg(exclude_id)::uuid
  AND status <> 'cancelled'
  AND start_time <> '' AND end_time <> ''
  AND start_time < sqlc.arg(new_end) AND end_time > sqlc.arg(new_start);

-- name: CountUsersInOrg :one
-- RLS-scoped: run inside WithTenant(org).
SELECT count(*) FROM users;

-- name: CountStudentsInOrg :one
-- RLS-scoped: run inside WithTenant(org).
SELECT count(*) FROM students;

-- name: CreateUser :one
INSERT INTO users (org_id, email, password_hash, full_name, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at ASC;

-- name: ListUsersByRole :many
SELECT * FROM users WHERE role = $1 ORDER BY full_name ASC;

-- name: UpdateUserProfile :one
UPDATE users SET full_name = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateTeacher :one
UPDATE users SET full_name = $2, email = $3, updated_at = now()
WHERE id = $1 AND role = 'teacher' RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: DeleteTeacher :exec
DELETE FROM users WHERE id = $1 AND role = 'teacher';

-- name: CreateRefreshSession :one
INSERT INTO refresh_sessions (user_id, org_id, token_hash, user_agent, ip, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRefreshSessionByHash :one
SELECT * FROM refresh_sessions WHERE token_hash = $1;

-- name: LockRefreshSessionByHash :one
SELECT * FROM refresh_sessions WHERE token_hash = $1 FOR UPDATE;

-- name: RevokeRefreshSession :exec
UPDATE refresh_sessions SET revoked_at = now() WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE refresh_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshSessions :exec
DELETE FROM refresh_sessions WHERE expires_at < now();

-- --- groups (RLS-scoped: run inside WithTenant) ---

-- name: CreateGroup :one
INSERT INTO groups (org_id, name, teacher_id, course_id, branch_id, direction, schedule_days, capacity, start_time, end_time, room_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListGroups :many
SELECT * FROM groups ORDER BY name ASC;

-- name: GetGroup :one
SELECT * FROM groups WHERE id = $1;

-- name: ListGroupsByTeacher :many
SELECT * FROM groups WHERE teacher_id = $1 ORDER BY name ASC;

-- name: UpdateGroup :one
UPDATE groups
SET name = $2, teacher_id = $3, course_id = $4, branch_id = $5, direction = $6, schedule_days = $7,
    capacity = $8, start_time = $9, end_time = $10, room_id = $11, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = $1;

-- name: CountStudentsInGroup :one
SELECT count(*) FROM students WHERE group_id = $1;

-- --- students + notes (RLS-scoped: run inside WithTenant; no WHERE org_id) ---

-- name: CreateStudent :one
INSERT INTO students (
    org_id, name, phone, telegram_id, course_name, group_name, group_id, mentor_name,
    start_date, onboarding_goal, six_month_target, weekly_study_hours,
    confidence_level, risk_score, risk_tier,
    email, birth_date, gender, second_phone, address, parent_name, parent_phone,
    student_code, status, mentor_id, branch_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26
)
RETURNING *;

-- name: UpdateStudent :one
-- General profile edit (name, contact, identity, onboarding, status, mentor). Risk fields and
-- group are updated by their own queries; this leaves them untouched.
UPDATE students SET
    name = $2, phone = $3, start_date = $4, onboarding_goal = $5, six_month_target = $6,
    weekly_study_hours = $7, confidence_level = $8,
    email = $9, birth_date = $10, gender = $11, second_phone = $12, address = $13,
    parent_name = $14, parent_phone = $15, student_code = $16, status = $17, mentor_id = $18,
    branch_id = $19, updated_at = now()
WHERE org_id = $1 AND id = $20
RETURNING *;

-- name: AssignStudentGroup :exec
-- Sets group_id AND denormalizes group_name to the group's name (or '' when unassigning), so the
-- legacy group_name field stays in sync — the student list + dashboard read group_name.
UPDATE students
SET group_id    = $2,
    group_name  = COALESCE((SELECT g.name FROM groups g WHERE g.id = $2), ''),
    updated_at  = now()
WHERE students.id = $1;

-- name: NullGroupForStudents :exec
UPDATE students SET group_id = NULL, updated_at = now() WHERE group_id = $1;

-- name: ListStudentsByGroup :many
SELECT * FROM students WHERE group_id = $1 ORDER BY risk_score DESC, name ASC;

-- name: ListStudents :many
SELECT * FROM students
ORDER BY risk_score DESC, name ASC;

-- name: GetStudent :one
SELECT * FROM students WHERE id = $1;

-- name: DeleteStudent :exec
DELETE FROM students WHERE id = $1;

-- name: UpdateStudentRisk :exec
UPDATE students
SET risk_score = $2, risk_tier = $3, updated_at = now()
WHERE id = $1;

-- name: CreateNote :one
INSERT INTO notes (org_id, student_id, author, text)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListNotesByStudent :many
SELECT * FROM notes
WHERE student_id = $1
ORDER BY created_at DESC;

-- --- surveys + attendance + intervention tasks (RLS-scoped: run inside WithTenant) ---

-- name: CreateSurvey :one
INSERT INTO surveys (org_id, student_id, week_number, motivation_score, progress_score, biggest_obstacle, comment)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (org_id, student_id, week_number) DO UPDATE
SET motivation_score = EXCLUDED.motivation_score,
    progress_score   = EXCLUDED.progress_score,
    biggest_obstacle = EXCLUDED.biggest_obstacle,
    comment          = EXCLUDED.comment,
    submitted_at     = now()
RETURNING *;

-- name: ListSurveysByStudent :many
SELECT * FROM surveys
WHERE student_id = $1
ORDER BY week_number DESC, submitted_at DESC;

-- name: CreateAttendance :one
INSERT INTO attendance_records (org_id, student_id, date, is_present, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (org_id, student_id, date) DO UPDATE SET is_present = EXCLUDED.is_present, status = EXCLUDED.status
RETURNING *;

-- name: ListAttendanceByStudent :many
SELECT * FROM attendance_records
WHERE student_id = $1
ORDER BY date ASC;

-- name: CreateHomework :one
INSERT INTO homework_records (org_id, student_id, date, is_done)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, student_id, date) DO UPDATE SET is_done = EXCLUDED.is_done
RETURNING *;

-- name: ListHomeworkByStudent :many
SELECT * FROM homework_records
WHERE student_id = $1
ORDER BY date ASC;

-- name: CreateAdviceFeedback :exec
INSERT INTO ai_advice_feedback (org_id, student_id, user_id, useful)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, student_id, user_id)
DO UPDATE SET useful = EXCLUDED.useful, created_at = now();

-- name: ListObstacleOptions :many
SELECT * FROM obstacle_options
WHERE org_id = $1 AND is_active = true
ORDER BY position ASC, created_at ASC;

-- name: CreateObstacleOption :one
INSERT INTO obstacle_options (org_id, label, position)
VALUES ($1, $2, $3)
ON CONFLICT (org_id, label) DO UPDATE SET is_active = true
RETURNING *;

-- name: DeleteObstacleOption :exec
DELETE FROM obstacle_options WHERE org_id = $1 AND id = $2;

-- name: ListCourses :many
SELECT * FROM courses WHERE org_id = $1 ORDER BY is_active DESC, name ASC;

-- name: GetCourse :one
SELECT * FROM courses WHERE org_id = $1 AND id = $2;

-- name: CreateCourse :one
INSERT INTO courses (org_id, name, level, price, duration_weeks, description)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateCourse :one
UPDATE courses
SET name = $3, level = $4, price = $5, duration_weeks = $6, description = $7, is_active = $8
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteCourse :exec
DELETE FROM courses WHERE org_id = $1 AND id = $2;

-- name: ListEnrollmentsByStudent :many
SELECT * FROM enrollments WHERE student_id = $1 ORDER BY created_at DESC;

-- name: CreateEnrollment :one
INSERT INTO enrollments (org_id, student_id, group_id, course_id, status, start_date, end_date, price, discount)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateEnrollment :one
UPDATE enrollments
SET group_id = $3, course_id = $4, status = $5, start_date = $6, end_date = $7, price = $8, discount = $9
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteEnrollment :exec
DELETE FROM enrollments WHERE org_id = $1 AND id = $2;

-- name: ListInvoicesByStudent :many
SELECT * FROM invoices WHERE student_id = $1 ORDER BY due_date DESC NULLS LAST, created_at DESC;

-- name: GetInvoice :one
SELECT * FROM invoices WHERE org_id = $1 AND id = $2;

-- name: CreateInvoice :one
INSERT INTO invoices (org_id, student_id, enrollment_id, amount, due_date, period, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: DeleteInvoice :exec
DELETE FROM invoices WHERE org_id = $1 AND id = $2;

-- name: AdjustInvoicePaid :exec
UPDATE invoices SET paid_amount = GREATEST(paid_amount + $3, 0) WHERE org_id = $1 AND id = $2;

-- name: StudentBalance :one
SELECT COALESCE(SUM(amount - paid_amount), 0)::bigint AS balance
FROM invoices WHERE student_id = $1 AND paid_amount < amount;

-- name: ListPaymentsByStudent :many
SELECT * FROM payments WHERE student_id = $1 ORDER BY paid_at DESC;

-- name: CreatePayment :one
INSERT INTO payments (org_id, invoice_id, student_id, amount, method, paid_at, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPayment :one
SELECT * FROM payments WHERE org_id = $1 AND id = $2;

-- name: DeletePayment :exec
DELETE FROM payments WHERE org_id = $1 AND id = $2;

-- name: FinanceSummary :one
SELECT
  COALESCE((SELECT SUM(amount) FROM payments WHERE org_id = $1::uuid AND paid_at::date = CURRENT_DATE), 0)::bigint AS today_income,
  COALESCE((SELECT SUM(amount) FROM payments WHERE org_id = $1::uuid AND date_trunc('month', paid_at) = date_trunc('month', CURRENT_DATE)), 0)::bigint AS month_income,
  COALESCE((SELECT SUM(amount - paid_amount) FROM invoices WHERE org_id = $1::uuid AND paid_amount < amount), 0)::bigint AS total_debt,
  COALESCE((SELECT SUM(amount) FROM expenses WHERE org_id = $1::uuid AND spent_at::date = CURRENT_DATE), 0)::bigint AS today_expense,
  COALESCE((SELECT SUM(amount) FROM expenses WHERE org_id = $1::uuid AND date_trunc('month', spent_at) = date_trunc('month', CURRENT_DATE)), 0)::bigint AS month_expense;

-- name: CreateExpense :one
INSERT INTO expenses (org_id, category, amount, spent_at, note)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListExpenses :many
SELECT * FROM expenses
WHERE org_id = $1 AND spent_at >= $2 AND spent_at <= $3
ORDER BY spent_at DESC, created_at DESC;

-- name: DeleteExpense :exec
DELETE FROM expenses WHERE id = $1;

-- name: ExpensesByCategory :many
SELECT category, COALESCE(SUM(amount), 0)::bigint AS total
FROM expenses
WHERE org_id = $1 AND spent_at >= $2 AND spent_at <= $3
GROUP BY category
ORDER BY total DESC;

-- name: UpsertSalaryRule :one
INSERT INTO salary_rules (org_id, teacher_id, kind, rate)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, teacher_id) DO UPDATE SET kind = EXCLUDED.kind, rate = EXCLUDED.rate, updated_at = now()
RETURNING *;

-- name: GetSalaryRule :one
SELECT * FROM salary_rules WHERE teacher_id = $1;

-- name: CreateSalarySlip :one
INSERT INTO salary_slips (org_id, teacher_id, period_start, period_end, gross, bonus, deduction, net, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListSalarySlips :many
SELECT ss.*, u.full_name AS teacher_name
FROM salary_slips ss
JOIN users u ON u.id = ss.teacher_id
WHERE ss.period_start >= $1 AND ss.period_end <= $2
ORDER BY ss.created_at DESC;

-- name: MarkSalarySlipPaid :one
UPDATE salary_slips SET status = 'paid', paid_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteSalarySlip :exec
DELETE FROM salary_slips WHERE id = $1;

-- name: CountTeacherDoneLessons :one
SELECT COUNT(*)::bigint FROM lessons
WHERE teacher_id = $1::uuid AND status = 'done' AND date >= $2::date AND date <= $3::date;

-- name: CountTeacherActiveStudents :one
SELECT COUNT(*)::bigint FROM students
WHERE status = 'active' AND group_id IN (SELECT id FROM groups WHERE teacher_id = $1::uuid);

-- name: TeacherRevenue :one
SELECT COALESCE(SUM(amount), 0)::bigint FROM payments
WHERE paid_at::date >= $2::date AND paid_at::date <= $3::date
  AND student_id IN (SELECT id FROM students WHERE group_id IN (SELECT id FROM groups WHERE teacher_id = $1::uuid));

-- name: ListDebtors :many
SELECT s.id AS student_id, s.name, COALESCE(SUM(i.amount - i.paid_amount), 0)::bigint AS balance
FROM students s
JOIN invoices i ON i.student_id = s.id AND i.paid_amount < i.amount
WHERE i.org_id = $1
GROUP BY s.id, s.name
ORDER BY balance DESC;

-- name: GroupFinanceByPeriod :many
SELECT s.id AS student_id, s.name AS student_name,
       COALESCE(SUM(i.amount), 0)::bigint  AS invoiced,
       COALESCE(SUM(i.paid_amount), 0)::bigint AS paid
FROM students s
LEFT JOIN invoices i ON i.student_id = s.id AND i.period = @period::text
WHERE s.group_id = @group_id::uuid
GROUP BY s.id, s.name
ORDER BY s.name ASC;

-- name: ListLeads :many
SELECT * FROM leads WHERE org_id = $1 ORDER BY created_at DESC;

-- name: GetLead :one
SELECT * FROM leads WHERE org_id = $1 AND id = $2;

-- name: CreateLead :one
INSERT INTO leads (org_id, name, phone, email, source, stage, interest, note, assigned_to)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateLead :one
UPDATE leads
SET name = $3, phone = $4, email = $5, source = $6, stage = $7, interest = $8, note = $9,
    assigned_to = $10, updated_at = now()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: SetLeadStage :one
UPDATE leads SET stage = $3, updated_at = now() WHERE org_id = $1 AND id = $2 RETURNING *;

-- name: MarkLeadConverted :exec
UPDATE leads SET stage = 'enrolled', student_id = $3, updated_at = now() WHERE org_id = $1 AND id = $2;

-- name: DeleteLead :exec
DELETE FROM leads WHERE org_id = $1 AND id = $2;

-- name: ListActivities :many
SELECT * FROM activities
WHERE org_id = $1 AND subject_type = $2 AND subject_id = $3
ORDER BY created_at DESC;

-- name: CreateActivity :one
INSERT INTO activities (org_id, subject_type, subject_id, type, body, author)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteActivity :exec
DELETE FROM activities WHERE org_id = $1 AND id = $2;

-- name: ListLessons :many
SELECT * FROM lessons
WHERE org_id = $1 AND date >= $2 AND date <= $3
ORDER BY date ASC, start_time ASC;

-- name: CreateLesson :one
INSERT INTO lessons (org_id, group_id, teacher_id, date, start_time, end_time, room, room_id, topic, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateLesson :one
UPDATE lessons SET
    group_id = $3, teacher_id = $4, date = $5, start_time = $6, end_time = $7,
    room = $8, room_id = $9, topic = $10, status = $11
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteLesson :exec
DELETE FROM lessons WHERE org_id = $1 AND id = $2;

-- name: GetLessonByGroupDate :one
SELECT * FROM lessons
WHERE group_id = @group_id::uuid AND date = @date::date
ORDER BY created_at ASC
LIMIT 1;

-- name: CreateInterventionTask :one
-- ON CONFLICT DO NOTHING (against the one-open-per-student partial index) makes this
-- idempotent WITHOUT raising 23505, which would otherwise abort the surrounding tx.
-- A conflict (a task is already open) returns no row → pgx.ErrNoRows for the caller.
INSERT INTO intervention_tasks (org_id, student_id, reasons, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (org_id, student_id) WHERE status <> 'Resolved' DO NOTHING
RETURNING *;

-- name: CountOpenTasksForStudent :one
SELECT count(*) FROM intervention_tasks
WHERE student_id = $1 AND status <> 'Resolved';

-- name: UpdateOpenTaskReasons :exec
-- Refresh the reasons on a student's already-open task so it reflects the latest risk drivers
-- (the auto-open insert is idempotent and does NOT update an existing task).
UPDATE intervention_tasks
SET reasons = $3
WHERE org_id = $1 AND student_id = $2 AND status <> 'Resolved';

-- name: ListInterventionTasks :many
SELECT t.*, s.name AS student_name
FROM intervention_tasks t
JOIN students s ON s.id = t.student_id
ORDER BY t.created_at DESC;

-- name: GetInterventionTask :one
SELECT * FROM intervention_tasks WHERE id = $1;

-- name: ResolveInterventionTask :one
UPDATE intervention_tasks
SET status = 'Resolved', resolution_comment = $2, resolved_at = now()
WHERE id = $1 AND status <> 'Resolved'
RETURNING *;

-- --- dashboard aggregates (RLS-scoped: run inside WithTenant) ---

-- name: CountStudentsByTier :many
SELECT risk_tier, count(*) AS count
FROM students
GROUP BY risk_tier;

-- name: RiskByGroup :many
SELECT (coalesce(nullif(group_name, ''), 'Boshqa'))::text AS group_name,
       count(*)                                            AS student_count,
       round(avg(risk_score))::int                         AS avg_risk
FROM students
GROUP BY 1
ORDER BY avg_risk DESC, group_name ASC;

-- name: TopObstacles :many
SELECT obstacle, count(*) AS count
FROM (
    SELECT DISTINCT ON (student_id) student_id, biggest_obstacle AS obstacle
    FROM surveys
    ORDER BY student_id, week_number DESC, submitted_at DESC
) latest
WHERE obstacle IS NOT NULL AND obstacle <> ''
GROUP BY obstacle
ORDER BY count DESC, obstacle ASC;

-- name: HighRiskStudents :many
SELECT * FROM students
WHERE risk_tier IN ('Red', 'Yellow')
ORDER BY risk_score DESC, name ASC
LIMIT 10;

-- name: AvgMotivationByWeek :many
SELECT week_number, round(avg(motivation_score), 1)::float8 AS avg_motivation
FROM surveys
GROUP BY week_number
ORDER BY week_number;

-- --- telegram bot (invite_tokens + bot_conversations are NON-RLS: the bot resolves them with
-- no tenant scope, by secret token or unique chat_id, then enters WithTenant for student writes) ---

-- name: CreateInviteToken :one
INSERT INTO invite_tokens (token, org_id, student_id, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetInviteToken :one
SELECT * FROM invite_tokens WHERE token = $1;

-- name: MarkInviteTokenUsed :exec
UPDATE invite_tokens SET used_at = now() WHERE token = $1;

-- name: BindBotChat :one
INSERT INTO bot_conversations (telegram_chat_id, org_id, student_id, flow, step, state)
VALUES ($1, $2, $3, NULL, NULL, '{}')
ON CONFLICT (telegram_chat_id) DO UPDATE
SET org_id = EXCLUDED.org_id, student_id = EXCLUDED.student_id,
    flow = NULL, step = NULL, state = '{}', updated_at = now()
RETURNING *;

-- name: GetBotConversation :one
SELECT * FROM bot_conversations WHERE telegram_chat_id = $1;

-- name: ListLinkedChats :many
-- NON-RLS: every chat bound to a student (for the weekly survey broadcast).
SELECT telegram_chat_id, org_id, student_id FROM bot_conversations WHERE student_id IS NOT NULL;

-- name: SetBotConversationFlow :exec
UPDATE bot_conversations
SET flow = $2, step = $3, state = $4, updated_at = now()
WHERE telegram_chat_id = $1;

-- name: SetStudentTelegramChat :exec
UPDATE students SET telegram_chat_id = $2, updated_at = now() WHERE id = $1;

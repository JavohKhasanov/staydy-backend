// Package repo defines persistence ports (interfaces). Concrete adapters live in
// repo/postgres. Usecases depend on these interfaces, never on sqlc or pgx directly.
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
)

var (
	ErrNotFound      = errors.New("repo: not found")
	ErrAlreadyExists = errors.New("repo: already exists")
	// ErrSessionReused signals that an already-revoked refresh token was replayed — a
	// breach indicator. RotateSession revokes the whole token family when it happens.
	ErrSessionReused = errors.New("repo: refresh token reuse detected")
	// ErrOverpay means a payment exceeds the invoice's remaining balance.
	ErrOverpay = errors.New("repo: payment exceeds invoice balance")
	// ErrInsufficientCoins means a student's coin balance can't cover a purchase.
	ErrInsufficientCoins = errors.New("repo: insufficient coins")
)

// ProvisionParams creates an organization plus its first admin atomically.
type ProvisionParams struct {
	OrgName      string
	Slug         string
	Plan         string
	Email        string
	PasswordHash string
	FullName     string
	Role         entity.UserRole
}

// AuthRepository is the persistence port for authentication/onboarding. It encapsulates
// the RLS-scoped provisioning transaction so the usecase stays free of sqlc/pgx.
type AuthRepository interface {
	ProvisionOrgWithAdmin(ctx context.Context, p ProvisionParams) (entity.Organization, entity.User, error)
	OrgBySlug(ctx context.Context, slug string) (entity.Organization, error)
	OrgByID(ctx context.Context, id uuid.UUID) (entity.Organization, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	UserByEmailInOrg(ctx context.Context, orgID uuid.UUID, email string) (entity.User, error)
	UserByID(ctx context.Context, orgID, id uuid.UUID) (entity.User, error)
	SetPassword(ctx context.Context, orgID, id uuid.UUID, hash string) error
	// AllOrgIDs lists every center (NON-RLS) — for the cross-tenant scheduler.
	AllOrgIDs(ctx context.Context) ([]uuid.UUID, error)
}

// RotateParams atomically replaces one refresh session with another.
type RotateParams struct {
	OldTokenHash string
	NewTokenHash string
	UserAgent    string
	IP           string
	ExpiresAt    time.Time
}

// RotateResult identifies the user/org of the rotated session.
type RotateResult struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
}

// CreateStudentParams is the input to persist a student (risk already computed by the
// usecase). org_id is supplied separately (from the JWT), never from this struct.
type CreateStudentParams struct {
	Name             string
	Phone            string
	TelegramID       string
	CourseName       string
	GroupName        string
	GroupID          *uuid.UUID
	MentorName       string
	StartDate        *time.Time
	OnboardingGoal   string
	SixMonthTarget   string
	WeeklyStudyHours string
	ConfidenceLevel  int
	RiskScore        int
	RiskTier         string
	// CRM contact / identity / lifecycle (grounded field audit).
	Email       string
	BirthDate   *time.Time
	Gender      string
	SecondPhone string
	Address     string
	ParentName  string
	ParentPhone string
	StudentCode string
	Status      string
	MentorID    *uuid.UUID
	BranchID    *uuid.UUID
}

// UpdateStudentParams is the editable profile (risk + group have their own update paths).
type UpdateStudentParams struct {
	Name             string
	Phone            string
	StartDate        *time.Time
	OnboardingGoal   string
	SixMonthTarget   string
	WeeklyStudyHours string
	ConfidenceLevel  int
	Email            string
	BirthDate        *time.Time
	Gender           string
	SecondPhone      string
	Address          string
	ParentName       string
	ParentPhone      string
	StudentCode      string
	Status           string
	MentorID         *uuid.UUID
	BranchID         *uuid.UUID
}

// StudentRepository is the persistence port for students + their notes. Every method is
// tenant-scoped via Postgres RLS (WithTenant keyed on the orgID argument).
type StudentRepository interface {
	Create(ctx context.Context, orgID uuid.UUID, p CreateStudentParams) (entity.Student, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p UpdateStudentParams) (entity.Student, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]entity.Student, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (entity.Student, error)
	UpdateRisk(ctx context.Context, orgID, id uuid.UUID, score int, tier string) error
	AddNote(ctx context.Context, orgID, studentID uuid.UUID, author, text string) (entity.Note, error)
	ListNotes(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Note, error)
	AssignGroup(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID) error
	ListByGroup(ctx context.Context, orgID, groupID uuid.UUID) ([]entity.Student, error)
	// Student mini-app: set a login password (admin), read the student's own profile (student token).
	SetLoginPassword(ctx context.Context, orgID, id uuid.UUID, hash string) error
	GetPasswordHash(ctx context.Context, orgID, id uuid.UUID) (string, error)
	Profile(ctx context.Context, orgID, id uuid.UUID) (entity.StudentProfile, error)
}

// StudentAuthRepository resolves a student login by phone across tenants (RLS-bypassing lookup).
type StudentAuthRepository interface {
	LookupByPhone(ctx context.Context, phone string) ([]entity.StudentAccount, error)
}

// PointsRepository records gamification awards/spends, reads leaderboards, and holds the per-center
// economy config.
type PointsRepository interface {
	Award(ctx context.Context, orgID, studentID uuid.UUID, kind string, xp int, ref string, cfg entity.GamificationConfig) (bool, error)
	Spend(ctx context.Context, orgID, studentID uuid.UUID, coins int, ref string) error
	Leaderboard(ctx context.Context, orgID uuid.UUID, limit int) ([]entity.LeaderRow, error)
	LeaderboardByGroup(ctx context.Context, orgID, groupID uuid.UUID, limit int) ([]entity.LeaderRow, error)
	GetConfig(ctx context.Context, orgID uuid.UUID) (entity.GamificationConfig, error)
	SetConfig(ctx context.Context, orgID uuid.UUID, cfg entity.GamificationConfig) error
}

// ShopRepository persists reward-shop items + purchases, RLS-scoped.
type ShopRepository interface {
	CreateItem(ctx context.Context, orgID uuid.UUID, name, icon string, price int, active bool) (entity.ShopItem, error)
	ListItems(ctx context.Context, orgID uuid.UUID) ([]entity.ShopItem, error)
	GetItem(ctx context.Context, orgID, id uuid.UUID) (entity.ShopItem, error)
	UpdateItem(ctx context.Context, orgID, id uuid.UUID, name, icon string, price int, active bool) (entity.ShopItem, error)
	DeleteItem(ctx context.Context, orgID, id uuid.UUID) error
	ListForStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentShopItem, error)
	BuyItem(ctx context.Context, orgID, studentID, itemID uuid.UUID, price int) error
}

// CreateAssignmentParams is a new homework assignment (org_id supplied separately, from the JWT).
type CreateAssignmentParams struct {
	GroupID     uuid.UUID
	LessonDate  *time.Time
	Title       string
	Description string
	Deadline    *time.Time
	MaxScore    int
}

// UpdateAssignmentParams edits an existing assignment (retitle, re-describe, extend deadline, max).
type UpdateAssignmentParams struct {
	Title       string
	Description string
	Deadline    *time.Time
	MaxScore    int
}

// AssignmentRepository persists homework assignments + submissions (LMS-style), RLS-scoped.
type AssignmentRepository interface {
	CreateAssignment(ctx context.Context, orgID uuid.UUID, p CreateAssignmentParams) (entity.HomeworkAssignment, error)
	UpdateAssignment(ctx context.Context, orgID, id uuid.UUID, p UpdateAssignmentParams) (entity.HomeworkAssignment, error)
	ListGroupAssignments(ctx context.Context, orgID, groupID uuid.UUID) ([]entity.HomeworkAssignment, error)
	GetAssignment(ctx context.Context, orgID, id uuid.UUID) (entity.HomeworkAssignment, error)
	DeleteAssignment(ctx context.Context, orgID, id uuid.UUID) error
	ListSubmissions(ctx context.Context, orgID, assignmentID uuid.UUID) ([]entity.HomeworkSubmission, error)
	Grade(ctx context.Context, orgID, submissionID uuid.UUID, status string, score *int, note string) (entity.HomeworkSubmission, error)
	GroupForSubmission(ctx context.Context, orgID, submissionID uuid.UUID) (uuid.UUID, error)
	GetAssignmentForStudent(ctx context.Context, orgID, assignmentID, studentID uuid.UUID) (entity.HomeworkAssignment, error)
	UpsertSubmission(ctx context.Context, orgID, assignmentID, studentID uuid.UUID, text, links string) (entity.HomeworkSubmission, error)
	ListStudentAssignments(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentAssignment, error)
}

// CreateGroupParams / UpdateGroupParams persist a group (org_id supplied separately, from the JWT).
type CreateGroupParams struct {
	Name         string
	TeacherID    *uuid.UUID
	CourseID     *uuid.UUID
	BranchID     *uuid.UUID
	Direction    string
	ScheduleDays string
	Capacity     int
	StartTime    string
	EndTime      string
	RoomID       *uuid.UUID
}

type UpdateGroupParams struct {
	Name         string
	TeacherID    *uuid.UUID
	CourseID     *uuid.UUID
	BranchID     *uuid.UUID
	Direction    string
	ScheduleDays string
	Capacity     int
	StartTime    string
	EndTime      string
	RoomID       *uuid.UUID
}

// GroupRepository is the persistence port for student groups (RLS-scoped). Create/Update map a
// teacher-not-in-org FK violation to ErrNotFound; Delete detaches students first (composite FK).
type GroupRepository interface {
	Create(ctx context.Context, orgID uuid.UUID, p CreateGroupParams) (entity.Group, error)
	List(ctx context.Context, orgID uuid.UUID) ([]entity.Group, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (entity.Group, error)
	ListByTeacher(ctx context.Context, orgID, teacherID uuid.UUID) ([]entity.Group, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p UpdateGroupParams) (entity.Group, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	CountStudents(ctx context.Context, orgID, id uuid.UUID) (int64, error)
	// MissingAttendance lists groups meeting on dayCode ("mon") with no attendance for date.
	MissingAttendance(ctx context.Context, orgID uuid.UUID, dayCode string, date time.Time) ([]entity.Group, error)
	// Membership (a student can study in several groups at once).
	AddMember(ctx context.Context, orgID, groupID, studentID uuid.UUID) error
	RemoveMember(ctx context.Context, orgID, groupID, studentID uuid.UUID) error
	ListForStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Group, error)
}

// StaffRepository manages back-office staff accounts (center_admin + finance), RLS-scoped.
type StaffRepository interface {
	Create(ctx context.Context, orgID uuid.UUID, email, passwordHash, fullName, role string) (entity.User, error)
	List(ctx context.Context, orgID uuid.UUID) ([]entity.User, error)
	Update(ctx context.Context, orgID, id uuid.UUID, email, fullName, role string) (entity.User, error)
	SetPassword(ctx context.Context, orgID, id uuid.UUID, passwordHash string) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	CountByRole(ctx context.Context, orgID uuid.UUID, role string) (int64, error)
}

// TeacherRepository manages teacher accounts (users with role 'teacher'), RLS-scoped.
type TeacherRepository interface {
	Create(ctx context.Context, orgID uuid.UUID, email, passwordHash, fullName string, branchID *uuid.UUID) (entity.User, error)
	List(ctx context.Context, orgID uuid.UUID) ([]entity.User, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (entity.User, error)
	Update(ctx context.Context, orgID, id uuid.UUID, email, fullName string, branchID *uuid.UUID) (entity.User, error)
	SetPassword(ctx context.Context, orgID, id uuid.UUID, passwordHash string) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

// CreateSurveyParams persists a weekly check-in (org_id supplied separately, from the JWT).
type CreateSurveyParams struct {
	StudentID       uuid.UUID
	WeekNumber      int
	MotivationScore int
	ProgressScore   int
	BiggestObstacle string
	Comment         string
}

// SurveyRepository persists + reads weekly surveys (RLS-scoped). Create maps a composite-FK
// violation (student not in this tenant) to ErrNotFound.
type SurveyRepository interface {
	Create(ctx context.Context, orgID uuid.UUID, p CreateSurveyParams) (entity.Survey, error)
	ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Survey, error)
}

// AttendanceRepository persists + reads attendance records (RLS-scoped).
type AttendanceRepository interface {
	Create(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID, date time.Time, present bool) (entity.AttendanceRecord, error)
	ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.AttendanceRecord, error)
}

// HomeworkRepository persists + reads homework-completion records (RLS-scoped).
type HomeworkRepository interface {
	Create(ctx context.Context, orgID, studentID uuid.UUID, date time.Time, done bool) (entity.HomeworkRecord, error)
	ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.HomeworkRecord, error)
}

// InterventionRepository manages auto-opened intervention tasks (RLS-scoped).
type InterventionRepository interface {
	Create(ctx context.Context, orgID, studentID uuid.UUID, reasons []string) (entity.InterventionTask, error)
	CountOpenForStudent(ctx context.Context, orgID, studentID uuid.UUID) (int64, error)
	List(ctx context.Context, orgID uuid.UUID) ([]entity.InterventionTask, error)
	Resolve(ctx context.Context, orgID, id uuid.UUID, comment string) (entity.InterventionTask, error)
	// Start moves an Open task to In Progress (ErrNotFound if missing or not Open).
	Start(ctx context.Context, orgID, id uuid.UUID) (entity.InterventionTask, error)
	// Assign sets (or clears, with nil) the staff member responsible for a task.
	Assign(ctx context.Context, orgID, id uuid.UUID, assignedTo *uuid.UUID) (entity.InterventionTask, error)
}

// StaleTask is an unresolved intervention past its SLA (for the director escalation).
type StaleTask struct {
	ID          uuid.UUID
	StudentID   uuid.UUID
	StudentName string
}

// NotificationRepository backs the in-app notification feed + the SLA escalation scan.
type NotificationRepository interface {
	Create(ctx context.Context, orgID, userID uuid.UUID, kind, title, body, link string) (entity.Notification, error)
	List(ctx context.Context, orgID, userID uuid.UUID) ([]entity.Notification, error)
	CountUnread(ctx context.Context, orgID, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, orgID, id, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error
	StaleTasks(ctx context.Context, orgID uuid.UUID, cutoff time.Time) ([]StaleTask, error)
	MarkTaskEscalated(ctx context.Context, orgID, taskID uuid.UUID) error
	DirectorIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
}

// RiskOutcome is the persisted result of recomputing a student's risk inside the
// retention transaction.
type RiskOutcome struct {
	Score       int
	Tier        string
	TaskReasons []string // non-empty → ensure an open intervention task exists (idempotent)
}

// Recompute is the pure risk computation the usecase injects into the retention transaction.
type Recompute func(st entity.Student, attendance []entity.AttendanceRecord, surveys []entity.Survey, homework []entity.HomeworkRecord) RiskOutcome

// RetentionRepository performs a survey/attendance/homework write AND the risk recompute
// (+ idempotent intervention-task auto-open) atomically in one tenant transaction — so the
// user's data and the derived risk state can never diverge on partial failure.
type RetentionRepository interface {
	SubmitSurvey(ctx context.Context, orgID, studentID uuid.UUID, p CreateSurveyParams, fn Recompute) (entity.Survey, error)
	RecordAttendance(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID, date time.Time, status string, fn Recompute) (entity.AttendanceRecord, error)
	RecordHomework(ctx context.Context, orgID, studentID uuid.UUID, date time.Time, done bool, fn Recompute) (entity.HomeworkRecord, error)
	// RecordAdviceFeedback upserts a mentor's useful/not-useful rating of a student's AI advice
	// (one per mentor per student). It performs no risk recompute.
	RecordAdviceFeedback(ctx context.Context, orgID, studentID, userID uuid.UUID, useful bool) error
	// RecomputeStudent re-runs the risk computation (+ idempotent task open) for one student
	// without a preceding write — used by the scheduler to apply time-based trigger rules.
	RecomputeStudent(ctx context.Context, orgID, studentID uuid.UUID, fn Recompute) error
}

// DashboardData is a read-only aggregate snapshot for one tenant, assembled in a single
// tenant transaction (consistent view). The usecase derives KPIs from it; it holds raw rows.
type DashboardData struct {
	TierCounts       map[string]int   // risk_tier → student count (e.g. "Red"→4)
	Groups           []GroupRisk      // per-group student count + avg risk, worst first
	Obstacles        []ObstacleCount  // most-common latest-survey obstacle, most-common first
	HighRisk         []entity.Student // up to 10 at-risk students, highest score first
	MotivationByWeek []WeekMotivation // avg motivation per survey week, ascending
}

// GroupRisk is the average risk of one student group.
type GroupRisk struct {
	Group        string
	StudentCount int
	AvgRisk      int
}

// ObstacleCount counts how many students report a given obstacle in their latest survey.
type ObstacleCount struct {
	Obstacle string
	Count    int
}

// WeekMotivation is the average motivation score reported in a given survey week.
type WeekMotivation struct {
	Week          int
	AvgMotivation float64
}

// DashboardRepository loads the tenant's dashboard aggregates (RLS-scoped, one transaction).
type DashboardRepository interface {
	Load(ctx context.Context, orgID uuid.UUID) (DashboardData, error)
}

// BotInvite is a resolved deep-link invite token.
type BotInvite struct {
	Token     string
	OrgID     uuid.UUID
	StudentID uuid.UUID
	Used      bool
	Expired   bool
}

// BotConversation is a chat's binding + FSM state. State is opaque JSON owned by the bot usecase.
type BotConversation struct {
	ChatID    int64
	OrgID     uuid.UUID
	StudentID uuid.UUID
	Flow      string
	Step      string
	State     []byte
}

// LinkedChat is a Telegram chat bound to a student (for the weekly survey broadcast).
type LinkedChat struct {
	ChatID    int64
	OrgID     uuid.UUID
	StudentID uuid.UUID
}

// BotRepository persists the Telegram bot's cross-tenant state. invite_tokens + bot_conversations
// are NON-RLS (resolved by secret token / unique chat_id before any tenant scope exists); the
// student-binding write enters WithTenant since students is RLS-scoped.
type BotRepository interface {
	CreateInvite(ctx context.Context, token string, orgID, studentID, createdBy uuid.UUID, expiresAt time.Time) error
	ResolveInvite(ctx context.Context, token string) (BotInvite, error)
	UseInvite(ctx context.Context, token string) error
	BindChat(ctx context.Context, chatID int64, orgID, studentID uuid.UUID) error
	GetConversation(ctx context.Context, chatID int64) (BotConversation, error)
	SetFlow(ctx context.Context, chatID int64, flow, step string, state []byte) error
	SetStudentChat(ctx context.Context, orgID, studentID uuid.UUID, chatID int64) error
	// DueHomeworkReminders / MarkHomeworkReminded drive the "deadline in 2h" push, per org.
	DueHomeworkReminders(ctx context.Context, orgID uuid.UUID) ([]entity.HomeworkReminder, error)
	MarkHomeworkReminded(ctx context.Context, orgID, assignmentID uuid.UUID) error
	// ListLinkedChats returns every chat bound to a student (NON-RLS), for the survey broadcast.
	ListLinkedChats(ctx context.Context) ([]LinkedChat, error)
	// ListObstacleLabels returns a center's active "biggest obstacle" choices (ordered), for the
	// check-in keyboard. Empty slice when the center hasn't configured any (caller uses defaults).
	ListObstacleLabels(ctx context.Context, orgID uuid.UUID) ([]string, error)
}

// ObstacleRepository manages a center's configurable "biggest obstacle" choices (center_admin).
type ObstacleRepository interface {
	List(ctx context.Context, orgID uuid.UUID) ([]entity.ObstacleOption, error)
	Create(ctx context.Context, orgID uuid.UUID, label string, position int) (entity.ObstacleOption, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

// CourseRepository manages a center's sellable courses/programs (center_admin).
type CourseRepository interface {
	List(ctx context.Context, orgID uuid.UUID) ([]entity.Course, error)
	Get(ctx context.Context, orgID, id uuid.UUID) (entity.Course, error)
	Create(ctx context.Context, orgID uuid.UUID, p CreateCourseParams) (entity.Course, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p UpdateCourseParams) (entity.Course, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type CreateCourseParams struct {
	Name          string
	Level         string
	Price         int64
	DurationWeeks int
	Description   string
}

type UpdateCourseParams struct {
	Name          string
	Level         string
	Price         int64
	DurationWeeks int
	Description   string
	IsActive      bool
}

// EnrollmentRepository manages student enrolments in groups/courses (center_admin).
type EnrollmentRepository interface {
	ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Enrollment, error)
	Create(ctx context.Context, orgID uuid.UUID, p CreateEnrollmentParams) (entity.Enrollment, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p UpdateEnrollmentParams) (entity.Enrollment, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type CreateEnrollmentParams struct {
	StudentID uuid.UUID
	GroupID   *uuid.UUID
	CourseID  *uuid.UUID
	Status    string
	StartDate *time.Time
	EndDate   *time.Time
	Price     int64
	Discount  int
}

type UpdateEnrollmentParams struct {
	GroupID   *uuid.UUID
	CourseID  *uuid.UUID
	Status    string
	StartDate *time.Time
	EndDate   *time.Time
	Price     int64
	Discount  int
}

// FinanceRepository manages a center's invoices + payments (manual ledger). center_admin.
type FinanceRepository interface {
	ListInvoices(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Invoice, error)
	CreateInvoice(ctx context.Context, orgID uuid.UUID, p CreateInvoiceParams) (entity.Invoice, error)
	GenerateMonthly(ctx context.Context, orgID uuid.UUID, period string) (int64, error)
	DeleteInvoice(ctx context.Context, orgID, id uuid.UUID) error
	StudentBalance(ctx context.Context, orgID, studentID uuid.UUID) (int64, error)
	ListPayments(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Payment, error)
	// RecordPayment inserts a payment and bumps the invoice's paid_amount atomically.
	RecordPayment(ctx context.Context, orgID uuid.UUID, p RecordPaymentParams) (entity.Payment, error)
	// DeletePayment removes a payment and decrements its invoice's paid_amount atomically.
	DeletePayment(ctx context.Context, orgID, id uuid.UUID) error
	Summary(ctx context.Context, orgID uuid.UUID) (entity.FinanceSummary, error)
	ListDebtors(ctx context.Context, orgID uuid.UUID) ([]entity.Debtor, error)
	OverdueInvoices(ctx context.Context, orgID uuid.UUID) ([]entity.OverdueInvoice, error)
	GetGraceLessons(ctx context.Context, orgID uuid.UUID) (int, error)
	SetGraceLessons(ctx context.Context, orgID uuid.UUID, n int) error
	GraceOverdue(ctx context.Context, orgID uuid.UUID, period string) ([]entity.GraceOverdue, error)
	// GroupFinance returns each group member's invoiced/paid totals for a period ("2026-07").
	GroupFinance(ctx context.Context, orgID, groupID uuid.UUID, period string) ([]entity.GroupFinanceRow, error)
	CreateExpense(ctx context.Context, orgID uuid.UUID, p CreateExpenseParams) (entity.Expense, error)
	ListExpenses(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.Expense, error)
	DeleteExpense(ctx context.Context, orgID, id uuid.UUID) error
	ExpensesByCategory(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.CategoryTotal, error)
}

type CreateExpenseParams struct {
	Category string
	Amount   int64
	SpentAt  *time.Time
	Note     string
}

// GroupRuleParams is one per-group salary override to persist.
type GroupRuleParams struct {
	GroupID uuid.UUID
	Kind    string
	Rate    int64
}

// SalaryRepository manages teacher payroll (rules + slips) and computes a period's pay basis.
type SalaryRepository interface {
	GetRule(ctx context.Context, orgID, teacherID uuid.UUID) (entity.SalaryRule, error)
	SetRule(ctx context.Context, orgID, teacherID uuid.UUID, kind string, rate, base int64) (entity.SalaryRule, error)
	ListGroupRules(ctx context.Context, orgID, teacherID uuid.UUID) ([]entity.SalaryGroupRule, error)
	ReplaceGroupRules(ctx context.Context, orgID, teacherID uuid.UUID, rules []GroupRuleParams) error
	Basis(ctx context.Context, orgID, teacherID uuid.UUID, from, to time.Time) (lessons, students, revenue int64, err error)
	GroupBasis(ctx context.Context, orgID, teacherID uuid.UUID, from, to time.Time) ([]entity.GroupBasis, error)
	CreateSlip(ctx context.Context, orgID uuid.UUID, p SalarySlipParams) (entity.SalarySlip, error)
	ListSlips(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.SalarySlip, error)
	MarkPaid(ctx context.Context, orgID, id uuid.UUID) (entity.SalarySlip, error)
	DeleteSlip(ctx context.Context, orgID, id uuid.UUID) error
}

type SalarySlipParams struct {
	TeacherID   uuid.UUID
	PeriodStart time.Time
	PeriodEnd   time.Time
	Gross       int64
	Bonus       int64
	Deduction   int64
	Net         int64
	Note        string
}

type CreateInvoiceParams struct {
	StudentID    uuid.UUID
	GroupID      *uuid.UUID // which group (course) the charge is for
	EnrollmentID *uuid.UUID
	Amount       int64
	DueDate      *time.Time
	Period       string
	Note         string
}

type RecordPaymentParams struct {
	InvoiceID uuid.UUID
	Amount    int64
	Method    string
	PaidAt    *time.Time
	Note      string
}

// LeadRepository manages the sales funnel (center_admin).
type LeadRepository interface {
	List(ctx context.Context, orgID uuid.UUID) ([]entity.Lead, error)
	Get(ctx context.Context, orgID, id uuid.UUID) (entity.Lead, error)
	Create(ctx context.Context, orgID uuid.UUID, p LeadParams) (entity.Lead, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p LeadParams) (entity.Lead, error)
	SetStage(ctx context.Context, orgID, id uuid.UUID, stage string) (entity.Lead, error)
	MarkConverted(ctx context.Context, orgID, id, studentID uuid.UUID) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

// LeadParams is the writable lead fields (org_id from the JWT).
type LeadParams struct {
	Name       string
	Phone      string
	Email      string
	Source     string
	Stage      string
	Interest   string
	Note       string
	AssignedTo *uuid.UUID
}

// ActivityRepository manages the polymorphic communication timeline (center_admin).
type ActivityRepository interface {
	List(ctx context.Context, orgID uuid.UUID, subjectType string, subjectID uuid.UUID) ([]entity.Activity, error)
	Create(ctx context.Context, orgID uuid.UUID, p CreateActivityParams) (entity.Activity, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type CreateActivityParams struct {
	SubjectType string
	SubjectID   uuid.UUID
	Type        string
	Body        string
	Author      string
}

// LessonRepository manages scheduled class sessions (center_admin).
type LessonRepository interface {
	List(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.Lesson, error)
	Create(ctx context.Context, orgID uuid.UUID, p LessonParams) (entity.Lesson, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p LessonParams) (entity.Lesson, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	// FindByGroupDate returns the session record for a group on a date (found=false if none).
	FindByGroupDate(ctx context.Context, orgID, groupID uuid.UUID, date time.Time) (entity.Lesson, bool, error)
	// CountRoomConflicts counts other lessons booking roomID at an overlapping time on date
	// (excludeID = self on update, or uuid.Nil).
	CountRoomConflicts(ctx context.Context, orgID, roomID uuid.UUID, date time.Time, start, end string, excludeID uuid.UUID) (int64, error)
}

type LessonParams struct {
	GroupID   *uuid.UUID
	TeacherID *uuid.UUID
	Date      time.Time
	StartTime string
	EndTime   string
	Room      string
	RoomID    *uuid.UUID
	Topic     string
	Status    string
}

// RoomRepository persists physical rooms (RLS-scoped).
type RoomRepository interface {
	List(ctx context.Context, orgID uuid.UUID) ([]entity.Room, error)
	Create(ctx context.Context, orgID uuid.UUID, p RoomParams) (entity.Room, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p RoomParams) (entity.Room, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type RoomParams struct {
	BranchID *uuid.UUID
	Name     string
	Capacity int
}

// CenterStats is one center (organization) plus its tenant-scoped counts, for the superadmin
// panel. Counts are gathered by entering each org's WithTenant scope (no RLS bypass needed).
type CenterStats struct {
	Org      entity.Organization
	Students int
	Users    int
	Green    int
	Yellow   int
	Red      int
}

// SuperadminRepository is the cross-tenant center-management port (super_admin only). It reads
// the NON-RLS organizations table and gathers per-center counts inside each org's tenant scope.
type SuperadminRepository interface {
	ListCenters(ctx context.Context, excludeSlug string) ([]CenterStats, error)
	GetCenter(ctx context.Context, id uuid.UUID) (CenterStats, error)
	UpdateCenter(ctx context.Context, id uuid.UUID, plan, status string) (entity.Organization, error)
	SetBilling(ctx context.Context, id uuid.UUID, plan, billingStatus string, trialEndsAt *time.Time) (entity.Organization, error)
	DeleteCenter(ctx context.Context, id uuid.UUID) error
}

// BranchRepository persists branches (filiallar) within an org (RLS-scoped).
type BranchRepository interface {
	List(ctx context.Context, orgID uuid.UUID) ([]entity.Branch, error)
	Create(ctx context.Context, orgID uuid.UUID, p BranchParams) (entity.Branch, error)
	Update(ctx context.Context, orgID, id uuid.UUID, p BranchParams) (entity.Branch, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type BranchParams struct {
	Name     string
	Address  string
	Phone    string
	IsActive bool
}

// SignupRepository persists public landing-page signup requests (platform-level, non-RLS).
type SignupRepository interface {
	Create(ctx context.Context, in entity.SignupRequest) (entity.SignupRequest, error)
	List(ctx context.Context) ([]entity.SignupRequest, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string) (entity.SignupRequest, error)
}

// PlanRepository persists landing-page pricing plans (platform-level, non-RLS): the public site
// renders the active ones; the super_admin edits them.
type PlanRepository interface {
	ListActive(ctx context.Context) ([]entity.Plan, error)
	ListAll(ctx context.Context) ([]entity.Plan, error)
	Create(ctx context.Context, in entity.Plan) (entity.Plan, error)
	Update(ctx context.Context, in entity.Plan) (entity.Plan, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// SessionRepository manages refresh-token sessions (not RLS-scoped; keyed by token hash).
type SessionRepository interface {
	CreateSession(ctx context.Context, s entity.RefreshSession) (entity.RefreshSession, error)
	// RotateSession atomically (one tx, row-locked) verifies the old token is active,
	// revokes it, and inserts the replacement. Replay of an already-revoked token returns
	// ErrSessionReused after revoking every active session for that user.
	RotateSession(ctx context.Context, p RotateParams) (RotateResult, error)
	// DeleteExpired prunes expired sessions (called by a periodic reaper).
	DeleteExpired(ctx context.Context) error
}

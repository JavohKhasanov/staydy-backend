// Package student is the students usecase: CRUD, weekly surveys, attendance, and the risk
// loop. Each survey/attendance write runs atomically with the risk recompute (and the
// auto-open of an intervention task when a student turns Red) inside the retention repo.
package student

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/risk"
	"github.com/student-success/backend/internal/security"
)

var (
	ErrValidation = errors.New("validation failed")
	ErrNotFound   = errors.New("student not found")
)

// CreateParams is the onboarding input for a new student.
type CreateParams struct {
	Name             string
	Phone            string
	TelegramID       string
	CourseName       string
	GroupName        string
	MentorName       string
	StartDate        *time.Time
	OnboardingGoal   string
	SixMonthTarget   string
	WeeklyStudyHours string
	ConfidenceLevel  int
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

// UpdateParams is the editable profile (risk + group have their own update paths). Editing
// confidence triggers a risk recompute.
type UpdateParams struct {
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

// normalizeStatus defaults to "active" and rejects unknown lifecycle values.
func normalizeStatus(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "active", nil
	}
	if !entity.StudentStatuses[s] {
		return "", fmt.Errorf("%w: invalid status", ErrValidation)
	}
	return s, nil
}

// SubmitSurveyParams is a weekly check-in.
type SubmitSurveyParams struct {
	WeekNumber      int
	MotivationScore int
	ProgressScore   int
	BiggestObstacle string
	Comment         string
}

// Detail bundles a student with notes, history, and the live risk-factor breakdown.
type Detail struct {
	Student    entity.Student
	Notes      []entity.Note
	Surveys    []entity.Survey
	Attendance []entity.AttendanceRecord
	Homework   []entity.HomeworkRecord
	Factors    []risk.Factor
}

// Awarder grants gamification points for attended lessons and completed check-ins (satisfied by the
// points service). Optional — nil means no gamification.
type Awarder interface {
	AwardAttendance(ctx context.Context, orgID, studentID, groupID uuid.UUID, date time.Time, status string) error
	AwardCheckin(ctx context.Context, orgID, studentID uuid.UUID, week int) error
}

type Service struct {
	students   repo.StudentRepository
	surveys    repo.SurveyRepository
	attendance repo.AttendanceRepository
	homework   repo.HomeworkRepository
	retention  repo.RetentionRepository
	awarder    Awarder
}

func NewService(
	students repo.StudentRepository,
	surveys repo.SurveyRepository,
	attendance repo.AttendanceRepository,
	homework repo.HomeworkRepository,
	retention repo.RetentionRepository,
) *Service {
	return &Service{students: students, surveys: surveys, attendance: attendance, homework: homework, retention: retention}
}

// SetAwarder wires gamification (call once at startup). Safe to leave unset.
func (s *Service) SetAwarder(a Awarder) { s.awarder = a }

// Create validates the onboarding data, computes the initial risk score (no attendance or
// surveys yet), and persists the student.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, p CreateParams) (entity.Student, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return entity.Student{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if p.ConfidenceLevel < 1 || p.ConfidenceLevel > 10 {
		return entity.Student{}, fmt.Errorf("%w: confidenceLevel must be between 1 and 10", ErrValidation)
	}
	status, err := normalizeStatus(p.Status)
	if err != nil {
		return entity.Student{}, err
	}

	result := computeRisk(entity.Student{ConfidenceLevel: p.ConfidenceLevel}, nil, nil, nil)
	return s.students.Create(ctx, orgID, repo.CreateStudentParams{
		Name:             p.Name,
		Phone:            strings.TrimSpace(p.Phone),
		TelegramID:       strings.TrimSpace(p.TelegramID),
		CourseName:       strings.TrimSpace(p.CourseName),
		GroupName:        strings.TrimSpace(p.GroupName),
		MentorName:       strings.TrimSpace(p.MentorName),
		StartDate:        p.StartDate,
		OnboardingGoal:   strings.TrimSpace(p.OnboardingGoal),
		SixMonthTarget:   strings.TrimSpace(p.SixMonthTarget),
		WeeklyStudyHours: strings.TrimSpace(p.WeeklyStudyHours),
		ConfidenceLevel:  p.ConfidenceLevel,
		RiskScore:        result.Score,
		RiskTier:         string(result.Tier),
		Email:            strings.TrimSpace(p.Email),
		BirthDate:        p.BirthDate,
		Gender:           strings.TrimSpace(p.Gender),
		SecondPhone:      strings.TrimSpace(p.SecondPhone),
		Address:          strings.TrimSpace(p.Address),
		ParentName:       strings.TrimSpace(p.ParentName),
		ParentPhone:      strings.TrimSpace(p.ParentPhone),
		StudentCode:      strings.TrimSpace(p.StudentCode),
		Status:           status,
		MentorID:         p.MentorID,
		BranchID:         p.BranchID,
	})
}

// Update edits a student's profile, then recomputes risk (confidence may have changed). Group and
// risk fields are managed by their own paths and left untouched here.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, p UpdateParams) (entity.Student, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return entity.Student{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	if p.ConfidenceLevel < 1 || p.ConfidenceLevel > 10 {
		return entity.Student{}, fmt.Errorf("%w: confidenceLevel must be between 1 and 10", ErrValidation)
	}
	status, err := normalizeStatus(p.Status)
	if err != nil {
		return entity.Student{}, err
	}
	st, err := s.students.Update(ctx, orgID, id, repo.UpdateStudentParams{
		Name:             p.Name,
		Phone:            strings.TrimSpace(p.Phone),
		StartDate:        p.StartDate,
		OnboardingGoal:   strings.TrimSpace(p.OnboardingGoal),
		SixMonthTarget:   strings.TrimSpace(p.SixMonthTarget),
		WeeklyStudyHours: strings.TrimSpace(p.WeeklyStudyHours),
		ConfidenceLevel:  p.ConfidenceLevel,
		Email:            strings.TrimSpace(p.Email),
		BirthDate:        p.BirthDate,
		Gender:           strings.TrimSpace(p.Gender),
		SecondPhone:      strings.TrimSpace(p.SecondPhone),
		Address:          strings.TrimSpace(p.Address),
		ParentName:       strings.TrimSpace(p.ParentName),
		ParentPhone:      strings.TrimSpace(p.ParentPhone),
		StudentCode:      strings.TrimSpace(p.StudentCode),
		Status:           status,
		MentorID:         p.MentorID,
		BranchID:         p.BranchID,
	})
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.Student{}, ErrNotFound
		}
		return entity.Student{}, err
	}
	// Confidence feeds risk — recompute (best-effort; the edit already succeeded).
	_ = s.retention.RecomputeStudent(ctx, orgID, id, s.outcome)
	return st, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.Student, error) {
	return s.students.List(ctx, orgID)
}

// Delete removes a student and all their child records (attendance, notes, invoices, … cascade).
func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.students.Delete(ctx, orgID, id)
}

// SetLoginPassword sets (or resets) a student's mini-app login password. A center admin calls this
// for their own students; the student then logs in with their phone + this password.
func (s *Service) SetLoginPassword(ctx context.Context, orgID, id uuid.UUID, password string) error {
	if len(password) < 6 {
		return ErrValidation
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	return s.students.SetLoginPassword(ctx, orgID, id, hash)
}

// Profile returns the signed-in student's own profile (for the mini app).
func (s *Service) Profile(ctx context.Context, orgID, id uuid.UUID) (entity.StudentProfile, error) {
	return s.students.Profile(ctx, orgID, id)
}

// Get returns a student with notes, survey/attendance history, and a fresh risk breakdown.
// The returned score is recomputed alongside the factors so the two always agree.
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Detail, error) {
	st, err := s.students.GetByID(ctx, orgID, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	notes, err := s.students.ListNotes(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	att, err := s.attendance.ListByStudent(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	surveys, err := s.surveys.ListByStudent(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	hw, err := s.homework.ListByStudent(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	// Use the trigger-aware evaluation so the displayed tier matches what gets persisted
	// (e.g. a consecutive-absence trigger can force Red even when the raw score is below 70).
	result, _ := evaluateWithTriggers(st, att, surveys, hw)
	st.RiskScore = result.Score
	st.RiskTier = string(result.Tier)
	return &Detail{Student: st, Notes: notes, Surveys: surveys, Attendance: att, Homework: hw, Factors: result.Factors}, nil
}

// AddNote attaches a mentor note (atomic; composite FK enforces the tenant relationship).
func (s *Service) AddNote(ctx context.Context, orgID, studentID uuid.UUID, author, text string) (entity.Note, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return entity.Note{}, fmt.Errorf("%w: text is required", ErrValidation)
	}
	note, err := s.students.AddNote(ctx, orgID, studentID, author, text)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.Note{}, ErrNotFound
		}
		return entity.Note{}, err
	}
	return note, nil
}

// SubmitSurvey records a weekly check-in and recomputes risk atomically (see RetentionRepository).
func (s *Service) SubmitSurvey(ctx context.Context, orgID, studentID uuid.UUID, p SubmitSurveyParams) (entity.Survey, error) {
	if p.WeekNumber < 1 {
		return entity.Survey{}, fmt.Errorf("%w: weekNumber must be >= 1", ErrValidation)
	}
	if p.MotivationScore < 1 || p.MotivationScore > 5 || p.ProgressScore < 1 || p.ProgressScore > 5 {
		return entity.Survey{}, fmt.Errorf("%w: motivationScore and progressScore must be between 1 and 5", ErrValidation)
	}
	survey, err := s.retention.SubmitSurvey(ctx, orgID, studentID, repo.CreateSurveyParams{
		StudentID:       studentID,
		WeekNumber:      p.WeekNumber,
		MotivationScore: p.MotivationScore,
		ProgressScore:   p.ProgressScore,
		BiggestObstacle: strings.TrimSpace(p.BiggestObstacle),
		Comment:         strings.TrimSpace(p.Comment),
	}, s.outcome)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.Survey{}, ErrNotFound
		}
		return entity.Survey{}, err
	}
	return survey, nil
}

// SubmitWeeklySurvey is the bot-facing entrypoint: the student never picks a week number, so it
// auto-assigns the next one (max existing + 1) and submits through the same retention path as the
// API (insert + risk recompute + auto-task, atomically).
func (s *Service) SubmitWeeklySurvey(ctx context.Context, orgID, studentID uuid.UUID, motivation, progress int, obstacle, comment string) (entity.Survey, error) {
	existing, err := s.surveys.ListByStudent(ctx, orgID, studentID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.Survey{}, ErrNotFound
		}
		return entity.Survey{}, err
	}
	next := 1
	for _, sv := range existing {
		if sv.WeekNumber >= next {
			next = sv.WeekNumber + 1
		}
	}
	sv, err := s.SubmitSurvey(ctx, orgID, studentID, SubmitSurveyParams{
		WeekNumber:      next,
		MotivationScore: motivation,
		ProgressScore:   progress,
		BiggestObstacle: obstacle,
		Comment:         comment,
	})
	if err != nil {
		return entity.Survey{}, err
	}
	// Award check-in XP at most once per real calendar week (anti-farming — the survey's own
	// week number just counts up, so it can't be the idempotency key).
	if s.awarder != nil {
		y, w := time.Now().ISOWeek()
		_ = s.awarder.AwardCheckin(ctx, orgID, studentID, y*100+w)
	}
	return sv, nil
}

// RecordAttendance records one lesson's attendance (status: present|absent|late|excused) and
// recomputes risk atomically. is_present is derived from status in the repo, so the risk engine is
// unaffected by the new granularity (late/excused count as attended).
func (s *Service) RecordAttendance(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID, date time.Time, status string) (entity.AttendanceRecord, error) {
	if !entity.ValidAttendanceStatus(status) {
		return entity.AttendanceRecord{}, ErrValidation
	}
	rec, err := s.retention.RecordAttendance(ctx, orgID, studentID, groupID, date, status, s.outcome)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.AttendanceRecord{}, ErrNotFound
		}
		return entity.AttendanceRecord{}, err
	}
	// Reward attendance (the awarder scales XP by status: on-time > late; absent earns nothing).
	if s.awarder != nil && groupID != nil {
		_ = s.awarder.AwardAttendance(ctx, orgID, studentID, *groupID, date, status)
	}
	return rec, nil
}

// RecordHomework records one lesson's homework completion and recomputes risk atomically.
func (s *Service) RecordHomework(ctx context.Context, orgID, studentID uuid.UUID, date time.Time, done bool) (entity.HomeworkRecord, error) {
	rec, err := s.retention.RecordHomework(ctx, orgID, studentID, date, done, s.outcome)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.HomeworkRecord{}, ErrNotFound
		}
		return entity.HomeworkRecord{}, err
	}
	return rec, nil
}

// RecordAdviceFeedback stores a mentor's useful/not-useful rating of a student's AI advice.
func (s *Service) RecordAdviceFeedback(ctx context.Context, orgID, studentID, userID uuid.UUID, useful bool) error {
	if err := s.retention.RecordAdviceFeedback(ctx, orgID, studentID, userID, useful); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Trigger thresholds (TZ §6.3): rules that act regardless of the raw score.
const (
	triggerConsecutiveAbsences = 4  // ~2 weeks of consecutive missed lessons → force Red
	triggerScoreJump           = 20 // a +20 risk increase vs the last stored score → open a task
)

// RecomputeAll re-runs every student's risk in an org (scheduler/manual ops). It applies the
// current trigger rules with fresh data and returns the number of students processed.
func (s *Service) RecomputeAll(ctx context.Context, orgID uuid.UUID) (int, error) {
	students, err := s.students.List(ctx, orgID)
	if err != nil {
		return 0, err
	}
	for _, st := range students {
		if err := s.retention.RecomputeStudent(ctx, orgID, st.ID, s.outcome); err != nil {
			return 0, err
		}
	}
	return len(students), nil
}

// ImportRow is one row of a bulk attendance/homework import (CRM-with mode; CSV-derived).
type ImportRow struct {
	Name         string
	Date         time.Time
	Present      *bool // nil → don't touch attendance for this row
	HomeworkDone *bool // nil → don't touch homework for this row
}

// SkippedRow explains why a row wasn't imported.
type SkippedRow struct {
	Name   string
	Reason string
}

// ImportResult summarizes a bulk import.
type ImportResult struct {
	Imported int
	Skipped  []SkippedRow
}

// ImportRecords bulk-records attendance and/or homework for students matched by name. Rows that
// match zero or multiple students are skipped with a reason. Each recorded row runs through the
// same atomic risk-recompute path as the single-record endpoints.
func (s *Service) ImportRecords(ctx context.Context, orgID uuid.UUID, rows []ImportRow) (ImportResult, error) {
	students, err := s.students.List(ctx, orgID)
	if err != nil {
		return ImportResult{}, err
	}
	byName := make(map[string][]entity.Student, len(students))
	for _, st := range students {
		key := normalizeName(st.Name)
		byName[key] = append(byName[key], st)
	}

	res := ImportResult{Skipped: []SkippedRow{}}
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		matches := byName[normalizeName(name)]
		switch {
		case len(matches) == 0:
			res.Skipped = append(res.Skipped, SkippedRow{Name: name, Reason: "talaba topilmadi"})
			continue
		case len(matches) > 1:
			res.Skipped = append(res.Skipped, SkippedRow{Name: name, Reason: "bir nechta talaba shu nom bilan"})
			continue
		}
		st := matches[0]
		acted := false
		if row.Present != nil {
			status := entity.AttendancePresent // CSV import is boolean; map false -> absent
			if !*row.Present {
				status = entity.AttendanceAbsent
			}
			if _, err := s.RecordAttendance(ctx, orgID, st.ID, nil, row.Date, status); err != nil {
				res.Skipped = append(res.Skipped, SkippedRow{Name: name, Reason: "davomat saqlanmadi"})
				continue
			}
			acted = true
		}
		if row.HomeworkDone != nil {
			if _, err := s.RecordHomework(ctx, orgID, st.ID, row.Date, *row.HomeworkDone); err != nil {
				res.Skipped = append(res.Skipped, SkippedRow{Name: name, Reason: "uy vazifa saqlanmadi"})
				continue
			}
			acted = true
		}
		if acted {
			res.Imported++
		} else {
			res.Skipped = append(res.Skipped, SkippedRow{Name: name, Reason: "present yoki homeworkDone yo'q"})
		}
	}
	return res, nil
}

func normalizeName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// outcome is the risk computation injected into the retention transaction: it returns the
// score/tier to persist and, when the student is Red OR a trigger rule fires, the reasons for
// an open intervention task.
func (s *Service) outcome(st entity.Student, att []entity.AttendanceRecord, surveys []entity.Survey, homework []entity.HomeworkRecord) repo.RiskOutcome {
	result, triggers := evaluateWithTriggers(st, att, surveys, homework)
	out := repo.RiskOutcome{Score: result.Score, Tier: string(result.Tier)}
	if result.Tier == risk.TierRed || len(triggers) > 0 {
		reasons := append(reasonsFrom(result.Factors), triggers...)
		out.TaskReasons = dedupeStrings(reasons)
		if len(out.TaskReasons) == 0 {
			out.TaskReasons = []string{"Xavf ostida"}
		}
	}
	return out
}

// evaluateWithTriggers computes the pure risk Result, then applies the TZ trigger rules:
// (a) >= triggerConsecutiveAbsences trailing absences force the tier to Red; (b) a >= triggerScoreJump
// increase vs the previously stored score flags an urgent task. It returns the (possibly
// tier-adjusted) result plus any trigger-derived task reasons.
func evaluateWithTriggers(st entity.Student, att []entity.AttendanceRecord, surveys []entity.Survey, homework []entity.HomeworkRecord) (risk.Result, []string) {
	result := computeRisk(st, att, surveys, homework)
	var triggers []string

	// A long trailing-absence run hard-forces Red regardless of the numeric score. The streak
	// already contributes graduated points inside computeRisk (and a labelled factor), so this
	// only needs to guarantee the tier — no duplicate factor/reason.
	if consecutiveTrailingAbsences(att) >= triggerConsecutiveAbsences {
		result.Tier = risk.TierRed
	}
	// st.RiskScore is the previously persisted score (pre-recompute). Only flag a genuine jump
	// into risk territory, not a brand-new student's first non-zero score.
	if delta := result.Score - st.RiskScore; st.RiskScore > 0 && delta >= triggerScoreJump && result.Score >= int(risk.TierThresholdYellow) {
		triggers = append(triggers, fmt.Sprintf("Xavf keskin oshdi (+%d)", delta))
	}
	return result, triggers
}

// consecutiveTrailingAbsences counts the run of most-recent absences (att is date-ascending).
func consecutiveTrailingAbsences(att []entity.AttendanceRecord) int {
	n := 0
	for i := len(att) - 1; i >= 0; i-- {
		if att[i].IsPresent {
			break
		}
		n++
	}
	return n
}

// computeRisk derives the risk Result from confidence + attendance + homework + latest survey.
func computeRisk(st entity.Student, att []entity.AttendanceRecord, surveys []entity.Survey, homework []entity.HomeworkRecord) risk.Result {
	return risk.Evaluate(risk.Inputs{
		AttendanceRate:      completionRate(len(att), countPresent(att)),
		HomeworkRate:        homeworkRate(homework),
		LatestMotivation:    latestMotivation(surveys),
		LatestProgress:      latestProgress(surveys),
		ConfidenceLevel:     st.ConfidenceLevel,
		ConsecutiveAbsences: consecutiveTrailingAbsences(att),
		// HasOverdueDebt is wired in the finance-signal step (recompute needs per-student invoices).
	})
}

func countPresent(att []entity.AttendanceRecord) int {
	n := 0
	for _, a := range att {
		if a.IsPresent {
			n++
		}
	}
	return n
}

// completionRate is done/total as a percentage; an empty history is a neutral 100%.
func completionRate(total, done int) float64 {
	if total == 0 {
		return 100
	}
	return float64(done) / float64(total) * 100
}

// homeworkRate returns the completion % as a pointer, or nil when nothing has been assigned yet
// (so the risk engine treats missing homework as neutral, not as 0%).
func homeworkRate(homework []entity.HomeworkRecord) *float64 {
	if len(homework) == 0 {
		return nil
	}
	done := 0
	for _, h := range homework {
		if h.IsDone {
			done++
		}
	}
	v := float64(done) / float64(len(homework)) * 100
	return &v
}

func latestMotivation(surveys []entity.Survey) *int {
	if latest := latestSurvey(surveys); latest != nil {
		m := latest.MotivationScore
		return &m
	}
	return nil
}

func latestProgress(surveys []entity.Survey) *int {
	if latest := latestSurvey(surveys); latest != nil {
		p := latest.ProgressScore
		return &p
	}
	return nil
}

// dedupeStrings returns in with duplicates removed, preserving first-seen order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func latestSurvey(surveys []entity.Survey) *entity.Survey {
	if len(surveys) == 0 {
		return nil
	}
	latest := surveys[0]
	for i := 1; i < len(surveys); i++ {
		if surveys[i].WeekNumber > latest.WeekNumber {
			latest = surveys[i]
		}
	}
	return &latest
}

// reasonsFrom turns the contributing (non-neutral) risk factors into the task's reasons.
func reasonsFrom(factors []risk.Factor) []string {
	out := make([]string, 0, len(factors))
	for _, f := range factors {
		if f.Points > 0 && !f.Neutral {
			out = append(out, f.Label)
		}
	}
	return out
}

package student

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/risk"
)

// --- fakes ---

type fakeRepo struct {
	lastCreate repo.CreateStudentParams
	getResult  entity.Student
	getErr     error
	addNoteErr error
	noteAdded  bool
}

func (f *fakeRepo) Create(_ context.Context, orgID uuid.UUID, p repo.CreateStudentParams) (entity.Student, error) {
	f.lastCreate = p
	return entity.Student{ID: uuid.New(), OrgID: orgID, Name: p.Name, ConfidenceLevel: p.ConfidenceLevel, RiskScore: p.RiskScore, RiskTier: p.RiskTier}, nil
}
func (f *fakeRepo) Update(_ context.Context, orgID, id uuid.UUID, p repo.UpdateStudentParams) (entity.Student, error) {
	return entity.Student{ID: id, OrgID: orgID, Name: p.Name, ConfidenceLevel: p.ConfidenceLevel, Status: p.Status}, nil
}
func (f *fakeRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error                     { return nil }
func (f *fakeRepo) List(context.Context, uuid.UUID) ([]entity.Student, error) { return nil, nil }
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (entity.Student, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) UpdateRisk(context.Context, uuid.UUID, uuid.UUID, int, string) error { return nil }
func (f *fakeRepo) AddNote(_ context.Context, _, _ uuid.UUID, author, text string) (entity.Note, error) {
	if f.addNoteErr != nil {
		return entity.Note{}, f.addNoteErr
	}
	f.noteAdded = true
	return entity.Note{ID: uuid.New(), Author: author, Text: text}, nil
}
func (f *fakeRepo) ListNotes(context.Context, uuid.UUID, uuid.UUID) ([]entity.Note, error) {
	return nil, nil
}
func (f *fakeRepo) AssignGroup(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) error { return nil }
func (f *fakeRepo) ListByGroup(context.Context, uuid.UUID, uuid.UUID) ([]entity.Student, error) {
	return nil, nil
}
func (f *fakeRepo) SetLoginPassword(context.Context, uuid.UUID, uuid.UUID, string) error { return nil }
func (f *fakeRepo) Profile(context.Context, uuid.UUID, uuid.UUID) (entity.StudentProfile, error) {
	return entity.StudentProfile{}, nil
}

type stubSurveys struct{}

func (stubSurveys) Create(context.Context, uuid.UUID, repo.CreateSurveyParams) (entity.Survey, error) {
	return entity.Survey{}, nil
}
func (stubSurveys) ListByStudent(context.Context, uuid.UUID, uuid.UUID) ([]entity.Survey, error) {
	return nil, nil
}

type stubAttendance struct{}

func (stubAttendance) Create(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, time.Time, bool) (entity.AttendanceRecord, error) {
	return entity.AttendanceRecord{}, nil
}
func (stubAttendance) ListByStudent(context.Context, uuid.UUID, uuid.UUID) ([]entity.AttendanceRecord, error) {
	return nil, nil
}

type stubHomework struct{}

func (stubHomework) Create(context.Context, uuid.UUID, uuid.UUID, time.Time, bool) (entity.HomeworkRecord, error) {
	return entity.HomeworkRecord{}, nil
}
func (stubHomework) ListByStudent(context.Context, uuid.UUID, uuid.UUID) ([]entity.HomeworkRecord, error) {
	return nil, nil
}

type stubRetention struct{}

func (stubRetention) SubmitSurvey(_ context.Context, _, sid uuid.UUID, p repo.CreateSurveyParams, _ repo.Recompute) (entity.Survey, error) {
	return entity.Survey{ID: uuid.New(), StudentID: sid, WeekNumber: p.WeekNumber, MotivationScore: p.MotivationScore, ProgressScore: p.ProgressScore}, nil
}
func (stubRetention) RecordAttendance(_ context.Context, _, _ uuid.UUID, _ *uuid.UUID, date time.Time, status string, _ repo.Recompute) (entity.AttendanceRecord, error) {
	return entity.AttendanceRecord{ID: uuid.New(), Date: date, IsPresent: entity.PresentFromStatus(status), Status: status}, nil
}
func (stubRetention) RecordHomework(_ context.Context, _, _ uuid.UUID, date time.Time, done bool, _ repo.Recompute) (entity.HomeworkRecord, error) {
	return entity.HomeworkRecord{ID: uuid.New(), Date: date, IsDone: done}, nil
}
func (stubRetention) RecordAdviceFeedback(_ context.Context, _, _, _ uuid.UUID, _ bool) error {
	return nil
}
func (stubRetention) RecomputeStudent(_ context.Context, _, _ uuid.UUID, _ repo.Recompute) error {
	return nil
}

func newTestSvc(students repo.StudentRepository) *Service {
	return NewService(students, stubSurveys{}, stubAttendance{}, stubHomework{}, stubRetention{})
}

// --- tests ---

func TestCreate_Validation(t *testing.T) {
	svc := newTestSvc(&fakeRepo{})
	org := uuid.New()
	for i, c := range []CreateParams{
		{Name: "", ConfidenceLevel: 8},
		{Name: "Ali", ConfidenceLevel: 0},
		{Name: "Ali", ConfidenceLevel: 11},
	} {
		if _, err := svc.Create(context.Background(), org, c); !errors.Is(err, ErrValidation) {
			t.Errorf("case %d: want ErrValidation, got %v", i, err)
		}
	}
}

func TestCreate_ComputesRisk(t *testing.T) {
	f := &fakeRepo{}
	svc := newTestSvc(f)
	if _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Ali", ConfidenceLevel: 8}); err != nil {
		t.Fatal(err)
	}
	if f.lastCreate.RiskScore != 15 || f.lastCreate.RiskTier != "Green" {
		t.Fatalf("confidence 8: got %d/%s, want 15/Green", f.lastCreate.RiskScore, f.lastCreate.RiskTier)
	}
	if _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Anvar", ConfidenceLevel: 2}); err != nil {
		t.Fatal(err)
	}
	if f.lastCreate.RiskScore != 25 {
		t.Fatalf("confidence 2: got %d, want 25", f.lastCreate.RiskScore)
	}
}

func TestAddNote(t *testing.T) {
	org, sid := uuid.New(), uuid.New()
	if _, err := newTestSvc(&fakeRepo{}).AddNote(context.Background(), org, sid, "Mentor", "   "); !errors.Is(err, ErrValidation) {
		t.Errorf("empty text: want ErrValidation, got %v", err)
	}
	if _, err := newTestSvc(&fakeRepo{addNoteErr: repo.ErrNotFound}).AddNote(context.Background(), org, sid, "Mentor", "hi"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing student: want ErrNotFound, got %v", err)
	}
	ok := &fakeRepo{}
	if _, err := newTestSvc(ok).AddNote(context.Background(), org, sid, "Mentor", "hi"); err != nil {
		t.Fatal(err)
	}
	if !ok.noteAdded {
		t.Error("expected note to be added")
	}
}

func TestGet_ScoreMatchesFactors(t *testing.T) {
	f := &fakeRepo{getResult: entity.Student{ID: uuid.New(), ConfidenceLevel: 5}}
	d, err := newTestSvc(f).Get(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, fc := range d.Factors {
		sum += fc.Points
	}
	if sum != d.Student.RiskScore {
		t.Fatalf("factor points sum %d != response score %d", sum, d.Student.RiskScore)
	}
}

func TestSubmitSurvey_Validation(t *testing.T) {
	svc := newTestSvc(&fakeRepo{})
	org, sid := uuid.New(), uuid.New()
	for i, p := range []SubmitSurveyParams{
		{WeekNumber: 0, MotivationScore: 3, ProgressScore: 3},
		{WeekNumber: 1, MotivationScore: 6, ProgressScore: 3},
		{WeekNumber: 1, MotivationScore: 3, ProgressScore: 0},
	} {
		if _, err := svc.SubmitSurvey(context.Background(), org, sid, p); !errors.Is(err, ErrValidation) {
			t.Errorf("case %d: want ErrValidation, got %v", i, err)
		}
	}
}

// A low survey on a poorly-attending student → Red, with task reasons.
func TestOutcome_RedHasReasons(t *testing.T) {
	svc := newTestSvc(&fakeRepo{})
	att := []entity.AttendanceRecord{{IsPresent: false}, {IsPresent: false}} // 0% attendance, 2 in a row
	surv := []entity.Survey{{WeekNumber: 1, MotivationScore: 1, ProgressScore: 1}}
	out := svc.outcome(entity.Student{ConfidenceLevel: 5}, att, surv, nil)
	// 40(att) + 25(mot) + 20(prog) + 10(conf) + 15(2 consecutive absences) = 110 → capped at 100.
	if out.Tier != "Red" || out.Score != 100 {
		t.Fatalf("got %d/%s, want 100/Red", out.Score, out.Tier)
	}
	if len(out.TaskReasons) == 0 {
		t.Fatal("expected task reasons for a Red student")
	}
}

func TestReasonsFrom_ExcludesNeutral(t *testing.T) {
	factors := []risk.Factor{
		{Label: "Davomat past", Points: 30},
		{Label: "So'rovnoma topshirilmagan", Points: 10, Neutral: true},
	}
	reasons := reasonsFrom(factors)
	if len(reasons) != 1 || reasons[0] != "Davomat past" {
		t.Fatalf("reasons = %v, want only the non-neutral factor", reasons)
	}
}

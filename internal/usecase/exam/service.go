// Package exam is structured graded assessments per group. Teachers create exams for their own
// groups and record scores; a result awards XP proportional to the score (feeding the rating).
package exam

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("exam: validation failed")
	ErrNotFound   = errors.New("exam: not found")
	ErrForbidden  = errors.New("exam: forbidden")
)

// examXPMax is the XP a perfect exam grants (scaled by score/max). Modest so exams nudge, not
// dominate, the rating.
const examXPMax = 10

// Awarder grants exam XP (satisfied by the points service). Optional — nil disables gamification.
type Awarder interface {
	AwardExam(ctx context.Context, orgID, studentID, examID uuid.UUID, xp int) error
}

type Service struct {
	repo    repo.ExamRepository
	groups  repo.GroupRepository
	awarder Awarder
}

func NewService(r repo.ExamRepository, g repo.GroupRepository) *Service {
	return &Service{repo: r, groups: g}
}

// SetAwarder wires gamification (call once at startup).
func (s *Service) SetAwarder(a Awarder) { s.awarder = a }

// canManageGroup lets admins/managers touch any group; a teacher only their own.
func (s *Service) canManageGroup(ctx context.Context, orgID, groupID uuid.UUID, actor entity.Principal) error {
	if actor.Role != entity.RoleTeacher {
		return nil
	}
	g, err := s.groups.GetByID(ctx, orgID, groupID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if g.TeacherID == nil || *g.TeacherID != actor.UserID {
		return ErrForbidden
	}
	return nil
}

func (s *Service) CreateExam(ctx context.Context, actor entity.Principal, groupID uuid.UUID, title string, date *time.Time, maxScore int) (entity.Exam, error) {
	title = strings.TrimSpace(title)
	if groupID == uuid.Nil || title == "" {
		return entity.Exam{}, ErrValidation
	}
	if maxScore <= 0 {
		maxScore = 100
	}
	if err := s.canManageGroup(ctx, actor.OrgID, groupID, actor); err != nil {
		return entity.Exam{}, err
	}
	return s.repo.Create(ctx, actor.OrgID, groupID, title, date, maxScore)
}

func (s *Service) ListGroupExams(ctx context.Context, actor entity.Principal, groupID uuid.UUID) ([]entity.Exam, error) {
	if err := s.canManageGroup(ctx, actor.OrgID, groupID, actor); err != nil {
		return nil, err
	}
	return s.repo.ListGroupExams(ctx, actor.OrgID, groupID)
}

func (s *Service) DeleteExam(ctx context.Context, actor entity.Principal, id uuid.UUID) error {
	ex, err := s.repo.GetExam(ctx, actor.OrgID, id)
	if err != nil {
		return err
	}
	if err := s.canManageGroup(ctx, actor.OrgID, ex.GroupID, actor); err != nil {
		return err
	}
	return s.repo.DeleteExam(ctx, actor.OrgID, id)
}

// Results returns an exam plus every student's score (for grading), owner-checked.
func (s *Service) Results(ctx context.Context, actor entity.Principal, examID uuid.UUID) (entity.Exam, []entity.ExamResult, error) {
	ex, err := s.repo.GetExam(ctx, actor.OrgID, examID)
	if err != nil {
		return entity.Exam{}, nil, err
	}
	if err := s.canManageGroup(ctx, actor.OrgID, ex.GroupID, actor); err != nil {
		return entity.Exam{}, nil, err
	}
	results, err := s.repo.ListResults(ctx, actor.OrgID, examID)
	return ex, results, err
}

// Grade records one student's score (clamped to the exam max) and awards proportional XP.
func (s *Service) Grade(ctx context.Context, actor entity.Principal, examID, studentID uuid.UUID, score int) (entity.ExamResult, error) {
	ex, err := s.repo.GetExam(ctx, actor.OrgID, examID)
	if err != nil {
		return entity.ExamResult{}, err
	}
	if err := s.canManageGroup(ctx, actor.OrgID, ex.GroupID, actor); err != nil {
		return entity.ExamResult{}, err
	}
	if score < 0 {
		score = 0
	}
	if score > ex.MaxScore {
		score = ex.MaxScore
	}
	res, err := s.repo.UpsertResult(ctx, actor.OrgID, examID, studentID, score)
	if err != nil {
		return entity.ExamResult{}, err
	}
	if s.awarder != nil && ex.MaxScore > 0 {
		xp := int(math.Round(float64(score) / float64(ex.MaxScore) * examXPMax))
		if e := s.awarder.AwardExam(ctx, actor.OrgID, studentID, examID, xp); e != nil {
			// Non-fatal: the score is recorded; XP is a bonus.
			_ = e
		}
	}
	return res, nil
}

// StudentResults is a student's own exam history (no ownership check — self only, from the token).
func (s *Service) StudentResults(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentExamResult, error) {
	return s.repo.StudentResults(ctx, orgID, studentID)
}

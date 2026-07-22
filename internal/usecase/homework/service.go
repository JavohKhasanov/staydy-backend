// Package homework is the LMS-style homework flow: a teacher/admin attaches an assignment to a
// group, students submit text + links, and the teacher grades (status + score). Teachers are scoped
// to their own groups; students to assignments in groups they belong to.
package homework

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("homework: validation failed")
	ErrForbidden  = errors.New("homework: not allowed")
	ErrNotFound   = errors.New("homework: not found")
)

type Service struct {
	repo   repo.AssignmentRepository
	groups repo.GroupRepository
}

func NewService(r repo.AssignmentRepository, g repo.GroupRepository) *Service {
	return &Service{repo: r, groups: g}
}

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

type CreateInput struct {
	GroupID     uuid.UUID
	Title       string
	Description string
	Deadline    *time.Time
	LessonDate  *time.Time
	MaxScore    int
}

func (s *Service) CreateAssignment(ctx context.Context, actor entity.Principal, in CreateInput) (entity.HomeworkAssignment, error) {
	title := strings.TrimSpace(in.Title)
	if in.GroupID == uuid.Nil || title == "" {
		return entity.HomeworkAssignment{}, ErrValidation
	}
	if in.MaxScore <= 0 {
		in.MaxScore = 100
	}
	if err := s.canManageGroup(ctx, actor.OrgID, in.GroupID, actor); err != nil {
		return entity.HomeworkAssignment{}, err
	}
	return s.repo.CreateAssignment(ctx, actor.OrgID, repo.CreateAssignmentParams{
		GroupID: in.GroupID, LessonDate: in.LessonDate, Title: title,
		Description: strings.TrimSpace(in.Description), Deadline: in.Deadline, MaxScore: in.MaxScore,
	})
}

func (s *Service) ListGroupAssignments(ctx context.Context, actor entity.Principal, groupID uuid.UUID) ([]entity.HomeworkAssignment, error) {
	if err := s.canManageGroup(ctx, actor.OrgID, groupID, actor); err != nil {
		return nil, err
	}
	return s.repo.ListGroupAssignments(ctx, actor.OrgID, groupID)
}

func (s *Service) DeleteAssignment(ctx context.Context, actor entity.Principal, id uuid.UUID) error {
	a, err := s.repo.GetAssignment(ctx, actor.OrgID, id)
	if err != nil {
		return err
	}
	if err := s.canManageGroup(ctx, actor.OrgID, a.GroupID, actor); err != nil {
		return err
	}
	return s.repo.DeleteAssignment(ctx, actor.OrgID, id)
}

func (s *Service) ListSubmissions(ctx context.Context, actor entity.Principal, assignmentID uuid.UUID) (entity.HomeworkAssignment, []entity.HomeworkSubmission, error) {
	a, err := s.repo.GetAssignment(ctx, actor.OrgID, assignmentID)
	if err != nil {
		return entity.HomeworkAssignment{}, nil, err
	}
	if err := s.canManageGroup(ctx, actor.OrgID, a.GroupID, actor); err != nil {
		return entity.HomeworkAssignment{}, nil, err
	}
	subs, err := s.repo.ListSubmissions(ctx, actor.OrgID, assignmentID)
	return a, subs, err
}

func (s *Service) Grade(ctx context.Context, actor entity.Principal, submissionID uuid.UUID, status string, score *int, note string) (entity.HomeworkSubmission, error) {
	if !entity.ValidHomeworkStatus(status) {
		return entity.HomeworkSubmission{}, ErrValidation
	}
	if score != nil && *score < 0 {
		return entity.HomeworkSubmission{}, ErrValidation
	}
	// A rejected submission carries no score.
	if status == entity.HomeworkRejected {
		score = nil
	}
	return s.repo.Grade(ctx, actor.OrgID, submissionID, status, score, strings.TrimSpace(note))
}

// --- student side ---

func (s *Service) StudentAssignments(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentAssignment, error) {
	return s.repo.ListStudentAssignments(ctx, orgID, studentID)
}

// Submit records (or replaces) a student's answer. It first verifies the student is a member of the
// assignment's group.
func (s *Service) Submit(ctx context.Context, orgID, studentID, assignmentID uuid.UUID, text, links string) (entity.HomeworkSubmission, error) {
	text = strings.TrimSpace(text)
	links = strings.TrimSpace(links)
	if text == "" && links == "" {
		return entity.HomeworkSubmission{}, ErrValidation
	}
	if _, err := s.repo.GetAssignmentForStudent(ctx, orgID, assignmentID, studentID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.HomeworkSubmission{}, ErrForbidden
		}
		return entity.HomeworkSubmission{}, err
	}
	return s.repo.UpsertSubmission(ctx, orgID, assignmentID, studentID, text, links)
}

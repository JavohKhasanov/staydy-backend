// Package group is the business logic for student groups (cohorts) and student↔group
// assignment. Group/teacher management is a center_admin concern; teacher-scoped reads
// (a teacher's own groups/students) are exposed through the teacher-facing handlers.
package group

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid input (empty name, or a teacher_id that isn't a teacher).
var ErrValidation = errors.New("group: validation failed")

type Service struct {
	groups   repo.GroupRepository
	teachers repo.TeacherRepository
	students repo.StudentRepository
}

func NewService(groups repo.GroupRepository, teachers repo.TeacherRepository, students repo.StudentRepository) *Service {
	return &Service{groups: groups, teachers: teachers, students: students}
}

type CreateInput struct {
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

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (entity.Group, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.Group{}, ErrValidation
	}
	if err := s.validateTeacher(ctx, orgID, in.TeacherID); err != nil {
		return entity.Group{}, err
	}
	return s.groups.Create(ctx, orgID, repo.CreateGroupParams{
		Name:         name,
		TeacherID:    in.TeacherID,
		CourseID:     in.CourseID,
		BranchID:     in.BranchID,
		Direction:    strings.TrimSpace(in.Direction),
		ScheduleDays: strings.TrimSpace(in.ScheduleDays),
		Capacity:     in.Capacity,
		StartTime:    strings.TrimSpace(in.StartTime),
		EndTime:      strings.TrimSpace(in.EndTime),
		RoomID:       in.RoomID,
	})
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.Group, error) {
	return s.groups.List(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (entity.Group, error) {
	return s.groups.GetByID(ctx, orgID, id)
}

// Students lists the members of a group (404 if the group isn't in this tenant).
func (s *Service) Students(ctx context.Context, orgID, id uuid.UUID) ([]entity.Student, error) {
	if _, err := s.groups.GetByID(ctx, orgID, id); err != nil {
		return nil, err
	}
	return s.students.ListByGroup(ctx, orgID, id)
}

type UpdateInput struct {
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

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (entity.Group, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.Group{}, ErrValidation
	}
	if err := s.validateTeacher(ctx, orgID, in.TeacherID); err != nil {
		return entity.Group{}, err
	}
	return s.groups.Update(ctx, orgID, id, repo.UpdateGroupParams{
		Name:         name,
		TeacherID:    in.TeacherID,
		CourseID:     in.CourseID,
		BranchID:     in.BranchID,
		Direction:    strings.TrimSpace(in.Direction),
		ScheduleDays: strings.TrimSpace(in.ScheduleDays),
		Capacity:     in.Capacity,
		StartTime:    strings.TrimSpace(in.StartTime),
		EndTime:      strings.TrimSpace(in.EndTime),
		RoomID:       in.RoomID,
	})
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	if _, err := s.groups.GetByID(ctx, orgID, id); err != nil {
		return err
	}
	return s.groups.Delete(ctx, orgID, id)
}

// AssignStudent puts a student into a group (or removes them when groupID is nil). Both the
// student and the group must belong to the caller's tenant.
func (s *Service) AssignStudent(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID) error {
	if _, err := s.students.GetByID(ctx, orgID, studentID); err != nil {
		return err
	}
	if groupID != nil {
		if _, err := s.groups.GetByID(ctx, orgID, *groupID); err != nil {
			return err
		}
	}
	return s.students.AssignGroup(ctx, orgID, studentID, groupID)
}

// MyGroups returns the groups owned by a teacher (their own dashboard).
func (s *Service) MyGroups(ctx context.Context, orgID, teacherID uuid.UUID) ([]entity.Group, error) {
	return s.groups.ListByTeacher(ctx, orgID, teacherID)
}

// MyStudents returns all students across a teacher's groups, highest risk first.
func (s *Service) MyStudents(ctx context.Context, orgID, teacherID uuid.UUID) ([]entity.Student, error) {
	groups, err := s.groups.ListByTeacher(ctx, orgID, teacherID)
	if err != nil {
		return nil, err
	}
	out := make([]entity.Student, 0)
	for _, g := range groups {
		sts, err := s.students.ListByGroup(ctx, orgID, g.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, sts...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RiskScore > out[j].RiskScore })
	return out, nil
}

// OwnsStudent reports whether a student belongs to one of the teacher's own groups. Used to gate
// a teacher's attendance/homework marking to their own students.
func (s *Service) OwnsStudent(ctx context.Context, orgID, teacherID, studentID uuid.UUID) (bool, error) {
	st, err := s.students.GetByID(ctx, orgID, studentID)
	if err != nil {
		return false, err
	}
	if st.GroupID == nil {
		return false, nil
	}
	g, err := s.groups.GetByID(ctx, orgID, *st.GroupID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return g.TeacherID != nil && *g.TeacherID == teacherID, nil
}

// validateTeacher ensures an assigned teacher_id (if any) is a teacher in this tenant.
func (s *Service) validateTeacher(ctx context.Context, orgID uuid.UUID, teacherID *uuid.UUID) error {
	if teacherID == nil {
		return nil
	}
	u, err := s.teachers.GetByID(ctx, orgID, *teacherID)
	if err != nil {
		return err // repo.ErrNotFound when the user isn't in this org
	}
	if u.Role != entity.RoleTeacher {
		return ErrValidation
	}
	return nil
}

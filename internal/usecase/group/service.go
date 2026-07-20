// Package group is the business logic for student groups (cohorts) and student↔group
// assignment. Group/teacher management is a center_admin concern; teacher-scoped reads
// (a teacher's own groups/students) are exposed through the teacher-facing handlers.
package group

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

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

// MissingAttendance lists today's (or the given date's) sessions with no attendance yet —
// the admin's "dars bo'ldi, davomat yo'q" alert. The weekday code is derived from the date.
func (s *Service) MissingAttendance(ctx context.Context, orgID uuid.UUID, date time.Time) ([]entity.Group, error) {
	dayCode := strings.ToLower(date.Weekday().String()[:3]) // "mon".."sun"
	return s.groups.MissingAttendance(ctx, orgID, dayCode, date)
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
	if err := s.students.AssignGroup(ctx, orgID, studentID, groupID); err != nil {
		return err
	}
	// Primary assignment also creates a membership (multi-group source of truth).
	if groupID != nil {
		return s.groups.AddMember(ctx, orgID, *groupID, studentID)
	}
	return nil
}

// AddMember puts a student into a group WITHOUT touching their primary group — a student can
// study in several groups (courses) at once. Sets the primary when they had none.
func (s *Service) AddMember(ctx context.Context, orgID, groupID, studentID uuid.UUID) error {
	st, err := s.students.GetByID(ctx, orgID, studentID)
	if err != nil {
		return err
	}
	if _, err := s.groups.GetByID(ctx, orgID, groupID); err != nil {
		return err
	}
	if err := s.groups.AddMember(ctx, orgID, groupID, studentID); err != nil {
		return err
	}
	if st.GroupID == nil {
		gid := groupID
		return s.students.AssignGroup(ctx, orgID, studentID, &gid)
	}
	return nil
}

// RemoveMember takes a student out of one group. If it was their primary group, the primary
// moves to another remaining membership (or clears).
func (s *Service) RemoveMember(ctx context.Context, orgID, groupID, studentID uuid.UUID) error {
	st, err := s.students.GetByID(ctx, orgID, studentID)
	if err != nil {
		return err
	}
	if err := s.groups.RemoveMember(ctx, orgID, groupID, studentID); err != nil {
		return err
	}
	if st.GroupID != nil && *st.GroupID == groupID {
		rest, lerr := s.groups.ListForStudent(ctx, orgID, studentID)
		if lerr != nil {
			return lerr
		}
		var next *uuid.UUID
		if len(rest) > 0 {
			next = &rest[0].ID
		}
		return s.students.AssignGroup(ctx, orgID, studentID, next)
	}
	return nil
}

// StudentGroups lists every group a student is a member of.
func (s *Service) StudentGroups(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Group, error) {
	return s.groups.ListForStudent(ctx, orgID, studentID)
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
	groups, err := s.groups.ListForStudent(ctx, orgID, studentID)
	if err != nil {
		return false, err
	}
	for _, g := range groups {
		if g.TeacherID != nil && *g.TeacherID == teacherID {
			return true, nil
		}
	}
	return false, nil
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

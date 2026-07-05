// Package course is the business logic for a center's sellable courses/programs. Management is a
// center_admin concern; groups, enrollments, and fees reference courses.
package course

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid input (empty/over-long name, or a negative price/duration).
var ErrValidation = errors.New("course: validation failed")

const maxNameLen = 120

type Service struct {
	repo repo.CourseRepository
}

func NewService(r repo.CourseRepository) *Service { return &Service{repo: r} }

type Input struct {
	Name          string
	Level         string
	Price         int64
	DurationWeeks int
	Description   string
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.Course, error) {
	return s.repo.List(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (entity.Course, error) {
	return s.repo.Get(ctx, orgID, id)
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in Input) (entity.Course, error) {
	in, err := s.normalize(in)
	if err != nil {
		return entity.Course{}, err
	}
	return s.repo.Create(ctx, orgID, repo.CreateCourseParams{
		Name:          in.Name,
		Level:         in.Level,
		Price:         in.Price,
		DurationWeeks: in.DurationWeeks,
		Description:   in.Description,
	})
}

// Update validates input and writes; isActive toggles archive/active. Caller passes the desired
// isActive value so a single endpoint covers edit + archive/restore.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in Input, isActive bool) (entity.Course, error) {
	in, err := s.normalize(in)
	if err != nil {
		return entity.Course{}, err
	}
	return s.repo.Update(ctx, orgID, id, repo.UpdateCourseParams{
		Name:          in.Name,
		Level:         in.Level,
		Price:         in.Price,
		DurationWeeks: in.DurationWeeks,
		Description:   in.Description,
		IsActive:      isActive,
	})
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

func (s *Service) normalize(in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Level = strings.TrimSpace(in.Level)
	in.Description = strings.TrimSpace(in.Description)
	if in.Name == "" || len([]rune(in.Name)) > maxNameLen {
		return Input{}, ErrValidation
	}
	if in.Price < 0 || in.DurationWeeks < 0 {
		return Input{}, ErrValidation
	}
	return in, nil
}

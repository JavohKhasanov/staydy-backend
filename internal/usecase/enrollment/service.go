// Package enrollment is the business logic for student enrolments in groups/courses. Management is
// a center_admin concern. It is additive: students.group_id and the risk engine are untouched.
package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid input (bad status, negative price, or discount out of 0..100).
var ErrValidation = errors.New("enrollment: validation failed")

type Service struct {
	repo repo.EnrollmentRepository
}

func NewService(r repo.EnrollmentRepository) *Service { return &Service{repo: r} }

type Input struct {
	GroupID   *uuid.UUID
	CourseID  *uuid.UUID
	Status    string
	StartDate *time.Time
	EndDate   *time.Time
	Price     int64
	Discount  int
}

func (s *Service) ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Enrollment, error) {
	return s.repo.ListByStudent(ctx, orgID, studentID)
}

func (s *Service) Create(ctx context.Context, orgID, studentID uuid.UUID, in Input) (entity.Enrollment, error) {
	in, err := s.normalize(in)
	if err != nil {
		return entity.Enrollment{}, err
	}
	return s.repo.Create(ctx, orgID, repo.CreateEnrollmentParams{
		StudentID: studentID,
		GroupID:   in.GroupID,
		CourseID:  in.CourseID,
		Status:    in.Status,
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
		Price:     in.Price,
		Discount:  in.Discount,
	})
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in Input) (entity.Enrollment, error) {
	in, err := s.normalize(in)
	if err != nil {
		return entity.Enrollment{}, err
	}
	return s.repo.Update(ctx, orgID, id, repo.UpdateEnrollmentParams{
		GroupID:   in.GroupID,
		CourseID:  in.CourseID,
		Status:    in.Status,
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
		Price:     in.Price,
		Discount:  in.Discount,
	})
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

func (s *Service) normalize(in Input) (Input, error) {
	if in.Status == "" {
		in.Status = "active"
	}
	if !entity.EnrollmentStatuses[in.Status] {
		return Input{}, ErrValidation
	}
	if in.Price < 0 || in.Discount < 0 || in.Discount > 100 {
		return Input{}, ErrValidation
	}
	return in, nil
}

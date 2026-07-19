// Package teacher is the business logic for teacher (ustoz) accounts — users with role
// 'teacher', created by a center_admin. They log into the same web app (role-gated views)
// and will later drive the teacher Telegram dashboard.
package teacher

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/security"
)

// ErrValidation signals invalid input (bad email, short password, or empty name).
var ErrValidation = errors.New("teacher: validation failed")

type Service struct {
	teachers repo.TeacherRepository
}

func NewService(teachers repo.TeacherRepository) *Service { return &Service{teachers: teachers} }

type CreateInput struct {
	Email    string
	Password string
	FullName string
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (entity.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fullName := strings.TrimSpace(in.FullName)
	if !strings.Contains(email, "@") || len(email) < 3 || fullName == "" || len(in.Password) < 8 {
		return entity.User{}, ErrValidation
	}
	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return entity.User{}, err
	}
	return s.teachers.Create(ctx, orgID, email, hash, fullName)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.User, error) {
	return s.teachers.List(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (entity.User, error) {
	return s.teachers.GetByID(ctx, orgID, id)
}

type UpdateInput struct {
	Email    string
	FullName string
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (entity.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fullName := strings.TrimSpace(in.FullName)
	if !strings.Contains(email, "@") || len(email) < 3 || fullName == "" {
		return entity.User{}, ErrValidation
	}
	return s.teachers.Update(ctx, orgID, id, email, fullName)
}

// SetPassword lets a center_admin reset a teacher's login password.
func (s *Service) SetPassword(ctx context.Context, orgID, id uuid.UUID, password string) error {
	if len(password) < 8 {
		return ErrValidation
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	return s.teachers.SetPassword(ctx, orgID, id, hash)
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.teachers.Delete(ctx, orgID, id)
}

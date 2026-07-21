// Package staff is the business logic for back-office staff accounts — users with role
// center_admin (administrator) or finance (moliya), created and managed by a center_admin. Teachers
// have their own path; the platform owner (super_admin) is out of scope here.
package staff

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/security"
)

var (
	ErrValidation = errors.New("staff: validation failed")
	// ErrLastAdmin blocks removing/downgrading the center's only administrator (lockout guard).
	ErrLastAdmin = errors.New("staff: cannot remove the last administrator")
)

type Service struct {
	staff repo.StaffRepository
}

func NewService(staff repo.StaffRepository) *Service { return &Service{staff: staff} }

func valid(email, fullName, role string) bool {
	return strings.Contains(email, "@") && len(email) >= 3 && fullName != "" &&
		entity.ValidStaffRole(entity.UserRole(role))
}

type CreateInput struct {
	Email    string
	Password string
	FullName string
	Role     string
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (entity.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fullName := strings.TrimSpace(in.FullName)
	if !valid(email, fullName, in.Role) || len(in.Password) < 8 {
		return entity.User{}, ErrValidation
	}
	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return entity.User{}, err
	}
	return s.staff.Create(ctx, orgID, email, hash, fullName, in.Role)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.User, error) {
	return s.staff.List(ctx, orgID)
}

type UpdateInput struct {
	Email    string
	FullName string
	Role     string
}

// Update edits a staff member. Downgrading the last administrator away from center_admin is blocked.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (entity.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fullName := strings.TrimSpace(in.FullName)
	if !valid(email, fullName, in.Role) {
		return entity.User{}, ErrValidation
	}
	if in.Role != string(entity.RoleCenterAdmin) {
		if err := s.guardLastAdmin(ctx, orgID, id); err != nil {
			return entity.User{}, err
		}
	}
	return s.staff.Update(ctx, orgID, id, email, fullName, in.Role)
}

func (s *Service) SetPassword(ctx context.Context, orgID, id uuid.UUID, password string) error {
	if len(password) < 8 {
		return ErrValidation
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	return s.staff.SetPassword(ctx, orgID, id, hash)
}

// Delete removes a staff account. Removing the center's last administrator is blocked so the tenant
// can never be locked out.
func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	if err := s.guardLastAdmin(ctx, orgID, id); err != nil {
		return err
	}
	return s.staff.Delete(ctx, orgID, id)
}

// guardLastAdmin returns ErrLastAdmin if id is a center_admin and it's the only one left.
func (s *Service) guardLastAdmin(ctx context.Context, orgID, id uuid.UUID) error {
	list, err := s.staff.List(ctx, orgID)
	if err != nil {
		return err
	}
	admins := 0
	targetIsAdmin := false
	for _, u := range list {
		if u.Role == entity.RoleCenterAdmin {
			admins++
			if u.ID == id {
				targetIsAdmin = true
			}
		}
	}
	if targetIsAdmin && admins <= 1 {
		return ErrLastAdmin
	}
	return nil
}

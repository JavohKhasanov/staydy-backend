// Package branch is the multi-branch (filiallar) business logic: CRUD of the physical locations of
// one education center. center_admin concern. Records reference a branch via a nullable branch_id.
package branch

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("branch: validation failed")
	ErrNotFound   = errors.New("branch: not found")
)

type Service struct {
	repo repo.BranchRepository
}

func NewService(r repo.BranchRepository) *Service { return &Service{repo: r} }

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.Branch, error) {
	return s.repo.List(ctx, orgID)
}

type Input struct {
	Name     string
	Address  string
	Phone    string
	IsActive bool
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in Input) (entity.Branch, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.Branch{}, ErrValidation
	}
	return s.repo.Create(ctx, orgID, repo.BranchParams{
		Name:    name,
		Address: strings.TrimSpace(in.Address),
		Phone:   strings.TrimSpace(in.Phone),
	})
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in Input) (entity.Branch, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return entity.Branch{}, ErrValidation
	}
	b, err := s.repo.Update(ctx, orgID, id, repo.BranchParams{
		Name:     name,
		Address:  strings.TrimSpace(in.Address),
		Phone:    strings.TrimSpace(in.Phone),
		IsActive: in.IsActive,
	})
	if errors.Is(err, repo.ErrNotFound) {
		return entity.Branch{}, ErrNotFound
	}
	return b, err
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

// Package room is CRUD of physical rooms (xonalar). Lessons reference a room; the lesson usecase
// enforces no double-booking. center_admin concern.
package room

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("room: validation failed")
	ErrNotFound   = errors.New("room: not found")
)

type Service struct {
	repo repo.RoomRepository
}

func NewService(r repo.RoomRepository) *Service { return &Service{repo: r} }

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.Room, error) {
	return s.repo.List(ctx, orgID)
}

type Input struct {
	BranchID *uuid.UUID
	Name     string
	Capacity int
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in Input) (entity.Room, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || in.Capacity < 0 {
		return entity.Room{}, ErrValidation
	}
	return s.repo.Create(ctx, orgID, repo.RoomParams{BranchID: in.BranchID, Name: name, Capacity: in.Capacity})
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in Input) (entity.Room, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || in.Capacity < 0 {
		return entity.Room{}, ErrValidation
	}
	room, err := s.repo.Update(ctx, orgID, id, repo.RoomParams{BranchID: in.BranchID, Name: name, Capacity: in.Capacity})
	if errors.Is(err, repo.ErrNotFound) {
		return entity.Room{}, ErrNotFound
	}
	return room, err
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

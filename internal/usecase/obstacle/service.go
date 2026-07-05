// Package obstacle is the business logic for a center's configurable "biggest obstacle" choices
// (the weekly check-in's domain-specific question). Management is a center_admin concern; the bot
// reads the active set through the bot repository.
package obstacle

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid input (empty or over-long label).
var ErrValidation = errors.New("obstacle: validation failed")

const maxLabelLen = 60

type Service struct {
	repo repo.ObstacleRepository
}

func NewService(r repo.ObstacleRepository) *Service { return &Service{repo: r} }

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.ObstacleOption, error) {
	return s.repo.List(ctx, orgID)
}

// Create appends an option. position is taken from the current count so new options sort last.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, label string) (entity.ObstacleOption, error) {
	label = strings.TrimSpace(label)
	if label == "" || len([]rune(label)) > maxLabelLen {
		return entity.ObstacleOption{}, ErrValidation
	}
	existing, err := s.repo.List(ctx, orgID)
	if err != nil {
		return entity.ObstacleOption{}, err
	}
	return s.repo.Create(ctx, orgID, label, len(existing))
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

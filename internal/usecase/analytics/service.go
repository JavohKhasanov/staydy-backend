// Package analytics computes director-facing retention analytics (retention rate, risk mix, cohort
// retention, intervention effectiveness) — the EWS payoff view.
package analytics

import (
	"context"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
)

// Repository is the slice of persistence analytics needs.
type Repository interface {
	Retention(ctx context.Context, orgID uuid.UUID) (entity.RetentionStats, error)
}

type Service struct {
	repo Repository
}

func NewService(r Repository) *Service {
	return &Service{repo: r}
}

// Retention returns the org's retention picture with derived rates filled in.
func (s *Service) Retention(ctx context.Context, orgID uuid.UUID) (entity.RetentionStats, error) {
	stats, err := s.repo.Retention(ctx, orgID)
	if err != nil {
		return entity.RetentionStats{}, err
	}
	stats.ComputeRetention()
	return stats, nil
}

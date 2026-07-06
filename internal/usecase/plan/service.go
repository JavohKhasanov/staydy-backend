// Package plan manages the landing-page pricing plans. The public site renders the active plans;
// the super_admin edits prices/features from the platform panel. No RLS — this is platform data.
package plan

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("plan: validation failed")
	ErrNotFound   = errors.New("plan: not found")
)

type Service struct {
	plans repo.PlanRepository
}

func NewService(plans repo.PlanRepository) *Service { return &Service{plans: plans} }

// ListPublic returns the active plans shown on the landing page.
func (s *Service) ListPublic(ctx context.Context) ([]entity.Plan, error) {
	return s.plans.ListActive(ctx)
}

// ListAll returns every plan (active + hidden) for the super_admin editor.
func (s *Service) ListAll(ctx context.Context) ([]entity.Plan, error) {
	return s.plans.ListAll(ctx)
}

// Input carries the editable fields of a plan.
type Input struct {
	PlanKey     string
	Name        string
	Price       string
	Period      string
	Tagline     string
	Features    []string
	Highlighted bool
	SortOrder   int
	IsActive    bool
}

func (in Input) normalize() (entity.Plan, error) {
	p := entity.Plan{
		PlanKey:     strings.TrimSpace(in.PlanKey),
		Name:        strings.TrimSpace(in.Name),
		Price:       strings.TrimSpace(in.Price),
		Period:      strings.TrimSpace(in.Period),
		Tagline:     strings.TrimSpace(in.Tagline),
		Highlighted: in.Highlighted,
		SortOrder:   in.SortOrder,
		IsActive:    in.IsActive,
	}
	// Drop blank feature lines so the landing checklist stays clean.
	p.Features = make([]string, 0, len(in.Features))
	for _, f := range in.Features {
		if f = strings.TrimSpace(f); f != "" {
			p.Features = append(p.Features, f)
		}
	}
	if p.Name == "" {
		return entity.Plan{}, ErrValidation
	}
	return p, nil
}

func (s *Service) Create(ctx context.Context, in Input) (entity.Plan, error) {
	p, err := in.normalize()
	if err != nil {
		return entity.Plan{}, err
	}
	return s.plans.Create(ctx, p)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (entity.Plan, error) {
	p, err := in.normalize()
	if err != nil {
		return entity.Plan{}, err
	}
	p.ID = id
	updated, err := s.plans.Update(ctx, p)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.Plan{}, ErrNotFound
		}
		return entity.Plan{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.plans.Delete(ctx, id)
}

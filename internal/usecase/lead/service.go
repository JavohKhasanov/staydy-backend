// Package lead is the business logic for the sales funnel (prospective students). Management is a
// center_admin concern. Conversion (lead → student) is orchestrated by the HTTP layer, which holds
// both the lead and student services.
package lead

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid input (empty name or unknown stage).
var ErrValidation = errors.New("lead: validation failed")

type Service struct {
	repo repo.LeadRepository
}

func NewService(r repo.LeadRepository) *Service { return &Service{repo: r} }

type Input struct {
	Name       string
	Phone      string
	Email      string
	Source     string
	Stage      string
	Interest   string
	Note       string
	AssignedTo *uuid.UUID
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.Lead, error) {
	return s.repo.List(ctx, orgID)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (entity.Lead, error) {
	return s.repo.Get(ctx, orgID, id)
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in Input) (entity.Lead, error) {
	p, err := s.normalize(in)
	if err != nil {
		return entity.Lead{}, err
	}
	return s.repo.Create(ctx, orgID, p)
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in Input) (entity.Lead, error) {
	p, err := s.normalize(in)
	if err != nil {
		return entity.Lead{}, err
	}
	return s.repo.Update(ctx, orgID, id, p)
}

// SetStage moves a lead along the pipeline (kanban drag / dropdown).
func (s *Service) SetStage(ctx context.Context, orgID, id uuid.UUID, stage string) (entity.Lead, error) {
	stage = strings.TrimSpace(stage)
	if !entity.LeadStages[stage] {
		return entity.Lead{}, ErrValidation
	}
	return s.repo.SetStage(ctx, orgID, id, stage)
}

// MarkConverted is called after the HTTP layer spawns a student from the lead.
func (s *Service) MarkConverted(ctx context.Context, orgID, id, studentID uuid.UUID) error {
	return s.repo.MarkConverted(ctx, orgID, id, studentID)
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

func (s *Service) normalize(in Input) (repo.LeadParams, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return repo.LeadParams{}, ErrValidation
	}
	stage := strings.TrimSpace(in.Stage)
	if stage == "" {
		stage = "new"
	}
	if !entity.LeadStages[stage] {
		return repo.LeadParams{}, ErrValidation
	}
	return repo.LeadParams{
		Name:       name,
		Phone:      strings.TrimSpace(in.Phone),
		Email:      strings.TrimSpace(in.Email),
		Source:     strings.TrimSpace(in.Source),
		Stage:      stage,
		Interest:   strings.TrimSpace(in.Interest),
		Note:       strings.TrimSpace(in.Note),
		AssignedTo: in.AssignedTo,
	}, nil
}

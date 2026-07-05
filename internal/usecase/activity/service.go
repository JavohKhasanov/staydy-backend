// Package activity is the business logic for the polymorphic communication timeline (calls, SMS,
// notes, meetings) attached to leads or students. center_admin concern.
package activity

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid input (unknown subject/type or empty body).
var ErrValidation = errors.New("activity: validation failed")

type Service struct {
	repo repo.ActivityRepository
}

func NewService(r repo.ActivityRepository) *Service { return &Service{repo: r} }

func (s *Service) List(ctx context.Context, orgID uuid.UUID, subjectType string, subjectID uuid.UUID) ([]entity.Activity, error) {
	if !entity.ActivitySubjects[subjectType] {
		return nil, ErrValidation
	}
	return s.repo.List(ctx, orgID, subjectType, subjectID)
}

type Input struct {
	SubjectType string
	SubjectID   uuid.UUID
	Type        string
	Body        string
	Author      string
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in Input) (entity.Activity, error) {
	if !entity.ActivitySubjects[in.SubjectType] {
		return entity.Activity{}, ErrValidation
	}
	t := strings.TrimSpace(in.Type)
	if t == "" {
		t = "note"
	}
	if !entity.ActivityTypes[t] {
		return entity.Activity{}, ErrValidation
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return entity.Activity{}, ErrValidation
	}
	return s.repo.Create(ctx, orgID, repo.CreateActivityParams{
		SubjectType: in.SubjectType,
		SubjectID:   in.SubjectID,
		Type:        t,
		Body:        body,
		Author:      in.Author,
	})
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

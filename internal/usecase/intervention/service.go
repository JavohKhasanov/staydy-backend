// Package intervention is the intervention-task usecase: list the kanban and resolve a
// task. Tasks are auto-opened by the student usecase when a student turns Red.
package intervention

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("validation failed")
	ErrNotFound   = errors.New("intervention task not found")
)

type Service struct {
	tasks repo.InterventionRepository
}

func NewService(tasks repo.InterventionRepository) *Service { return &Service{tasks: tasks} }

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]entity.InterventionTask, error) {
	return s.tasks.List(ctx, orgID)
}

// Resolve closes an open task with a mentor's resolution note.
// Start moves an open task to In Progress ("kim shug'ullanyapti" marker on the Kanban).
func (s *Service) Start(ctx context.Context, orgID, id uuid.UUID) (entity.InterventionTask, error) {
	return s.tasks.Start(ctx, orgID, id)
}

func (s *Service) Resolve(ctx context.Context, orgID, id uuid.UUID, comment string) (entity.InterventionTask, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return entity.InterventionTask{}, fmt.Errorf("%w: resolutionComment is required", ErrValidation)
	}
	task, err := s.tasks.Resolve(ctx, orgID, id, comment)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.InterventionTask{}, ErrNotFound
		}
		return entity.InterventionTask{}, err
	}
	return task, nil
}

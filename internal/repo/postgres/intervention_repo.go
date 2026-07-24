package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

type InterventionRepository struct {
	db *postgres.DB
}

func NewInterventionRepository(db *postgres.DB) *InterventionRepository {
	return &InterventionRepository{db: db}
}

func (r *InterventionRepository) Create(ctx context.Context, orgID, studentID uuid.UUID, reasons []string) (entity.InterventionTask, error) {
	var row sqlc.InterventionTask
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateInterventionTask(ctx, sqlc.CreateInterventionTaskParams{
			OrgID:     orgID,
			StudentID: studentID,
			Reasons:   reasons,
			Status:    "Open",
		})
		return e
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.InterventionTask{}, repo.ErrNotFound
		}
		return entity.InterventionTask{}, err
	}
	return mapTask(row), nil
}

func (r *InterventionRepository) CountOpenForStudent(ctx context.Context, orgID, studentID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		n, e = sqlc.New(tx).CountOpenTasksForStudent(ctx, studentID)
		return e
	})
	return n, err
}

func (r *InterventionRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.InterventionTask, error) {
	var rows []sqlc.ListInterventionTasksRow
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListInterventionTasks(ctx)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.InterventionTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapTaskRow(row))
	}
	return out, nil
}

func (r *InterventionRepository) Assign(ctx context.Context, orgID, id uuid.UUID, assignedTo *uuid.UUID) (entity.InterventionTask, error) {
	var row sqlc.InterventionTask
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).AssignInterventionTask(ctx, sqlc.AssignInterventionTaskParams{
			ID:         id,
			AssignedTo: nullableUUID(assignedTo),
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.InterventionTask{}, repo.ErrNotFound
		}
		return entity.InterventionTask{}, err
	}
	return mapTask(row), nil
}

func (r *InterventionRepository) Start(ctx context.Context, orgID, id uuid.UUID) (entity.InterventionTask, error) {
	var row sqlc.InterventionTask
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).StartInterventionTask(ctx, id)
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.InterventionTask{}, repo.ErrNotFound
		}
		return entity.InterventionTask{}, err
	}
	return mapTask(row), nil
}

func (r *InterventionRepository) Resolve(ctx context.Context, orgID, id uuid.UUID, comment string) (entity.InterventionTask, error) {
	var row sqlc.InterventionTask
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).ResolveInterventionTask(ctx, sqlc.ResolveInterventionTaskParams{
			ID:                id,
			ResolutionComment: textVal(comment),
		})
		return e
	})
	if err != nil {
		// No row updated → task is missing or already resolved (in this tenant).
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.InterventionTask{}, repo.ErrNotFound
		}
		return entity.InterventionTask{}, err
	}
	return mapTask(row), nil
}

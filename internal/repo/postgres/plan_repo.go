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

// PlanRepository persists landing-page pricing plans. plans is a platform-level (NON-RLS) table,
// so it is read/written with the pool directly — like signup_requests/organizations.
type PlanRepository struct {
	db *postgres.DB
}

func NewPlanRepository(db *postgres.DB) *PlanRepository { return &PlanRepository{db: db} }

func (r *PlanRepository) ListActive(ctx context.Context) ([]entity.Plan, error) {
	rows, err := sqlc.New(r.db.Pool).ListActivePlans(ctx)
	if err != nil {
		return nil, err
	}
	return mapPlans(rows), nil
}

func (r *PlanRepository) ListAll(ctx context.Context) ([]entity.Plan, error) {
	rows, err := sqlc.New(r.db.Pool).ListAllPlans(ctx)
	if err != nil {
		return nil, err
	}
	return mapPlans(rows), nil
}

func (r *PlanRepository) Create(ctx context.Context, in entity.Plan) (entity.Plan, error) {
	row, err := sqlc.New(r.db.Pool).CreatePlan(ctx, sqlc.CreatePlanParams{
		PlanKey:     in.PlanKey,
		Name:        in.Name,
		Price:       in.Price,
		Period:      in.Period,
		Tagline:     in.Tagline,
		Features:    in.Features,
		Highlighted: in.Highlighted,
		SortOrder:   int32(in.SortOrder),
		IsActive:    in.IsActive,
	})
	if err != nil {
		return entity.Plan{}, err
	}
	return mapPlan(row), nil
}

func (r *PlanRepository) Update(ctx context.Context, in entity.Plan) (entity.Plan, error) {
	row, err := sqlc.New(r.db.Pool).UpdatePlan(ctx, sqlc.UpdatePlanParams{
		ID:          in.ID,
		PlanKey:     in.PlanKey,
		Name:        in.Name,
		Price:       in.Price,
		Period:      in.Period,
		Tagline:     in.Tagline,
		Features:    in.Features,
		Highlighted: in.Highlighted,
		SortOrder:   int32(in.SortOrder),
		IsActive:    in.IsActive,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Plan{}, repo.ErrNotFound
		}
		return entity.Plan{}, err
	}
	return mapPlan(row), nil
}

func (r *PlanRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return sqlc.New(r.db.Pool).DeletePlan(ctx, id)
}

func mapPlans(rows []sqlc.Plan) []entity.Plan {
	out := make([]entity.Plan, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPlan(row))
	}
	return out
}

package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// ObstacleRepository manages a center's configurable "biggest obstacle" choices. obstacle_options
// is RLS-protected, so every call runs inside the tenant scope.
type ObstacleRepository struct {
	db *postgres.DB
}

func NewObstacleRepository(db *postgres.DB) *ObstacleRepository {
	return &ObstacleRepository{db: db}
}

func (r *ObstacleRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.ObstacleOption, error) {
	var out []entity.ObstacleOption
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListObstacleOptions(ctx, orgID)
		if e != nil {
			return e
		}
		out = make([]entity.ObstacleOption, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapObstacle(row))
		}
		return nil
	})
	return out, err
}

func (r *ObstacleRepository) Create(ctx context.Context, orgID uuid.UUID, label string, position int) (entity.ObstacleOption, error) {
	var row sqlc.ObstacleOption
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateObstacleOption(ctx, sqlc.CreateObstacleOptionParams{
			OrgID:    orgID,
			Label:    label,
			Position: int32(position),
		})
		return e
	})
	if err != nil {
		return entity.ObstacleOption{}, err
	}
	return mapObstacle(row), nil
}

func (r *ObstacleRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteObstacleOption(ctx, sqlc.DeleteObstacleOptionParams{OrgID: orgID, ID: id})
	})
}

func mapObstacle(o sqlc.ObstacleOption) entity.ObstacleOption {
	return entity.ObstacleOption{
		ID:        o.ID,
		OrgID:     o.OrgID,
		Label:     o.Label,
		Position:  int(o.Position),
		IsActive:  o.IsActive,
		CreatedAt: o.CreatedAt.Time,
	}
}

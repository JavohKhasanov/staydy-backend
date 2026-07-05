package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// ActivityRepository persists the polymorphic communication timeline. activities is RLS-protected,
// so every call runs inside the tenant scope.
type ActivityRepository struct {
	db *postgres.DB
}

func NewActivityRepository(db *postgres.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) List(ctx context.Context, orgID uuid.UUID, subjectType string, subjectID uuid.UUID) ([]entity.Activity, error) {
	var out []entity.Activity
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListActivities(ctx, sqlc.ListActivitiesParams{
			OrgID:       orgID,
			SubjectType: subjectType,
			SubjectID:   subjectID,
		})
		if e != nil {
			return e
		}
		out = make([]entity.Activity, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapActivity(row))
		}
		return nil
	})
	return out, err
}

func (r *ActivityRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.CreateActivityParams) (entity.Activity, error) {
	var row sqlc.Activity
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateActivity(ctx, sqlc.CreateActivityParams{
			OrgID:       orgID,
			SubjectType: p.SubjectType,
			SubjectID:   p.SubjectID,
			Type:        p.Type,
			Body:        p.Body,
			Author:      p.Author,
		})
		return e
	})
	if err != nil {
		return entity.Activity{}, err
	}
	return mapActivity(row), nil
}

func (r *ActivityRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteActivity(ctx, sqlc.DeleteActivityParams{OrgID: orgID, ID: id})
	})
}

func mapActivity(a sqlc.Activity) entity.Activity {
	return entity.Activity{
		ID:          a.ID,
		OrgID:       a.OrgID,
		SubjectType: a.SubjectType,
		SubjectID:   a.SubjectID,
		Type:        a.Type,
		Body:        a.Body,
		Author:      a.Author,
		CreatedAt:   a.CreatedAt.Time,
	}
}

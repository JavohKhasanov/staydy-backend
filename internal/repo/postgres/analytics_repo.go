package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo/sqlc"
)

type AnalyticsRepository struct {
	db *postgres.DB
}

func NewAnalyticsRepository(db *postgres.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// Retention runs the three retention aggregates for one org (RLS-scoped) and returns the raw counts;
// derived rates are filled by entity.RetentionStats.ComputeRetention in the usecase.
func (r *AnalyticsRepository) Retention(ctx context.Context, orgID uuid.UUID) (entity.RetentionStats, error) {
	var out entity.RetentionStats
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)

		sc, e := q.RetentionStatusCounts(ctx)
		if e != nil {
			return e
		}
		out.Total, out.Active, out.Dropped = int(sc.Total), int(sc.Active), int(sc.Dropped)
		out.Green, out.Yellow, out.Red = int(sc.Green), int(sc.Yellow), int(sc.Red)

		cohorts, e := q.RetentionCohorts(ctx)
		if e != nil {
			return e
		}
		out.Cohorts = make([]entity.Cohort, 0, len(cohorts))
		for _, c := range cohorts {
			out.Cohorts = append(out.Cohorts, entity.Cohort{
				Month: c.Cohort, Total: int(c.Total), Active: int(c.Active), Dropped: int(c.Dropped),
			})
		}

		ie, e := q.InterventionEffectiveness(ctx)
		if e != nil {
			return e
		}
		out.Interventions = entity.InterventionStats{
			Open: int(ie.Open), Resolved: int(ie.Resolved), Resolved30d: int(ie.Resolved30d),
			AvgResolveDays: ie.AvgResolveDays,
		}
		return nil
	})
	return out, err
}

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

// DashboardRepository loads a tenant's dashboard aggregates. All queries run in a single
// tenant transaction so the snapshot is internally consistent (RLS scopes every row).
type DashboardRepository struct {
	db *postgres.DB
}

func NewDashboardRepository(db *postgres.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

func (r *DashboardRepository) Load(ctx context.Context, orgID uuid.UUID) (repo.DashboardData, error) {
	var data repo.DashboardData
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)

		tiers, err := q.CountStudentsByTier(ctx)
		if err != nil {
			return err
		}
		data.TierCounts = make(map[string]int, len(tiers))
		for _, t := range tiers {
			data.TierCounts[t.RiskTier] = int(t.Count)
		}

		groups, err := q.RiskByGroup(ctx)
		if err != nil {
			return err
		}
		data.Groups = make([]repo.GroupRisk, len(groups))
		for i, g := range groups {
			data.Groups[i] = repo.GroupRisk{
				Group:        g.GroupName,
				StudentCount: int(g.StudentCount),
				AvgRisk:      int(g.AvgRisk),
			}
		}

		obstacles, err := q.TopObstacles(ctx)
		if err != nil {
			return err
		}
		data.Obstacles = make([]repo.ObstacleCount, len(obstacles))
		for i, o := range obstacles {
			data.Obstacles[i] = repo.ObstacleCount{
				Obstacle: textToString(o.Obstacle),
				Count:    int(o.Count),
			}
		}

		highRisk, err := q.HighRiskStudents(ctx)
		if err != nil {
			return err
		}
		data.HighRisk = make([]entity.Student, len(highRisk))
		for i := range highRisk {
			data.HighRisk[i] = mapStudent(highRisk[i])
		}

		motivation, err := q.AvgMotivationByWeek(ctx)
		if err != nil {
			return err
		}
		data.MotivationByWeek = make([]repo.WeekMotivation, len(motivation))
		for i, m := range motivation {
			data.MotivationByWeek[i] = repo.WeekMotivation{
				Week:          int(m.WeekNumber),
				AvgMotivation: m.AvgMotivation,
			}
		}
		return nil
	})
	if err != nil {
		return repo.DashboardData{}, err
	}
	return data, nil
}

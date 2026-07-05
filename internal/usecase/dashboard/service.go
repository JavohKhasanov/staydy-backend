// Package dashboard is the read-only analytics usecase: it loads a tenant's aggregates and
// derives the KPI snapshot the UI renders (risk distribution, at-risk groups, obstacles,
// the highest-risk students, and the weekly motivation trend).
package dashboard

import (
	"context"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// Risk tier labels (mirror internal/risk).
const (
	tierRed    = "Red"
	tierYellow = "Yellow"
	tierGreen  = "Green"
)

type Service struct {
	repo repo.DashboardRepository
}

func NewService(r repo.DashboardRepository) *Service { return &Service{repo: r} }

// Snapshot is the assembled dashboard the API returns.
type Snapshot struct {
	Total            int
	Green            int
	Yellow           int
	Red              int
	AtRisk           int // Yellow + Red
	Groups           []repo.GroupRisk
	Obstacles        []repo.ObstacleCount
	HighRisk         []entity.Student
	MotivationByWeek []repo.WeekMotivation
}

// Get loads the tenant's aggregates and derives the KPI counts. Slices are always non-nil
// (empty when there's no data) so the JSON response is stable for the UI.
func (s *Service) Get(ctx context.Context, orgID uuid.UUID) (Snapshot, error) {
	data, err := s.repo.Load(ctx, orgID)
	if err != nil {
		return Snapshot{}, err
	}

	green := data.TierCounts[tierGreen]
	yellow := data.TierCounts[tierYellow]
	red := data.TierCounts[tierRed]

	snap := Snapshot{
		Total:            green + yellow + red,
		Green:            green,
		Yellow:           yellow,
		Red:              red,
		AtRisk:           yellow + red,
		Groups:           data.Groups,
		Obstacles:        data.Obstacles,
		HighRisk:         data.HighRisk,
		MotivationByWeek: data.MotivationByWeek,
	}
	if snap.Groups == nil {
		snap.Groups = []repo.GroupRisk{}
	}
	if snap.Obstacles == nil {
		snap.Obstacles = []repo.ObstacleCount{}
	}
	if snap.HighRisk == nil {
		snap.HighRisk = []entity.Student{}
	}
	if snap.MotivationByWeek == nil {
		snap.MotivationByWeek = []repo.WeekMotivation{}
	}
	return snap, nil
}

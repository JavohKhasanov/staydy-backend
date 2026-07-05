package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	dashboardusecase "github.com/student-success/backend/internal/usecase/dashboard"
)

// --- DTOs ---

type dashboardResponse struct {
	Total            int                  `json:"total"`
	Green            int                  `json:"green"`
	Yellow           int                  `json:"yellow"`
	Red              int                  `json:"red"`
	AtRisk           int                  `json:"atRisk" doc:"Yellow + Red student count (the full at-risk total)"`
	Groups           []groupRiskResponse  `json:"groups"`
	Obstacles        []obstacleResponse   `json:"obstacles"`
	HighRisk         []studentResponse    `json:"highRisk" doc:"Top at-risk students, highest risk first, capped at 10. Use atRisk for the full count."`
	MotivationByWeek []weekMotivationResp `json:"motivationByWeek"`
}

type groupRiskResponse struct {
	Group        string `json:"group"`
	StudentCount int    `json:"studentCount"`
	AvgRisk      int    `json:"avgRisk"`
}

type obstacleResponse struct {
	Obstacle string `json:"obstacle"`
	Count    int    `json:"count"`
}

type weekMotivationResp struct {
	Week          int     `json:"week"`
	AvgMotivation float64 `json:"avgMotivation"`
}

type dashboardInput struct{}
type dashboardOutput struct{ Body dashboardResponse }

// registerDashboard mounts the tenant-scoped, read-only dashboard endpoint.
func registerDashboard(api huma.API, svc *dashboardusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "dashboard-get",
		Method:      http.MethodGet,
		Path:        "/dashboard",
		Summary:     "Dashboard KPIs: risk distribution, at-risk groups, obstacles, top-10 highest-risk students, motivation trend",
		Tags:        []string{"dashboard"},
		Errors:      []int{http.StatusInternalServerError},
	}), func(ctx context.Context, _ *dashboardInput) (*dashboardOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		snap, err := svc.Get(ctx, p.OrgID)
		if err != nil {
			log.Error().Err(err).Msg("dashboard: load")
			return nil, huma.Error500InternalServerError(msgInternal.For(LangFromContext(ctx)))
		}
		return &dashboardOutput{Body: toDashboardResponse(snap)}, nil
	})
}

func toDashboardResponse(s dashboardusecase.Snapshot) dashboardResponse {
	r := dashboardResponse{
		Total:            s.Total,
		Green:            s.Green,
		Yellow:           s.Yellow,
		Red:              s.Red,
		AtRisk:           s.AtRisk,
		Groups:           make([]groupRiskResponse, len(s.Groups)),
		Obstacles:        make([]obstacleResponse, len(s.Obstacles)),
		HighRisk:         make([]studentResponse, len(s.HighRisk)),
		MotivationByWeek: make([]weekMotivationResp, len(s.MotivationByWeek)),
	}
	for i, g := range s.Groups {
		r.Groups[i] = groupRiskResponse{Group: g.Group, StudentCount: g.StudentCount, AvgRisk: g.AvgRisk}
	}
	for i, o := range s.Obstacles {
		r.Obstacles[i] = obstacleResponse{Obstacle: o.Obstacle, Count: o.Count}
	}
	for i := range s.HighRisk {
		r.HighRisk[i] = toStudentResponse(s.HighRisk[i])
	}
	for i, m := range s.MotivationByWeek {
		r.MotivationByWeek[i] = weekMotivationResp{Week: m.Week, AvgMotivation: m.AvgMotivation}
	}
	return r
}

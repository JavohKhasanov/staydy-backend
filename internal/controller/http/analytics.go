package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	analyticsusecase "github.com/student-success/backend/internal/usecase/analytics"
)

type retentionOutput struct{ Body entity.RetentionStats }

// registerAnalytics mounts the retention analytics endpoint. Management view — mount on a
// director/administrator group.
func registerAnalytics(api huma.API, svc *analyticsusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "analytics-retention",
		Method:      http.MethodGet,
		Path:        "/retention",
		Summary:     "Retention analytics: retention rate, risk mix, cohorts, intervention effectiveness",
		Tags:        []string{"analytics"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*retentionOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		stats, err := svc.Retention(ctx, p.OrgID)
		if err != nil {
			log.Error().Err(err).Msg("retention analytics failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &retentionOutput{Body: stats}, nil
	})
}

package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

type recomputeOutput struct {
	Body struct {
		Recomputed int `json:"recomputed"`
	}
}

// registerMaintenance mounts ops endpoints. Mount on a group gated to center_admin / super_admin.
func registerMaintenance(api huma.API, students *studentusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "maintenance-recompute",
		Method:      http.MethodPost,
		Path:        "/maintenance/recompute",
		Summary:     "Recompute risk for every student in the center (manual ops trigger)",
		Tags:        []string{"maintenance"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *meEmptyInput) (*recomputeOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		n, err := students.RecomputeAll(ctx, p.OrgID)
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		out := &recomputeOutput{}
		out.Body.Recomputed = n
		return out, nil
	})
}

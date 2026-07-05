package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	obstacleusecase "github.com/student-success/backend/internal/usecase/obstacle"
)

type obstacleOptionResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type listObstaclesInput struct{}
type listObstaclesOutput struct{ Body []obstacleOptionResponse }
type createObstacleInput struct {
	Body struct {
		Label string `json:"label" minLength:"1" maxLength:"60" example:"Darslarni tushunmaslik"`
	}
}
type obstacleOutput struct{ Body obstacleOptionResponse }
type obstacleIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type deleteObstacleOutput struct{}

// registerObstacleOptions mounts management of a center's "biggest obstacle" choices. Mount on a
// group gated to center_admin / super_admin.
func registerObstacleOptions(api huma.API, svc *obstacleusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "obstacle-options-list",
		Method:      http.MethodGet,
		Path:        "/obstacle-options",
		Summary:     "List the center's configurable 'biggest obstacle' choices",
		Tags:        []string{"settings"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listObstaclesInput) (*listObstaclesOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		opts, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapObstacleError(LangFromContext(ctx), err, log)
		}
		out := &listObstaclesOutput{Body: make([]obstacleOptionResponse, 0, len(opts))}
		for _, o := range opts {
			out.Body = append(out.Body, toObstacleOptionResponse(o))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "obstacle-options-create",
		Method:        http.MethodPost,
		Path:          "/obstacle-options",
		Summary:       "Add a 'biggest obstacle' choice",
		Tags:          []string{"settings"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createObstacleInput) (*obstacleOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		o, err := svc.Create(ctx, p.OrgID, in.Body.Label)
		if err != nil {
			return nil, mapObstacleError(LangFromContext(ctx), err, log)
		}
		return &obstacleOutput{Body: toObstacleOptionResponse(o)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "obstacle-options-delete",
		Method:        http.MethodDelete,
		Path:          "/obstacle-options/{id}",
		Summary:       "Remove a 'biggest obstacle' choice",
		Tags:          []string{"settings"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *obstacleIDInput) (*deleteObstacleOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid option id")
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapObstacleError(LangFromContext(ctx), err, log)
		}
		return &deleteObstacleOutput{}, nil
	})
}

func toObstacleOptionResponse(o entity.ObstacleOption) obstacleOptionResponse {
	return obstacleOptionResponse{ID: o.ID.String(), Label: o.Label}
}

func mapObstacleError(lang i18n.Lang, err error, log zerolog.Logger) error {
	if errors.Is(err, obstacleusecase.ErrValidation) {
		return huma.Error422UnprocessableEntity(err.Error())
	}
	log.Error().Err(err).Msg("obstacle-options: unexpected error")
	return huma.Error500InternalServerError(msgInternal.For(lang))
}

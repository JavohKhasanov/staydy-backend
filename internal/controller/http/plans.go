package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	planusecase "github.com/student-success/backend/internal/usecase/plan"
)

type planResponse struct {
	ID          string   `json:"id"`
	PlanKey     string   `json:"planKey"`
	Name        string   `json:"name"`
	Price       string   `json:"price"`
	Period      string   `json:"period"`
	Tagline     string   `json:"tagline"`
	Features    []string `json:"features"`
	Highlighted bool     `json:"highlighted"`
	SortOrder   int      `json:"sortOrder"`
	IsActive    bool     `json:"isActive"`
}

func toPlanResponse(p entity.Plan) planResponse {
	feats := p.Features
	if feats == nil {
		feats = []string{}
	}
	return planResponse{
		ID:          p.ID.String(),
		PlanKey:     p.PlanKey,
		Name:        p.Name,
		Price:       p.Price,
		Period:      p.Period,
		Tagline:     p.Tagline,
		Features:    feats,
		Highlighted: p.Highlighted,
		SortOrder:   p.SortOrder,
		IsActive:    p.IsActive,
	}
}

func toPlanResponses(list []entity.Plan) []planResponse {
	out := make([]planResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toPlanResponse(p))
	}
	return out
}

type planListOutput struct{ Body []planResponse }
type planDetailOutput struct{ Body planResponse }

// planBody is the editable payload the super_admin sends when creating/updating a plan.
type planBody struct {
	PlanKey     string   `json:"planKey,omitempty" maxLength:"32" doc:"trial | basic | pro — preselects the signup CTA"`
	Name        string   `json:"name" minLength:"1" maxLength:"120"`
	Price       string   `json:"price,omitempty" maxLength:"120"`
	Period      string   `json:"period,omitempty" maxLength:"32"`
	Tagline     string   `json:"tagline,omitempty" maxLength:"255"`
	Features    []string `json:"features,omitempty" maxItems:"30"`
	Highlighted bool     `json:"highlighted,omitempty"`
	SortOrder   int      `json:"sortOrder,omitempty"`
	IsActive    bool     `json:"isActive,omitempty"`
}

func (b planBody) toInput() planusecase.Input {
	return planusecase.Input{
		PlanKey:     b.PlanKey,
		Name:        b.Name,
		Price:       b.Price,
		Period:      b.Period,
		Tagline:     b.Tagline,
		Features:    b.Features,
		Highlighted: b.Highlighted,
		SortOrder:   b.SortOrder,
		IsActive:    b.IsActive,
	}
}

type createPlanInput struct{ Body planBody }
type updatePlanInput struct {
	ID   string `path:"id" format:"uuid"`
	Body planBody
}
type planIDInput struct {
	ID string `path:"id" format:"uuid"`
}

// registerPlans wires the public landing pricing endpoint (publicAPI) and the super_admin editor
// endpoints (adminAPI). Keeping them together keeps the whole pricing feature in one file.
func registerPlans(publicAPI, adminAPI huma.API, svc *planusecase.Service, log zerolog.Logger) {
	// Public: the landing page renders the active plans.
	Register(publicAPI, huma.Operation{
		OperationID: "public-plans-list",
		Method:      http.MethodGet,
		Path:        "/public/plans",
		Summary:     "List active pricing plans for the landing page (public, no auth)",
		Tags:        []string{"public"},
		Errors:      []int{http.StatusInternalServerError},
	}, func(ctx context.Context, _ *struct{}) (*planListOutput, error) {
		list, err := svc.ListPublic(ctx)
		if err != nil {
			return nil, mapPlanError(err, log)
		}
		return &planListOutput{Body: toPlanResponses(list)}, nil
	})

	// Admin: full list (active + hidden) for the editor.
	Register(adminAPI, BearerOperation(huma.Operation{
		OperationID: "admin-plans-list",
		Method:      http.MethodGet,
		Path:        "/admin/plans",
		Summary:     "List all pricing plans (super_admin)",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*planListOutput, error) {
		list, err := svc.ListAll(ctx)
		if err != nil {
			return nil, mapPlanError(err, log)
		}
		return &planListOutput{Body: toPlanResponses(list)}, nil
	})

	Register(adminAPI, BearerOperation(huma.Operation{
		OperationID:   "admin-plans-create",
		Method:        http.MethodPost,
		Path:          "/admin/plans",
		Summary:       "Create a pricing plan (super_admin)",
		Tags:          []string{"admin"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createPlanInput) (*planDetailOutput, error) {
		p, err := svc.Create(ctx, in.Body.toInput())
		if err != nil {
			return nil, mapPlanError(err, log)
		}
		return &planDetailOutput{Body: toPlanResponse(p)}, nil
	})

	Register(adminAPI, BearerOperation(huma.Operation{
		OperationID: "admin-plans-update",
		Method:      http.MethodPut,
		Path:        "/admin/plans/{id}",
		Summary:     "Update a pricing plan (super_admin)",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updatePlanInput) (*planDetailOutput, error) {
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid id")
		}
		p, err := svc.Update(ctx, id, in.Body.toInput())
		if err != nil {
			return nil, mapPlanError(err, log)
		}
		return &planDetailOutput{Body: toPlanResponse(p)}, nil
	})

	Register(adminAPI, BearerOperation(huma.Operation{
		OperationID:   "admin-plans-delete",
		Method:        http.MethodDelete,
		Path:          "/admin/plans/{id}",
		Summary:       "Delete a pricing plan (super_admin)",
		Tags:          []string{"admin"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *planIDInput) (*struct{}, error) {
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid id")
		}
		if err := svc.Delete(ctx, id); err != nil {
			return nil, mapPlanError(err, log)
		}
		return nil, nil
	})
}

func mapPlanError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, planusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("ma'lumotlar to'liq emas")
	case errors.Is(err, planusecase.ErrNotFound):
		return huma.Error404NotFound("tarif topilmadi")
	default:
		log.Error().Err(err).Msg("plan request failed")
		return huma.Error500InternalServerError("internal error")
	}
}

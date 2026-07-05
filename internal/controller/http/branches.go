package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	branchusecase "github.com/student-success/backend/internal/usecase/branch"
)

type branchResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address,omitempty"`
	Phone    string `json:"phone,omitempty"`
	IsActive bool   `json:"isActive"`
}

type branchBody struct {
	Name     string `json:"name" minLength:"1" maxLength:"255"`
	Address  string `json:"address,omitempty" maxLength:"500"`
	Phone    string `json:"phone,omitempty" maxLength:"32"`
	IsActive bool   `json:"isActive,omitempty"`
}
type createBranchInput struct{ Body branchBody }
type updateBranchInput struct {
	ID   string `path:"id" format:"uuid"`
	Body branchBody
}
type branchIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type branchesListOutput struct{ Body []branchResponse }
type branchOutput struct{ Body branchResponse }

func registerBranches(api huma.API, svc *branchusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "branches-list",
		Method:      http.MethodGet,
		Path:        "/branches",
		Summary:     "List branches (filiallar)",
		Tags:        []string{"branches"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *meEmptyInput) (*branchesListOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		list, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapBranchError(err, log)
		}
		out := &branchesListOutput{}
		out.Body = make([]branchResponse, 0, len(list))
		for _, b := range list {
			out.Body = append(out.Body, toBranchResponse(b))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "branch-create",
		Method:        http.MethodPost,
		Path:          "/branches",
		Summary:       "Create a branch",
		Tags:          []string{"branches"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createBranchInput) (*branchOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		b, err := svc.Create(ctx, p.OrgID, branchusecase.Input{
			Name: in.Body.Name, Address: in.Body.Address, Phone: in.Body.Phone,
		})
		if err != nil {
			return nil, mapBranchError(err, log)
		}
		return &branchOutput{Body: toBranchResponse(b)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "branch-update",
		Method:      http.MethodPut,
		Path:        "/branches/{id}",
		Summary:     "Update a branch",
		Tags:        []string{"branches"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateBranchInput) (*branchOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		b, err := svc.Update(ctx, p.OrgID, id, branchusecase.Input{
			Name: in.Body.Name, Address: in.Body.Address, Phone: in.Body.Phone, IsActive: in.Body.IsActive,
		})
		if err != nil {
			return nil, mapBranchError(err, log)
		}
		return &branchOutput{Body: toBranchResponse(b)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "branch-delete",
		Method:        http.MethodDelete,
		Path:          "/branches/{id}",
		Summary:       "Delete a branch (records become org-wide)",
		Tags:          []string{"branches"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *branchIDInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapBranchError(err, log)
		}
		return &noContentOutput{}, nil
	})
}

func toBranchResponse(b entity.Branch) branchResponse {
	return branchResponse{
		ID:       b.ID.String(),
		Name:     b.Name,
		Address:  b.Address,
		Phone:    b.Phone,
		IsActive: b.IsActive,
	}
}

func mapBranchError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, branchusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("filial nomini kiriting")
	case errors.Is(err, branchusecase.ErrNotFound):
		return huma.Error404NotFound("filial topilmadi")
	default:
		log.Error().Err(err).Msg("branch op failed")
		return huma.Error500InternalServerError("internal error")
	}
}

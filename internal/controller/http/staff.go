package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	staffusecase "github.com/student-success/backend/internal/usecase/staff"
)

type staffResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FullName  string `json:"fullName"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

type createStaffRequest struct {
	Email    string `json:"email" format:"email" maxLength:"255" example:"moliya@markaz.uz"`
	Password string `json:"password" minLength:"8" maxLength:"128"`
	FullName string `json:"fullName" minLength:"1" maxLength:"255" example:"Dilnoza Karimova"`
	Role     string `json:"role" enum:"center_admin,finance" doc:"center_admin (administrator) or finance (moliya)"`
}
type updateStaffRequest struct {
	Email    string `json:"email" format:"email" maxLength:"255"`
	FullName string `json:"fullName" minLength:"1" maxLength:"255"`
	Role     string `json:"role" enum:"center_admin,finance"`
}
type setStaffPasswordRequest struct {
	Password string `json:"password" minLength:"8" maxLength:"128"`
}

type listStaffInput struct{}
type listStaffOutput struct{ Body []staffResponse }
type createStaffInput struct{ Body createStaffRequest }
type updateStaffInput struct {
	ID   string `path:"id" format:"uuid"`
	Body updateStaffRequest
}
type setStaffPasswordInput struct {
	ID   string `path:"id" format:"uuid"`
	Body setStaffPasswordRequest
}
type staffIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type staffOutput struct{ Body staffResponse }

// registerStaff mounts back-office staff (administrator + finance) management. Mount on a group
// gated to center_admin / super_admin — only administrators manage staff accounts.
func registerStaff(api huma.API, svc *staffusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "staff-list",
		Method:      http.MethodGet,
		Path:        "/staff",
		Summary:     "List staff accounts (administrators + finance)",
		Tags:        []string{"staff"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listStaffInput) (*listStaffOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		staff, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapStaffError(err, log)
		}
		out := &listStaffOutput{Body: make([]staffResponse, 0, len(staff))}
		for _, u := range staff {
			out.Body = append(out.Body, toStaffResponse(u))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "staff-create",
		Method:        http.MethodPost,
		Path:          "/staff",
		Summary:       "Create a staff account",
		Tags:          []string{"staff"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createStaffInput) (*staffOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		u, err := svc.Create(ctx, p.OrgID, staffusecase.CreateInput{
			Email:    in.Body.Email,
			Password: in.Body.Password,
			FullName: in.Body.FullName,
			Role:     in.Body.Role,
		})
		if err != nil {
			return nil, mapStaffError(err, log)
		}
		return &staffOutput{Body: toStaffResponse(u)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "staff-update",
		Method:      http.MethodPut,
		Path:        "/staff/{id}",
		Summary:     "Update a staff account",
		Tags:        []string{"staff"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateStaffInput) (*staffOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		u, err := svc.Update(ctx, p.OrgID, id, staffusecase.UpdateInput{
			Email:    in.Body.Email,
			FullName: in.Body.FullName,
			Role:     in.Body.Role,
		})
		if err != nil {
			return nil, mapStaffError(err, log)
		}
		return &staffOutput{Body: toStaffResponse(u)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "staff-set-password",
		Method:      http.MethodPut,
		Path:        "/staff/{id}/password",
		Summary:     "Reset a staff member's password",
		Tags:        []string{"staff"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *setStaffPasswordInput) (*staffOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.SetPassword(ctx, p.OrgID, id, in.Body.Password); err != nil {
			return nil, mapStaffError(err, log)
		}
		return &staffOutput{Body: staffResponse{ID: id.String()}}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "staff-delete",
		Method:        http.MethodDelete,
		Path:          "/staff/{id}",
		Summary:       "Delete a staff account",
		Tags:          []string{"staff"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *staffIDInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapStaffError(err, log)
		}
		return &noContentOutput{}, nil
	})
}

func toStaffResponse(u entity.User) staffResponse {
	return staffResponse{
		ID:        u.ID.String(),
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt.Format("2006-01-02"),
	}
}

func mapStaffError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, staffusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("ma'lumotlar noto'g'ri")
	case errors.Is(err, staffusecase.ErrLastAdmin):
		return huma.Error422UnprocessableEntity("Oxirgi administratorni o'chirib bo'lmaydi")
	case errors.Is(err, repo.ErrAlreadyExists):
		return huma.Error409Conflict("Bu email allaqachon band")
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound("topilmadi")
	default:
		log.Error().Err(err).Msg("staff op failed")
		return huma.Error500InternalServerError("internal error")
	}
}

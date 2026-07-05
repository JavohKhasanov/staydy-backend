package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	signupusecase "github.com/student-success/backend/internal/usecase/signup"
)

type signupRequestResponse struct {
	ID          string `json:"id"`
	CenterName  string `json:"centerName"`
	ContactName string `json:"contactName"`
	Phone       string `json:"phone"`
	Email       string `json:"email,omitempty"`
	Plan        string `json:"plan,omitempty"`
	Message     string `json:"message,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
}

func toSignupResponse(s entity.SignupRequest) signupRequestResponse {
	return signupRequestResponse{
		ID:          s.ID.String(),
		CenterName:  s.CenterName,
		ContactName: s.ContactName,
		Phone:       s.Phone,
		Email:       s.Email,
		Plan:        s.Plan,
		Message:     s.Message,
		Status:      s.Status,
		CreatedAt:   s.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// --- public submit (no auth) ---

type createSignupRequest struct {
	CenterName  string `json:"centerName" minLength:"1" maxLength:"255"`
	ContactName string `json:"contactName" minLength:"1" maxLength:"255"`
	Phone       string `json:"phone" minLength:"7" maxLength:"32"`
	Email       string `json:"email,omitempty" maxLength:"255"`
	Plan        string `json:"plan,omitempty" maxLength:"32" doc:"requested tier: trial | basic | pro"`
	Message     string `json:"message,omitempty" maxLength:"2000"`
}
type createSignupInput struct{ Body createSignupRequest }

// signupAckResponse is the minimal public acknowledgement (no listing of others' data).
type signupAckResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
type signupAckOutput struct{ Body signupAckResponse }

// --- admin review ---

type signupListOutput struct{ Body []signupRequestResponse }
type signupIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type updateSignupStatusRequest struct {
	Status string `json:"status" enum:"new,contacted,converted,rejected"`
}
type updateSignupStatusInput struct {
	ID   string `path:"id" format:"uuid"`
	Body updateSignupStatusRequest
}
type signupDetailOutput struct{ Body signupRequestResponse }

// registerSignup wires the public submit endpoint (publicAPI) and the super_admin review endpoints
// (adminAPI). Keeping them here keeps the whole funnel in one file.
func registerSignup(publicAPI, adminAPI huma.API, svc *signupusecase.Service, log zerolog.Logger) {
	Register(publicAPI, huma.Operation{
		OperationID:   "public-signup-request",
		Method:        http.MethodPost,
		Path:          "/public/signup-requests",
		Summary:       "Submit interest from the landing page (public, no auth)",
		Tags:          []string{"public"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, func(ctx context.Context, in *createSignupInput) (*signupAckOutput, error) {
		req, err := svc.Create(ctx, signupusecase.CreateInput{
			CenterName:  in.Body.CenterName,
			ContactName: in.Body.ContactName,
			Phone:       in.Body.Phone,
			Email:       in.Body.Email,
			Plan:        in.Body.Plan,
			Message:     in.Body.Message,
		})
		if err != nil {
			return nil, mapSignupError(err, log)
		}
		return &signupAckOutput{Body: signupAckResponse{ID: req.ID.String(), Status: req.Status}}, nil
	})

	Register(adminAPI, BearerOperation(huma.Operation{
		OperationID: "admin-signup-requests-list",
		Method:      http.MethodGet,
		Path:        "/admin/signup-requests",
		Summary:     "List landing-page signup requests (super_admin)",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*signupListOutput, error) {
		list, err := svc.List(ctx)
		if err != nil {
			return nil, mapSignupError(err, log)
		}
		out := make([]signupRequestResponse, 0, len(list))
		for _, r := range list {
			out = append(out, toSignupResponse(r))
		}
		return &signupListOutput{Body: out}, nil
	})

	Register(adminAPI, BearerOperation(huma.Operation{
		OperationID: "admin-signup-requests-status",
		Method:      http.MethodPatch,
		Path:        "/admin/signup-requests/{id}",
		Summary:     "Update a signup request's status (super_admin)",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateSignupStatusInput) (*signupDetailOutput, error) {
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid id")
		}
		req, err := svc.SetStatus(ctx, id, in.Body.Status)
		if err != nil {
			return nil, mapSignupError(err, log)
		}
		return &signupDetailOutput{Body: toSignupResponse(req)}, nil
	})
}

func mapSignupError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, signupusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("ma'lumotlar to'liq emas")
	case errors.Is(err, signupusecase.ErrNotFound):
		return huma.Error404NotFound("so'rov topilmadi")
	default:
		log.Error().Err(err).Msg("signup request failed")
		return huma.Error500InternalServerError("internal error")
	}
}

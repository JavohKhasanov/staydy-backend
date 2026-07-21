package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	authusecase "github.com/student-success/backend/internal/usecase/auth"
)

type changePasswordInput struct {
	Body struct {
		CurrentPassword string `json:"currentPassword" minLength:"1" maxLength:"128"`
		NewPassword     string `json:"newPassword" minLength:"8" maxLength:"128"`
	}
}

// registerAccount mounts self-service account operations for the signed-in user. Mount on a group
// gated by RequireAuth only (any authenticated role, including the platform owner).
func registerAccount(api huma.API, svc *authusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "me-change-password",
		Method:      http.MethodPost,
		Path:        "/me/change-password",
		Summary:     "Change your own password",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}), func(ctx context.Context, in *changePasswordInput) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		if err := svc.ChangePassword(ctx, p.OrgID, p.UserID, in.Body.CurrentPassword, in.Body.NewPassword); err != nil {
			return nil, mapAuthError(LangFromContext(ctx), err, log)
		}
		return &noContentOutput{}, nil
	})
}

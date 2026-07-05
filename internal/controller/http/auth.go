package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/i18n"
	authusecase "github.com/student-success/backend/internal/usecase/auth"
)

// --- request DTOs (Huma validates these tags and documents them in OpenAPI) ---

type registerRequest struct {
	OrganizationName string `json:"organizationName" minLength:"1" maxLength:"255" example:"Najot Talim"`
	Slug             string `json:"slug,omitempty" maxLength:"63" pattern:"^[a-z0-9-]*$" doc:"Optional; derived from the name when empty." example:"najot"`
	Email            string `json:"email" format:"email" example:"admin@najot.uz"`
	Password         string `json:"password" minLength:"8" maxLength:"128" example:"secret123"`
	FullName         string `json:"fullName" minLength:"1" maxLength:"255" example:"Admin"`
}

type loginRequest struct {
	Slug     string `json:"slug" minLength:"1" example:"najot"`
	Email    string `json:"email" minLength:"1" example:"admin@najot.uz"`
	Password string `json:"password" minLength:"1"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" minLength:"1"`
}

// Header params are declared at the top level of each input (Huma does not read them from
// an embedded struct). UserAgent/IP feed the refresh-session audit; Accept-Language drives
// the localized error messages on these public routes.
type registerInput struct {
	UserAgent      string `header:"User-Agent" doc:"Recorded on the refresh session."`
	ForwardedFor   string `header:"X-Forwarded-For"`
	RealIP         string `header:"X-Real-IP"`
	AcceptLanguage string `header:"Accept-Language" doc:"uz | ru | en — picks the error message language."`
	Body           registerRequest
}
type loginInput struct {
	UserAgent      string `header:"User-Agent"`
	ForwardedFor   string `header:"X-Forwarded-For"`
	RealIP         string `header:"X-Real-IP"`
	AcceptLanguage string `header:"Accept-Language"`
	Body           loginRequest
}
type refreshInput struct {
	UserAgent      string `header:"User-Agent"`
	ForwardedFor   string `header:"X-Forwarded-For"`
	RealIP         string `header:"X-Real-IP"`
	AcceptLanguage string `header:"Accept-Language"`
	Body           refreshRequest
}

// --- response DTOs ---

type userResponse struct {
	ID       string `json:"id"`
	OrgID    string `json:"orgId"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
}

type orgResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Plan string `json:"plan"`
}

type authResponse struct {
	AccessToken      string       `json:"accessToken"`
	AccessExpiresAt  string       `json:"accessExpiresAt"`
	RefreshToken     string       `json:"refreshToken"`
	RefreshExpiresAt string       `json:"refreshExpiresAt"`
	User             userResponse `json:"user"`
	Organization     orgResponse  `json:"organization"`
}

type authOutput struct {
	Body authResponse
}

// registerAuth mounts the public auth operations on the group.
func registerAuth(api huma.API, svc *authusecase.Service, log zerolog.Logger) {
	Register(api, huma.Operation{
		OperationID:   "auth-register",
		Method:        http.MethodPost,
		Path:          "/auth/register",
		Summary:       "Register a center and its first admin",
		Description:   "Provisions a new educational center (tenant) plus its first center_admin, then returns an access + refresh token pair.",
		Tags:          []string{"auth"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusInternalServerError},
	}, func(ctx context.Context, in *registerInput) (*authOutput, error) {
		tokens, err := svc.Register(ctx, authusecase.RegisterParams{
			OrganizationName: in.Body.OrganizationName,
			Slug:             in.Body.Slug,
			Email:            in.Body.Email,
			Password:         in.Body.Password,
			FullName:         in.Body.FullName,
		}, device(in.UserAgent, in.ForwardedFor, in.RealIP))
		if err != nil {
			return nil, mapAuthError(i18n.Detect(in.AcceptLanguage), err, log)
		}
		return &authOutput{Body: toAuthResponse(tokens)}, nil
	})

	Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        "/auth/login",
		Summary:     "Log in",
		Description: "Resolves the center by slug, verifies the password, and returns an access + refresh token pair.",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, func(ctx context.Context, in *loginInput) (*authOutput, error) {
		tokens, err := svc.Login(ctx, authusecase.LoginParams{
			Slug:     in.Body.Slug,
			Email:    in.Body.Email,
			Password: in.Body.Password,
		}, device(in.UserAgent, in.ForwardedFor, in.RealIP))
		if err != nil {
			return nil, mapAuthError(i18n.Detect(in.AcceptLanguage), err, log)
		}
		return &authOutput{Body: toAuthResponse(tokens)}, nil
	})

	Register(api, huma.Operation{
		OperationID: "auth-refresh",
		Method:      http.MethodPost,
		Path:        "/auth/refresh",
		Summary:     "Rotate a refresh token for a new token pair",
		Tags:        []string{"auth"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, func(ctx context.Context, in *refreshInput) (*authOutput, error) {
		tokens, err := svc.Refresh(ctx, in.Body.RefreshToken, device(in.UserAgent, in.ForwardedFor, in.RealIP))
		if err != nil {
			return nil, mapAuthError(i18n.Detect(in.AcceptLanguage), err, log)
		}
		return &authOutput{Body: toAuthResponse(tokens)}, nil
	})
}

// device builds session metadata from request headers. Client IP prefers the first
// X-Forwarded-For hop, then X-Real-IP (both proxy-set).
func device(userAgent, forwardedFor, realIP string) authusecase.DeviceMetadata {
	ip := strings.TrimSpace(realIP)
	if forwardedFor != "" {
		first := forwardedFor
		if i := strings.IndexByte(forwardedFor, ','); i >= 0 {
			first = forwardedFor[:i]
		}
		ip = strings.TrimSpace(first)
	}
	return authusecase.DeviceMetadata{UserAgent: userAgent, IP: ip}
}

func toAuthResponse(t *authusecase.Tokens) authResponse {
	return authResponse{
		AccessToken:      t.AccessToken,
		AccessExpiresAt:  t.AccessExpiresAt.UTC().Format(time.RFC3339),
		RefreshToken:     t.RefreshToken,
		RefreshExpiresAt: t.RefreshExpiresAt.UTC().Format(time.RFC3339),
		User: userResponse{
			ID:       t.User.ID.String(),
			OrgID:    t.User.OrgID.String(),
			Email:    t.User.Email,
			FullName: t.User.FullName,
			Role:     string(t.User.Role),
		},
		Organization: orgResponse{
			ID:   t.Organization.ID.String(),
			Name: t.Organization.Name,
			Slug: t.Organization.Slug,
			Plan: t.Organization.Plan,
		},
	}
}

// --- localized error mapping ---

var (
	msgInvalidCreds   = i18n.Message{UZ: "Login yoki parol noto'g'ri.", RU: "Неверный логин или пароль.", EN: "Invalid credentials."}
	msgInvalidRefresh = i18n.Message{UZ: "Refresh token yaroqsiz yoki muddati o'tgan.", RU: "Недействительный или просроченный refresh token.", EN: "Invalid or expired refresh token."}
	msgSlugTaken      = i18n.Message{UZ: "Bu manzil (slug) allaqachon band.", RU: "Этот slug уже занят.", EN: "Organization slug already taken."}
	msgSuspended      = i18n.Message{UZ: "Markaz hozir faol emas. Administrator bilan bog'laning.", RU: "Центр сейчас неактивен. Свяжитесь с администратором.", EN: "This center is not active. Contact the administrator."}
	msgInternal       = i18n.Message{UZ: "Ichki xatolik yuz berdi.", RU: "Произошла внутренняя ошибка.", EN: "Internal error."}
)

func mapAuthError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, authusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, authusecase.ErrInvalidCredentials):
		return huma.Error401Unauthorized(msgInvalidCreds.For(lang))
	case errors.Is(err, authusecase.ErrInvalidRefresh):
		return huma.Error401Unauthorized(msgInvalidRefresh.For(lang))
	case errors.Is(err, authusecase.ErrSlugTaken):
		return huma.Error409Conflict(msgSlugTaken.For(lang))
	case errors.Is(err, authusecase.ErrSuspended):
		return huma.Error403Forbidden(msgSuspended.For(lang))
	default:
		log.Error().Err(err).Msg("auth: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

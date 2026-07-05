package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	"github.com/student-success/backend/internal/security"
)

type ctxKey string

const (
	principalKey ctxKey = "principal"
	langKey      ctxKey = "lang"
)

// PrincipalFromContext returns the authenticated identity injected by RequireAuth.
func PrincipalFromContext(ctx context.Context) (entity.Principal, bool) {
	p, ok := ctx.Value(principalKey).(entity.Principal)
	return p, ok
}

// LangFromContext returns the request language (from Accept-Language), defaulting to uz.
func LangFromContext(ctx context.Context) i18n.Lang {
	if l, ok := ctx.Value(langKey).(i18n.Lang); ok {
		return l
	}
	return i18n.DefaultLang
}

// RequireAuth validates the Bearer access token and injects the Principal + request
// language into the context. Use on a Huma group to protect all its operations.
func RequireAuth(api huma.API, tokens *security.TokenManager) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		header := strings.TrimSpace(ctx.Header("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "missing bearer token")
			return
		}
		principal, err := tokens.ParseAccessToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid token")
			return
		}
		lang := i18n.Detect(ctx.Header("Accept-Language"))
		ctx = huma.WithValue(ctx, principalKey, principal)
		ctx = huma.WithValue(ctx, langKey, lang)
		next(ctx)
	}
}

// RequireRole gates a group to one of the allowed roles. Use after RequireAuth.
func RequireRole(api huma.API, allowed ...entity.UserRole) func(huma.Context, func(huma.Context)) {
	set := make(map[entity.UserRole]struct{}, len(allowed))
	for _, r := range allowed {
		set[r] = struct{}{}
	}
	return func(ctx huma.Context, next func(huma.Context)) {
		p, ok := PrincipalFromContext(ctx.Context())
		if !ok {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "missing principal")
			return
		}
		if _, ok := set[p.Role]; !ok {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "forbidden")
			return
		}
		next(ctx)
	}
}

// requestLogger logs one structured line per request.
func requestLogger(log zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			log.Info().
				Str("method", c.Request().Method).
				Str("path", c.Request().URL.Path).
				Int("status", c.Response().Status).
				Dur("dur", time.Since(start)).
				Str("request_id", c.Response().Header().Get(echo.HeaderXRequestID)).
				Msg("http")
			return err
		}
	}
}

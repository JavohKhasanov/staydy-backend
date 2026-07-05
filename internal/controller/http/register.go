// Package http is the transport layer: Echo router + Huma operations (OpenAPI 3.1 +
// Swagger UI at /docs). Operations live in per-feature files; this file holds the
// shared registration helpers.
package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Register wraps huma.Register, defaulting Tags so every operation is grouped in the docs.
func Register[I, O any](api huma.API, op huma.Operation, handler func(context.Context, *I) (*O, error)) {
	if len(op.Tags) == 0 {
		op.Tags = []string{"default"}
	}
	huma.Register(api, op, handler)
}

// BearerOperation marks an operation as requiring the bearerAuth (JWT) security scheme.
// Pair it with a group that has RequireAuth middleware.
func BearerOperation(op huma.Operation) huma.Operation {
	op.Security = []map[string][]string{{"bearerAuth": {}}}
	// Every authenticated operation can return 401 — document it once here.
	op.Errors = append(op.Errors, http.StatusUnauthorized)
	return op
}

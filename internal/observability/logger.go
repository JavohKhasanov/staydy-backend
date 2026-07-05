// Package observability wires structured logging and Prometheus metrics.
package observability

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// NewLogger builds a structured JSON logger at the given level (debug|info|warn|error).
func NewLogger(level string) zerolog.Logger {
	lvl := zerolog.InfoLevel
	if parsed, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level))); err == nil {
		lvl = parsed
	}
	return zerolog.New(os.Stdout).With().Timestamp().Logger().Level(lvl)
}

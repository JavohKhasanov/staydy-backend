// Command api serves the admin REST API (Huma/OpenAPI) and, later, the Telegram webhook.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/student-success/backend/internal/app"
	"github.com/student-success/backend/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		panic(err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		panic(err)
	}

	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}

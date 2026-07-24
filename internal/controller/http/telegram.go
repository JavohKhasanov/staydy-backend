package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/platform/telegram"
	botusecase "github.com/student-success/backend/internal/usecase/bot"
)

type studentTelegramLinkInput struct {
	Body struct {
		InitData string `json:"initData" minLength:"1" maxLength:"8192" doc:"Telegram Mini App initData (signed)"`
	}
}

// registerStudentTelegramLink lets the signed-in student bind their Telegram from the Mini App's
// initData, so the bot may push notifications to them. Mount on the student group.
func registerStudentTelegramLink(api huma.API, bot *botusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID:   "student-telegram-link",
		Method:        http.MethodPost,
		Path:          "/student/telegram-link",
		Summary:       "Bind the signed-in student's Telegram (Mini App initData) for push notifications",
		Tags:          []string{"student"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, in *studentTelegramLinkInput) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		if err := bot.LinkTelegram(ctx, p.OrgID, p.UserID, in.Body.InitData); err != nil {
			if errors.Is(err, telegram.ErrInvalidInitData) {
				return nil, huma.Error422UnprocessableEntity("telegram ma'lumoti noto'g'ri")
			}
			log.Error().Err(err).Msg("student telegram link failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &noContentOutput{}, nil
	})
}


package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/i18n"
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

type telegramLinkResponse struct {
	Link      string `json:"link" doc:"t.me deep link to share with the student"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

type telegramLinkInput struct {
	ID string `path:"id" format:"uuid"`
}
type telegramLinkOutput struct{ Body telegramLinkResponse }

var msgBotUnavailable = i18n.Message{
	UZ: "Telegram bot sozlanmagan.",
	RU: "Telegram-бот не настроен.",
	EN: "Telegram bot is not configured.",
}

// registerTelegram mounts the admin endpoint that mints a student's deep-link invite.
func registerTelegram(api huma.API, bot *botusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "students-telegram-link",
		Method:      http.MethodPost,
		Path:        "/students/{id}/telegram-link",
		Summary:     "Generate a Telegram deep-link invite for a student to connect their account",
		Tags:        []string{"students"},
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}), func(ctx context.Context, in *telegramLinkInput) (*telegramLinkOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		lang := LangFromContext(ctx)

		link, err := bot.CreateInviteLink(ctx, p.OrgID, id, p.UserID)
		if err != nil {
			if errors.Is(err, botusecase.ErrNotConfigured) {
				return nil, huma.Error503ServiceUnavailable(msgBotUnavailable.For(lang))
			}
			return nil, mapStudentError(lang, err, log) // student-not-found → 404, else 500
		}
		return &telegramLinkOutput{Body: telegramLinkResponse{
			Link:      link.Link,
			Token:     link.Token,
			ExpiresAt: link.ExpiresAt.UTC().Format(time.RFC3339),
		}}, nil
	})
}

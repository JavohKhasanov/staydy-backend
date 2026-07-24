package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	chatusecase "github.com/student-success/backend/internal/usecase/chat"
)

type chatTurnInput struct {
	Role string `json:"role" enum:"user,assistant"`
	Text string `json:"text" maxLength:"4000"`
}

type studentChatInput struct {
	Body struct {
		Message string          `json:"message" minLength:"1" maxLength:"2000"`
		History []chatTurnInput `json:"history,omitempty" doc:"recent turns for context (oldest first)"`
	}
}

type chatReplyOutput struct {
	Body struct {
		Reply string `json:"reply"`
	}
}

// registerStudentChat mounts the student AI study-assistant chat. Mount on the student group.
func registerStudentChat(api huma.API, svc *chatusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-chat",
		Method:      http.MethodPost,
		Path:        "/student/chat",
		Summary:     "Send a message to the AI study assistant and get a reply",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, in *studentChatInput) (*chatReplyOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		history := make([]chatusecase.Turn, 0, len(in.Body.History))
		for _, t := range in.Body.History {
			history = append(history, chatusecase.Turn{Role: t.Role, Text: t.Text})
		}
		reply := svc.Reply(ctx, p.UserID, p.FullName, in.Body.Message, history)
		out := &chatReplyOutput{}
		out.Body.Reply = reply
		return out, nil
	})
}

package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	notifyusecase "github.com/student-success/backend/internal/usecase/notify"
)

type notificationResponse struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Body      string `json:"body,omitempty"`
	Link      string `json:"link,omitempty"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}

type notificationsListOutput struct{ Body []notificationResponse }
type unreadCountOutput struct {
	Body struct {
		Count int `json:"count"`
	}
}
type notifIDInput struct {
	ID string `path:"id" format:"uuid"`
}

func toNotification(n entity.Notification) notificationResponse {
	return notificationResponse{
		ID:        n.ID.String(),
		Kind:      n.Kind,
		Title:     n.Title,
		Body:      n.Body,
		Link:      n.Link,
		Read:      n.ReadAt != nil,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// registerNotifications mounts the signed-in user's in-app notification feed. Every back-office
// user reads only their own notifications, so mount on protectedAPI.
func registerNotifications(api huma.API, svc *notifyusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "notifications-list",
		Method:      http.MethodGet,
		Path:        "/notifications",
		Summary:     "The signed-in user's recent notifications",
		Tags:        []string{"notifications"},
		Errors:      []int{http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*notificationsListOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		items, err := svc.List(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("notifications list failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &notificationsListOutput{Body: make([]notificationResponse, 0, len(items))}
		for _, n := range items {
			out.Body = append(out.Body, toNotification(n))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "notifications-unread-count",
		Method:      http.MethodGet,
		Path:        "/notifications/unread-count",
		Summary:     "How many unread notifications the signed-in user has",
		Tags:        []string{"notifications"},
		Errors:      []int{http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*unreadCountOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		n, err := svc.CountUnread(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("notifications unread-count failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &unreadCountOutput{}
		out.Body.Count = n
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "notifications-read",
		Method:        http.MethodPost,
		Path:          "/notifications/{id}/read",
		Summary:       "Mark a notification read",
		Tags:          []string{"notifications"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}), func(ctx context.Context, in *notifIDInput) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid id")
		}
		if err := svc.MarkRead(ctx, p.OrgID, id, p.UserID); err != nil {
			log.Error().Err(err).Msg("notifications read failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &noContentOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "notifications-read-all",
		Method:        http.MethodPost,
		Path:          "/notifications/read-all",
		Summary:       "Mark all of the signed-in user's notifications read",
		Tags:          []string{"notifications"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		if err := svc.MarkAllRead(ctx, p.OrgID, p.UserID); err != nil {
			log.Error().Err(err).Msg("notifications read-all failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &noContentOutput{}, nil
	})
}

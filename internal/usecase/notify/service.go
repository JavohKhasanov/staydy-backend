// Package notify is the in-app notification feed for back-office users (staff alerts stay in the
// platform — no Telegram for staff) plus the SLA escalation that turns overdue at-risk students
// into a director notification.
package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

type Service struct {
	repo repo.NotificationRepository
	log  zerolog.Logger
}

func NewService(r repo.NotificationRepository, log zerolog.Logger) *Service {
	return &Service{repo: r, log: log}
}

// Notify creates one notification for a user (implements the intervention Notifier interface). A
// failure is logged, never propagated — a notification must not break the action that triggered it.
func (s *Service) Notify(ctx context.Context, orgID, userID uuid.UUID, kind, title, body, link string) {
	if _, err := s.repo.Create(ctx, orgID, userID, kind, title, body, link); err != nil {
		s.log.Warn().Err(err).Str("user", userID.String()).Msg("notify: create notification")
	}
}

func (s *Service) List(ctx context.Context, orgID, userID uuid.UUID) ([]entity.Notification, error) {
	return s.repo.List(ctx, orgID, userID)
}

func (s *Service) CountUnread(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	return s.repo.CountUnread(ctx, orgID, userID)
}

func (s *Service) MarkRead(ctx context.Context, orgID, id, userID uuid.UUID) error {
	return s.repo.MarkRead(ctx, orgID, id, userID)
}

func (s *Service) MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, orgID, userID)
}

// EscalateStaleTasks notifies each director of every unresolved task older than the cutoff, then
// marks those tasks escalated (once). Returns the number of notifications created.
func (s *Service) EscalateStaleTasks(ctx context.Context, orgID uuid.UUID, cutoff time.Time) (int, error) {
	stale, err := s.repo.StaleTasks(ctx, orgID, cutoff)
	if err != nil {
		return 0, err
	}
	if len(stale) == 0 {
		return 0, nil
	}
	directors, err := s.repo.DirectorIDs(ctx, orgID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, t := range stale {
		for _, d := range directors {
			s.Notify(ctx, orgID, d, entity.NotifyTaskSLA,
				"Xavfli talaba — muddat o'tdi",
				fmt.Sprintf("%s bo'yicha intervention hali hal qilinmagan.", t.StudentName),
				"/students/"+t.StudentID.String())
			count++
		}
		if err := s.repo.MarkTaskEscalated(ctx, orgID, t.ID); err != nil {
			s.log.Warn().Err(err).Msg("notify: mark task escalated")
		}
	}
	return count, nil
}

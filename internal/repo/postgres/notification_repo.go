package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

type NotificationRepository struct {
	db *postgres.DB
}

func NewNotificationRepository(db *postgres.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func mapNotification(n sqlc.Notification) entity.Notification {
	return entity.Notification{
		ID:        n.ID,
		OrgID:     n.OrgID,
		UserID:    n.UserID,
		Kind:      n.Kind,
		Title:     n.Title,
		Body:      n.Body,
		Link:      n.Link,
		ReadAt:    tsToPtr(n.ReadAt),
		CreatedAt: n.CreatedAt.Time,
	}
}

func (r *NotificationRepository) Create(ctx context.Context, orgID, userID uuid.UUID, kind, title, body, link string) (entity.Notification, error) {
	var out entity.Notification
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).CreateNotification(ctx, sqlc.CreateNotificationParams{
			OrgID: orgID, UserID: userID, Kind: kind, Title: title, Body: body, Link: link,
		})
		if e != nil {
			return e
		}
		out = mapNotification(row)
		return nil
	})
	return out, err
}

func (r *NotificationRepository) List(ctx context.Context, orgID, userID uuid.UUID) ([]entity.Notification, error) {
	var out []entity.Notification
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListNotifications(ctx, userID)
		if e != nil {
			return e
		}
		out = make([]entity.Notification, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapNotification(row))
		}
		return nil
	})
	return out, err
}

func (r *NotificationRepository) CountUnread(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	var n int64
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		n, e = sqlc.New(tx).CountUnreadNotifications(ctx, userID)
		return e
	})
	return int(n), err
}

func (r *NotificationRepository) MarkRead(ctx context.Context, orgID, id, userID uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{ID: id, UserID: userID})
	})
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).MarkAllNotificationsRead(ctx, userID)
	})
}

// StaleTasks returns unresolved, not-yet-escalated tasks created on/before the cutoff (SLA breach).
func (r *NotificationRepository) StaleTasks(ctx context.Context, orgID uuid.UUID, cutoff time.Time) ([]repo.StaleTask, error) {
	var out []repo.StaleTask
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListTasksForEscalation(ctx, tsVal(cutoff))
		if e != nil {
			return e
		}
		out = make([]repo.StaleTask, 0, len(rows))
		for _, row := range rows {
			out = append(out, repo.StaleTask{ID: row.ID, StudentID: row.StudentID, StudentName: row.StudentName})
		}
		return nil
	})
	return out, err
}

func (r *NotificationRepository) MarkTaskEscalated(ctx context.Context, orgID, taskID uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).MarkTaskEscalated(ctx, taskID)
	})
}

func (r *NotificationRepository) DirectorIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		ids, e := sqlc.New(tx).ListOrgDirectorIDs(ctx)
		if e != nil {
			return e
		}
		out = ids
		return nil
	})
	return out, err
}

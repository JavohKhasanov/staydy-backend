package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// BotRepository persists the Telegram bot's state. invite_tokens + bot_conversations are non-RLS,
// so those calls use the pool directly (q on db.Pool); SetStudentChat writes a tenant table, so
// it runs inside WithTenant.
type BotRepository struct {
	db *postgres.DB
	q  *sqlc.Queries
}

func NewBotRepository(db *postgres.DB) *BotRepository {
	return &BotRepository{db: db, q: sqlc.New(db.Pool)}
}

func (r *BotRepository) CreateInvite(ctx context.Context, token string, orgID, studentID, createdBy uuid.UUID, expiresAt time.Time) error {
	_, err := r.q.CreateInviteToken(ctx, sqlc.CreateInviteTokenParams{
		Token:     token,
		OrgID:     orgID,
		StudentID: uuidPtr(studentID),
		CreatedBy: uuidPtr(createdBy),
		ExpiresAt: tsVal(expiresAt),
	})
	return err
}

func (r *BotRepository) ResolveInvite(ctx context.Context, token string) (repo.BotInvite, error) {
	row, err := r.q.GetInviteToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.BotInvite{}, repo.ErrNotFound
		}
		return repo.BotInvite{}, err
	}
	return repo.BotInvite{
		Token:     row.Token,
		OrgID:     row.OrgID,
		StudentID: pgUUID(row.StudentID),
		Used:      row.UsedAt.Valid,
		Expired:   row.ExpiresAt.Valid && row.ExpiresAt.Time.Before(time.Now()),
	}, nil
}

func (r *BotRepository) UseInvite(ctx context.Context, token string) error {
	return r.q.MarkInviteTokenUsed(ctx, token)
}

func (r *BotRepository) BindChat(ctx context.Context, chatID int64, orgID, studentID uuid.UUID) error {
	_, err := r.q.BindBotChat(ctx, sqlc.BindBotChatParams{
		TelegramChatID: chatID,
		OrgID:          uuidPtr(orgID),
		StudentID:      uuidPtr(studentID),
	})
	return err
}

func (r *BotRepository) GetConversation(ctx context.Context, chatID int64) (repo.BotConversation, error) {
	row, err := r.q.GetBotConversation(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.BotConversation{}, repo.ErrNotFound
		}
		return repo.BotConversation{}, err
	}
	return repo.BotConversation{
		ChatID:    row.TelegramChatID,
		OrgID:     pgUUID(row.OrgID),
		StudentID: pgUUID(row.StudentID),
		Flow:      textToString(row.Flow),
		Step:      textToString(row.Step),
		State:     row.State,
	}, nil
}

func (r *BotRepository) SetFlow(ctx context.Context, chatID int64, flow, step string, state []byte) error {
	if len(state) == 0 {
		state = []byte("{}")
	}
	return r.q.SetBotConversationFlow(ctx, sqlc.SetBotConversationFlowParams{
		TelegramChatID: chatID,
		Flow:           textVal(flow),
		Step:           textVal(step),
		State:          state,
	})
}

// SetStudentChat writes students.telegram_chat_id, which is RLS-scoped, so it runs in a tenant tx.
func (r *BotRepository) SetStudentChat(ctx context.Context, orgID, studentID uuid.UUID, chatID int64) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).SetStudentTelegramChat(ctx, sqlc.SetStudentTelegramChatParams{
			ID:             studentID,
			TelegramChatID: pgInt8(chatID),
		})
	})
}

// ListObstacleLabels returns a center's active obstacle choices (ordered) for the check-in
// keyboard. obstacle_options is RLS-protected, so the read runs inside the tenant scope.
func (r *BotRepository) ListObstacleLabels(ctx context.Context, orgID uuid.UUID) ([]string, error) {
	var labels []string
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListObstacleOptions(ctx, orgID)
		if e != nil {
			return e
		}
		labels = make([]string, 0, len(rows))
		for _, row := range rows {
			labels = append(labels, row.Label)
		}
		return nil
	})
	return labels, err
}

// ListLinkedChats returns every chat bound to a student (non-RLS, pool query).
func (r *BotRepository) ListLinkedChats(ctx context.Context) ([]repo.LinkedChat, error) {
	rows, err := r.q.ListLinkedChats(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]repo.LinkedChat, 0, len(rows))
	for _, row := range rows {
		out = append(out, repo.LinkedChat{
			ChatID:    row.TelegramChatID,
			OrgID:     pgUUID(row.OrgID),
			StudentID: pgUUID(row.StudentID),
		})
	}
	return out, nil
}

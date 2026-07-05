package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// SessionRepository implements repo.SessionRepository (refresh-token sessions).
type SessionRepository struct {
	db *postgres.DB
}

func NewSessionRepository(db *postgres.DB) *SessionRepository { return &SessionRepository{db: db} }

func (r *SessionRepository) CreateSession(ctx context.Context, s entity.RefreshSession) (entity.RefreshSession, error) {
	row, err := sqlc.New(r.db.Pool).CreateRefreshSession(ctx, sqlc.CreateRefreshSessionParams{
		UserID:    s.UserID,
		OrgID:     s.OrgID,
		TokenHash: s.TokenHash,
		UserAgent: textVal(s.UserAgent),
		Ip:        textVal(s.IP),
		ExpiresAt: tsVal(s.ExpiresAt),
	})
	if err != nil {
		return entity.RefreshSession{}, err
	}
	return mapSession(row), nil
}

// RotateSession locks the presented session FOR UPDATE, then either rotates it (revoke +
// insert replacement) atomically, or — on replay of an already-revoked token — revokes
// the whole token family and returns ErrSessionReused.
func (r *SessionRepository) RotateSession(ctx context.Context, p repo.RotateParams) (repo.RotateResult, error) {
	var (
		res    repo.RotateResult
		reused bool
	)
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		old, err := q.LockRefreshSessionByHash(ctx, p.OldTokenHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return repo.ErrNotFound
			}
			return err
		}
		if old.RevokedAt.Valid {
			// Replay of an already-rotated token → compromise: revoke the whole family,
			// then COMMIT it (return nil) and signal reuse to the caller afterwards. If we
			// returned the error here, WithTx would roll the revocation back.
			if e := q.RevokeAllUserSessions(ctx, old.UserID); e != nil {
				return e
			}
			reused = true
			return nil
		}
		if old.ExpiresAt.Time.Before(time.Now()) {
			return repo.ErrNotFound
		}
		if e := q.RevokeRefreshSession(ctx, old.ID); e != nil {
			return e
		}
		if _, e := q.CreateRefreshSession(ctx, sqlc.CreateRefreshSessionParams{
			UserID:    old.UserID,
			OrgID:     old.OrgID,
			TokenHash: p.NewTokenHash,
			UserAgent: textVal(p.UserAgent),
			Ip:        textVal(p.IP),
			ExpiresAt: tsVal(p.ExpiresAt),
		}); e != nil {
			return e
		}
		res = repo.RotateResult{UserID: old.UserID, OrgID: old.OrgID}
		return nil
	})
	if err != nil {
		return repo.RotateResult{}, err
	}
	if reused {
		return repo.RotateResult{}, repo.ErrSessionReused
	}
	return res, nil
}

func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	return sqlc.New(r.db.Pool).DeleteExpiredRefreshSessions(ctx)
}

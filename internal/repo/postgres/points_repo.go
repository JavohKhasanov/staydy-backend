package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// PointsRepository records gamification awards and keeps the student's cached XP/coin totals in
// sync, RLS-scoped. Awards are idempotent on (kind, ref).
type PointsRepository struct {
	db *postgres.DB
}

func NewPointsRepository(db *postgres.DB) *PointsRepository { return &PointsRepository{db: db} }

// Award inserts a ledger row and increments the student's totals — but only the first time for a
// given (kind, ref). Returns whether a new award was applied (false when it was a duplicate).
func (r *PointsRepository) Award(ctx context.Context, orgID, studentID uuid.UUID, kind string, xp, coins int, ref string) (bool, error) {
	awarded := false
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		_, e := q.InsertPointsIfNew(ctx, sqlc.InsertPointsIfNewParams{
			OrgID: orgID, StudentID: studentID, Kind: kind,
			Xp: int32(xp), Coins: int32(coins), Ref: ref,
		})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil // duplicate — already awarded, no-op
		}
		if e != nil {
			return e
		}
		awarded = true
		return q.AddStudentPoints(ctx, sqlc.AddStudentPointsParams{ID: studentID, Xp: int32(xp), Coins: int32(coins)})
	})
	return awarded, err
}

// Spend deducts coins for a purchase (guarded by balance) and records a negative ledger row. Returns
// repo.ErrInsufficientCoins when the balance can't cover the price.
func (r *PointsRepository) Spend(ctx context.Context, orgID, studentID uuid.UUID, coins int, ref string) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		if _, e := q.SpendStudentCoins(ctx, sqlc.SpendStudentCoinsParams{ID: studentID, Coins: int32(coins)}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return repo.ErrInsufficientCoins
			}
			return e
		}
		_, e := q.InsertPointsIfNew(ctx, sqlc.InsertPointsIfNewParams{
			OrgID: orgID, StudentID: studentID, Kind: "purchase", Xp: 0, Coins: int32(-coins), Ref: ref,
		})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		return e
	})
}

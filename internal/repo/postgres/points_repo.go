package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
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

// Award grants xp for an action, deriving coins from the config at the student's current level, and
// increments their totals — but only the first time for a given (kind, ref). The student row is
// locked so concurrent awards price coins consistently. Returns whether a new award was applied.
func (r *PointsRepository) Award(ctx context.Context, orgID, studentID uuid.UUID, kind string, xp int, ref string, cfg entity.GamificationConfig) (bool, error) {
	// xp == 0 is a no-op; negative xp is a valid manual penalty (deducts, floored at 0 in the UPDATE).
	if xp == 0 {
		return false, nil
	}
	awarded := false
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var curXP int
		if e := tx.QueryRow(ctx, `SELECT xp FROM students WHERE id = $1 FOR UPDATE`, studentID).Scan(&curXP); e != nil {
			return e
		}
		coins := cfg.CoinsForAward(xp, curXP)
		q := sqlc.New(tx)
		_, e := q.InsertPointsIfNew(ctx, sqlc.InsertPointsIfNewParams{
			OrgID: orgID, StudentID: studentID, Kind: kind, Xp: int32(xp), Coins: int32(coins), Ref: ref,
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

// GetConfig loads a center's gamification settings (pool-direct; organizations is non-RLS).
func (r *PointsRepository) GetConfig(ctx context.Context, orgID uuid.UUID) (entity.GamificationConfig, error) {
	row, err := sqlc.New(r.db.Pool).GetGamification(ctx, orgID)
	if err != nil {
		return entity.DefaultGamification(), err
	}
	return entity.GamificationConfig{
		XPAttend: int(row.GxXpAttend), XPLate: int(row.GxXpLate), XPHomeworkMax: int(row.GxXpHomeworkMax),
		XPCheckin: int(row.GxXpCheckin), LevelSize: int(row.GxLevelSize),
		CoinBase: int(row.GxCoinBase), CoinStep: int(row.GxCoinStep),
	}, nil
}

// SetConfig saves a center's gamification settings.
func (r *PointsRepository) SetConfig(ctx context.Context, orgID uuid.UUID, cfg entity.GamificationConfig) error {
	return sqlc.New(r.db.Pool).SetGamification(ctx, sqlc.SetGamificationParams{
		ID: orgID, GxXpAttend: int32(cfg.XPAttend), GxXpLate: int32(cfg.XPLate),
		GxXpHomeworkMax: int32(cfg.XPHomeworkMax), GxXpCheckin: int32(cfg.XPCheckin),
		GxLevelSize: int32(cfg.LevelSize), GxCoinBase: int32(cfg.CoinBase), GxCoinStep: int32(cfg.CoinStep),
	})
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

// Leaderboard returns the top active students by XP for the whole center.
func (r *PointsRepository) Leaderboard(ctx context.Context, orgID uuid.UUID, limit int) ([]entity.LeaderRow, error) {
	var out []entity.LeaderRow
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).LeaderboardOrg(ctx, int32(limit))
		if e != nil {
			return e
		}
		out = make([]entity.LeaderRow, 0, len(rows))
		for i, row := range rows {
			out = append(out, entity.LeaderRow{StudentID: row.ID, Name: row.Name, XP: int(row.Xp), Coins: int(row.Coins), Rank: i + 1})
		}
		return nil
	})
	return out, err
}

// LeaderboardByGroup returns the top active students by XP within one group.
func (r *PointsRepository) LeaderboardByGroup(ctx context.Context, orgID, groupID uuid.UUID, limit int) ([]entity.LeaderRow, error) {
	var out []entity.LeaderRow
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).LeaderboardGroup(ctx, sqlc.LeaderboardGroupParams{Column1: groupID, Limit: int32(limit)})
		if e != nil {
			return e
		}
		out = make([]entity.LeaderRow, 0, len(rows))
		for i, row := range rows {
			out = append(out, entity.LeaderRow{StudentID: row.ID, Name: row.Name, XP: int(row.Xp), Coins: int(row.Coins), Rank: i + 1})
		}
		return nil
	})
	return out, err
}

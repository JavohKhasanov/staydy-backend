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

// ShopRepository persists reward-shop items + purchases, RLS-scoped. A purchase atomically deducts
// coins, records the purchase, and writes a ledger entry.
type ShopRepository struct {
	db *postgres.DB
}

func NewShopRepository(db *postgres.DB) *ShopRepository { return &ShopRepository{db: db} }

func mapShopItem(i sqlc.ShopItem) entity.ShopItem {
	return entity.ShopItem{
		ID: i.ID, OrgID: i.OrgID, Name: i.Name, Icon: i.Icon,
		Price: int(i.Price), IsActive: i.IsActive, CreatedAt: i.CreatedAt.Time,
	}
}

func (r *ShopRepository) CreateItem(ctx context.Context, orgID uuid.UUID, name, icon string, price int, active bool) (entity.ShopItem, error) {
	var it entity.ShopItem
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).CreateShopItem(ctx, sqlc.CreateShopItemParams{
			OrgID: orgID, Name: name, Icon: icon, Price: int32(price), IsActive: active,
		})
		if e != nil {
			return e
		}
		it = mapShopItem(row)
		return nil
	})
	return it, err
}

func (r *ShopRepository) ListItems(ctx context.Context, orgID uuid.UUID) ([]entity.ShopItem, error) {
	var out []entity.ShopItem
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListShopItems(ctx)
		if e != nil {
			return e
		}
		out = make([]entity.ShopItem, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapShopItem(row))
		}
		return nil
	})
	return out, err
}

func (r *ShopRepository) UpdateItem(ctx context.Context, orgID, id uuid.UUID, name, icon string, price int, active bool) (entity.ShopItem, error) {
	var it entity.ShopItem
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).UpdateShopItem(ctx, sqlc.UpdateShopItemParams{
			ID: id, Name: name, Icon: icon, Price: int32(price), IsActive: active,
		})
		if e != nil {
			return e
		}
		it = mapShopItem(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ShopItem{}, repo.ErrNotFound
	}
	return it, err
}

func (r *ShopRepository) DeleteItem(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteShopItem(ctx, id)
	})
}

func (r *ShopRepository) ListForStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentShopItem, error) {
	var out []entity.StudentShopItem
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListShopForStudent(ctx, studentID)
		if e != nil {
			return e
		}
		out = make([]entity.StudentShopItem, 0, len(rows))
		for _, row := range rows {
			owned, _ := row.Owned.(bool)
			out = append(out, entity.StudentShopItem{
				ID: row.ID, Name: row.Name, Icon: row.Icon, Price: int(row.Price), Owned: owned,
			})
		}
		return nil
	})
	return out, err
}

// BuyItem atomically records the purchase, deducts coins (guarded by balance), and writes a negative
// ledger entry. Returns ErrAlreadyExists if already owned or ErrInsufficientCoins if the balance is
// too low.
func (r *ShopRepository) BuyItem(ctx context.Context, orgID, studentID, itemID uuid.UUID, price int) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		// Reserve the purchase first; a conflict means already owned (no coins spent).
		if _, e := q.InsertPurchase(ctx, sqlc.InsertPurchaseParams{
			OrgID: orgID, StudentID: studentID, ItemID: itemID, Price: int32(price),
		}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return repo.ErrAlreadyExists
			}
			return e
		}
		// Deduct coins only if the balance covers it.
		if _, e := q.SpendStudentCoins(ctx, sqlc.SpendStudentCoinsParams{ID: studentID, Coins: int32(price)}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return repo.ErrInsufficientCoins
			}
			return e
		}
		// Ledger entry (idempotent per item).
		_, e := q.InsertPointsIfNew(ctx, sqlc.InsertPointsIfNewParams{
			OrgID: orgID, StudentID: studentID, Kind: entity.PointPurchase, Xp: 0, Coins: int32(-price), Ref: "shop:" + itemID.String(),
		})
		if errors.Is(e, pgx.ErrNoRows) {
			return nil
		}
		return e
	})
}

func (r *ShopRepository) GetItem(ctx context.Context, orgID, id uuid.UUID) (entity.ShopItem, error) {
	var it entity.ShopItem
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).GetShopItem(ctx, id)
		if e != nil {
			return e
		}
		it = mapShopItem(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ShopItem{}, repo.ErrNotFound
	}
	return it, err
}

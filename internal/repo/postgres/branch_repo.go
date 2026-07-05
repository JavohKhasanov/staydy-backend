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

// BranchRepository persists branches (filiallar). RLS-scoped to the org.
type BranchRepository struct {
	db *postgres.DB
}

func NewBranchRepository(db *postgres.DB) *BranchRepository { return &BranchRepository{db: db} }

func (r *BranchRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.Branch, error) {
	var out []entity.Branch
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListBranches(ctx, orgID)
		if e != nil {
			return e
		}
		out = make([]entity.Branch, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapBranch(row))
		}
		return nil
	})
	return out, err
}

func (r *BranchRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.BranchParams) (entity.Branch, error) {
	var b entity.Branch
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).CreateBranch(ctx, sqlc.CreateBranchParams{
			OrgID:   orgID,
			Name:    p.Name,
			Address: p.Address,
			Phone:   p.Phone,
		})
		if e != nil {
			return e
		}
		b = mapBranch(row)
		return nil
	})
	return b, err
}

func (r *BranchRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.BranchParams) (entity.Branch, error) {
	var b entity.Branch
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).UpdateBranch(ctx, sqlc.UpdateBranchParams{
			ID:       id,
			Name:     p.Name,
			Address:  p.Address,
			Phone:    p.Phone,
			IsActive: p.IsActive,
		})
		if e != nil {
			return e
		}
		b = mapBranch(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Branch{}, repo.ErrNotFound
	}
	return b, err
}

func (r *BranchRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteBranch(ctx, id)
	})
}

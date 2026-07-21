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

// StaffRepository manages back-office staff accounts (users with role center_admin or finance),
// RLS-scoped via WithTenant. Teachers have their own repo; the platform owner is out of scope.
type StaffRepository struct {
	db *postgres.DB
}

func NewStaffRepository(db *postgres.DB) *StaffRepository { return &StaffRepository{db: db} }

func (r *StaffRepository) Create(ctx context.Context, orgID uuid.UUID, email, passwordHash, fullName, role string) (entity.User, error) {
	var row sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateStaffUser(ctx, sqlc.CreateStaffUserParams{
			OrgID:        orgID,
			Email:        email,
			PasswordHash: passwordHash,
			FullName:     fullName,
			Role:         role,
		})
		return e
	})
	if err != nil {
		if isUniqueViolation(err) {
			return entity.User{}, repo.ErrAlreadyExists
		}
		return entity.User{}, err
	}
	return mapUser(row), nil
}

func (r *StaffRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.User, error) {
	var rows []sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListStaffUsers(ctx)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.User, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapUser(row))
	}
	return out, nil
}

func (r *StaffRepository) Update(ctx context.Context, orgID, id uuid.UUID, email, fullName, role string) (entity.User, error) {
	var row sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateStaffUser(ctx, sqlc.UpdateStaffUserParams{ID: id, FullName: fullName, Email: email, Role: role})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, repo.ErrNotFound
		}
		if isUniqueViolation(err) {
			return entity.User{}, repo.ErrAlreadyExists
		}
		return entity.User{}, err
	}
	return mapUser(row), nil
}

func (r *StaffRepository) SetPassword(ctx context.Context, orgID, id uuid.UUID, passwordHash string) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: id, PasswordHash: passwordHash})
	})
}

func (r *StaffRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteStaffUser(ctx, id)
	})
}

// CountByRole reports how many users in the org hold the given role — used to block removing the
// last administrator (which would lock the center out).
func (r *StaffRepository) CountByRole(ctx context.Context, orgID uuid.UUID, role string) (int64, error) {
	var n int64
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		n, e = sqlc.New(tx).CountStaffByRole(ctx, role)
		return e
	})
	return n, err
}

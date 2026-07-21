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

// AuthRepository implements repo.AuthRepository over pgx + sqlc, encapsulating the
// RLS-scoped provisioning transaction.
type AuthRepository struct {
	db *postgres.DB
}

func NewAuthRepository(db *postgres.DB) *AuthRepository { return &AuthRepository{db: db} }

func (r *AuthRepository) ProvisionOrgWithAdmin(ctx context.Context, p repo.ProvisionParams) (entity.Organization, entity.User, error) {
	var (
		orgRow  sqlc.Organization
		userRow sqlc.User
	)
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var err error
		orgRow, err = q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
			Name: p.OrgName,
			Slug: p.Slug,
			Plan: p.Plan,
		})
		if err != nil {
			return err
		}
		// Scope the tx to the new org so the users RLS policy admits the insert.
		if err := postgres.SetTenant(ctx, tx, orgRow.ID.String()); err != nil {
			return err
		}
		userRow, err = q.CreateUser(ctx, sqlc.CreateUserParams{
			OrgID:        orgRow.ID,
			Email:        p.Email,
			PasswordHash: p.PasswordHash,
			FullName:     p.FullName,
			Role:         string(p.Role),
		})
		return err
	})
	if err != nil {
		if isUniqueViolation(err) {
			return entity.Organization{}, entity.User{}, repo.ErrAlreadyExists
		}
		return entity.Organization{}, entity.User{}, err
	}
	return mapOrg(orgRow), mapUser(userRow), nil
}

func (r *AuthRepository) OrgBySlug(ctx context.Context, slug string) (entity.Organization, error) {
	row, err := sqlc.New(r.db.Pool).GetOrganizationBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Organization{}, repo.ErrNotFound
		}
		return entity.Organization{}, err
	}
	return mapOrg(row), nil
}

func (r *AuthRepository) OrgByID(ctx context.Context, id uuid.UUID) (entity.Organization, error) {
	row, err := sqlc.New(r.db.Pool).GetOrganizationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Organization{}, repo.ErrNotFound
		}
		return entity.Organization{}, err
	}
	return mapOrg(row), nil
}

func (r *AuthRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	return sqlc.New(r.db.Pool).SlugExists(ctx, slug)
}

// AllOrgIDs lists every center (non-RLS) for the cross-tenant scheduler.
func (r *AuthRepository) AllOrgIDs(ctx context.Context) ([]uuid.UUID, error) {
	return sqlc.New(r.db.Pool).ListAllOrgIDs(ctx)
}

func (r *AuthRepository) UserByEmailInOrg(ctx context.Context, orgID uuid.UUID, email string) (entity.User, error) {
	var row sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).GetUserByEmail(ctx, email)
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, repo.ErrNotFound
		}
		return entity.User{}, err
	}
	return mapUser(row), nil
}

func (r *AuthRepository) UserByID(ctx context.Context, orgID, id uuid.UUID) (entity.User, error) {
	var row sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).GetUserByID(ctx, id)
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, repo.ErrNotFound
		}
		return entity.User{}, err
	}
	return mapUser(row), nil
}

// SetPassword updates a user's password hash (self-service change-password), RLS-scoped.
func (r *AuthRepository) SetPassword(ctx context.Context, orgID, id uuid.UUID, hash string) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: id, PasswordHash: hash})
	})
}

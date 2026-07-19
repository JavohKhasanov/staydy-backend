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

// TeacherRepository implements repo.TeacherRepository over the users table (role 'teacher'),
// RLS-scoped via WithTenant.
type TeacherRepository struct {
	db *postgres.DB
}

func NewTeacherRepository(db *postgres.DB) *TeacherRepository { return &TeacherRepository{db: db} }

func (r *TeacherRepository) Create(ctx context.Context, orgID uuid.UUID, email, passwordHash, fullName string) (entity.User, error) {
	var row sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateUser(ctx, sqlc.CreateUserParams{
			OrgID:        orgID,
			Email:        email,
			PasswordHash: passwordHash,
			FullName:     fullName,
			Role:         string(entity.RoleTeacher),
		})
		return e
	})
	if err != nil {
		if isUniqueViolation(err) {
			return entity.User{}, repo.ErrAlreadyExists // (org_id, email) already taken
		}
		return entity.User{}, err
	}
	return mapUser(row), nil
}

func (r *TeacherRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.User, error) {
	var rows []sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListUsersByRole(ctx, string(entity.RoleTeacher))
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

func (r *TeacherRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (entity.User, error) {
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

func (r *TeacherRepository) Update(ctx context.Context, orgID, id uuid.UUID, email, fullName string) (entity.User, error) {
	var row sqlc.User
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateTeacher(ctx, sqlc.UpdateTeacherParams{ID: id, FullName: fullName, Email: email})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, repo.ErrNotFound
		}
		if isUniqueViolation(err) {
			return entity.User{}, repo.ErrAlreadyExists // email taken by another account
		}
		return entity.User{}, err
	}
	return mapUser(row), nil
}

func (r *TeacherRepository) SetPassword(ctx context.Context, orgID, id uuid.UUID, passwordHash string) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{ID: id, PasswordHash: passwordHash})
	})
}

func (r *TeacherRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteTeacher(ctx, id)
	})
}

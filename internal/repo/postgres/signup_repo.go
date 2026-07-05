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

// SignupRepository persists landing-page signup requests. signup_requests is a platform-level
// (NON-RLS) table, so it is read/written with the pool directly — like organizations.
type SignupRepository struct {
	db *postgres.DB
}

func NewSignupRepository(db *postgres.DB) *SignupRepository { return &SignupRepository{db: db} }

func (r *SignupRepository) Create(ctx context.Context, in entity.SignupRequest) (entity.SignupRequest, error) {
	row, err := sqlc.New(r.db.Pool).CreateSignupRequest(ctx, sqlc.CreateSignupRequestParams{
		CenterName:  in.CenterName,
		ContactName: in.ContactName,
		Phone:       in.Phone,
		Email:       in.Email,
		Plan:        in.Plan,
		Message:     in.Message,
	})
	if err != nil {
		return entity.SignupRequest{}, err
	}
	return mapSignupRequest(row), nil
}

func (r *SignupRepository) List(ctx context.Context) ([]entity.SignupRequest, error) {
	rows, err := sqlc.New(r.db.Pool).ListSignupRequests(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]entity.SignupRequest, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSignupRequest(row))
	}
	return out, nil
}

func (r *SignupRepository) SetStatus(ctx context.Context, id uuid.UUID, status string) (entity.SignupRequest, error) {
	row, err := sqlc.New(r.db.Pool).UpdateSignupRequestStatus(ctx, sqlc.UpdateSignupRequestStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.SignupRequest{}, repo.ErrNotFound
		}
		return entity.SignupRequest{}, err
	}
	return mapSignupRequest(row), nil
}

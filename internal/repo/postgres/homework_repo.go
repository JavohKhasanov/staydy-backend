package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

type HomeworkRepository struct {
	db *postgres.DB
}

func NewHomeworkRepository(db *postgres.DB) *HomeworkRepository {
	return &HomeworkRepository{db: db}
}

func (r *HomeworkRepository) Create(ctx context.Context, orgID, studentID uuid.UUID, date time.Time, done bool) (entity.HomeworkRecord, error) {
	var row sqlc.HomeworkRecord
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateHomework(ctx, sqlc.CreateHomeworkParams{
			OrgID:     orgID,
			StudentID: studentID,
			Date:      pgtype.Date{Time: date, Valid: true},
			IsDone:    done,
		})
		return e
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.HomeworkRecord{}, repo.ErrNotFound
		}
		return entity.HomeworkRecord{}, err
	}
	return mapHomework(row), nil
}

func (r *HomeworkRepository) ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.HomeworkRecord, error) {
	var rows []sqlc.HomeworkRecord
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListHomeworkByStudent(ctx, studentID)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.HomeworkRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapHomework(row))
	}
	return out, nil
}

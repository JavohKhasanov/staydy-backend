package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

type SurveyRepository struct {
	db *postgres.DB
}

func NewSurveyRepository(db *postgres.DB) *SurveyRepository { return &SurveyRepository{db: db} }

func (r *SurveyRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.CreateSurveyParams) (entity.Survey, error) {
	var row sqlc.Survey
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateSurvey(ctx, sqlc.CreateSurveyParams{
			OrgID:           orgID,
			StudentID:       p.StudentID,
			WeekNumber:      int32(p.WeekNumber),
			MotivationScore: int32(p.MotivationScore),
			ProgressScore:   int32(p.ProgressScore),
			BiggestObstacle: textVal(p.BiggestObstacle),
			Comment:         textVal(p.Comment),
		})
		return e
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.Survey{}, repo.ErrNotFound
		}
		return entity.Survey{}, err
	}
	return mapSurvey(row), nil
}

func (r *SurveyRepository) ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Survey, error) {
	var rows []sqlc.Survey
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListSurveysByStudent(ctx, studentID)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.Survey, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSurvey(row))
	}
	return out, nil
}

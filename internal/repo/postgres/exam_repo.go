package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

type ExamRepository struct {
	db *postgres.DB
}

func NewExamRepository(db *postgres.DB) *ExamRepository {
	return &ExamRepository{db: db}
}

func mapExam(e sqlc.Exam) entity.Exam {
	return entity.Exam{
		ID: e.ID, OrgID: e.OrgID, GroupID: e.GroupID, Title: e.Title,
		ExamDate: dateToPtr(e.ExamDate), MaxScore: int(e.MaxScore), CreatedAt: e.CreatedAt.Time,
	}
}

func (r *ExamRepository) Create(ctx context.Context, orgID, groupID uuid.UUID, title string, date *time.Time, maxScore int) (entity.Exam, error) {
	var out entity.Exam
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).CreateExam(ctx, sqlc.CreateExamParams{
			OrgID: orgID, GroupID: groupID, Title: title, ExamDate: dateVal(date), MaxScore: int32(maxScore),
		})
		if e != nil {
			return e
		}
		out = mapExam(row)
		return nil
	})
	if isForeignKeyViolation(err) {
		return entity.Exam{}, repo.ErrNotFound
	}
	return out, err
}

func (r *ExamRepository) ListGroupExams(ctx context.Context, orgID, groupID uuid.UUID) ([]entity.Exam, error) {
	var out []entity.Exam
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListGroupExams(ctx, groupID)
		if e != nil {
			return e
		}
		out = make([]entity.Exam, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.Exam{
				ID: row.ID, OrgID: row.OrgID, GroupID: row.GroupID, Title: row.Title,
				ExamDate: dateToPtr(row.ExamDate), MaxScore: int(row.MaxScore), CreatedAt: row.CreatedAt.Time,
				ResultCount: row.ResultCount,
			})
		}
		return nil
	})
	return out, err
}

func (r *ExamRepository) GetExam(ctx context.Context, orgID, id uuid.UUID) (entity.Exam, error) {
	var out entity.Exam
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).GetExam(ctx, id)
		if e != nil {
			return e
		}
		out = mapExam(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Exam{}, repo.ErrNotFound
	}
	return out, err
}

func (r *ExamRepository) DeleteExam(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteExam(ctx, id)
	})
}

func (r *ExamRepository) UpsertResult(ctx context.Context, orgID, examID, studentID uuid.UUID, score int) (entity.ExamResult, error) {
	var out entity.ExamResult
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).UpsertExamResult(ctx, sqlc.UpsertExamResultParams{
			OrgID: orgID, ExamID: examID, StudentID: studentID, Score: int32(score),
		})
		if e != nil {
			return e
		}
		out = entity.ExamResult{ID: row.ID, ExamID: row.ExamID, StudentID: row.StudentID, Score: int(row.Score)}
		return nil
	})
	if isForeignKeyViolation(err) {
		return entity.ExamResult{}, repo.ErrNotFound
	}
	return out, err
}

func (r *ExamRepository) ListResults(ctx context.Context, orgID, examID uuid.UUID) ([]entity.ExamResult, error) {
	var out []entity.ExamResult
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListExamResults(ctx, examID)
		if e != nil {
			return e
		}
		out = make([]entity.ExamResult, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.ExamResult{
				ID: row.ID, ExamID: row.ExamID, StudentID: row.StudentID,
				StudentName: row.StudentName, Score: int(row.Score),
			})
		}
		return nil
	})
	return out, err
}

func (r *ExamRepository) StudentResults(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentExamResult, error) {
	var out []entity.StudentExamResult
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListStudentExamResults(ctx, studentID)
		if e != nil {
			return e
		}
		out = make([]entity.StudentExamResult, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.StudentExamResult{
				ExamID: row.ExamID, Title: row.Title, ExamDate: dateToPtr(row.ExamDate),
				MaxScore: int(row.MaxScore), Score: int(row.Score),
			})
		}
		return nil
	})
	return out, err
}

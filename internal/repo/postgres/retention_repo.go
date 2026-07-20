package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// RetentionRepository runs each survey/attendance write together with the risk recompute
// (and idempotent task auto-open) in a single tenant transaction.
type RetentionRepository struct {
	db *postgres.DB
}

func NewRetentionRepository(db *postgres.DB) *RetentionRepository { return &RetentionRepository{db: db} }

func (r *RetentionRepository) SubmitSurvey(ctx context.Context, orgID, studentID uuid.UUID, p repo.CreateSurveyParams, fn repo.Recompute) (entity.Survey, error) {
	var survey sqlc.Survey
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var e error
		survey, e = q.CreateSurvey(ctx, sqlc.CreateSurveyParams{
			OrgID:           orgID,
			StudentID:       studentID,
			WeekNumber:      int32(p.WeekNumber),
			MotivationScore: int32(p.MotivationScore),
			ProgressScore:   int32(p.ProgressScore),
			BiggestObstacle: textVal(p.BiggestObstacle),
			Comment:         textVal(p.Comment),
		})
		if e != nil {
			return e
		}
		return recomputeInTx(ctx, q, orgID, studentID, fn)
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.Survey{}, repo.ErrNotFound
		}
		return entity.Survey{}, err
	}
	return mapSurvey(survey), nil
}

func (r *RetentionRepository) RecordAttendance(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID, date time.Time, status string, fn repo.Recompute) (entity.AttendanceRecord, error) {
	var rec sqlc.AttendanceRecord
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var e error
		rec, e = q.CreateAttendance(ctx, sqlc.CreateAttendanceParams{
			OrgID:     orgID,
			StudentID: studentID,
			GroupID:   nullableUUID(groupID),
			Date:      pgtype.Date{Time: date, Valid: true},
			IsPresent: entity.PresentFromStatus(status), // risk-facing signal: only 'absent' is not present
			Status:    status,
		})
		if e != nil {
			return e
		}
		return recomputeInTx(ctx, q, orgID, studentID, fn)
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.AttendanceRecord{}, repo.ErrNotFound
		}
		return entity.AttendanceRecord{}, err
	}
	return mapAttendance(rec), nil
}

func (r *RetentionRepository) RecordHomework(ctx context.Context, orgID, studentID uuid.UUID, date time.Time, done bool, fn repo.Recompute) (entity.HomeworkRecord, error) {
	var rec sqlc.HomeworkRecord
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var e error
		rec, e = q.CreateHomework(ctx, sqlc.CreateHomeworkParams{
			OrgID:     orgID,
			StudentID: studentID,
			Date:      pgtype.Date{Time: date, Valid: true},
			IsDone:    done,
		})
		if e != nil {
			return e
		}
		return recomputeInTx(ctx, q, orgID, studentID, fn)
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.HomeworkRecord{}, repo.ErrNotFound
		}
		return entity.HomeworkRecord{}, err
	}
	return mapHomework(rec), nil
}

// RecordAdviceFeedback upserts a mentor's rating of a student's AI advice (no risk recompute).
func (r *RetentionRepository) RecordAdviceFeedback(ctx context.Context, orgID, studentID, userID uuid.UUID, useful bool) error {
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).CreateAdviceFeedback(ctx, sqlc.CreateAdviceFeedbackParams{
			OrgID:     orgID,
			StudentID: studentID,
			UserID:    userID,
			Useful:    useful,
		})
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return repo.ErrNotFound
		}
		return err
	}
	return nil
}

// RecomputeStudent re-runs the risk computation for one student with no preceding write
// (used by the scheduler to apply time-based trigger rules).
func (r *RetentionRepository) RecomputeStudent(ctx context.Context, orgID, studentID uuid.UUID, fn repo.Recompute) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return recomputeInTx(ctx, sqlc.New(tx), orgID, studentID, fn)
	})
}

// recomputeInTx reloads the student's signals, applies the injected risk computation,
// persists the new score, and ensures an open task exists when the result calls for one.
// The one-open-per-student unique index makes the task insert idempotent under races.
func recomputeInTx(ctx context.Context, q *sqlc.Queries, orgID, studentID uuid.UUID, fn repo.Recompute) error {
	stRow, err := q.GetStudent(ctx, studentID)
	if err != nil {
		return err
	}
	attRows, err := q.ListAttendanceByStudent(ctx, studentID)
	if err != nil {
		return err
	}
	survRows, err := q.ListSurveysByStudent(ctx, studentID)
	if err != nil {
		return err
	}
	hwRows, err := q.ListHomeworkByStudent(ctx, studentID)
	if err != nil {
		return err
	}

	att := make([]entity.AttendanceRecord, len(attRows))
	for i := range attRows {
		att[i] = mapAttendance(attRows[i])
	}
	surveys := make([]entity.Survey, len(survRows))
	for i := range survRows {
		surveys[i] = mapSurvey(survRows[i])
	}
	homework := make([]entity.HomeworkRecord, len(hwRows))
	for i := range hwRows {
		homework[i] = mapHomework(hwRows[i])
	}

	out := fn(mapStudent(stRow), att, surveys, homework)
	if err := q.UpdateStudentRisk(ctx, sqlc.UpdateStudentRiskParams{
		ID:        studentID,
		RiskScore: int32(out.Score),
		RiskTier:  out.Tier,
	}); err != nil {
		return err
	}

	if len(out.TaskReasons) > 0 {
		// ON CONFLICT DO NOTHING returns no row when a task is already open for this
		// student → ErrNoRows. In that case the task already exists; refresh its reasons so
		// it reflects the latest risk drivers (escalations after the task first opened).
		_, err := q.CreateInterventionTask(ctx, sqlc.CreateInterventionTaskParams{
			OrgID:     orgID,
			StudentID: studentID,
			Reasons:   out.TaskReasons,
			Status:    "Open",
		})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if e := q.UpdateOpenTaskReasons(ctx, sqlc.UpdateOpenTaskReasonsParams{
				OrgID:     orgID,
				StudentID: studentID,
				Reasons:   out.TaskReasons,
			}); e != nil {
				return e
			}
		case err != nil:
			return err
		}
	}
	return nil
}

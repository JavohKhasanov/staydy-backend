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

// AssignmentRepository persists homework assignments + submissions (the LMS-style flow), RLS-scoped.
// Distinct from HomeworkRepository, which handles the legacy homework_records retention flag.
type AssignmentRepository struct {
	db *postgres.DB
}

func NewAssignmentRepository(db *postgres.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

func mapAssignment(a sqlc.HomeworkAssignment) entity.HomeworkAssignment {
	return entity.HomeworkAssignment{
		ID: a.ID, OrgID: a.OrgID, GroupID: a.GroupID,
		LessonDate: dateToPtr(a.LessonDate), Title: a.Title, Description: a.Description,
		Deadline: tsToPtr(a.Deadline), MaxScore: int(a.MaxScore), CreatedAt: a.CreatedAt.Time,
	}
}

func mapSubmission(s sqlc.HomeworkSubmission) entity.HomeworkSubmission {
	return entity.HomeworkSubmission{
		ID: s.ID, AssignmentID: s.AssignmentID, StudentID: s.StudentID,
		Text: s.Text, Links: s.Links, Status: s.Status, Score: int4ToPtr(s.Score),
		ReviewNote: s.ReviewNote, SubmittedAt: s.SubmittedAt.Time, ReviewedAt: tsToPtr(s.ReviewedAt),
	}
}

func (r *AssignmentRepository) CreateAssignment(ctx context.Context, orgID uuid.UUID, p repo.CreateAssignmentParams) (entity.HomeworkAssignment, error) {
	var a entity.HomeworkAssignment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).CreateHomeworkAssignment(ctx, sqlc.CreateHomeworkAssignmentParams{
			OrgID:       orgID,
			GroupID:     p.GroupID,
			LessonDate:  dateVal(p.LessonDate),
			Title:       p.Title,
			Description: p.Description,
			Deadline:    tsPtrVal(p.Deadline),
			MaxScore:    int32(p.MaxScore),
		})
		if e != nil {
			return e
		}
		a = mapAssignment(row)
		return nil
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.HomeworkAssignment{}, repo.ErrNotFound
		}
		return entity.HomeworkAssignment{}, err
	}
	return a, nil
}

func (r *AssignmentRepository) ListGroupAssignments(ctx context.Context, orgID, groupID uuid.UUID) ([]entity.HomeworkAssignment, error) {
	var out []entity.HomeworkAssignment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListGroupAssignments(ctx, groupID)
		if e != nil {
			return e
		}
		out = make([]entity.HomeworkAssignment, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.HomeworkAssignment{
				ID: row.ID, OrgID: row.OrgID, GroupID: row.GroupID,
				LessonDate: dateToPtr(row.LessonDate), Title: row.Title, Description: row.Description,
				Deadline: tsToPtr(row.Deadline), MaxScore: int(row.MaxScore), CreatedAt: row.CreatedAt.Time,
				SubmissionCount: row.SubmissionCount,
			})
		}
		return nil
	})
	return out, err
}

func (r *AssignmentRepository) GetAssignment(ctx context.Context, orgID, id uuid.UUID) (entity.HomeworkAssignment, error) {
	var a entity.HomeworkAssignment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).GetAssignment(ctx, id)
		if e != nil {
			return e
		}
		a = mapAssignment(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.HomeworkAssignment{}, repo.ErrNotFound
	}
	return a, err
}

func (r *AssignmentRepository) DeleteAssignment(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteAssignment(ctx, id)
	})
}

func (r *AssignmentRepository) ListSubmissions(ctx context.Context, orgID, assignmentID uuid.UUID) ([]entity.HomeworkSubmission, error) {
	var out []entity.HomeworkSubmission
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListAssignmentSubmissions(ctx, assignmentID)
		if e != nil {
			return e
		}
		out = make([]entity.HomeworkSubmission, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.HomeworkSubmission{
				ID: row.ID, AssignmentID: row.AssignmentID, StudentID: row.StudentID,
				StudentName: row.StudentName, Text: row.Text, Links: row.Links, Status: row.Status,
				Score: int4ToPtr(row.Score), ReviewNote: row.ReviewNote,
				SubmittedAt: row.SubmittedAt.Time, ReviewedAt: tsToPtr(row.ReviewedAt),
			})
		}
		return nil
	})
	return out, err
}

func (r *AssignmentRepository) Grade(ctx context.Context, orgID, submissionID uuid.UUID, status string, score *int, note string) (entity.HomeworkSubmission, error) {
	var s entity.HomeworkSubmission
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).GradeSubmission(ctx, sqlc.GradeSubmissionParams{
			ID: submissionID, Status: status, Score: pgInt4Ptr(score), ReviewNote: note,
		})
		if e != nil {
			return e
		}
		s = mapSubmission(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.HomeworkSubmission{}, repo.ErrNotFound
	}
	return s, err
}

// GroupForSubmission returns the group id a submission belongs to (via its assignment), so the
// usecase can verify a grading teacher actually owns that group.
func (r *AssignmentRepository) GroupForSubmission(ctx context.Context, orgID, submissionID uuid.UUID) (uuid.UUID, error) {
	var gid uuid.UUID
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		g, e := sqlc.New(tx).GetSubmissionGroup(ctx, submissionID)
		if e != nil {
			return e
		}
		gid = g
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, repo.ErrNotFound
	}
	return gid, err
}

// GetAssignmentForStudent returns the assignment only if the student is a member of its group.
func (r *AssignmentRepository) GetAssignmentForStudent(ctx context.Context, orgID, assignmentID, studentID uuid.UUID) (entity.HomeworkAssignment, error) {
	var a entity.HomeworkAssignment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).GetAssignmentForStudent(ctx, sqlc.GetAssignmentForStudentParams{ID: assignmentID, Column2: studentID})
		if e != nil {
			return e
		}
		a = mapAssignment(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.HomeworkAssignment{}, repo.ErrNotFound
	}
	return a, err
}

func (r *AssignmentRepository) UpsertSubmission(ctx context.Context, orgID, assignmentID, studentID uuid.UUID, text, links string) (entity.HomeworkSubmission, error) {
	var s entity.HomeworkSubmission
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).UpsertSubmission(ctx, sqlc.UpsertSubmissionParams{
			OrgID: orgID, AssignmentID: assignmentID, StudentID: studentID, Text: text, Links: links,
		})
		if e != nil {
			return e
		}
		s = mapSubmission(row)
		return nil
	})
	return s, err
}

func (r *AssignmentRepository) ListStudentAssignments(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentAssignment, error) {
	var out []entity.StudentAssignment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListStudentAssignments(ctx, studentID)
		if e != nil {
			return e
		}
		out = make([]entity.StudentAssignment, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.StudentAssignment{
				ID: row.ID, GroupID: row.GroupID, GroupName: row.GroupName,
				LessonDate: dateToPtr(row.LessonDate), Title: row.Title, Description: row.Description,
				Deadline: tsToPtr(row.Deadline), MaxScore: int(row.MaxScore),
				SubmissionID: uuidToPtr(row.SubmissionID), SubmissionStatus: row.SubmissionStatus,
				Score: int4ToPtr(row.Score), SubmissionText: row.SubmissionText,
				SubmissionLinks: row.SubmissionLinks, ReviewNote: row.ReviewNote,
				SubmittedAt: tsToPtr(row.SubmittedAt),
			})
		}
		return nil
	})
	return out, err
}

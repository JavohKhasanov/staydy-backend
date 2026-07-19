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

// StudentRepository implements repo.StudentRepository. Every method runs inside
// WithTenant so RLS scopes the rows to orgID.
type StudentRepository struct {
	db *postgres.DB
}

func NewStudentRepository(db *postgres.DB) *StudentRepository { return &StudentRepository{db: db} }

func (r *StudentRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.CreateStudentParams) (entity.Student, error) {
	var row sqlc.Student
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateStudent(ctx, sqlc.CreateStudentParams{
			OrgID:            orgID,
			Name:             p.Name,
			Phone:            textVal(p.Phone),
			TelegramID:       textVal(p.TelegramID),
			CourseName:       textVal(p.CourseName),
			GroupName:        textVal(p.GroupName),
			GroupID:          nullableUUID(p.GroupID),
			MentorName:       textVal(p.MentorName),
			StartDate:        dateVal(p.StartDate),
			OnboardingGoal:   textVal(p.OnboardingGoal),
			SixMonthTarget:   textVal(p.SixMonthTarget),
			WeeklyStudyHours: textVal(p.WeeklyStudyHours),
			ConfidenceLevel:  int32(p.ConfidenceLevel),
			RiskScore:        int32(p.RiskScore),
			RiskTier:         p.RiskTier,
			Email:            textVal(p.Email),
			BirthDate:        dateVal(p.BirthDate),
			Gender:           p.Gender,
			SecondPhone:      textVal(p.SecondPhone),
			Address:          textVal(p.Address),
			ParentName:       textVal(p.ParentName),
			ParentPhone:      textVal(p.ParentPhone),
			StudentCode:      textVal(p.StudentCode),
			Status:           p.Status,
			MentorID:         nullableUUID(p.MentorID),
			BranchID:         nullableUUID(p.BranchID),
		})
		return e
	})
	if err != nil {
		return entity.Student{}, err
	}
	return mapStudent(row), nil
}

// Update edits the student's profile (not risk/group — those have their own paths).
func (r *StudentRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.UpdateStudentParams) (entity.Student, error) {
	var row sqlc.Student
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateStudent(ctx, sqlc.UpdateStudentParams{
			OrgID:            orgID,
			ID:               id,
			Name:             p.Name,
			Phone:            textVal(p.Phone),
			StartDate:        dateVal(p.StartDate),
			OnboardingGoal:   textVal(p.OnboardingGoal),
			SixMonthTarget:   textVal(p.SixMonthTarget),
			WeeklyStudyHours: textVal(p.WeeklyStudyHours),
			ConfidenceLevel:  int32(p.ConfidenceLevel),
			Email:            textVal(p.Email),
			BirthDate:        dateVal(p.BirthDate),
			Gender:           p.Gender,
			SecondPhone:      textVal(p.SecondPhone),
			Address:          textVal(p.Address),
			ParentName:       textVal(p.ParentName),
			ParentPhone:      textVal(p.ParentPhone),
			StudentCode:      textVal(p.StudentCode),
			Status:           p.Status,
			MentorID:         nullableUUID(p.MentorID),
			BranchID:         nullableUUID(p.BranchID),
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Student{}, repo.ErrNotFound
		}
		return entity.Student{}, err
	}
	return mapStudent(row), nil
}

func (r *StudentRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.Student, error) {
	var rows []sqlc.Student
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListStudents(ctx)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.Student, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapStudent(row))
	}
	return out, nil
}

func (r *StudentRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (entity.Student, error) {
	var row sqlc.Student
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).GetStudent(ctx, id)
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Student{}, repo.ErrNotFound
		}
		return entity.Student{}, err
	}
	return mapStudent(row), nil
}

func (r *StudentRepository) UpdateRisk(ctx context.Context, orgID, id uuid.UUID, score int, tier string) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).UpdateStudentRisk(ctx, sqlc.UpdateStudentRiskParams{
			ID:        id,
			RiskScore: int32(score),
			RiskTier:  tier,
		})
	})
}

func (r *StudentRepository) AddNote(ctx context.Context, orgID, studentID uuid.UUID, author, text string) (entity.Note, error) {
	var row sqlc.Note
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateNote(ctx, sqlc.CreateNoteParams{
			OrgID:     orgID,
			StudentID: studentID,
			Author:    author,
			Text:      text,
		})
		return e
	})
	if err != nil {
		// Composite (org_id, student_id) FK: a student not in this org → no matching parent
		// row → FK violation. Surface as not-found (also covers a truly missing student).
		if isForeignKeyViolation(err) {
			return entity.Note{}, repo.ErrNotFound
		}
		return entity.Note{}, err
	}
	return mapNote(row), nil
}

// AssignGroup sets (or clears, when groupID is nil) a student's group. A group not in this
// tenant trips the composite (org_id, group_id) FK → ErrNotFound.
func (r *StudentRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteStudent(ctx, id)
	})
}

func (r *StudentRepository) AssignGroup(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID) error {
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).AssignStudentGroup(ctx, sqlc.AssignStudentGroupParams{
			ID:      studentID,
			GroupID: nullableUUID(groupID),
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

func (r *StudentRepository) ListByGroup(ctx context.Context, orgID, groupID uuid.UUID) ([]entity.Student, error) {
	var rows []sqlc.Student
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListStudentsByGroup(ctx, uuidPtr(groupID))
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.Student, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapStudent(row))
	}
	return out, nil
}

func (r *StudentRepository) ListNotes(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Note, error) {
	var rows []sqlc.Note
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListNotesByStudent(ctx, studentID)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.Note, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapNote(row))
	}
	return out, nil
}

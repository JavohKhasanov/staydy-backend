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

// EnrollmentRepository persists student enrolments. enrollments is RLS-protected, so every call
// runs inside the tenant scope.
type EnrollmentRepository struct {
	db *postgres.DB
}

func NewEnrollmentRepository(db *postgres.DB) *EnrollmentRepository {
	return &EnrollmentRepository{db: db}
}

func (r *EnrollmentRepository) ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Enrollment, error) {
	var out []entity.Enrollment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListEnrollmentsByStudent(ctx, studentID)
		if e != nil {
			return e
		}
		out = make([]entity.Enrollment, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapEnrollment(row))
		}
		return nil
	})
	return out, err
}

func (r *EnrollmentRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.CreateEnrollmentParams) (entity.Enrollment, error) {
	var row sqlc.Enrollment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateEnrollment(ctx, sqlc.CreateEnrollmentParams{
			OrgID:     orgID,
			StudentID: p.StudentID,
			GroupID:   nullableUUID(p.GroupID),
			CourseID:  nullableUUID(p.CourseID),
			Status:    p.Status,
			StartDate: dateVal(p.StartDate),
			EndDate:   dateVal(p.EndDate),
			Price:     p.Price,
			Discount:  int32(p.Discount),
		})
		return e
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.Enrollment{}, repo.ErrNotFound // student not in this tenant
		}
		return entity.Enrollment{}, err
	}
	return mapEnrollment(row), nil
}

func (r *EnrollmentRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.UpdateEnrollmentParams) (entity.Enrollment, error) {
	var row sqlc.Enrollment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateEnrollment(ctx, sqlc.UpdateEnrollmentParams{
			OrgID:     orgID,
			ID:        id,
			GroupID:   nullableUUID(p.GroupID),
			CourseID:  nullableUUID(p.CourseID),
			Status:    p.Status,
			StartDate: dateVal(p.StartDate),
			EndDate:   dateVal(p.EndDate),
			Price:     p.Price,
			Discount:  int32(p.Discount),
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Enrollment{}, repo.ErrNotFound
		}
		return entity.Enrollment{}, err
	}
	return mapEnrollment(row), nil
}

func (r *EnrollmentRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteEnrollment(ctx, sqlc.DeleteEnrollmentParams{OrgID: orgID, ID: id})
	})
}

func mapEnrollment(e sqlc.Enrollment) entity.Enrollment {
	return entity.Enrollment{
		ID:        e.ID,
		OrgID:     e.OrgID,
		StudentID: e.StudentID,
		GroupID:   uuidToPtr(e.GroupID),
		CourseID:  uuidToPtr(e.CourseID),
		Status:    e.Status,
		StartDate: dateToPtr(e.StartDate),
		EndDate:   dateToPtr(e.EndDate),
		Price:     e.Price,
		Discount:  int(e.Discount),
		CreatedAt: e.CreatedAt.Time,
	}
}

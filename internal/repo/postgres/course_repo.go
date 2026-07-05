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

// CourseRepository persists a center's courses. courses is RLS-protected, so every call runs
// inside the tenant scope.
type CourseRepository struct {
	db *postgres.DB
}

func NewCourseRepository(db *postgres.DB) *CourseRepository {
	return &CourseRepository{db: db}
}

func (r *CourseRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.Course, error) {
	var out []entity.Course
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListCourses(ctx, orgID)
		if e != nil {
			return e
		}
		out = make([]entity.Course, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapCourse(row))
		}
		return nil
	})
	return out, err
}

func (r *CourseRepository) Get(ctx context.Context, orgID, id uuid.UUID) (entity.Course, error) {
	var row sqlc.Course
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).GetCourse(ctx, sqlc.GetCourseParams{OrgID: orgID, ID: id})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Course{}, repo.ErrNotFound
		}
		return entity.Course{}, err
	}
	return mapCourse(row), nil
}

func (r *CourseRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.CreateCourseParams) (entity.Course, error) {
	var row sqlc.Course
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateCourse(ctx, sqlc.CreateCourseParams{
			OrgID:         orgID,
			Name:          p.Name,
			Level:         p.Level,
			Price:         p.Price,
			DurationWeeks: int32(p.DurationWeeks),
			Description:   p.Description,
		})
		return e
	})
	if err != nil {
		if isUniqueViolation(err) {
			return entity.Course{}, repo.ErrAlreadyExists
		}
		return entity.Course{}, err
	}
	return mapCourse(row), nil
}

func (r *CourseRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.UpdateCourseParams) (entity.Course, error) {
	var row sqlc.Course
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateCourse(ctx, sqlc.UpdateCourseParams{
			OrgID:         orgID,
			ID:            id,
			Name:          p.Name,
			Level:         p.Level,
			Price:         p.Price,
			DurationWeeks: int32(p.DurationWeeks),
			Description:   p.Description,
			IsActive:      p.IsActive,
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Course{}, repo.ErrNotFound
		}
		if isUniqueViolation(err) {
			return entity.Course{}, repo.ErrAlreadyExists
		}
		return entity.Course{}, err
	}
	return mapCourse(row), nil
}

func (r *CourseRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteCourse(ctx, sqlc.DeleteCourseParams{OrgID: orgID, ID: id})
	})
}

func mapCourse(c sqlc.Course) entity.Course {
	return entity.Course{
		ID:            c.ID,
		OrgID:         c.OrgID,
		Name:          c.Name,
		Level:         c.Level,
		Price:         c.Price,
		DurationWeeks: int(c.DurationWeeks),
		Description:   c.Description,
		IsActive:      c.IsActive,
		CreatedAt:     c.CreatedAt.Time,
	}
}

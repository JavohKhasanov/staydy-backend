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

// GroupRepository implements repo.GroupRepository. Every method runs inside WithTenant so
// RLS scopes rows to orgID.
type GroupRepository struct {
	db *postgres.DB
}

func NewGroupRepository(db *postgres.DB) *GroupRepository { return &GroupRepository{db: db} }

func (r *GroupRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.CreateGroupParams) (entity.Group, error) {
	var row sqlc.Group
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateGroup(ctx, sqlc.CreateGroupParams{
			OrgID:        orgID,
			Name:         p.Name,
			TeacherID:    nullableUUID(p.TeacherID),
			CourseID:     nullableUUID(p.CourseID),
			BranchID:     nullableUUID(p.BranchID),
			Direction:    textVal(p.Direction),
			ScheduleDays: textVal(p.ScheduleDays),
			Capacity:     int32(p.Capacity),
		})
		return e
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.Group{}, repo.ErrNotFound // teacher_id not in this org
		}
		return entity.Group{}, err
	}
	return mapGroup(row), nil
}

func (r *GroupRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.Group, error) {
	var rows []sqlc.Group
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListGroups(ctx)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapGroup(row))
	}
	return out, nil
}

func (r *GroupRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (entity.Group, error) {
	var row sqlc.Group
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).GetGroup(ctx, id)
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Group{}, repo.ErrNotFound
		}
		return entity.Group{}, err
	}
	return mapGroup(row), nil
}

func (r *GroupRepository) ListByTeacher(ctx context.Context, orgID, teacherID uuid.UUID) ([]entity.Group, error) {
	var rows []sqlc.Group
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListGroupsByTeacher(ctx, uuidPtr(teacherID))
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapGroup(row))
	}
	return out, nil
}

func (r *GroupRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.UpdateGroupParams) (entity.Group, error) {
	var row sqlc.Group
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateGroup(ctx, sqlc.UpdateGroupParams{
			ID:           id,
			Name:         p.Name,
			TeacherID:    nullableUUID(p.TeacherID),
			CourseID:     nullableUUID(p.CourseID),
			BranchID:     nullableUUID(p.BranchID),
			Direction:    textVal(p.Direction),
			ScheduleDays: textVal(p.ScheduleDays),
			Capacity:     int32(p.Capacity),
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Group{}, repo.ErrNotFound
		}
		if isForeignKeyViolation(err) {
			return entity.Group{}, repo.ErrNotFound // teacher_id not in this org
		}
		return entity.Group{}, err
	}
	return mapGroup(row), nil
}

// Delete detaches any students (composite FK is ON DELETE NO ACTION) then removes the group,
// both in one tenant transaction.
func (r *GroupRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		if e := q.NullGroupForStudents(ctx, uuidPtr(id)); e != nil {
			return e
		}
		return q.DeleteGroup(ctx, id)
	})
}

func (r *GroupRepository) CountStudents(ctx context.Context, orgID, id uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		n, e = sqlc.New(tx).CountStudentsInGroup(ctx, uuidPtr(id))
		return e
	})
	return n, err
}

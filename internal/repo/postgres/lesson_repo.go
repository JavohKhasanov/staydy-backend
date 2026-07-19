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

// LessonRepository persists scheduled class sessions. lessons is RLS-protected, so every call runs
// inside the tenant scope.
type LessonRepository struct {
	db *postgres.DB
}

func NewLessonRepository(db *postgres.DB) *LessonRepository {
	return &LessonRepository{db: db}
}

func (r *LessonRepository) List(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.Lesson, error) {
	var out []entity.Lesson
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListLessons(ctx, sqlc.ListLessonsParams{
			OrgID:  orgID,
			Date:   dateVal(&from),
			Date_2: dateVal(&to),
		})
		if e != nil {
			return e
		}
		out = make([]entity.Lesson, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapLesson(row))
		}
		return nil
	})
	return out, err
}

// FindByGroupDate returns the (single) lesson/session record for a group on a date, if any.
func (r *LessonRepository) FindByGroupDate(ctx context.Context, orgID, groupID uuid.UUID, date time.Time) (entity.Lesson, bool, error) {
	var row sqlc.Lesson
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).GetLessonByGroupDate(ctx, sqlc.GetLessonByGroupDateParams{
			GroupID: groupID,
			Date:    dateVal(&date),
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Lesson{}, false, nil
		}
		return entity.Lesson{}, false, err
	}
	return mapLesson(row), true, nil
}

func (r *LessonRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.LessonParams) (entity.Lesson, error) {
	var row sqlc.Lesson
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateLesson(ctx, sqlc.CreateLessonParams{
			OrgID:     orgID,
			GroupID:   nullableUUID(p.GroupID),
			TeacherID: nullableUUID(p.TeacherID),
			Date:      dateVal(&p.Date),
			StartTime: p.StartTime,
			EndTime:   p.EndTime,
			Room:      p.Room,
			RoomID:    nullableUUID(p.RoomID),
			Topic:     p.Topic,
			Status:    p.Status,
		})
		return e
	})
	if err != nil {
		return entity.Lesson{}, err
	}
	return mapLesson(row), nil
}

func (r *LessonRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.LessonParams) (entity.Lesson, error) {
	var row sqlc.Lesson
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateLesson(ctx, sqlc.UpdateLessonParams{
			OrgID:     orgID,
			ID:        id,
			GroupID:   nullableUUID(p.GroupID),
			TeacherID: nullableUUID(p.TeacherID),
			Date:      dateVal(&p.Date),
			StartTime: p.StartTime,
			EndTime:   p.EndTime,
			Room:      p.Room,
			RoomID:    nullableUUID(p.RoomID),
			Topic:     p.Topic,
			Status:    p.Status,
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Lesson{}, repo.ErrNotFound
		}
		return entity.Lesson{}, err
	}
	return mapLesson(row), nil
}

func (r *LessonRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteLesson(ctx, sqlc.DeleteLessonParams{OrgID: orgID, ID: id})
	})
}

// CountRoomConflicts returns how many other lessons already book roomID at an overlapping time on
// date. excludeID skips a lesson (self, on update) — pass uuid.Nil to skip nothing.
func (r *LessonRepository) CountRoomConflicts(ctx context.Context, orgID, roomID uuid.UUID, date time.Time, start, end string, excludeID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		n, e = sqlc.New(tx).CountRoomConflicts(ctx, sqlc.CountRoomConflictsParams{
			RoomID:    roomID,
			Date:      dateVal(&date),
			ExcludeID: excludeID,
			NewEnd:    end,
			NewStart:  start,
		})
		return e
	})
	return n, err
}

func mapLesson(l sqlc.Lesson) entity.Lesson {
	return entity.Lesson{
		ID:        l.ID,
		OrgID:     l.OrgID,
		GroupID:   uuidToPtr(l.GroupID),
		TeacherID: uuidToPtr(l.TeacherID),
		Date:      l.Date.Time,
		StartTime: l.StartTime,
		EndTime:   l.EndTime,
		Room:      l.Room,
		RoomID:    uuidToPtr(l.RoomID),
		Topic:     l.Topic,
		Status:    l.Status,
		CreatedAt: l.CreatedAt.Time,
	}
}

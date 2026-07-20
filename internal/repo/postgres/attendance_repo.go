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

type AttendanceRepository struct {
	db *postgres.DB
}

func NewAttendanceRepository(db *postgres.DB) *AttendanceRepository {
	return &AttendanceRepository{db: db}
}

func (r *AttendanceRepository) Create(ctx context.Context, orgID, studentID uuid.UUID, groupID *uuid.UUID, date time.Time, present bool) (entity.AttendanceRecord, error) {
	var row sqlc.AttendanceRecord
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateAttendance(ctx, sqlc.CreateAttendanceParams{
			OrgID:     orgID,
			StudentID: studentID,
			GroupID:   nullableUUID(groupID),
			Date:      pgtype.Date{Time: date, Valid: true},
			IsPresent: present,
		})
		return e
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.AttendanceRecord{}, repo.ErrNotFound
		}
		return entity.AttendanceRecord{}, err
	}
	return mapAttendance(row), nil
}

func (r *AttendanceRepository) ListByStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.AttendanceRecord, error) {
	var rows []sqlc.AttendanceRecord
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		rows, e = sqlc.New(tx).ListAttendanceByStudent(ctx, studentID)
		return e
	})
	if err != nil {
		return nil, err
	}
	out := make([]entity.AttendanceRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAttendance(row))
	}
	return out, nil
}

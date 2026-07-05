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

// RoomRepository persists physical rooms (xonalar). RLS-scoped to the org.
type RoomRepository struct {
	db *postgres.DB
}

func NewRoomRepository(db *postgres.DB) *RoomRepository { return &RoomRepository{db: db} }

func (r *RoomRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.Room, error) {
	var out []entity.Room
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListRooms(ctx, orgID)
		if e != nil {
			return e
		}
		out = make([]entity.Room, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapRoom(row))
		}
		return nil
	})
	return out, err
}

func (r *RoomRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.RoomParams) (entity.Room, error) {
	var room entity.Room
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).CreateRoom(ctx, sqlc.CreateRoomParams{
			OrgID:    orgID,
			BranchID: nullableUUID(p.BranchID),
			Name:     p.Name,
			Capacity: int32(p.Capacity),
		})
		if e != nil {
			return e
		}
		room = mapRoom(row)
		return nil
	})
	return room, err
}

func (r *RoomRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.RoomParams) (entity.Room, error) {
	var room entity.Room
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).UpdateRoom(ctx, sqlc.UpdateRoomParams{
			ID:       id,
			Name:     p.Name,
			BranchID: nullableUUID(p.BranchID),
			Capacity: int32(p.Capacity),
		})
		if e != nil {
			return e
		}
		room = mapRoom(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Room{}, repo.ErrNotFound
	}
	return room, err
}

func (r *RoomRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteRoom(ctx, id)
	})
}

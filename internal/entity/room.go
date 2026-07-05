package entity

import (
	"time"

	"github.com/google/uuid"
)

// Room is a physical space (xona), optionally at a branch. Lessons reference a room; the schedule
// usecase prevents double-booking a room at overlapping times on the same date.
type Room struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	BranchID  *uuid.UUID
	Name      string
	Capacity  int
	CreatedAt time.Time
}

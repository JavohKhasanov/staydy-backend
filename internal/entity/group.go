package entity

import (
	"time"

	"github.com/google/uuid"
)

// Group is a student cohort, optionally taught by a teacher (a User with role 'teacher').
type Group struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Name         string
	TeacherID    *uuid.UUID
	CourseID     *uuid.UUID
	BranchID     *uuid.UUID
	Direction    string
	ScheduleDays string
	Capacity     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

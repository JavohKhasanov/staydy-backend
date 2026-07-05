package entity

import (
	"time"

	"github.com/google/uuid"
)

// Enrollment is a student's enrolment in a group/course with a lifecycle status, price and dates.
// It's the richer successor to students.group_id (which stays for now — enrollments coexists
// additively, so the risk engine + dashboard keep using group_id until a later cleanup).
type Enrollment struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	StudentID uuid.UUID
	GroupID   *uuid.UUID
	CourseID  *uuid.UUID
	Status    string // active | completed | dropped | frozen
	StartDate *time.Time
	EndDate   *time.Time
	Price     int64 // whole UZS so'm (usually copied from the course, can be overridden)
	Discount  int   // percent 0..100
	CreatedAt time.Time
}

// EnrollmentStatuses is the allowed lifecycle set.
var EnrollmentStatuses = map[string]bool{
	"active": true, "completed": true, "dropped": true, "frozen": true,
}

package entity

import (
	"time"

	"github.com/google/uuid"
)

// Course is a sellable program a center offers (e.g. "IELTS Intermediate", "Python Backend").
// A group (cohort) runs one course; enrollments and fees reference it. Price is whole UZS so'm.
type Course struct {
	ID            uuid.UUID
	OrgID         uuid.UUID
	Name          string
	Level         string // free-text: "Beginner", "A1", "Intermediate", ...
	Price         int64  // whole UZS so'm; 0 = unset
	DurationWeeks int    // 0 = unset
	Description   string
	IsActive      bool
	CreatedAt     time.Time
}

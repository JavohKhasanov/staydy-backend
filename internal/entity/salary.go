package entity

import (
	"time"

	"github.com/google/uuid"
)

// Salary computation kinds. rate is so'm for fixed/per_lesson/per_student; a percent (0-100) for
// percent_revenue.
const (
	SalaryFixed          = "fixed"
	SalaryPerLesson      = "per_lesson"
	SalaryPerStudent     = "per_student"
	SalaryPercentRevenue = "percent_revenue"
)

// ValidSalaryKind reports whether k is a known computation kind.
func ValidSalaryKind(k string) bool {
	return k == SalaryFixed || k == SalaryPerLesson || k == SalaryPerStudent || k == SalaryPercentRevenue
}

// SalaryRule configures how one teacher is paid.
type SalaryRule struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	TeacherID uuid.UUID
	Kind      string
	Rate      int64
	Base      int64
}

// SalarySlip is one period's pay for a teacher (gross ± bonus/deduction = net).
type SalarySlip struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	TeacherID   uuid.UUID
	TeacherName string // denormalised in list responses
	PeriodStart time.Time
	PeriodEnd   time.Time
	Gross       int64
	Bonus       int64
	Deduction   int64
	Net         int64
	Status      string // draft | paid
	Note        string
	PaidAt      *time.Time
	CreatedAt   time.Time
}

// SalaryBasis is the computed data behind a gross figure (shown in the preview).
type SalaryBasis struct {
	Kind     string
	Rate     int64
	Base     int64
	Lessons  int64
	Students int64
	Revenue  int64
	Gross    int64
}

// ComputeGross = fixed base + a variable component derived from the rule's kind/rate and the
// period's basis counts. A pure-fixed teacher has base>0 and kind 'fixed' (no variable).
func ComputeGross(kind string, base, rate, lessons, students, revenue int64) int64 {
	variable := int64(0)
	switch kind {
	case SalaryPerLesson:
		variable = rate * lessons
	case SalaryPerStudent:
		variable = rate * students
	case SalaryPercentRevenue:
		variable = revenue * rate / 100
	}
	return base + variable
}

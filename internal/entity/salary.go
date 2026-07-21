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

// SalaryGroupRule overrides a teacher's variable pay for one specific group.
type SalaryGroupRule struct {
	TeacherID uuid.UUID
	GroupID   uuid.UUID
	Kind      string
	Rate      int64
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

// GroupBasis is one group's contribution to a teacher's gross: its effective rule (an override, or
// the teacher default) applied to the group's own counts.
type GroupBasis struct {
	GroupID   uuid.UUID
	GroupName string
	Kind      string // effective kind for this group
	Rate      int64  // effective rate for this group
	Override  bool   // true when this group has an explicit override (not the default)
	Students  int64
	Lessons   int64
	Revenue   int64
	Amount    int64 // computed variable pay for this group
}

// SalaryBasis is the computed data behind a gross figure (shown in the preview). Gross = base + the
// sum of each group's Amount; Lessons/Students/Revenue are period totals across the groups.
type SalaryBasis struct {
	Kind     string // teacher default kind
	Rate     int64  // teacher default rate
	Base     int64
	Lessons  int64
	Students int64
	Revenue  int64
	Groups   []GroupBasis
	Gross    int64
}

// ComputeVariable is the variable pay from one (kind, rate) applied to a group's counts.
func ComputeVariable(kind string, rate, lessons, students, revenue int64) int64 {
	switch kind {
	case SalaryPerLesson:
		return rate * lessons
	case SalaryPerStudent:
		return rate * students
	case SalaryPercentRevenue:
		return revenue * rate / 100
	}
	return 0
}

// ComputeGross = fixed base + a single variable component (used when there is no per-group
// breakdown). A pure-fixed teacher has base>0 and kind 'fixed' (no variable).
func ComputeGross(kind string, base, rate, lessons, students, revenue int64) int64 {
	return base + ComputeVariable(kind, rate, lessons, students, revenue)
}

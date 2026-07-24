package entity

import (
	"time"

	"github.com/google/uuid"
)

// Survey is a weekly check-in submitted by (or on behalf of) a student.
type Survey struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	StudentID       uuid.UUID
	WeekNumber      int
	MotivationScore int // 1..5
	ProgressScore   int // 1..5
	BiggestObstacle string
	Comment         string
	SubmittedAt     time.Time
}

// Attendance statuses. is_present (the risk-facing signal) is derived as status != absent, so
// 'late' and 'excused' count as attended and never lower the attendance rate — the risk engine
// stays untouched. Only an unexcused 'absent' counts against the student.
const (
	AttendancePresent = "present"
	AttendanceAbsent  = "absent"
	AttendanceLate    = "late"
	AttendanceExcused = "excused"
)

// ValidAttendanceStatus reports whether s is a known attendance status.
func ValidAttendanceStatus(s string) bool {
	return s == AttendancePresent || s == AttendanceAbsent || s == AttendanceLate || s == AttendanceExcused
}

// PresentFromStatus derives the risk-facing boolean: only an unexcused absence is "not present".
func PresentFromStatus(status string) bool { return status != AttendanceAbsent }

// AttendanceRecord is one lesson's attendance for a student.
type AttendanceRecord struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	StudentID uuid.UUID
	Date      time.Time
	IsPresent bool
	Status    string
	CreatedAt time.Time
}

// HomeworkRecord is one lesson's homework completion for a student.
type HomeworkRecord struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	StudentID uuid.UUID
	Date      time.Time
	IsDone    bool
	CreatedAt time.Time
}

// InterventionTask is an action item auto-created when a student turns Red.
type InterventionTask struct {
	ID                uuid.UUID
	OrgID             uuid.UUID
	StudentID         uuid.UUID
	StudentName       string // populated on list (joined from students)
	Reasons           []string
	Status            string // Open | In Progress | Resolved
	ResolutionComment string
	AssignedTo        *uuid.UUID // staff member responsible (nil = unassigned)
	AssignedToName    string     // joined on list
	CreatedAt         time.Time
	ResolvedAt        *time.Time
}

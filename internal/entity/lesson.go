package entity

import (
	"time"

	"github.com/google/uuid"
)

// Lesson is one scheduled class session for a group (date + time + teacher + room + topic). It's
// the timetable layer; attendance links to it in a later phase (for now attendance stays per-day).
type Lesson struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	GroupID   *uuid.UUID
	TeacherID *uuid.UUID
	Date      time.Time
	StartTime string // "14:00"
	EndTime   string // "15:30"
	Room      string
	RoomID    *uuid.UUID
	Topic     string
	Status    string // scheduled | done | cancelled
	CreatedAt time.Time
}

var LessonStatuses = map[string]bool{"scheduled": true, "done": true, "cancelled": true}

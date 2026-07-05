package entity

import (
	"time"

	"github.com/google/uuid"
)

// Activity is one entry in a subject's communication timeline (a call, SMS, note or meeting). It's
// polymorphic — it attaches to a lead or a student via (subject_type, subject_id) — so one table
// serves the whole CRM (the Gibbon "Discussion" pattern).
type Activity struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	SubjectType string // lead | student
	SubjectID   uuid.UUID
	Type        string // note | call | sms | meeting
	Body        string
	Author      string // the creator's name at the time (denormalised, survives user removal)
	CreatedAt   time.Time
}

var ActivityTypes = map[string]bool{"note": true, "call": true, "sms": true, "meeting": true}
var ActivitySubjects = map[string]bool{"lead": true, "student": true}

package entity

import (
	"time"

	"github.com/google/uuid"
)

// Lead is a prospective student in the sales funnel (the CRM front, before enrolment). When a lead
// converts it spawns a Student and its stage moves to "enrolled" (student_id back-links the two).
type Lead struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	Name       string
	Phone      string
	Email      string
	Source     string // instagram | telegram | referral | walk-in | ...
	Stage      string // new | contacted | trial | enrolled | lost
	Interest   string // which course/program they asked about (free text for now)
	Note       string
	AssignedTo *uuid.UUID // a users row (the manager who owns this lead)
	StudentID  *uuid.UUID // set on conversion
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// LeadStages is the allowed pipeline set, in funnel order.
var LeadStages = map[string]bool{
	"new": true, "contacted": true, "trial": true, "enrolled": true, "lost": true,
}

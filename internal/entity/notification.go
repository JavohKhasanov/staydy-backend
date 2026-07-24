package entity

import (
	"time"

	"github.com/google/uuid"
)

// Notification kinds (the in-app feed for back-office users).
const (
	NotifyTaskAssigned = "task_assigned" // a task was assigned to you
	NotifyTaskSLA      = "task_sla"       // an at-risk student's task is overdue (director escalation)
)

// Notification is one in-app alert targeted at a single back-office user.
type Notification struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	UserID    uuid.UUID
	Kind      string
	Title     string
	Body      string
	Link      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

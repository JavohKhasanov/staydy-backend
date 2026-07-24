package entity

import (
	"time"

	"github.com/google/uuid"
)

// Valid homework submission states.
const (
	HomeworkSubmitted = "submitted"
	HomeworkAccepted  = "accepted"
	HomeworkRejected  = "rejected"
)

// ValidHomeworkStatus reports whether s is a gradeable submission status a reviewer may set.
func ValidHomeworkStatus(s string) bool {
	return s == HomeworkAccepted || s == HomeworkRejected
}

// HomeworkAssignment is a task a teacher attaches to a group.
type HomeworkAssignment struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	GroupID         uuid.UUID
	GroupName       string // denormalised in the student view
	LessonDate      *time.Time
	Title           string
	Description     string
	Deadline        *time.Time
	MaxScore        int
	CreatedAt       time.Time
	SubmissionCount int64 // in the teacher list view
}

// HomeworkReminder is one "deadline soon" push target: a linked student who hasn't submitted a
// specific assignment whose deadline is near.
type HomeworkReminder struct {
	AssignmentID uuid.UUID
	Title        string
	Deadline     time.Time
	ChatID       int64
}

// HomeworkSubmission is one student's answer to an assignment (graded by the teacher).
type HomeworkSubmission struct {
	ID           uuid.UUID
	AssignmentID uuid.UUID
	StudentID    uuid.UUID
	StudentName  string // denormalised in the grading view
	Text         string
	Links        string // newline-separated URLs
	Status       string
	Score        *int
	ReviewNote   string
	SubmittedAt  time.Time
	ReviewedAt   *time.Time
}

// StudentAssignment is the student's merged view: an assignment plus their own submission (if any).
type StudentAssignment struct {
	ID          uuid.UUID
	GroupID     uuid.UUID
	GroupName   string
	LessonDate  *time.Time
	Title       string
	Description string
	Deadline    *time.Time
	MaxScore    int
	// Submission — Status is "" when the student hasn't submitted yet.
	SubmissionID     *uuid.UUID
	SubmissionStatus string
	Score            *int
	SubmissionText   string
	SubmissionLinks  string
	ReviewNote       string
	SubmittedAt      *time.Time
}

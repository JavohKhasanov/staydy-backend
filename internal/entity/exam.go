package entity

import (
	"time"

	"github.com/google/uuid"
)

// Exam is a structured graded assessment for a group.
type Exam struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	GroupID     uuid.UUID
	Title       string
	ExamDate    *time.Time
	MaxScore    int
	CreatedAt   time.Time
	ResultCount int64 // teacher list view
}

// ExamResult is one student's score on an exam.
type ExamResult struct {
	ID          uuid.UUID
	ExamID      uuid.UUID
	StudentID   uuid.UUID
	StudentName string // list view
	Score       int
}

// StudentExamResult is a student's own result with the exam meta (mini app).
type StudentExamResult struct {
	ExamID   uuid.UUID
	Title    string
	ExamDate *time.Time
	MaxScore int
	Score    int
}

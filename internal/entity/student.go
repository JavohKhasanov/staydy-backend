package entity

import (
	"time"

	"github.com/google/uuid"
)

// StudentAccount is the minimal record used to authenticate a student login (cross-org lookup).
type StudentAccount struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	PasswordHash string
	Name         string
}

// StudentProfile is the signed-in student's own profile + gamification shown in the mini app.
type StudentProfile struct {
	ID         uuid.UUID
	OrgID      uuid.UUID
	Name       string
	Phone      string
	CourseName string
	GroupName  string
	XP         int
	Coins      int
}

// Student is an enrolled learner tracked for retention risk.
type Student struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	Name             string
	Phone            string
	TelegramID       string
	TelegramChatID   *int64
	CourseName       string
	GroupName        string
	GroupID          *uuid.UUID
	MentorName       string
	StartDate        *time.Time
	OnboardingGoal   string
	SixMonthTarget   string
	WeeklyStudyHours string
	ConfidenceLevel  int
	RiskScore        int
	RiskTier         string
	// CRM contact / identity / lifecycle (grounded field audit, 2026-07-01).
	Email        string
	BirthDate    *time.Time
	Gender       string
	SecondPhone  string
	Address      string
	ParentName   string
	ParentPhone  string
	StudentCode  string
	Status       string // active | inactive | graduated | lead | dropped
	MentorID     *uuid.UUID
	BranchID     *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StudentStatuses is the allowed lifecycle set.
var StudentStatuses = map[string]bool{
	"active": true, "inactive": true, "graduated": true, "lead": true, "dropped": true,
}

// Note is a mentor's note attached to a student.
type Note struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	StudentID uuid.UUID
	Author    string
	Text      string
	CreatedAt time.Time
}

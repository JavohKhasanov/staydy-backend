package entity

import (
	"time"

	"github.com/google/uuid"
)

// SignupRequest is a prospective center's interest submitted from the public landing page.
// It is platform-level (no org) — a super_admin reviews it and follows up manually; there is no
// automatic purchase yet (Payme/Click is post-MVP).
type SignupRequest struct {
	ID          uuid.UUID
	CenterName  string
	ContactName string
	Phone       string
	Email       string
	Plan        string // requested tier (trial|basic|pro), informational
	Message     string
	Status      string // new|contacted|converted|rejected
	CreatedAt   time.Time
}

// SignupStatuses are the review states a request moves through.
var SignupStatuses = []string{"new", "contacted", "converted", "rejected"}

// ValidSignupStatus reports whether s is a known signup-request status.
func ValidSignupStatus(s string) bool {
	for _, v := range SignupStatuses {
		if v == s {
			return true
		}
	}
	return false
}

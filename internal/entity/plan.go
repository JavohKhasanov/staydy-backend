package entity

import (
	"time"

	"github.com/google/uuid"
)

// Plan is a landing-page pricing plan (platform-level, edited by super_admin, shown on the public
// site). Price/period are free text so the super_admin can write "Bepul" / "299 000 so'm" / etc.
type Plan struct {
	ID          uuid.UUID
	PlanKey     string // trial|basic|pro — preselects the signup CTA
	Name        string
	Price       string
	Period      string
	Tagline     string
	Features    []string
	Highlighted bool
	SortOrder   int
	IsActive    bool
	CreatedAt   time.Time
}

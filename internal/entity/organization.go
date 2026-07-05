// Package entity holds framework-agnostic domain models. No business logic, no
// transport/persistence concerns — just the core types the whole app shares.
package entity

import (
	"time"

	"github.com/google/uuid"
)

// Organization is a tenant: one educational center (o'quv markazi).
type Organization struct {
	ID            uuid.UUID
	Name          string
	Slug          string // used at login to resolve the tenant
	Plan          string // trial | basic | pro ...
	Status        string // active | suspended (suspended → its users can't log in)
	TrialEndsAt   *time.Time // end of the free month; nil = no trial window set
	BillingStatus string     // trial | active (paid) | expired — gateway (Payme/Click) is post-MVP
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TrialDaysLeft returns whole days remaining in the free trial relative to now (negative if past).
// Returns 0 when no trial window is set.
func (o Organization) TrialDaysLeft(now time.Time) int {
	if o.TrialEndsAt == nil {
		return 0
	}
	return int(o.TrialEndsAt.Sub(now).Hours() / 24)
}

// IsSuspended reports whether the center is suspended (login blocked, still listed).
func (o Organization) IsSuspended() bool { return o.Status == "suspended" }

// IsActive reports whether the center is active (the only state that permits login).
// 'suspended' (temporary block) and 'archived' (soft-deleted) both deny login.
func (o Organization) IsActive() bool { return o.Status == "active" }

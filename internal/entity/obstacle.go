package entity

import (
	"time"

	"github.com/google/uuid"
)

// ObstacleOption is one center-configurable choice for the weekly check-in's "biggest obstacle"
// question. The bot builds its keyboard from the org's active options (falling back to a default
// set when none are configured), so different centers can offer domain-relevant choices.
type ObstacleOption struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Label     string
	Position  int
	IsActive  bool
	CreatedAt time.Time
}

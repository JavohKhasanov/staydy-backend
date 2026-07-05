package entity

import (
	"time"

	"github.com/google/uuid"
)

// Branch is a physical location of an education center (filial). It lives inside one org; records
// (students/groups/expenses/users) carry a nullable branch_id (NULL = org-wide / unassigned).
type Branch struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Name      string
	Address   string
	Phone     string
	IsActive  bool
	CreatedAt time.Time
}

package entity

import "github.com/google/uuid"

// Principal is the authenticated identity carried in the request context, decoded from
// the access-token claims. OrgID is the tenant the request acts within.
type Principal struct {
	UserID   uuid.UUID
	OrgID    uuid.UUID
	Email    string
	FullName string
	Role     UserRole
}

func (p Principal) IsSuperAdmin() bool { return p.Role == RoleSuperAdmin }

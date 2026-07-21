package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserRole is a back-office account role.
type UserRole string

const (
	RoleSuperAdmin  UserRole = "super_admin"  // platform owner
	RoleCenterAdmin UserRole = "center_admin" // educational-center administrator (full center panel)
	RoleFinance     UserRole = "finance"      // moliya: finance + salary + reports only
	RoleMentor      UserRole = "mentor"       // mentor within a center (resolves intervention tasks)
	RoleTeacher     UserRole = "teacher"      // ustoz: owns groups, marks attendance/homework
)

// ValidStaffRole reports whether r is a role a center admin may assign to a staff account it
// creates (administrator or finance — not teacher, which has its own path, nor the platform owner).
func ValidStaffRole(r UserRole) bool {
	return r == RoleCenterAdmin || r == RoleFinance
}

// User is a back-office account (center admin or mentor). PasswordHash never leaves
// the persistence/usecase layers — the HTTP response DTO omits it.
type User struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Email        string
	PasswordHash string
	FullName     string
	Role         UserRole
	BranchID     *uuid.UUID // primary branch (teachers); nil = org-wide
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

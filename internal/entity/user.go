package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserRole is a back-office account role.
type UserRole string

const (
	RoleSuperAdmin  UserRole = "super_admin"  // platform owner
	RoleCenterAdmin UserRole = "center_admin" // direktor (center owner): full center panel
	RoleManager     UserRole = "manager"      // administrator (front-desk): operations + collect payments, no salaries/expenses/staff
	RoleFinance     UserRole = "finance"      // moliya: finance + salary + reports
	RoleMentor      UserRole = "mentor"       // mentor within a center (resolves intervention tasks)
	RoleTeacher     UserRole = "teacher"      // ustoz: owns groups, marks attendance/homework
	RoleStudent     UserRole = "student"      // talaba: the student mini app (not a back-office user)
)

// BackOfficeRoles are every non-student role — the ones allowed on tenant back-office endpoints.
// A student token must never reach those, so groups gated only by RequireAuth use this to exclude it.
func BackOfficeRoles() []UserRole {
	return []UserRole{RoleSuperAdmin, RoleCenterAdmin, RoleManager, RoleFinance, RoleMentor, RoleTeacher}
}

// ValidStaffRole reports whether r is a role a director may assign to a staff account it creates:
// another director, an administrator (manager), or finance. Teachers have their own path; the
// platform owner is out of scope.
func ValidStaffRole(r UserRole) bool {
	return r == RoleCenterAdmin || r == RoleManager || r == RoleFinance
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

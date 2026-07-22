package postgres

import (
	"context"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
)

// StudentAuthRepository handles the one cross-tenant operation students need: resolving a login by
// phone. It runs pool-direct (no WithTenant) against the SECURITY DEFINER student_auth_lookup
// function, which bypasses RLS on the server side and returns only auth fields.
type StudentAuthRepository struct {
	db *postgres.DB
}

func NewStudentAuthRepository(db *postgres.DB) *StudentAuthRepository {
	return &StudentAuthRepository{db: db}
}

// LookupByPhone returns every student account (across orgs) whose phone matches and who has a login
// password set — the caller verifies the password against each match.
func (r *StudentAuthRepository) LookupByPhone(ctx context.Context, phone string) ([]entity.StudentAccount, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, org_id, password_hash, name FROM student_auth_lookup($1)`, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.StudentAccount
	for rows.Next() {
		var a entity.StudentAccount
		if err := rows.Scan(&a.ID, &a.OrgID, &a.PasswordHash, &a.Name); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

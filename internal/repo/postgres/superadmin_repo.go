package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// SuperadminRepository implements repo.SuperadminRepository. organizations is NON-RLS, so it is
// read with the pool directly; per-center counts are gathered inside each org's WithTenant scope
// (RLS admits that org's rows — no bypass needed).
type SuperadminRepository struct {
	db *postgres.DB
}

func NewSuperadminRepository(db *postgres.DB) *SuperadminRepository {
	return &SuperadminRepository{db: db}
}

func (r *SuperadminRepository) ListCenters(ctx context.Context, excludeSlug string) ([]repo.CenterStats, error) {
	orgs, err := sqlc.New(r.db.Pool).ListAllOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]repo.CenterStats, 0, len(orgs))
	for _, o := range orgs {
		if o.Slug == excludeSlug {
			continue
		}
		stats, err := r.statsFor(ctx, mapOrg(o))
		if err != nil {
			return nil, err
		}
		out = append(out, stats)
	}
	return out, nil
}

func (r *SuperadminRepository) GetCenter(ctx context.Context, id uuid.UUID) (repo.CenterStats, error) {
	o, err := sqlc.New(r.db.Pool).GetOrganizationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.CenterStats{}, repo.ErrNotFound
		}
		return repo.CenterStats{}, err
	}
	return r.statsFor(ctx, mapOrg(o))
}

// statsFor gathers a center's counts inside its own tenant scope.
func (r *SuperadminRepository) statsFor(ctx context.Context, org entity.Organization) (repo.CenterStats, error) {
	cs := repo.CenterStats{Org: org}
	err := r.db.WithTenant(ctx, org.ID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		students, e := q.CountStudentsInOrg(ctx)
		if e != nil {
			return e
		}
		users, e := q.CountUsersInOrg(ctx)
		if e != nil {
			return e
		}
		tiers, e := q.CountStudentsByTier(ctx)
		if e != nil {
			return e
		}
		cs.Students = int(students)
		cs.Users = int(users)
		for _, t := range tiers {
			switch t.RiskTier {
			case "Green":
				cs.Green = int(t.Count)
			case "Yellow":
				cs.Yellow = int(t.Count)
			case "Red":
				cs.Red = int(t.Count)
			}
		}
		return nil
	})
	if err != nil {
		return repo.CenterStats{}, err
	}
	return cs, nil
}

func (r *SuperadminRepository) UpdateCenter(ctx context.Context, id uuid.UUID, plan, status string) (entity.Organization, error) {
	row, err := sqlc.New(r.db.Pool).UpdateOrgPlanStatus(ctx, sqlc.UpdateOrgPlanStatusParams{
		ID:     id,
		Plan:   plan,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Organization{}, repo.ErrNotFound
		}
		return entity.Organization{}, err
	}
	return mapOrg(row), nil
}

// SetBilling updates a center's plan tier, billing_status, and trial window (super_admin only).
// The Payme/Click gateway will later drive these automatically; for now the super_admin sets them.
func (r *SuperadminRepository) SetBilling(ctx context.Context, id uuid.UUID, plan, billingStatus string, trialEndsAt *time.Time) (entity.Organization, error) {
	row, err := sqlc.New(r.db.Pool).UpdateOrgBilling(ctx, sqlc.UpdateOrgBillingParams{
		ID:            id,
		Plan:          plan,
		BillingStatus: billingStatus,
		TrialEndsAt:   tsPtrVal(trialEndsAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Organization{}, repo.ErrNotFound
		}
		return entity.Organization{}, err
	}
	return mapOrg(row), nil
}

func (r *SuperadminRepository) DeleteCenter(ctx context.Context, id uuid.UUID) error {
	return sqlc.New(r.db.Pool).DeleteOrganization(ctx, id)
}

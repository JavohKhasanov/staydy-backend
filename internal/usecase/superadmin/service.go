// Package superadmin is the platform-owner business logic: cross-tenant management of the
// educational centers (organizations) — list, create, change plan/status (suspend), delete, and
// global metrics. The super_admin account lives in a reserved "platform" org.
package superadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/security"
)

// PlatformSlug is the reserved org that hosts super_admin accounts (hidden from center lists).
const PlatformSlug = "platform"

var (
	ErrValidation = errors.New("superadmin: validation failed")
	ErrNotFound   = errors.New("superadmin: center not found")
	ErrSlugTaken  = errors.New("superadmin: slug or email already taken")
)

// Provisioner is the slice of AuthRepository used to create a center plus its first admin.
type Provisioner interface {
	ProvisionOrgWithAdmin(ctx context.Context, p repo.ProvisionParams) (entity.Organization, entity.User, error)
}

type Service struct {
	centers  repo.SuperadminRepository
	provider Provisioner
}

func NewService(centers repo.SuperadminRepository, provider Provisioner) *Service {
	return &Service{centers: centers, provider: provider}
}

func (s *Service) ListCenters(ctx context.Context) ([]repo.CenterStats, error) {
	return s.centers.ListCenters(ctx, PlatformSlug)
}

func (s *Service) GetCenter(ctx context.Context, id uuid.UUID) (repo.CenterStats, error) {
	cs, err := s.centers.GetCenter(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.CenterStats{}, ErrNotFound
		}
		return repo.CenterStats{}, err
	}
	if cs.Org.Slug == PlatformSlug {
		return repo.CenterStats{}, ErrNotFound // never expose the platform org as a center
	}
	return cs, nil
}

type CreateCenterInput struct {
	OrganizationName string
	Slug             string
	Plan             string
	AdminFullName    string
	AdminEmail       string
	AdminPassword    string
}

func (s *Service) CreateCenter(ctx context.Context, in CreateCenterInput) (entity.Organization, entity.User, error) {
	name := strings.TrimSpace(in.OrganizationName)
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	email := strings.ToLower(strings.TrimSpace(in.AdminEmail))
	fullName := strings.TrimSpace(in.AdminFullName)
	plan := strings.TrimSpace(in.Plan)
	if plan == "" {
		plan = "trial"
	}
	if name == "" || !validSlug(slug) || slug == PlatformSlug || !validPlan(plan) ||
		!strings.Contains(email, "@") || fullName == "" || len(in.AdminPassword) < 8 {
		return entity.Organization{}, entity.User{}, ErrValidation
	}
	hash, err := security.HashPassword(in.AdminPassword)
	if err != nil {
		return entity.Organization{}, entity.User{}, err
	}
	org, user, err := s.provider.ProvisionOrgWithAdmin(ctx, repo.ProvisionParams{
		OrgName: name, Slug: slug, Plan: plan,
		Email: email, PasswordHash: hash, FullName: fullName, Role: entity.RoleCenterAdmin,
	})
	if err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			return entity.Organization{}, entity.User{}, ErrSlugTaken
		}
		return entity.Organization{}, entity.User{}, err
	}
	return org, user, nil
}

type UpdateCenterInput struct {
	Plan   *string
	Status *string
}

func (s *Service) UpdateCenter(ctx context.Context, id uuid.UUID, in UpdateCenterInput) (repo.CenterStats, error) {
	cs, err := s.GetCenter(ctx, id) // validates existence + not the platform org
	if err != nil {
		return repo.CenterStats{}, err
	}
	plan, status := cs.Org.Plan, cs.Org.Status
	if in.Plan != nil {
		plan = strings.TrimSpace(*in.Plan)
	}
	if in.Status != nil {
		status = strings.TrimSpace(*in.Status)
	}
	if !validPlan(plan) || !validStatus(status) {
		return repo.CenterStats{}, ErrValidation
	}
	if _, err := s.centers.UpdateCenter(ctx, id, plan, status); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.CenterStats{}, ErrNotFound
		}
		return repo.CenterStats{}, err
	}
	return s.centers.GetCenter(ctx, id)
}

// SetBillingInput carries the fields a super_admin can change on a center's billing. Each is
// optional (nil = leave as-is). The Payme/Click gateway will later drive these automatically.
type SetBillingInput struct {
	Plan          *string
	BillingStatus *string
	TrialEndsAt   *time.Time
}

// SetBilling updates a center's plan tier, billing status (trial/active/expired), and trial window.
// Used by the super_admin to mark a center paid, extend a trial, or downgrade — until Payme/Click
// automates it post-MVP.
func (s *Service) SetBilling(ctx context.Context, id uuid.UUID, in SetBillingInput) (repo.CenterStats, error) {
	cs, err := s.GetCenter(ctx, id) // validates existence + not the platform org
	if err != nil {
		return repo.CenterStats{}, err
	}
	plan, status, trialEnds := cs.Org.Plan, cs.Org.BillingStatus, cs.Org.TrialEndsAt
	if in.Plan != nil {
		plan = strings.TrimSpace(*in.Plan)
	}
	if in.BillingStatus != nil {
		status = strings.TrimSpace(*in.BillingStatus)
	}
	if in.TrialEndsAt != nil {
		trialEnds = in.TrialEndsAt
	}
	if !validPlan(plan) || !validBillingStatus(status) {
		return repo.CenterStats{}, ErrValidation
	}
	if _, err := s.centers.SetBilling(ctx, id, plan, status, trialEnds); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.CenterStats{}, ErrNotFound
		}
		return repo.CenterStats{}, err
	}
	return s.centers.GetCenter(ctx, id)
}

// DeleteCenter SOFT-deletes a center: it sets status='archived' (hidden from the active list,
// login blocked, data preserved & restorable) — never a hard delete.
func (s *Service) DeleteCenter(ctx context.Context, id uuid.UUID) error {
	cs, err := s.GetCenter(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.centers.UpdateCenter(ctx, id, cs.Org.Plan, "archived")
	return err
}

// Metrics is the global KPI snapshot derived from the center list.
type Metrics struct {
	TotalCenters     int
	ActiveCenters    int
	SuspendedCenters int
	TotalStudents    int
	TotalAtRisk      int
	Green            int
	Yellow           int
	Red              int
	CentersByPlan    map[string]int
	Recent           []repo.CenterStats // newest first, up to 10
}

func (s *Service) Metrics(ctx context.Context) (Metrics, error) {
	centers, err := s.ListCenters(ctx)
	if err != nil {
		return Metrics{}, err
	}
	// Archived (soft-deleted) centers don't count toward the platform KPIs.
	active := make([]repo.CenterStats, 0, len(centers))
	for _, c := range centers {
		if c.Org.Status != "archived" {
			active = append(active, c)
		}
	}
	m := Metrics{CentersByPlan: map[string]int{}, TotalCenters: len(active)}
	for _, c := range active {
		if c.Org.IsSuspended() {
			m.SuspendedCenters++
		} else {
			m.ActiveCenters++
		}
		m.TotalStudents += c.Students
		m.Green += c.Green
		m.Yellow += c.Yellow
		m.Red += c.Red
		m.CentersByPlan[c.Org.Plan]++
	}
	m.TotalAtRisk = m.Yellow + m.Red
	if len(active) > 10 { // ListAllOrganizations is already newest-first
		m.Recent = active[:10]
	} else {
		m.Recent = active
	}
	return m, nil
}

func validSlug(s string) bool {
	if len(s) < 2 {
		return false
	}
	for _, r := range s {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit && r != '-' {
			return false
		}
	}
	return true
}

func validPlan(p string) bool   { return p == "trial" || p == "basic" || p == "pro" || p == "internal" }
func validStatus(s string) bool { return s == "active" || s == "suspended" || s == "archived" }
func validBillingStatus(s string) bool {
	return s == "trial" || s == "active" || s == "expired"
}

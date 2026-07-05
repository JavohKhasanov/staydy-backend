// Package auth is the authentication usecase: register (provision a center + first
// admin), login, and refresh-token rotation. It depends on repo interfaces + security,
// never on transport or sqlc.
package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/security"
)

var (
	ErrValidation         = errors.New("validation failed")
	ErrSlugTaken          = errors.New("organization slug already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRefresh     = errors.New("invalid refresh token")
	ErrSuspended          = errors.New("organization suspended")
)

type RegisterParams struct {
	OrganizationName string
	Slug             string
	Email            string
	Password         string
	FullName         string
}

type LoginParams struct {
	Slug     string
	Email    string
	Password string
}

type DeviceMetadata struct {
	UserAgent string
	IP        string
}

// Tokens is the result of register/login/refresh: a token pair plus the resolved
// user and organization.
type Tokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             entity.User
	Organization     entity.Organization
}

type Service struct {
	auth     repo.AuthRepository
	sessions repo.SessionRepository
	tokens   *security.TokenManager
}

func NewService(authRepo repo.AuthRepository, sessions repo.SessionRepository, tm *security.TokenManager) *Service {
	return &Service{auth: authRepo, sessions: sessions, tokens: tm}
}

// Register provisions a new center + its first center_admin, then issues a token pair.
func (s *Service) Register(ctx context.Context, p RegisterParams, dev DeviceMetadata) (*Tokens, error) {
	p.OrganizationName = strings.TrimSpace(p.OrganizationName)
	p.Email = normalizeEmail(p.Email)
	p.FullName = strings.TrimSpace(p.FullName)

	slug := slugify(p.Slug)
	if slug == "" {
		slug = slugify(p.OrganizationName)
	}
	if err := validateRegister(p, slug); err != nil {
		return nil, err
	}

	exists, err := s.auth.SlugExists(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("auth: slug check: %w", err)
	}
	if exists {
		return nil, ErrSlugTaken
	}

	hash, err := security.HashPassword(p.Password)
	if err != nil {
		return nil, fmt.Errorf("auth: hash password: %w", err)
	}

	org, user, err := s.auth.ProvisionOrgWithAdmin(ctx, repo.ProvisionParams{
		OrgName:      p.OrganizationName,
		Slug:         slug,
		Plan:         "trial",
		Email:        p.Email,
		PasswordHash: hash,
		FullName:     p.FullName,
		Role:         entity.RoleCenterAdmin,
	})
	if err != nil {
		if errors.Is(err, repo.ErrAlreadyExists) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("auth: provision: %w", err)
	}
	return s.issue(ctx, org, user, dev)
}

// Login resolves the center by slug, verifies the password, and issues a token pair.
// Every failure returns ErrInvalidCredentials so callers can't probe which field failed.
func (s *Service) Login(ctx context.Context, p LoginParams, dev DeviceMetadata) (*Tokens, error) {
	slug := slugify(p.Slug)
	email := normalizeEmail(p.Email)
	if slug == "" || email == "" || p.Password == "" {
		return nil, ErrInvalidCredentials
	}

	org, err := s.auth.OrgBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			security.DummyCheck(p.Password) // equalize timing (anti-enumeration)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: org lookup: %w", err)
	}
	user, err := s.auth.UserByEmailInOrg(ctx, org.ID, email)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			security.DummyCheck(p.Password) // equalize timing (anti-enumeration)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("auth: user lookup: %w", err)
	}
	if !security.CheckPassword(user.PasswordHash, p.Password) {
		return nil, ErrInvalidCredentials
	}
	// Checked only after a valid password so the center's status can't be probed anonymously.
	// Both suspended (temporary) and archived (soft-deleted) centers deny login.
	if !org.IsActive() {
		return nil, ErrSuspended
	}
	return s.issue(ctx, org, user, dev)
}

// Refresh rotates a refresh token: it verifies and revokes the presented one, then
// issues a fresh token pair for the same user.
func (s *Service) Refresh(ctx context.Context, refreshToken string, dev DeviceMetadata) (*Tokens, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, ErrInvalidRefresh
	}
	// Generate the replacement token up front so the rotation (revoke old + insert new)
	// is a single atomic, row-locked transaction in the repo.
	newRefresh, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh token: %w", err)
	}
	refreshExp := time.Now().Add(s.tokens.RefreshTTL())

	res, err := s.sessions.RotateSession(ctx, repo.RotateParams{
		OldTokenHash: security.HashRefreshToken(refreshToken),
		NewTokenHash: security.HashRefreshToken(newRefresh),
		UserAgent:    dev.UserAgent,
		IP:           dev.IP,
		ExpiresAt:    refreshExp,
	})
	if err != nil {
		// ErrNotFound, expired, or ErrSessionReused (family already revoked) → invalid.
		return nil, ErrInvalidRefresh
	}

	user, err := s.auth.UserByID(ctx, res.OrgID, res.UserID)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	org, err := s.auth.OrgByID(ctx, res.OrgID)
	if err != nil {
		return nil, fmt.Errorf("auth: org lookup: %w", err)
	}

	principal := entity.Principal{
		UserID:   user.ID,
		OrgID:    user.OrgID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
	}
	access, accessExp, err := s.tokens.GenerateAccessToken(principal)
	if err != nil {
		return nil, fmt.Errorf("auth: access token: %w", err)
	}
	return &Tokens{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     newRefresh,
		RefreshExpiresAt: refreshExp,
		User:             user,
		Organization:     org,
	}, nil
}

// issue mints an access+refresh pair and persists the refresh session.
func (s *Service) issue(ctx context.Context, org entity.Organization, user entity.User, dev DeviceMetadata) (*Tokens, error) {
	principal := entity.Principal{
		UserID:   user.ID,
		OrgID:    user.OrgID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
	}
	access, accessExp, err := s.tokens.GenerateAccessToken(principal)
	if err != nil {
		return nil, fmt.Errorf("auth: access token: %w", err)
	}
	refresh, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh token: %w", err)
	}
	refreshExp := time.Now().Add(s.tokens.RefreshTTL())
	if _, err := s.sessions.CreateSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		OrgID:     user.OrgID,
		TokenHash: security.HashRefreshToken(refresh),
		UserAgent: dev.UserAgent,
		IP:        dev.IP,
		ExpiresAt: refreshExp,
	}); err != nil {
		return nil, fmt.Errorf("auth: create session: %w", err)
	}

	return &Tokens{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     refresh,
		RefreshExpiresAt: refreshExp,
		User:             user,
		Organization:     org,
	}, nil
}

// --- helpers ---

var (
	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
	emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlug.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validateRegister(p RegisterParams, slug string) error {
	var problems []string
	if p.OrganizationName == "" {
		problems = append(problems, "organizationName is required")
	}
	if len(slug) < 2 {
		problems = append(problems, "slug must be at least 2 characters (a-z, 0-9, hyphen)")
	}
	if !emailRe.MatchString(p.Email) {
		problems = append(problems, "email is invalid")
	}
	if len(p.Password) < 8 {
		problems = append(problems, "password must be at least 8 characters")
	}
	if p.FullName == "" {
		problems = append(problems, "fullName is required")
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, strings.Join(problems, "; "))
	}
	return nil
}

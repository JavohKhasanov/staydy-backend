// Package studentauth authenticates students for the mini app: a phone + password login that
// resolves the student across tenants and issues a single long-lived access token (no refresh).
// Students are learners, not back-office users; their token carries only Role "student".
package studentauth

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

var ErrInvalidCredentials = errors.New("studentauth: invalid credentials")

type Service struct {
	repo   repo.StudentAuthRepository
	tokens *security.TokenManager
}

func NewService(r repo.StudentAuthRepository, tm *security.TokenManager) *Service {
	return &Service{repo: r, tokens: tm}
}

// LoginResult is the token + identity returned on a successful student login.
type LoginResult struct {
	AccessToken     string
	AccessExpiresAt time.Time
	StudentID       uuid.UUID
	OrgID           uuid.UUID
	Name            string
}

// Login verifies phone + password and returns a student access token. It checks the password
// against every account matching the phone (across tenants) and equalizes timing on failure.
func (s *Service) Login(ctx context.Context, phone, password string) (*LoginResult, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	accounts, err := s.repo.LookupByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		if security.CheckPassword(a.PasswordHash, password) {
			p := entity.Principal{UserID: a.ID, OrgID: a.OrgID, FullName: a.Name, Role: entity.RoleStudent}
			token, exp, terr := s.tokens.GenerateStudentToken(p)
			if terr != nil {
				return nil, terr
			}
			return &LoginResult{
				AccessToken: token, AccessExpiresAt: exp,
				StudentID: a.ID, OrgID: a.OrgID, Name: a.Name,
			}, nil
		}
	}
	security.DummyCheck(password) // equalize timing (anti-enumeration)
	return nil, ErrInvalidCredentials
}

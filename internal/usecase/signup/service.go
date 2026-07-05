// Package signup is the landing-page "request a plan" funnel: the public site submits interest and
// a super_admin reviews it. No purchase yet (Payme/Click is post-MVP) — follow-up is manual.
package signup

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("signup: validation failed")
	ErrNotFound   = errors.New("signup: request not found")
)

type Service struct {
	requests repo.SignupRepository
}

func NewService(requests repo.SignupRepository) *Service { return &Service{requests: requests} }

// CreateInput is the public submission from the landing page.
type CreateInput struct {
	CenterName  string
	ContactName string
	Phone       string
	Email       string
	Plan        string
	Message     string
}

// Create validates and stores a public signup request.
func (s *Service) Create(ctx context.Context, in CreateInput) (entity.SignupRequest, error) {
	req := entity.SignupRequest{
		CenterName:  strings.TrimSpace(in.CenterName),
		ContactName: strings.TrimSpace(in.ContactName),
		Phone:       strings.TrimSpace(in.Phone),
		Email:       strings.ToLower(strings.TrimSpace(in.Email)),
		Plan:        strings.TrimSpace(in.Plan),
		Message:     strings.TrimSpace(in.Message),
	}
	if req.CenterName == "" || req.ContactName == "" || len(req.Phone) < 7 {
		return entity.SignupRequest{}, ErrValidation
	}
	return s.requests.Create(ctx, req)
}

func (s *Service) List(ctx context.Context) ([]entity.SignupRequest, error) {
	return s.requests.List(ctx)
}

// SetStatus moves a request through the review pipeline (new|contacted|converted|rejected).
func (s *Service) SetStatus(ctx context.Context, id uuid.UUID, status string) (entity.SignupRequest, error) {
	status = strings.TrimSpace(status)
	if !entity.ValidSignupStatus(status) {
		return entity.SignupRequest{}, ErrValidation
	}
	req, err := s.requests.SetStatus(ctx, id, status)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return entity.SignupRequest{}, ErrNotFound
		}
		return entity.SignupRequest{}, err
	}
	return req, nil
}

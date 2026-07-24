package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	pointsusecase "github.com/student-success/backend/internal/usecase/points"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
	studentauthusecase "github.com/student-success/backend/internal/usecase/studentauth"
)

type studentLoginInput struct {
	Body struct {
		Phone    string `json:"phone" minLength:"1" maxLength:"32"`
		Password string `json:"password" minLength:"1" maxLength:"128"`
	}
}
type studentLoginOutput struct {
	Body struct {
		AccessToken string `json:"accessToken"`
		ExpiresAt   string `json:"expiresAt"`
		Student     struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			OrgID string `json:"orgId"`
		} `json:"student"`
	}
}

type studentProfileOutput struct {
	Body struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Phone      string `json:"phone"`
		CourseName string `json:"courseName,omitempty"`
		GroupName  string `json:"groupName,omitempty"`
		XP         int    `json:"xp"`
		Coins      int    `json:"coins"`
		Level      int    `json:"level"`
		XPIntoLevel int   `json:"xpIntoLevel"`
		XPPerLevel int    `json:"xpPerLevel"`
	}
}

// registerStudentAuth mounts the public student login. Mount on the public group.
func registerStudentAuth(api huma.API, svc *studentauthusecase.Service, log zerolog.Logger) {
	Register(api, huma.Operation{
		OperationID: "student-login",
		Method:      http.MethodPost,
		Path:        "/student/login",
		Summary:     "Student mini-app login (phone + password)",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, func(ctx context.Context, in *studentLoginInput) (*studentLoginOutput, error) {
		res, err := svc.Login(ctx, in.Body.Phone, in.Body.Password)
		if err != nil {
			if errors.Is(err, studentauthusecase.ErrInvalidCredentials) {
				return nil, huma.Error401Unauthorized("Telefon yoki parol noto'g'ri.")
			}
			log.Error().Err(err).Msg("student login failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &studentLoginOutput{}
		out.Body.AccessToken = res.AccessToken
		out.Body.ExpiresAt = res.AccessExpiresAt.UTC().Format(time.RFC3339)
		out.Body.Student.ID = res.StudentID.String()
		out.Body.Student.Name = res.Name
		out.Body.Student.OrgID = res.OrgID.String()
		return out, nil
	})
}

type studentCheckinInput struct {
	Body struct {
		Motivation int    `json:"motivation" minimum:"1" maximum:"5"`
		Progress   int    `json:"progress" minimum:"1" maximum:"5"`
		Obstacle   string `json:"obstacle,omitempty" maxLength:"120"`
		Comment    string `json:"comment,omitempty" maxLength:"1000"`
	}
}

// registerStudentApp mounts the signed-in student's own endpoints. Mount on a group gated to the
// student role.
type studentChangePasswordInput struct {
	Body struct {
		CurrentPassword string `json:"currentPassword" minLength:"1" maxLength:"128"`
		NewPassword     string `json:"newPassword" minLength:"6" maxLength:"128"`
	}
}

func registerStudentApp(api huma.API, svc *studentusecase.Service, points *pointsusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID:   "student-checkin",
		Method:        http.MethodPost,
		Path:          "/student/checkin",
		Summary:       "Weekly check-in (motivation/progress/obstacle) — feeds the early-warning signals and awards XP",
		Tags:          []string{"student"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, in *studentCheckinInput) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		if _, err := svc.SubmitWeeklySurvey(ctx, p.OrgID, p.UserID, in.Body.Motivation, in.Body.Progress, in.Body.Obstacle, in.Body.Comment); err != nil {
			log.Error().Err(err).Msg("student checkin failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &noContentOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-me",
		Method:      http.MethodGet,
		Path:        "/student/me",
		Summary:     "The signed-in student's own profile",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*studentProfileOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		prof, err := svc.Profile(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student profile failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &studentProfileOutput{}
		out.Body.ID = prof.ID.String()
		out.Body.Name = prof.Name
		out.Body.Phone = prof.Phone
		out.Body.CourseName = prof.CourseName
		out.Body.GroupName = prof.GroupName
		cfg := points.Config(ctx, p.OrgID)
		out.Body.XP = prof.XP
		out.Body.Coins = prof.Coins
		out.Body.Level = cfg.LevelForXP(prof.XP)
		out.Body.XPIntoLevel = cfg.XPIntoLevel(prof.XP)
		out.Body.XPPerLevel = cfg.LevelSize
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "student-change-password",
		Method:        http.MethodPut,
		Path:          "/student/change-password",
		Summary:       "Change your own mini-app password (requires the current one)",
		Tags:          []string{"student"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, in *studentChangePasswordInput) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		if err := svc.ChangeLoginPassword(ctx, p.OrgID, p.UserID, in.Body.CurrentPassword, in.Body.NewPassword); err != nil {
			switch {
			case errors.Is(err, studentusecase.ErrValidation):
				return nil, huma.Error422UnprocessableEntity("parol kamida 6 belgi bo'lishi kerak")
			case errors.Is(err, studentusecase.ErrWrongPassword):
				return nil, huma.Error422UnprocessableEntity("joriy parol noto'g'ri")
			default:
				log.Error().Err(err).Msg("student change password failed")
				return nil, huma.Error500InternalServerError("internal error")
			}
		}
		return &noContentOutput{}, nil
	})
}

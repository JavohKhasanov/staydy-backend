package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	"github.com/student-success/backend/internal/repo"
	teacherusecase "github.com/student-success/backend/internal/usecase/teacher"
)

type teacherResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FullName  string `json:"fullName"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

type createTeacherRequest struct {
	Email    string `json:"email" format:"email" maxLength:"255" example:"ustoz@najot.uz"`
	Password string `json:"password" minLength:"8" maxLength:"128"`
	FullName string `json:"fullName" minLength:"1" maxLength:"255" example:"Ali Valiyev"`
}

type updateTeacherRequest struct {
	Email    string `json:"email" format:"email" maxLength:"255"`
	FullName string `json:"fullName" minLength:"1" maxLength:"255"`
}
type setTeacherPasswordRequest struct {
	Password string `json:"password" minLength:"8" maxLength:"128"`
}

type listTeachersInput struct{}
type listTeachersOutput struct{ Body []teacherResponse }
type createTeacherInput struct{ Body createTeacherRequest }
type updateTeacherInput struct {
	ID   string `path:"id" format:"uuid"`
	Body updateTeacherRequest
}
type setTeacherPasswordInput struct {
	ID   string `path:"id" format:"uuid"`
	Body setTeacherPasswordRequest
}
type teacherIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type teacherOutput struct{ Body teacherResponse }

// registerTeachers mounts teacher-account management. Mount on a group gated to
// center_admin / super_admin.
func registerTeachers(api huma.API, svc *teacherusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "teachers-list",
		Method:      http.MethodGet,
		Path:        "/teachers",
		Summary:     "List teacher accounts",
		Tags:        []string{"teachers"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listTeachersInput) (*listTeachersOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		teachers, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapTeacherError(LangFromContext(ctx), err, log)
		}
		out := &listTeachersOutput{Body: make([]teacherResponse, 0, len(teachers))}
		for _, t := range teachers {
			out.Body = append(out.Body, toTeacherResponse(t))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "teachers-create",
		Method:      http.MethodPost,
		Path:        "/teachers",
		Summary:     "Create a teacher account (logs into the web app; role 'teacher')",
		Tags:        []string{"teachers"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createTeacherInput) (*teacherOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		t, err := svc.Create(ctx, p.OrgID, teacherusecase.CreateInput{
			Email:    in.Body.Email,
			Password: in.Body.Password,
			FullName: in.Body.FullName,
		})
		if err != nil {
			return nil, mapTeacherError(LangFromContext(ctx), err, log)
		}
		return &teacherOutput{Body: toTeacherResponse(t)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "teachers-get",
		Method:      http.MethodGet,
		Path:        "/teachers/{id}",
		Summary:     "Get a teacher account",
		Tags:        []string{"teachers"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDInput) (*teacherOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid teacher id")
		}
		t, err := svc.Get(ctx, p.OrgID, id)
		if err != nil {
			return nil, mapTeacherError(LangFromContext(ctx), err, log)
		}
		return &teacherOutput{Body: toTeacherResponse(t)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "teachers-update",
		Method:      http.MethodPut,
		Path:        "/teachers/{id}",
		Summary:     "Update a teacher account (name, email)",
		Tags:        []string{"teachers"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateTeacherInput) (*teacherOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid teacher id")
		}
		t, err := svc.Update(ctx, p.OrgID, id, teacherusecase.UpdateInput{Email: in.Body.Email, FullName: in.Body.FullName})
		if err != nil {
			return nil, mapTeacherError(LangFromContext(ctx), err, log)
		}
		return &teacherOutput{Body: toTeacherResponse(t)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "teachers-set-password",
		Method:      http.MethodPut,
		Path:        "/teachers/{id}/password",
		Summary:     "Reset a teacher's login password (center_admin)",
		Tags:        []string{"teachers"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *setTeacherPasswordInput) (*struct{}, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid teacher id")
		}
		if err := svc.SetPassword(ctx, p.OrgID, id, in.Body.Password); err != nil {
			return nil, mapTeacherError(LangFromContext(ctx), err, log)
		}
		return nil, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "teachers-delete",
		Method:        http.MethodDelete,
		Path:          "/teachers/{id}",
		Summary:       "Delete a teacher account",
		Tags:          []string{"teachers"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDInput) (*struct{}, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid teacher id")
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapTeacherError(LangFromContext(ctx), err, log)
		}
		return nil, nil
	})
}

func toTeacherResponse(u entity.User) teacherResponse {
	return teacherResponse{
		ID:        u.ID.String(),
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

var (
	msgTeacherNotFound = i18n.Message{UZ: "Ustoz topilmadi.", RU: "Преподаватель не найден.", EN: "Teacher not found."}
	msgTeacherEmail    = i18n.Message{UZ: "Bu email allaqachon band.", RU: "Этот email уже занят.", EN: "Email already taken."}
)

func mapTeacherError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, teacherusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, repo.ErrAlreadyExists):
		return huma.Error409Conflict(msgTeacherEmail.For(lang))
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgTeacherNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("teachers: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

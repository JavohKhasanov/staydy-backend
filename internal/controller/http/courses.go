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
	courseusecase "github.com/student-success/backend/internal/usecase/course"
)

type courseResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Level         string `json:"level"`
	Price         int64  `json:"price"`
	DurationWeeks int    `json:"durationWeeks"`
	Description   string `json:"description"`
	IsActive      bool   `json:"isActive"`
	CreatedAt     string `json:"createdAt"`
}

type courseBody struct {
	Name          string `json:"name" minLength:"1" maxLength:"120" example:"IELTS Intermediate"`
	Level         string `json:"level,omitempty" maxLength:"60" example:"Intermediate"`
	Price         int64  `json:"price,omitempty" minimum:"0" doc:"whole UZS so'm"`
	DurationWeeks int    `json:"durationWeeks,omitempty" minimum:"0"`
	Description   string `json:"description,omitempty" maxLength:"2000"`
	IsActive      *bool  `json:"isActive,omitempty" doc:"defaults to true on create; on update controls archive/active"`
}

type listCoursesInput struct{}
type listCoursesOutput struct{ Body []courseResponse }
type createCourseInput struct{ Body courseBody }
type courseIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type updateCourseInput struct {
	ID   string `path:"id" format:"uuid"`
	Body courseBody
}
type courseOutput struct{ Body courseResponse }
type deleteCourseOutput struct{}

// registerCourses mounts course management. Mount on a group gated to center_admin / super_admin.
func registerCourses(api huma.API, svc *courseusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "courses-list",
		Method:      http.MethodGet,
		Path:        "/courses",
		Summary:     "List courses",
		Tags:        []string{"courses"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listCoursesInput) (*listCoursesOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		cs, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapCourseError(LangFromContext(ctx), err, log)
		}
		out := &listCoursesOutput{Body: make([]courseResponse, 0, len(cs))}
		for _, c := range cs {
			out.Body = append(out.Body, toCourseResponse(c))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "courses-create",
		Method:        http.MethodPost,
		Path:          "/courses",
		Summary:       "Create a course",
		Tags:          []string{"courses"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createCourseInput) (*courseOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		c, err := svc.Create(ctx, p.OrgID, courseusecase.Input{
			Name:          in.Body.Name,
			Level:         in.Body.Level,
			Price:         in.Body.Price,
			DurationWeeks: in.Body.DurationWeeks,
			Description:   in.Body.Description,
		})
		if err != nil {
			return nil, mapCourseError(LangFromContext(ctx), err, log)
		}
		return &courseOutput{Body: toCourseResponse(c)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "courses-update",
		Method:      http.MethodPut,
		Path:        "/courses/{id}",
		Summary:     "Update a course (also archives/restores via isActive)",
		Tags:        []string{"courses"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateCourseInput) (*courseOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid course id")
		}
		isActive := true
		if in.Body.IsActive != nil {
			isActive = *in.Body.IsActive
		}
		c, err := svc.Update(ctx, p.OrgID, id, courseusecase.Input{
			Name:          in.Body.Name,
			Level:         in.Body.Level,
			Price:         in.Body.Price,
			DurationWeeks: in.Body.DurationWeeks,
			Description:   in.Body.Description,
		}, isActive)
		if err != nil {
			return nil, mapCourseError(LangFromContext(ctx), err, log)
		}
		return &courseOutput{Body: toCourseResponse(c)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "courses-delete",
		Method:        http.MethodDelete,
		Path:          "/courses/{id}",
		Summary:       "Delete a course",
		Tags:          []string{"courses"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *courseIDInput) (*deleteCourseOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid course id")
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapCourseError(LangFromContext(ctx), err, log)
		}
		return &deleteCourseOutput{}, nil
	})
}

func toCourseResponse(c entity.Course) courseResponse {
	return courseResponse{
		ID:            c.ID.String(),
		Name:          c.Name,
		Level:         c.Level,
		Price:         c.Price,
		DurationWeeks: c.DurationWeeks,
		Description:   c.Description,
		IsActive:      c.IsActive,
		CreatedAt:     c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

var (
	msgCourseNotFound = i18n.Message{UZ: "Kurs topilmadi.", RU: "Курс не найден.", EN: "Course not found."}
	msgCourseExists   = i18n.Message{UZ: "Bu nomli kurs allaqachon bor.", RU: "Курс с таким названием уже существует.", EN: "A course with this name already exists."}
)

func mapCourseError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, courseusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, repo.ErrAlreadyExists):
		return huma.Error409Conflict(msgCourseExists.For(lang))
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgCourseNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("courses: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

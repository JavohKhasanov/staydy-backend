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
	enrollmentusecase "github.com/student-success/backend/internal/usecase/enrollment"
)

type enrollmentResponse struct {
	ID        string  `json:"id"`
	StudentID string  `json:"studentId"`
	GroupID   *string `json:"groupId,omitempty"`
	CourseID  *string `json:"courseId,omitempty"`
	Status    string  `json:"status"`
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
	Price     int64   `json:"price"`
	Discount  int     `json:"discount"`
	CreatedAt string  `json:"createdAt"`
}

type enrollmentBody struct {
	GroupID   string `json:"groupId,omitempty"`
	CourseID  string `json:"courseId,omitempty"`
	Status    string `json:"status,omitempty" enum:"active,completed,dropped,frozen"`
	StartDate string `json:"startDate,omitempty" doc:"YYYY-MM-DD"`
	EndDate   string `json:"endDate,omitempty" doc:"YYYY-MM-DD"`
	Price     int64  `json:"price,omitempty" minimum:"0"`
	Discount  int    `json:"discount,omitempty" minimum:"0" maximum:"100"`
}

type listEnrollmentsInput struct {
	ID string `path:"id" format:"uuid"`
}
type listEnrollmentsOutput struct{ Body []enrollmentResponse }
type createEnrollmentInput struct {
	ID   string `path:"id" format:"uuid"`
	Body enrollmentBody
}
type updateEnrollmentInput struct {
	ID   string `path:"id" format:"uuid"`
	Body enrollmentBody
}
type enrollmentIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type enrollmentOutput struct{ Body enrollmentResponse }
type deleteEnrollmentOutput struct{}

// registerEnrollments mounts enrolment management. Mount on a group gated to center_admin.
func registerEnrollments(api huma.API, svc *enrollmentusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "enrollments-list",
		Method:      http.MethodGet,
		Path:        "/students/{id}/enrollments",
		Summary:     "List a student's enrolments",
		Tags:        []string{"enrollments"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *listEnrollmentsInput) (*listEnrollmentsOutput, error) {
		p, sid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		es, err := svc.ListByStudent(ctx, p.OrgID, sid)
		if err != nil {
			return nil, mapEnrollmentError(LangFromContext(ctx), err, log)
		}
		out := &listEnrollmentsOutput{Body: make([]enrollmentResponse, 0, len(es))}
		for _, e := range es {
			out.Body = append(out.Body, toEnrollmentResponse(e))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "enrollments-create",
		Method:        http.MethodPost,
		Path:          "/students/{id}/enrollments",
		Summary:       "Enrol a student in a group/course",
		Tags:          []string{"enrollments"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createEnrollmentInput) (*enrollmentOutput, error) {
		p, sid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		input, err := in.Body.toInput()
		if err != nil {
			return nil, err
		}
		e, err := svc.Create(ctx, p.OrgID, sid, input)
		if err != nil {
			return nil, mapEnrollmentError(LangFromContext(ctx), err, log)
		}
		return &enrollmentOutput{Body: toEnrollmentResponse(e)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "enrollments-update",
		Method:      http.MethodPut,
		Path:        "/enrollments/{id}",
		Summary:     "Update an enrolment (status/group/course/price/dates)",
		Tags:        []string{"enrollments"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateEnrollmentInput) (*enrollmentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		input, err := in.Body.toInput()
		if err != nil {
			return nil, err
		}
		e, err := svc.Update(ctx, p.OrgID, id, input)
		if err != nil {
			return nil, mapEnrollmentError(LangFromContext(ctx), err, log)
		}
		return &enrollmentOutput{Body: toEnrollmentResponse(e)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "enrollments-delete",
		Method:        http.MethodDelete,
		Path:          "/enrollments/{id}",
		Summary:       "Delete an enrolment",
		Tags:          []string{"enrollments"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *enrollmentIDInput) (*deleteEnrollmentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapEnrollmentError(LangFromContext(ctx), err, log)
		}
		return &deleteEnrollmentOutput{}, nil
	})
}

// orgAndID resolves the principal and parses a path uuid in one step.
func orgAndID(ctx context.Context, rawID string) (entity.Principal, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return entity.Principal{}, uuid.Nil, err
	}
	id, perr := uuid.Parse(rawID)
	if perr != nil {
		return entity.Principal{}, uuid.Nil, huma.Error422UnprocessableEntity("invalid id")
	}
	return p, id, nil
}

func (b enrollmentBody) toInput() (enrollmentusecase.Input, error) {
	gid, err := parseOptUUID(b.GroupID)
	if err != nil {
		return enrollmentusecase.Input{}, huma.Error422UnprocessableEntity("invalid groupId")
	}
	cid, err := parseOptUUID(b.CourseID)
	if err != nil {
		return enrollmentusecase.Input{}, huma.Error422UnprocessableEntity("invalid courseId")
	}
	start, err := parseOptDate(b.StartDate)
	if err != nil {
		return enrollmentusecase.Input{}, huma.Error422UnprocessableEntity("startDate must be YYYY-MM-DD")
	}
	end, err := parseOptDate(b.EndDate)
	if err != nil {
		return enrollmentusecase.Input{}, huma.Error422UnprocessableEntity("endDate must be YYYY-MM-DD")
	}
	return enrollmentusecase.Input{
		GroupID:   gid,
		CourseID:  cid,
		Status:    b.Status,
		StartDate: start,
		EndDate:   end,
		Price:     b.Price,
		Discount:  b.Discount,
	}, nil
}

func parseOptUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseOptDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func toEnrollmentResponse(e entity.Enrollment) enrollmentResponse {
	r := enrollmentResponse{
		ID:        e.ID.String(),
		StudentID: e.StudentID.String(),
		Status:    e.Status,
		Price:     e.Price,
		Discount:  e.Discount,
		CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339),
	}
	if e.GroupID != nil {
		s := e.GroupID.String()
		r.GroupID = &s
	}
	if e.CourseID != nil {
		s := e.CourseID.String()
		r.CourseID = &s
	}
	if e.StartDate != nil {
		s := e.StartDate.Format("2006-01-02")
		r.StartDate = &s
	}
	if e.EndDate != nil {
		s := e.EndDate.Format("2006-01-02")
		r.EndDate = &s
	}
	return r
}

var msgEnrollmentNotFound = i18n.Message{UZ: "Yozuv topilmadi.", RU: "Запись не найдена.", EN: "Enrolment not found."}

func mapEnrollmentError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, enrollmentusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgEnrollmentNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("enrollments: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

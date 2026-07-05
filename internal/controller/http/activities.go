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
	activityusecase "github.com/student-success/backend/internal/usecase/activity"
)

type activityResponse struct {
	ID          string `json:"id"`
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
	Type        string `json:"type"`
	Body        string `json:"body"`
	Author      string `json:"author,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

type listActivitiesInput struct {
	SubjectType string `query:"subjectType" enum:"lead,student"`
	SubjectID   string `query:"subjectId" format:"uuid"`
}
type listActivitiesOutput struct{ Body []activityResponse }
type createActivityInput struct {
	Body struct {
		SubjectType string `json:"subjectType" enum:"lead,student"`
		SubjectID   string `json:"subjectId" format:"uuid"`
		Type        string `json:"type,omitempty" enum:"note,call,sms,meeting"`
		Body        string `json:"body" minLength:"1" maxLength:"2000"`
	}
}
type activityOutput struct{ Body activityResponse }
type activityIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type deleteActivityOutput struct{}

// registerActivities mounts the communication timeline. Mount on a group gated to center_admin.
func registerActivities(api huma.API, svc *activityusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "activities-list",
		Method:      http.MethodGet,
		Path:        "/activities",
		Summary:     "List a subject's (lead/student) communication timeline",
		Tags:        []string{"activities"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *listActivitiesInput) (*listActivitiesOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		sid, perr := uuid.Parse(in.SubjectID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid subjectId")
		}
		as, err := svc.List(ctx, p.OrgID, in.SubjectType, sid)
		if err != nil {
			return nil, mapActivityError(LangFromContext(ctx), err, log)
		}
		out := &listActivitiesOutput{Body: make([]activityResponse, 0, len(as))}
		for _, a := range as {
			out.Body = append(out.Body, toActivityResponse(a))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "activities-create",
		Method:        http.MethodPost,
		Path:          "/activities",
		Summary:       "Log an activity (call/sms/note/meeting) on a lead or student",
		Tags:          []string{"activities"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createActivityInput) (*activityOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		sid, perr := uuid.Parse(in.Body.SubjectID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid subjectId")
		}
		a, err := svc.Create(ctx, p.OrgID, activityusecase.Input{
			SubjectType: in.Body.SubjectType,
			SubjectID:   sid,
			Type:        in.Body.Type,
			Body:        in.Body.Body,
			Author:      p.FullName,
		})
		if err != nil {
			return nil, mapActivityError(LangFromContext(ctx), err, log)
		}
		return &activityOutput{Body: toActivityResponse(a)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "activities-delete",
		Method:        http.MethodDelete,
		Path:          "/activities/{id}",
		Summary:       "Delete an activity",
		Tags:          []string{"activities"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *activityIDInput) (*deleteActivityOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapActivityError(LangFromContext(ctx), err, log)
		}
		return &deleteActivityOutput{}, nil
	})
}

func toActivityResponse(a entity.Activity) activityResponse {
	return activityResponse{
		ID:          a.ID.String(),
		SubjectType: a.SubjectType,
		SubjectID:   a.SubjectID.String(),
		Type:        a.Type,
		Body:        a.Body,
		Author:      a.Author,
		CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func mapActivityError(lang i18n.Lang, err error, log zerolog.Logger) error {
	if errors.Is(err, activityusecase.ErrValidation) {
		return huma.Error422UnprocessableEntity(err.Error())
	}
	log.Error().Err(err).Msg("activities: unexpected error")
	return huma.Error500InternalServerError(msgInternal.For(lang))
}

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
	interventionusecase "github.com/student-success/backend/internal/usecase/intervention"
)

type taskResponse struct {
	ID                string   `json:"id"`
	StudentID         string   `json:"studentId"`
	StudentName       string   `json:"studentName"`
	Reasons           []string `json:"reasons"`
	Status            string   `json:"status"`
	ResolutionComment string   `json:"resolutionComment,omitempty"`
	CreatedAt         string   `json:"createdAt"`
	ResolvedAt        string   `json:"resolvedAt,omitempty"`
}

type resolveTaskRequest struct {
	ResolutionComment string `json:"resolutionComment" minLength:"1" maxLength:"2000"`
}

type listTasksInput struct{}
type listTasksOutput struct{ Body []taskResponse }
type resolveTaskInput struct {
	ID   string `path:"id" format:"uuid"`
	Body resolveTaskRequest
}
type taskOutput struct{ Body taskResponse }

// registerInterventions mounts the tenant-scoped intervention-task operations.
func registerInterventions(api huma.API, svc *interventionusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "intervention-tasks-list",
		Method:      http.MethodGet,
		Path:        "/intervention-tasks",
		Summary:     "List intervention tasks (kanban: Open / In Progress / Resolved)",
		Tags:        []string{"intervention-tasks"},
		Errors:      []int{http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listTasksInput) (*listTasksOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		tasks, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapInterventionError(LangFromContext(ctx), err, log)
		}
		out := &listTasksOutput{Body: make([]taskResponse, 0, len(tasks))}
		for _, t := range tasks {
			out.Body = append(out.Body, toTaskResponse(t))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "intervention-tasks-resolve",
		Method:      http.MethodPost,
		Path:        "/intervention-tasks/{id}/resolve",
		Summary:     "Resolve an intervention task",
		Tags:        []string{"intervention-tasks"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *resolveTaskInput) (*taskOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid task id")
		}
		task, err := svc.Resolve(ctx, p.OrgID, id, in.Body.ResolutionComment)
		if err != nil {
			return nil, mapInterventionError(LangFromContext(ctx), err, log)
		}
		return &taskOutput{Body: toTaskResponse(task)}, nil
	})
}

func toTaskResponse(t entity.InterventionTask) taskResponse {
	r := taskResponse{
		ID:                t.ID.String(),
		StudentID:         t.StudentID.String(),
		StudentName:       t.StudentName,
		Reasons:           t.Reasons,
		Status:            t.Status,
		ResolutionComment: t.ResolutionComment,
		CreatedAt:         t.CreatedAt.UTC().Format(time.RFC3339),
	}
	if t.ResolvedAt != nil {
		r.ResolvedAt = t.ResolvedAt.UTC().Format(time.RFC3339)
	}
	return r
}

var msgTaskNotFound = i18n.Message{UZ: "Vazifa topilmadi.", RU: "Задача не найдена.", EN: "Task not found."}

func mapInterventionError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, interventionusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, interventionusecase.ErrNotFound):
		return huma.Error404NotFound(msgTaskNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("intervention: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

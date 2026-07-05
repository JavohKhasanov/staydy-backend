package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	"github.com/student-success/backend/internal/repo"
	leadusecase "github.com/student-success/backend/internal/usecase/lead"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

type leadResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Phone      string `json:"phone,omitempty"`
	Email      string `json:"email,omitempty"`
	Source     string `json:"source,omitempty"`
	Stage      string `json:"stage"`
	Interest   string `json:"interest,omitempty"`
	Note       string `json:"note,omitempty"`
	AssignedTo string `json:"assignedTo,omitempty"`
	StudentID  string `json:"studentId,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

type leadBody struct {
	Name       string `json:"name" minLength:"1" maxLength:"255" example:"Dilnoza Karimova"`
	Phone      string `json:"phone,omitempty" maxLength:"32"`
	Email      string `json:"email,omitempty" maxLength:"255"`
	Source     string `json:"source,omitempty" maxLength:"64" example:"instagram"`
	Stage      string `json:"stage,omitempty" enum:"new,contacted,trial,enrolled,lost"`
	Interest   string `json:"interest,omitempty" maxLength:"255"`
	Note       string `json:"note,omitempty" maxLength:"2000"`
	AssignedTo string `json:"assignedTo,omitempty"`
}

type listLeadsInput struct{}
type listLeadsOutput struct{ Body []leadResponse }
type createLeadInput struct{ Body leadBody }
type updateLeadInput struct {
	ID   string `path:"id" format:"uuid"`
	Body leadBody
}
type leadStageInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Stage string `json:"stage" enum:"new,contacted,trial,enrolled,lost"`
	}
}
type leadIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type leadOutput struct{ Body leadResponse }
type convertLeadOutput struct {
	Body struct {
		StudentID string `json:"studentId"`
	}
}
type deleteLeadOutput struct{}

// registerLeads mounts the sales funnel. Conversion spawns a student, so it needs the student
// service too. Mount on a group gated to center_admin.
func registerLeads(api huma.API, svc *leadusecase.Service, students *studentusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "leads-list",
		Method:      http.MethodGet,
		Path:        "/leads",
		Summary:     "List leads (sales funnel)",
		Tags:        []string{"leads"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listLeadsInput) (*listLeadsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		ls, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapLeadError(LangFromContext(ctx), err, log)
		}
		out := &listLeadsOutput{Body: make([]leadResponse, 0, len(ls))}
		for _, l := range ls {
			out.Body = append(out.Body, toLeadResponse(l))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "leads-create",
		Method:        http.MethodPost,
		Path:          "/leads",
		Summary:       "Add a lead",
		Tags:          []string{"leads"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createLeadInput) (*leadOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		input, err := in.Body.toInput()
		if err != nil {
			return nil, err
		}
		l, err := svc.Create(ctx, p.OrgID, input)
		if err != nil {
			return nil, mapLeadError(LangFromContext(ctx), err, log)
		}
		return &leadOutput{Body: toLeadResponse(l)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "leads-update",
		Method:      http.MethodPut,
		Path:        "/leads/{id}",
		Summary:     "Edit a lead",
		Tags:        []string{"leads"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateLeadInput) (*leadOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		input, ierr := in.Body.toInput()
		if ierr != nil {
			return nil, ierr
		}
		l, err := svc.Update(ctx, p.OrgID, id, input)
		if err != nil {
			return nil, mapLeadError(LangFromContext(ctx), err, log)
		}
		return &leadOutput{Body: toLeadResponse(l)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "leads-set-stage",
		Method:      http.MethodPut,
		Path:        "/leads/{id}/stage",
		Summary:     "Move a lead to another pipeline stage",
		Tags:        []string{"leads"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *leadStageInput) (*leadOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		l, err := svc.SetStage(ctx, p.OrgID, id, in.Body.Stage)
		if err != nil {
			return nil, mapLeadError(LangFromContext(ctx), err, log)
		}
		return &leadOutput{Body: toLeadResponse(l)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "leads-convert",
		Method:        http.MethodPost,
		Path:          "/leads/{id}/convert",
		Summary:       "Convert a lead into a student (stage → enrolled)",
		Tags:          []string{"leads"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *leadIDInput) (*convertLeadOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		lead, err := svc.Get(ctx, p.OrgID, id)
		if err != nil {
			return nil, mapLeadError(LangFromContext(ctx), err, log)
		}
		st, err := students.Create(ctx, p.OrgID, studentusecase.CreateParams{
			Name:            lead.Name,
			Phone:           lead.Phone,
			Email:           lead.Email,
			ConfidenceLevel: 5,
			Status:          "active",
		})
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		if err := svc.MarkConverted(ctx, p.OrgID, id, st.ID); err != nil {
			return nil, mapLeadError(LangFromContext(ctx), err, log)
		}
		out := &convertLeadOutput{}
		out.Body.StudentID = st.ID.String()
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "leads-delete",
		Method:        http.MethodDelete,
		Path:          "/leads/{id}",
		Summary:       "Delete a lead",
		Tags:          []string{"leads"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *leadIDInput) (*deleteLeadOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapLeadError(LangFromContext(ctx), err, log)
		}
		return &deleteLeadOutput{}, nil
	})
}

func (b leadBody) toInput() (leadusecase.Input, error) {
	assigned, err := parseOptUUID(b.AssignedTo)
	if err != nil {
		return leadusecase.Input{}, huma.Error422UnprocessableEntity("invalid assignedTo")
	}
	return leadusecase.Input{
		Name:       b.Name,
		Phone:      b.Phone,
		Email:      b.Email,
		Source:     b.Source,
		Stage:      b.Stage,
		Interest:   b.Interest,
		Note:       b.Note,
		AssignedTo: assigned,
	}, nil
}

func toLeadResponse(l entity.Lead) leadResponse {
	r := leadResponse{
		ID:        l.ID.String(),
		Name:      l.Name,
		Phone:     l.Phone,
		Email:     l.Email,
		Source:    l.Source,
		Stage:     l.Stage,
		Interest:  l.Interest,
		Note:      l.Note,
		CreatedAt: l.CreatedAt.UTC().Format(time.RFC3339),
	}
	if l.AssignedTo != nil {
		r.AssignedTo = l.AssignedTo.String()
	}
	if l.StudentID != nil {
		r.StudentID = l.StudentID.String()
	}
	return r
}

var msgLeadNotFound = i18n.Message{UZ: "Lid topilmadi.", RU: "Лид не найден.", EN: "Lead not found."}

func mapLeadError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, leadusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgLeadNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("leads: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

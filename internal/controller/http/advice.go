package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/i18n"
	adviceusecase "github.com/student-success/backend/internal/usecase/advice"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

type adviceResponse struct {
	Advice      string `json:"advice" doc:"AI-generated mentor recommendation (Uzbek, markdown)"`
	GeneratedAt string `json:"generatedAt"`
	Cached      bool   `json:"cached" doc:"true when served from the short-lived cache (inputs unchanged)"`
}

type adviceInput struct {
	ID string `path:"id" format:"uuid"`
}
type adviceOutput struct{ Body adviceResponse }

type adviceFeedbackInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Useful bool `json:"useful" doc:"true = the recommendation was helpful"`
	}
}
type adviceFeedbackOutput struct {
	Body struct {
		OK bool `json:"ok"`
	}
}

var msgAIUnavailable = i18n.Message{
	UZ: "AI maslahatchi hozircha mavjud emas.",
	RU: "AI-советник сейчас недоступен.",
	EN: "AI advisor is currently unavailable.",
}

// registerAdvice mounts POST /students/{id}/ai-advice. It composes two usecases: the student
// service loads the detail (tenant-scoped, 404 on miss), the advice service generates the text.
func registerAdvice(api huma.API, students *studentusecase.Service, advisor *adviceusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "students-ai-advice",
		Method:      http.MethodPost,
		Path:        "/students/{id}/ai-advice",
		Summary:     "Generate an AI retention recommendation for a student",
		Tags:        []string{"students"},
		Errors:      []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusServiceUnavailable, http.StatusInternalServerError},
	}), func(ctx context.Context, in *adviceInput) (*adviceOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		lang := LangFromContext(ctx)

		detail, err := students.Get(ctx, p.OrgID, id)
		if err != nil {
			return nil, mapStudentError(lang, err, log)
		}

		out, err := advisor.Advise(ctx, adviceusecase.Input{
			Student:    detail.Student,
			Surveys:    detail.Surveys,
			Attendance: detail.Attendance,
			Notes:      detail.Notes,
			Factors:    detail.Factors,
		})
		if err != nil {
			return nil, mapAdviceError(lang, err, log)
		}

		return &adviceOutput{Body: adviceResponse{
			Advice:      out.Text,
			GeneratedAt: out.GeneratedAt.UTC().Format(time.RFC3339),
			Cached:      out.Cached,
		}}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "students-ai-advice-feedback",
		Method:        http.MethodPost,
		Path:          "/students/{id}/ai-advice/feedback",
		Summary:       "Record whether an AI recommendation was useful",
		Tags:          []string{"students"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}), func(ctx context.Context, in *adviceFeedbackInput) (*adviceFeedbackOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		if err := students.RecordAdviceFeedback(ctx, p.OrgID, id, p.UserID, in.Body.Useful); err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		out := &adviceFeedbackOutput{}
		out.Body.OK = true
		return out, nil
	})
}

func mapAdviceError(lang i18n.Lang, err error, log zerolog.Logger) error {
	if errors.Is(err, adviceusecase.ErrNotConfigured) {
		return huma.Error503ServiceUnavailable(msgAIUnavailable.For(lang))
	}
	// Upstream model failure (network, API error, safety block): a transient 503 the UI can retry,
	// without leaking provider details.
	log.Error().Err(err).Msg("advice: generation failed")
	return huma.Error503ServiceUnavailable(msgAIUnavailable.For(lang))
}

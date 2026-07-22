package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	homeworkusecase "github.com/student-success/backend/internal/usecase/homework"
)

type studentAssignmentResponse struct {
	ID              string  `json:"id"`
	GroupID         string  `json:"groupId"`
	GroupName       string  `json:"groupName"`
	LessonDate      *string `json:"lessonDate,omitempty"`
	Title           string  `json:"title"`
	Description     string  `json:"description,omitempty"`
	Deadline        *string `json:"deadline,omitempty"`
	MaxScore        int     `json:"maxScore"`
	Status          string  `json:"status"` // "" = not submitted yet
	Score           *int    `json:"score,omitempty"`
	SubmissionText  string  `json:"submissionText,omitempty"`
	SubmissionLinks string  `json:"submissionLinks,omitempty"`
	ReviewNote      string  `json:"reviewNote,omitempty"`
	SubmittedAt     *string `json:"submittedAt,omitempty"`
}

type submitHomeworkInput struct {
	ID   string `path:"id" format:"uuid"` // assignment id
	Body struct {
		Text  string `json:"text,omitempty" maxLength:"4000"`
		Links string `json:"links,omitempty" maxLength:"2000" doc:"newline-separated URLs"`
	}
}

type studentAssignmentsOutput struct{ Body []studentAssignmentResponse }

// registerStudentHomework mounts the student-facing homework endpoints. Mount on the student group.
func registerStudentHomework(api huma.API, svc *homeworkusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-homework-list",
		Method:      http.MethodGet,
		Path:        "/student/homework",
		Summary:     "The student's homework across their groups (with their submission)",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*studentAssignmentsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		list, err := svc.StudentAssignments(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student homework list failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &studentAssignmentsOutput{Body: make([]studentAssignmentResponse, 0, len(list))}
		for _, a := range list {
			out.Body = append(out.Body, toStudentAssignmentResponse(a))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-homework-submit",
		Method:      http.MethodPost,
		Path:        "/student/homework/{id}/submit",
		Summary:     "Submit (or re-submit) a homework assignment",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, in *submitHomeworkInput) (*submissionOutput, error) {
		p, aid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		s, err := svc.Submit(ctx, p.OrgID, p.UserID, aid, in.Body.Text, in.Body.Links)
		if err != nil {
			return nil, mapHomeworkError(err, log)
		}
		return &submissionOutput{Body: toSubmissionResponse(s)}, nil
	})
}

func toStudentAssignmentResponse(a entity.StudentAssignment) studentAssignmentResponse {
	r := studentAssignmentResponse{
		ID: a.ID.String(), GroupID: a.GroupID.String(), GroupName: a.GroupName,
		Title: a.Title, Description: a.Description, MaxScore: a.MaxScore,
		Status: a.SubmissionStatus, Score: a.Score,
		SubmissionText: a.SubmissionText, SubmissionLinks: a.SubmissionLinks, ReviewNote: a.ReviewNote,
	}
	if a.LessonDate != nil {
		v := a.LessonDate.Format("2006-01-02")
		r.LessonDate = &v
	}
	if a.Deadline != nil {
		v := a.Deadline.UTC().Format(time.RFC3339)
		r.Deadline = &v
	}
	if a.SubmittedAt != nil {
		v := a.SubmittedAt.UTC().Format(time.RFC3339)
		r.SubmittedAt = &v
	}
	return r
}

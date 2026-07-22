package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	homeworkusecase "github.com/student-success/backend/internal/usecase/homework"
)

type assignmentResponse struct {
	ID              string  `json:"id"`
	GroupID         string  `json:"groupId"`
	GroupName       string  `json:"groupName,omitempty"`
	LessonDate      *string `json:"lessonDate,omitempty"`
	Title           string  `json:"title"`
	Description     string  `json:"description,omitempty"`
	Deadline        *string `json:"deadline,omitempty"`
	MaxScore        int     `json:"maxScore"`
	CreatedAt       string  `json:"createdAt"`
	SubmissionCount int64   `json:"submissionCount"`
}

type submissionResponse struct {
	ID          string  `json:"id"`
	StudentID   string  `json:"studentId"`
	StudentName string  `json:"studentName,omitempty"`
	Text        string  `json:"text,omitempty"`
	Links       string  `json:"links,omitempty"`
	Status      string  `json:"status"`
	Score       *int    `json:"score,omitempty"`
	ReviewNote  string  `json:"reviewNote,omitempty"`
	SubmittedAt string  `json:"submittedAt"`
	ReviewedAt  *string `json:"reviewedAt,omitempty"`
}

type createAssignmentInput struct {
	ID   string `path:"id" format:"uuid"` // group id
	Body struct {
		Title       string `json:"title" minLength:"1" maxLength:"200"`
		Description string `json:"description,omitempty" maxLength:"4000"`
		Deadline    string `json:"deadline,omitempty" doc:"RFC3339 datetime"`
		LessonDate  string `json:"lessonDate,omitempty" doc:"YYYY-MM-DD"`
		MaxScore    int    `json:"maxScore,omitempty" minimum:"0"`
	}
}
type gradeInput struct {
	ID   string `path:"id" format:"uuid"` // submission id
	Body struct {
		Status string `json:"status" enum:"accepted,rejected"`
		Score  *int   `json:"score,omitempty" minimum:"0"`
		Note   string `json:"note,omitempty" maxLength:"1000"`
	}
}

type assignmentOutput struct{ Body assignmentResponse }
type assignmentsListOutput struct{ Body []assignmentResponse }
type submissionsListOutput struct{ Body []submissionResponse }
type submissionOutput struct{ Body submissionResponse }

// registerHomework mounts teacher/admin homework management. Mount on a group that admits
// teacher + manager + center_admin + super_admin; teachers are scoped to their own groups in the usecase.
func registerHomework(api huma.API, svc *homeworkusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID:   "homework-create",
		Method:        http.MethodPost,
		Path:          "/groups/{id}/homework",
		Summary:       "Create a homework assignment for a group",
		Tags:          []string{"homework"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createAssignmentInput) (*assignmentOutput, error) {
		p, gid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		deadline, lessonDate, derr := parseHomeworkDates(in.Body.Deadline, in.Body.LessonDate)
		if derr != nil {
			return nil, derr
		}
		a, err := svc.CreateAssignment(ctx, p, homeworkusecase.CreateInput{
			GroupID: gid, Title: in.Body.Title, Description: in.Body.Description,
			Deadline: deadline, LessonDate: lessonDate, MaxScore: in.Body.MaxScore,
		})
		if err != nil {
			return nil, mapHomeworkError(err, log)
		}
		return &assignmentOutput{Body: toAssignmentResponse(a)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "homework-list",
		Method:      http.MethodGet,
		Path:        "/groups/{id}/homework",
		Summary:     "List a group's homework assignments",
		Tags:        []string{"homework"},
		Errors:      []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *groupIDInput) (*assignmentsListOutput, error) {
		p, gid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		list, err := svc.ListGroupAssignments(ctx, p, gid)
		if err != nil {
			return nil, mapHomeworkError(err, log)
		}
		out := &assignmentsListOutput{Body: make([]assignmentResponse, 0, len(list))}
		for _, a := range list {
			out.Body = append(out.Body, toAssignmentResponse(a))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "homework-delete",
		Method:        http.MethodDelete,
		Path:          "/homework/{id}",
		Summary:       "Delete a homework assignment",
		Tags:          []string{"homework"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteAssignment(ctx, p, id); err != nil {
			return nil, mapHomeworkError(err, log)
		}
		return &noContentOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "homework-submissions",
		Method:      http.MethodGet,
		Path:        "/homework/{id}/submissions",
		Summary:     "List submissions for an assignment (grading view)",
		Tags:        []string{"homework"},
		Errors:      []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*submissionsListOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		_, subs, err := svc.ListSubmissions(ctx, p, id)
		if err != nil {
			return nil, mapHomeworkError(err, log)
		}
		out := &submissionsListOutput{Body: make([]submissionResponse, 0, len(subs))}
		for _, s := range subs {
			out.Body = append(out.Body, toSubmissionResponse(s))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "homework-grade",
		Method:      http.MethodPatch,
		Path:        "/submissions/{id}/grade",
		Summary:     "Grade a submission (accepted/rejected + score)",
		Tags:        []string{"homework"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *gradeInput) (*submissionOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		s, err := svc.Grade(ctx, p, id, in.Body.Status, in.Body.Score, in.Body.Note)
		if err != nil {
			return nil, mapHomeworkError(err, log)
		}
		return &submissionOutput{Body: toSubmissionResponse(s)}, nil
	})
}

func parseHomeworkDates(deadline, lessonDate string) (*time.Time, *time.Time, error) {
	var dl, ld *time.Time
	if deadline != "" {
		t, e := time.Parse(time.RFC3339, deadline)
		if e != nil {
			return nil, nil, huma.Error422UnprocessableEntity("deadline must be RFC3339")
		}
		dl = &t
	}
	if lessonDate != "" {
		t, e := time.Parse("2006-01-02", lessonDate)
		if e != nil {
			return nil, nil, huma.Error422UnprocessableEntity("lessonDate must be YYYY-MM-DD")
		}
		ld = &t
	}
	return dl, ld, nil
}

func toAssignmentResponse(a entity.HomeworkAssignment) assignmentResponse {
	r := assignmentResponse{
		ID: a.ID.String(), GroupID: a.GroupID.String(), GroupName: a.GroupName,
		Title: a.Title, Description: a.Description, MaxScore: a.MaxScore,
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339), SubmissionCount: a.SubmissionCount,
	}
	if a.LessonDate != nil {
		v := a.LessonDate.Format("2006-01-02")
		r.LessonDate = &v
	}
	if a.Deadline != nil {
		v := a.Deadline.UTC().Format(time.RFC3339)
		r.Deadline = &v
	}
	return r
}

func toSubmissionResponse(s entity.HomeworkSubmission) submissionResponse {
	r := submissionResponse{
		ID: s.ID.String(), StudentID: s.StudentID.String(), StudentName: s.StudentName,
		Text: s.Text, Links: s.Links, Status: s.Status, Score: s.Score, ReviewNote: s.ReviewNote,
		SubmittedAt: s.SubmittedAt.UTC().Format(time.RFC3339),
	}
	if s.ReviewedAt != nil {
		v := s.ReviewedAt.UTC().Format(time.RFC3339)
		r.ReviewedAt = &v
	}
	return r
}

func mapHomeworkError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, homeworkusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("ma'lumotlar noto'g'ri")
	case errors.Is(err, homeworkusecase.ErrForbidden):
		return huma.Error403Forbidden("ruxsat yo'q")
	case errors.Is(err, homeworkusecase.ErrNotFound):
		return huma.Error404NotFound("topilmadi")
	default:
		log.Error().Err(err).Msg("homework op failed")
		return huma.Error500InternalServerError("internal error")
	}
}

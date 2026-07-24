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
	examusecase "github.com/student-success/backend/internal/usecase/exam"
)

type examResponse struct {
	ID          string `json:"id"`
	GroupID     string `json:"groupId"`
	Title       string `json:"title"`
	ExamDate    string `json:"examDate,omitempty"`
	MaxScore    int    `json:"maxScore"`
	CreatedAt   string `json:"createdAt"`
	ResultCount int64  `json:"resultCount"`
}

type examResultResponse struct {
	StudentID   string `json:"studentId"`
	StudentName string `json:"studentName,omitempty"`
	Score       int    `json:"score"`
}

type studentExamResponse struct {
	ExamID   string `json:"examId"`
	Title    string `json:"title"`
	ExamDate string `json:"examDate,omitempty"`
	MaxScore int    `json:"maxScore"`
	Score    int    `json:"score"`
}

type createExamInput struct {
	ID   string `path:"id" format:"uuid"` // group id
	Body struct {
		Title    string `json:"title" minLength:"1" maxLength:"200"`
		ExamDate string `json:"examDate,omitempty" doc:"YYYY-MM-DD"`
		MaxScore int    `json:"maxScore,omitempty" minimum:"0"`
	}
}
type examOutput struct{ Body examResponse }
type examListOutput struct{ Body []examResponse }

type examResultsBody struct {
	Exam    examResponse         `json:"exam"`
	Results []examResultResponse `json:"results"`
}
type examResultsOutput struct{ Body examResultsBody }

type gradeExamInput struct {
	ID        string `path:"id" format:"uuid"`        // exam id
	StudentID string `path:"studentId" format:"uuid"` // student id
	Body      struct {
		Score int `json:"score" minimum:"0"`
	}
}
type studentExamsOutput struct{ Body []studentExamResponse }
type examGradeOutput struct{ Body examResultResponse }

func toExamResponse(e entity.Exam) examResponse {
	r := examResponse{
		ID: e.ID.String(), GroupID: e.GroupID.String(), Title: e.Title,
		MaxScore: e.MaxScore, CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339), ResultCount: e.ResultCount,
	}
	if e.ExamDate != nil {
		r.ExamDate = e.ExamDate.Format("2006-01-02")
	}
	return r
}

func mapExamError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, examusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("ma'lumotlar noto'g'ri")
	case errors.Is(err, examusecase.ErrForbidden):
		return huma.Error403Forbidden("bu guruh sizniki emas")
	case errors.Is(err, examusecase.ErrNotFound):
		return huma.Error404NotFound("topilmadi")
	default:
		log.Error().Err(err).Msg("exam op failed")
		return huma.Error500InternalServerError("internal error")
	}
}

// registerExams mounts the teacher/admin exam operations (create, grade, results). Mount on the
// teaching group.
func registerExams(api huma.API, svc *examusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID:   "exam-create",
		Method:        http.MethodPost,
		Path:          "/groups/{id}/exams",
		Summary:       "Create an exam for a group",
		Tags:          []string{"exams"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createExamInput) (*examOutput, error) {
		p, gid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		var date *time.Time
		if in.Body.ExamDate != "" {
			d, perr := time.Parse("2006-01-02", in.Body.ExamDate)
			if perr != nil {
				return nil, huma.Error422UnprocessableEntity("examDate must be YYYY-MM-DD")
			}
			date = &d
		}
		ex, err := svc.CreateExam(ctx, p, gid, in.Body.Title, date, in.Body.MaxScore)
		if err != nil {
			return nil, mapExamError(err, log)
		}
		return &examOutput{Body: toExamResponse(ex)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "exam-list",
		Method:      http.MethodGet,
		Path:        "/groups/{id}/exams",
		Summary:     "List a group's exams",
		Tags:        []string{"exams"},
		Errors:      []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*examListOutput, error) {
		p, gid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		exams, err := svc.ListGroupExams(ctx, p, gid)
		if err != nil {
			return nil, mapExamError(err, log)
		}
		out := &examListOutput{Body: make([]examResponse, 0, len(exams))}
		for _, e := range exams {
			out.Body = append(out.Body, toExamResponse(e))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "exam-delete",
		Method:        http.MethodDelete,
		Path:          "/exams/{id}",
		Summary:       "Delete an exam",
		Tags:          []string{"exams"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteExam(ctx, p, id); err != nil {
			return nil, mapExamError(err, log)
		}
		return &noContentOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "exam-results",
		Method:      http.MethodGet,
		Path:        "/exams/{id}/results",
		Summary:     "An exam with every student's score (for grading)",
		Tags:        []string{"exams"},
		Errors:      []int{http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*examResultsOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		ex, results, err := svc.Results(ctx, p, id)
		if err != nil {
			return nil, mapExamError(err, log)
		}
		out := &examResultsOutput{}
		out.Body.Exam = toExamResponse(ex)
		out.Body.Results = make([]examResultResponse, 0, len(results))
		for _, r := range results {
			out.Body.Results = append(out.Body.Results, examResultResponse{
				StudentID: r.StudentID.String(), StudentName: r.StudentName, Score: r.Score,
			})
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "exam-grade",
		Method:      http.MethodPut,
		Path:        "/exams/{id}/results/{studentId}",
		Summary:     "Record a student's exam score (awards proportional XP)",
		Tags:        []string{"exams"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *gradeExamInput) (*examGradeOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		examID, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid exam id")
		}
		studentID, err := uuid.Parse(in.StudentID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		res, err := svc.Grade(ctx, p, examID, studentID, in.Body.Score)
		if err != nil {
			return nil, mapExamError(err, log)
		}
		return &examGradeOutput{
			Body: examResultResponse{StudentID: res.StudentID.String(), Score: res.Score},
		}, nil
	})
}

// registerStudentExams mounts the student's own exam results. Mount on the student group.
func registerStudentExams(api huma.API, svc *examusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-exams",
		Method:      http.MethodGet,
		Path:        "/student/exams",
		Summary:     "The signed-in student's exam results",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*studentExamsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		results, err := svc.StudentResults(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student exams failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &studentExamsOutput{Body: make([]studentExamResponse, 0, len(results))}
		for _, r := range results {
			er := studentExamResponse{
				ExamID: r.ExamID.String(), Title: r.Title, MaxScore: r.MaxScore, Score: r.Score,
			}
			if r.ExamDate != nil {
				er.ExamDate = r.ExamDate.Format("2006-01-02")
			}
			out.Body = append(out.Body, er)
		}
		return out, nil
	})
}

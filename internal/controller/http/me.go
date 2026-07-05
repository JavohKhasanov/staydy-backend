package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	groupusecase "github.com/student-success/backend/internal/usecase/group"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

type meEmptyInput struct{}
type meGroupsOutput struct{ Body []groupResponse }
type meStudentsOutput struct{ Body []studentResponse }

// registerMe mounts the signed-in teacher's own dashboard: their groups/students plus
// attendance & homework marking scoped to students in their own groups. Mount on a group gated
// to the teacher role.
func registerMe(api huma.API, groups *groupusecase.Service, students *studentusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "me-groups",
		Method:      http.MethodGet,
		Path:        "/me/groups",
		Summary:     "List the signed-in teacher's own groups",
		Tags:        []string{"teacher"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *meEmptyInput) (*meGroupsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		gs, err := groups.MyGroups(ctx, p.OrgID, p.UserID)
		if err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		out := &meGroupsOutput{Body: make([]groupResponse, 0, len(gs))}
		for _, g := range gs {
			out.Body = append(out.Body, toGroupResponse(g))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "me-students",
		Method:      http.MethodGet,
		Path:        "/me/students",
		Summary:     "List students in the signed-in teacher's groups (highest risk first)",
		Tags:        []string{"teacher"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *meEmptyInput) (*meStudentsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		sts, err := groups.MyStudents(ctx, p.OrgID, p.UserID)
		if err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		out := &meStudentsOutput{Body: make([]studentResponse, 0, len(sts))}
		for _, st := range sts {
			out.Body = append(out.Body, toStudentResponse(st))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "me-record-attendance",
		Method:        http.MethodPost,
		Path:          "/me/students/{id}/attendance",
		Summary:       "Record attendance for a student in the teacher's own group",
		Tags:          []string{"teacher"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *recordAttendanceInput) (*attendanceOutput, error) {
		p, sid, err := teacherStudent(ctx, groups, log, in.ID)
		if err != nil {
			return nil, err
		}
		date, perr := time.Parse("2006-01-02", in.Body.Date)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("date must be YYYY-MM-DD")
		}
		rec, err := students.RecordAttendance(ctx, p.OrgID, sid, date, attendanceStatus(in.Body))
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &attendanceOutput{Body: toAttendanceResponse(rec)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "me-record-homework",
		Method:        http.MethodPost,
		Path:          "/me/students/{id}/homework",
		Summary:       "Record homework for a student in the teacher's own group",
		Tags:          []string{"teacher"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *recordHomeworkInput) (*homeworkOutput, error) {
		p, sid, err := teacherStudent(ctx, groups, log, in.ID)
		if err != nil {
			return nil, err
		}
		date, perr := time.Parse("2006-01-02", in.Body.Date)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("date must be YYYY-MM-DD")
		}
		rec, err := students.RecordHomework(ctx, p.OrgID, sid, date, in.Body.IsDone)
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &homeworkOutput{Body: toHomeworkResponse(rec)}, nil
	})
}

// teacherStudent resolves the principal and the path student id, and asserts the student belongs
// to one of the caller's groups (403 otherwise).
func teacherStudent(ctx context.Context, groups *groupusecase.Service, log zerolog.Logger, rawID string) (entity.Principal, uuid.UUID, error) {
	p, err := principal(ctx)
	if err != nil {
		return entity.Principal{}, uuid.Nil, err
	}
	sid, perr := uuid.Parse(rawID)
	if perr != nil {
		return entity.Principal{}, uuid.Nil, huma.Error422UnprocessableEntity("invalid student id")
	}
	owns, err := groups.OwnsStudent(ctx, p.OrgID, p.UserID, sid)
	if err != nil {
		return entity.Principal{}, uuid.Nil, mapGroupError(LangFromContext(ctx), err, log)
	}
	if !owns {
		return entity.Principal{}, uuid.Nil, huma.Error403Forbidden("student is not in your group")
	}
	return p, sid, nil
}

package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	"github.com/student-success/backend/internal/repo"
	groupusecase "github.com/student-success/backend/internal/usecase/group"
)

type groupResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TeacherID    string `json:"teacherId,omitempty"`
	CourseID     string `json:"courseId,omitempty"`
	BranchID     string `json:"branchId,omitempty"`
	Direction    string `json:"direction,omitempty"`
	ScheduleDays string `json:"scheduleDays,omitempty"`
	Capacity     int    `json:"capacity"`
	StartTime    string `json:"startTime,omitempty"`
	EndTime      string `json:"endTime,omitempty"`
	RoomID       string `json:"roomId,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type groupRequest struct {
	Name         string `json:"name" minLength:"1" maxLength:"255" example:"Matematika-1"`
	TeacherID    string `json:"teacherId,omitempty" format:"uuid" doc:"Teacher (user) id; omit/empty to leave unassigned"`
	CourseID     string `json:"courseId,omitempty" format:"uuid" doc:"Course this group runs; omit to leave unset"`
	BranchID     string `json:"branchId,omitempty" format:"uuid" doc:"Branch this group runs at; omit to leave org-wide"`
	Direction    string `json:"direction,omitempty" maxLength:"255"`
	ScheduleDays string `json:"scheduleDays,omitempty" maxLength:"255" doc:"weekday codes, e.g. 'mon,wed,fri'"`
	Capacity     int    `json:"capacity,omitempty" minimum:"0"`
	StartTime    string `json:"startTime,omitempty" maxLength:"5" doc:"class start time HH:MM"`
	EndTime      string `json:"endTime,omitempty" maxLength:"5" doc:"class end time HH:MM"`
	RoomID       string `json:"roomId,omitempty" format:"uuid" doc:"room this group meets in; omit to leave unset"`
}

type assignGroupRequest struct {
	GroupID string `json:"groupId,omitempty" format:"uuid" doc:"Group id; omit/empty to remove the student from any group"`
}

type listGroupsInput struct{}
type listGroupsOutput struct{ Body []groupResponse }
type groupIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type createGroupInput struct{ Body groupRequest }
type updateGroupInput struct {
	ID   string `path:"id" format:"uuid"`
	Body groupRequest
}
type groupOutput struct{ Body groupResponse }
type groupStudentsOutput struct{ Body []studentResponse }
type assignGroupInput struct {
	ID   string `path:"id" format:"uuid"`
	Body assignGroupRequest
}
type noContentOutput struct{}

// registerGroups mounts group management + student↔group assignment. Mount on a group gated to
// center_admin / super_admin.
func registerGroups(api huma.API, svc *groupusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "groups-list",
		Method:      http.MethodGet,
		Path:        "/groups",
		Summary:     "List student groups",
		Tags:        []string{"groups"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listGroupsInput) (*listGroupsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		groups, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		out := &listGroupsOutput{Body: make([]groupResponse, 0, len(groups))}
		for _, g := range groups {
			out.Body = append(out.Body, toGroupResponse(g))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "groups-create",
		Method:      http.MethodPost,
		Path:        "/groups",
		Summary:     "Create a student group",
		Tags:        []string{"groups"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createGroupInput) (*groupOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		teacherID, perr := parseOptionalUUID(in.Body.TeacherID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid teacherId")
		}
		courseID, cerr := parseOptionalUUID(in.Body.CourseID)
		if cerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid courseId")
		}
		branchID, brerr := parseOptionalUUID(in.Body.BranchID)
		if brerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid branchId")
		}
		roomID, rmerr := parseOptionalUUID(in.Body.RoomID)
		if rmerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid roomId")
		}
		g, err := svc.Create(ctx, p.OrgID, groupusecase.CreateInput{
			Name:         in.Body.Name,
			TeacherID:    teacherID,
			CourseID:     courseID,
			BranchID:     branchID,
			Direction:    in.Body.Direction,
			ScheduleDays: in.Body.ScheduleDays,
			Capacity:     in.Body.Capacity,
			StartTime:    in.Body.StartTime,
			EndTime:      in.Body.EndTime,
			RoomID:       roomID,
		})
		if err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		return &groupOutput{Body: toGroupResponse(g)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "groups-get",
		Method:      http.MethodGet,
		Path:        "/groups/{id}",
		Summary:     "Get a group",
		Tags:        []string{"groups"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *groupIDInput) (*groupOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid group id")
		}
		g, err := svc.Get(ctx, p.OrgID, id)
		if err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		return &groupOutput{Body: toGroupResponse(g)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "groups-update",
		Method:      http.MethodPut,
		Path:        "/groups/{id}",
		Summary:     "Update a group",
		Tags:        []string{"groups"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateGroupInput) (*groupOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid group id")
		}
		teacherID, perr := parseOptionalUUID(in.Body.TeacherID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid teacherId")
		}
		courseID, cerr := parseOptionalUUID(in.Body.CourseID)
		if cerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid courseId")
		}
		branchID, brerr := parseOptionalUUID(in.Body.BranchID)
		if brerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid branchId")
		}
		roomID, rmerr := parseOptionalUUID(in.Body.RoomID)
		if rmerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid roomId")
		}
		g, err := svc.Update(ctx, p.OrgID, id, groupusecase.UpdateInput{
			Name:         in.Body.Name,
			TeacherID:    teacherID,
			CourseID:     courseID,
			BranchID:     branchID,
			Direction:    in.Body.Direction,
			ScheduleDays: in.Body.ScheduleDays,
			Capacity:     in.Body.Capacity,
			StartTime:    in.Body.StartTime,
			EndTime:      in.Body.EndTime,
			RoomID:       roomID,
		})
		if err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		return &groupOutput{Body: toGroupResponse(g)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "groups-delete",
		Method:      http.MethodDelete,
		Path:        "/groups/{id}",
		Summary:     "Delete a group (detaches its students first)",
		Tags:        []string{"groups"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *groupIDInput) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid group id")
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		return &noContentOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "groups-students",
		Method:      http.MethodGet,
		Path:        "/groups/{id}/students",
		Summary:     "List students in a group (highest risk first)",
		Tags:        []string{"groups"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *groupIDInput) (*groupStudentsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid group id")
		}
		students, err := svc.Students(ctx, p.OrgID, id)
		if err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		out := &groupStudentsOutput{Body: make([]studentResponse, 0, len(students))}
		for _, st := range students {
			out.Body = append(out.Body, toStudentResponse(st))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "students-assign-group",
		Method:      http.MethodPut,
		Path:        "/students/{id}/group",
		Summary:     "Assign a student to a group (empty groupId removes them)",
		Tags:        []string{"groups"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *assignGroupInput) (*noContentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		studentID, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		groupID, perr := parseOptionalUUID(in.Body.GroupID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid groupId")
		}
		if err := svc.AssignStudent(ctx, p.OrgID, studentID, groupID); err != nil {
			return nil, mapGroupError(LangFromContext(ctx), err, log)
		}
		return &noContentOutput{}, nil
	})
}

func toGroupResponse(g entity.Group) groupResponse {
	r := groupResponse{
		ID:           g.ID.String(),
		Name:         g.Name,
		Direction:    g.Direction,
		ScheduleDays: g.ScheduleDays,
		Capacity:     g.Capacity,
		StartTime:    g.StartTime,
		EndTime:      g.EndTime,
		CreatedAt:    g.CreatedAt.UTC().Format(time.RFC3339),
	}
	if g.RoomID != nil {
		r.RoomID = g.RoomID.String()
	}
	if g.TeacherID != nil {
		r.TeacherID = g.TeacherID.String()
	}
	if g.CourseID != nil {
		r.CourseID = g.CourseID.String()
	}
	if g.BranchID != nil {
		r.BranchID = g.BranchID.String()
	}
	return r
}

// parseOptionalUUID parses an optional UUID field: empty string → nil (no error).
func parseOptionalUUID(s string) (*uuid.UUID, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

var msgGroupNotFound = i18n.Message{UZ: "Guruh yoki talaba topilmadi.", RU: "Группа или студент не найдены.", EN: "Group or student not found."}

func mapGroupError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, groupusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgGroupNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("groups: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

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
	lessonusecase "github.com/student-success/backend/internal/usecase/lesson"
)

type lessonResponse struct {
	ID        string `json:"id"`
	GroupID   string `json:"groupId,omitempty"`
	TeacherID string `json:"teacherId,omitempty"`
	Date      string `json:"date"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
	Room      string `json:"room,omitempty"`
	RoomID    string `json:"roomId,omitempty"`
	Topic     string `json:"topic,omitempty"`
	Status    string `json:"status"`
}

type lessonBody struct {
	GroupID   string `json:"groupId,omitempty"`
	TeacherID string `json:"teacherId,omitempty"`
	Date      string `json:"date" format:"date" doc:"YYYY-MM-DD"`
	StartTime string `json:"startTime,omitempty" maxLength:"5" example:"14:00"`
	EndTime   string `json:"endTime,omitempty" maxLength:"5" example:"15:30"`
	Room      string `json:"room,omitempty" maxLength:"64"`
	RoomID    string `json:"roomId,omitempty" format:"uuid" doc:"Room; double-booking is rejected"`
	Topic     string `json:"topic,omitempty" maxLength:"255"`
	Status    string `json:"status,omitempty" enum:"scheduled,done,cancelled"`
}

type listLessonsInput struct {
	From string `query:"from" doc:"YYYY-MM-DD (default: today)"`
	To   string `query:"to" doc:"YYYY-MM-DD (default: +30 days)"`
}
type listLessonsOutput struct{ Body []lessonResponse }
type createLessonInput struct{ Body lessonBody }
type updateLessonInput struct {
	ID   string `path:"id" format:"uuid"`
	Body lessonBody
}
type lessonIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type lessonOutput struct{ Body lessonResponse }
type deleteLessonOutput struct{}

// registerLessons mounts the timetable. Mount on a group gated to center_admin.
func registerLessons(api huma.API, svc *lessonusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "lessons-list",
		Method:      http.MethodGet,
		Path:        "/lessons",
		Summary:     "List scheduled lessons in a date range",
		Tags:        []string{"lessons"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *listLessonsInput) (*listLessonsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		to := from.AddDate(0, 0, 30)
		if in.From != "" {
			t, perr := time.Parse("2006-01-02", in.From)
			if perr != nil {
				return nil, huma.Error422UnprocessableEntity("from must be YYYY-MM-DD")
			}
			from = t
		}
		if in.To != "" {
			t, perr := time.Parse("2006-01-02", in.To)
			if perr != nil {
				return nil, huma.Error422UnprocessableEntity("to must be YYYY-MM-DD")
			}
			to = t
		}
		ls, err := svc.List(ctx, p.OrgID, from, to)
		if err != nil {
			return nil, mapLessonError(LangFromContext(ctx), err, log)
		}
		out := &listLessonsOutput{Body: make([]lessonResponse, 0, len(ls))}
		for _, l := range ls {
			out.Body = append(out.Body, toLessonResponse(l))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "lessons-create",
		Method:        http.MethodPost,
		Path:          "/lessons",
		Summary:       "Schedule a lesson",
		Tags:          []string{"lessons"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createLessonInput) (*lessonOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		input, ierr := in.Body.toInput()
		if ierr != nil {
			return nil, ierr
		}
		l, err := svc.Create(ctx, p.OrgID, input)
		if err != nil {
			return nil, mapLessonError(LangFromContext(ctx), err, log)
		}
		return &lessonOutput{Body: toLessonResponse(l)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "lessons-update",
		Method:      http.MethodPut,
		Path:        "/lessons/{id}",
		Summary:     "Update a lesson",
		Tags:        []string{"lessons"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateLessonInput) (*lessonOutput, error) {
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
			return nil, mapLessonError(LangFromContext(ctx), err, log)
		}
		return &lessonOutput{Body: toLessonResponse(l)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "lessons-delete",
		Method:        http.MethodDelete,
		Path:          "/lessons/{id}",
		Summary:       "Delete a lesson",
		Tags:          []string{"lessons"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *lessonIDInput) (*deleteLessonOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapLessonError(LangFromContext(ctx), err, log)
		}
		return &deleteLessonOutput{}, nil
	})
}

func (b lessonBody) toInput() (lessonusecase.Input, error) {
	date, derr := time.Parse("2006-01-02", b.Date)
	if derr != nil {
		return lessonusecase.Input{}, huma.Error422UnprocessableEntity("date must be YYYY-MM-DD")
	}
	gid, gerr := parseOptUUID(b.GroupID)
	if gerr != nil {
		return lessonusecase.Input{}, huma.Error422UnprocessableEntity("invalid groupId")
	}
	tid, terr := parseOptUUID(b.TeacherID)
	if terr != nil {
		return lessonusecase.Input{}, huma.Error422UnprocessableEntity("invalid teacherId")
	}
	rid, rerr := parseOptUUID(b.RoomID)
	if rerr != nil {
		return lessonusecase.Input{}, huma.Error422UnprocessableEntity("invalid roomId")
	}
	return lessonusecase.Input{
		GroupID:   gid,
		TeacherID: tid,
		Date:      date,
		StartTime: b.StartTime,
		EndTime:   b.EndTime,
		Room:      b.Room,
		RoomID:    rid,
		Topic:     b.Topic,
		Status:    b.Status,
	}, nil
}

func toLessonResponse(l entity.Lesson) lessonResponse {
	r := lessonResponse{
		ID:        l.ID.String(),
		Date:      l.Date.Format("2006-01-02"),
		StartTime: l.StartTime,
		EndTime:   l.EndTime,
		Room:      l.Room,
		Topic:     l.Topic,
		Status:    l.Status,
	}
	if l.GroupID != nil {
		r.GroupID = l.GroupID.String()
	}
	if l.TeacherID != nil {
		r.TeacherID = l.TeacherID.String()
	}
	if l.RoomID != nil {
		r.RoomID = l.RoomID.String()
	}
	return r
}

var msgLessonNotFound = i18n.Message{UZ: "Dars topilmadi.", RU: "Занятие не найдено.", EN: "Lesson not found."}

func mapLessonError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, lessonusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, lessonusecase.ErrRoomBusy):
		return huma.Error409Conflict("Bu xona shu vaqtda band")
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgLessonNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("lessons: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}

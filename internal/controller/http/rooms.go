package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	roomusecase "github.com/student-success/backend/internal/usecase/room"
)

type roomResponse struct {
	ID       string `json:"id"`
	BranchID string `json:"branchId,omitempty"`
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
}
type roomBody struct {
	Name     string `json:"name" minLength:"1" maxLength:"255"`
	BranchID string `json:"branchId,omitempty" format:"uuid" doc:"Branch this room is at; omit for org-wide"`
	Capacity int    `json:"capacity,omitempty" minimum:"0"`
}
type createRoomInput struct{ Body roomBody }
type updateRoomInput struct {
	ID   string `path:"id" format:"uuid"`
	Body roomBody
}
type roomIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type roomsListOutput struct{ Body []roomResponse }
type roomOutput struct{ Body roomResponse }

func registerRooms(api huma.API, svc *roomusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "rooms-list",
		Method:      http.MethodGet,
		Path:        "/rooms",
		Summary:     "List rooms (xonalar)",
		Tags:        []string{"rooms"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *meEmptyInput) (*roomsListOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		list, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapRoomError(err, log)
		}
		out := &roomsListOutput{}
		out.Body = make([]roomResponse, 0, len(list))
		for _, r := range list {
			out.Body = append(out.Body, toRoomResponse(r))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "room-create",
		Method:        http.MethodPost,
		Path:          "/rooms",
		Summary:       "Create a room",
		Tags:          []string{"rooms"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createRoomInput) (*roomOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		branchID, berr := parseOptUUID(in.Body.BranchID)
		if berr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid branchId")
		}
		r, err := svc.Create(ctx, p.OrgID, roomusecase.Input{
			BranchID: branchID, Name: in.Body.Name, Capacity: in.Body.Capacity,
		})
		if err != nil {
			return nil, mapRoomError(err, log)
		}
		return &roomOutput{Body: toRoomResponse(r)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "room-update",
		Method:      http.MethodPut,
		Path:        "/rooms/{id}",
		Summary:     "Update a room",
		Tags:        []string{"rooms"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateRoomInput) (*roomOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		branchID, berr := parseOptUUID(in.Body.BranchID)
		if berr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid branchId")
		}
		r, err := svc.Update(ctx, p.OrgID, id, roomusecase.Input{
			BranchID: branchID, Name: in.Body.Name, Capacity: in.Body.Capacity,
		})
		if err != nil {
			return nil, mapRoomError(err, log)
		}
		return &roomOutput{Body: toRoomResponse(r)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "room-delete",
		Method:        http.MethodDelete,
		Path:          "/rooms/{id}",
		Summary:       "Delete a room",
		Tags:          []string{"rooms"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *roomIDInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapRoomError(err, log)
		}
		return &noContentOutput{}, nil
	})
}

func toRoomResponse(r entity.Room) roomResponse {
	resp := roomResponse{ID: r.ID.String(), Name: r.Name, Capacity: r.Capacity}
	if r.BranchID != nil {
		resp.BranchID = r.BranchID.String()
	}
	return resp
}

func mapRoomError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, roomusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("xona nomini kiriting")
	case errors.Is(err, roomusecase.ErrNotFound):
		return huma.Error404NotFound("xona topilmadi")
	default:
		log.Error().Err(err).Msg("room op failed")
		return huma.Error500InternalServerError("internal error")
	}
}
